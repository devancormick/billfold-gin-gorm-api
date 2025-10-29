package handlers

import (
	"net/http"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TransferPostInput defines the request for transferring post ownership
type TransferPostInput struct {
	PostID    uint `json:"post_id" binding:"required"`
	NewUserID uint `json:"new_user_id" binding:"required"`
}

// TransferPost godoc
// @Summary      Transfer post ownership
// @Description  Atomically reassigns a post to a new owner inside a DB transaction.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        input body TransferPostInput true "Transfer request"
// @Success      200 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /posts/transfer [post]
func TransferPost(c *gin.Context) {
	var input TransferPostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var post models.Post
		if err := tx.First(&post, input.PostID).Error; err != nil {
			return err
		}

		var newOwner models.User
		if err := tx.First(&newOwner, input.NewUserID).Error; err != nil {
			return err
		}

		if err := tx.Model(&post).Update("user_id", input.NewUserID).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Transfer failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Post transferred successfully",
	})
}
