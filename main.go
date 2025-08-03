// Package main - billfold-gin-gorm-api
//
// @title Billfold Payments API
// @version 1.0
// @description Payments and credits service: Go/Gin backend on MariaDB (GORM) with InfluxDB metrics and Sentry error tracking.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT from /auth/login.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	_ "github.com/devancormick/billfold-gin-gorm-api/docs"
	"github.com/devancormick/billfold-gin-gorm-api/middleware"
	"github.com/devancormick/billfold-gin-gorm-api/routes"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"
)

func main() {
	config.RequireEnv()

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	config.ConnectDatabase()
	config.RunMigrations()
	config.ConnectInflux()
	defer config.FlushInflux(context.Background())
	config.ConnectSentry()
	defer config.FlushSentry()

	router := gin.Default()

	router.Use(corsMiddleware())
	router.Use(middleware.SentryMiddleware())
	router.Use(middleware.MetricsMiddleware())
	router.Use(middleware.TimeoutMiddleware(15 * time.Second))

	routes.SetupRoutes(router)

	router.GET("/swagger/*any", ginswagger.WrapHandler(swaggerfiles.Handler))

	router.StaticFile("/", "./static/dashboard.html")
	router.GET("/api", apiIndex)
	router.GET("/api/", apiIndex)
	router.GET("/api/v1", apiIndex)
	router.GET("/api/v1/", apiIndex)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.GET("/ready", func(c *gin.Context) {
		sqlDB, err := config.DB.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for SIGINT/SIGTERM (systemd, Docker stop, deploy rollout) and drain
	// in-flight requests before exiting — a payment adjustment mid-request
	// must be allowed to finish, not be cut off.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited cleanly")
}

// apiIndex answers the root and unversioned API paths with a directory of
// available endpoints, instead of a bare 404, for anyone landing there directly.
func apiIndex(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":     "Billfold Payments API",
		"version":  "1.0",
		"swagger":  "/swagger/index.html",
		"health":   "/health",
		"ready":    "/ready",
		"api_base": "/api/v1",
		"endpoints": gin.H{
			"auth": []string{
				"POST /api/v1/auth/register",
				"POST /api/v1/auth/login",
			},
			"payments": []string{
				"POST /api/v1/payments/adjust (auth required)",
				"GET /api/v1/payments/wallets/:user_id (auth required)",
			},
			"users": []string{
				"POST /api/v1/users",
				"GET /api/v1/users",
				"GET /api/v1/users/:id",
				"PATCH /api/v1/users/:id",
				"DELETE /api/v1/users/:id",
				"POST /api/v1/users/:id/restore",
			},
			"posts": []string{
				"POST /api/v1/posts",
				"GET /api/v1/posts",
				"GET /api/v1/posts/:id",
				"POST /api/v1/posts/transfer",
			},
		},
	})
}

// corsMiddleware restricts cross-origin requests to the configured frontend
// origin(s). ALLOWED_ORIGINS is a comma-separated list; falls back to
// billfold.ddns.net if unset.
func corsMiddleware() gin.HandlerFunc {
	allowed := os.Getenv("ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "https://billfold.ddns.net"
	}
	origins := strings.Split(allowed, ",")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		for _, o := range origins {
			if strings.TrimSpace(o) == origin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
