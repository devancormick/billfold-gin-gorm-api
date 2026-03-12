package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// TimeoutMiddleware bounds request context lifetime so a stalled DB call or
// downstream dependency can't hold a connection (and a peak-load worker slot)
// open indefinitely.
func TimeoutMiddleware(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

