package middleware

import (
	"time"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/gin-gonic/gin"
)

// MetricsMiddleware records per-request latency to InfluxDB for
// dashboarding p95/p99 under peak load.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		config.RecordRequestLatency(c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}
