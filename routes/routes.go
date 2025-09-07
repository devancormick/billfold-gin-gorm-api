package routes

import (
	"github.com/devancormick/billfold-gin-gorm-api/handlers"
	"github.com/devancormick/billfold-gin-gorm-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
		}

		users := v1.Group("/users")
		{
			users.POST("", handlers.CreateUser)
			users.GET("", handlers.ListUsers)
			users.GET("/:id", handlers.GetUser)
			users.PATCH("/:id", handlers.UpdateUser)
			users.DELETE("/:id", handlers.DeleteUser)
			users.POST("/:id/restore", handlers.RestoreUser)
		}

		posts := v1.Group("/posts")
		{
			posts.POST("", handlers.CreatePost)
			posts.GET("", handlers.ListPosts)
			posts.GET("/:id", handlers.GetPost)
			posts.POST("/transfer", handlers.TransferPost)
		}

		payments := v1.Group("/payments")
		payments.Use(middleware.RequireAuth())
		{
			payments.POST("/adjust", handlers.AdjustBalance)
			payments.GET("/wallets/:user_id", handlers.GetWallet)
		}
	}
}
