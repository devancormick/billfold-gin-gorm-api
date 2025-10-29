package middleware

import (
	"net/http"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TransactionMiddleware wraps the request in a database transaction
func TransactionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := config.DB.Begin()
		if tx.Error != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to start transaction",
			})
			return
		}

		c.Set("tx", tx)

		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				panic(r)
			}
		}()

		c.Next()

		if c.Writer.Status() >= 400 {
			tx.Rollback()
		} else {
			if err := tx.Commit().Error; err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to commit transaction",
				})
			}
		}
	}
}

// GetTx retrieves the transaction from context
func GetTx(c *gin.Context) *gorm.DB {
	if tx, exists := c.Get("tx"); exists {
		return tx.(*gorm.DB)
	}
	return config.DB
}
