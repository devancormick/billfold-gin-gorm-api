package handlers

import (
	"net/http"
	"strconv"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// CreateUserInput defines the expected request body
type CreateUserInput struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Bio      string `json:"bio" binding:"max=500"`
}

// CreateUser godoc
// @Summary      Create a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        input body CreateUserInput true "New user"
// @Success      201 {object} map[string]interface{}
// @Failure      409 {object} map[string]string
// @Router       /users [post]
func CreateUser(c *gin.Context) {
	var input CreateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(input.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process password",
		})
		return
	}

	user := models.User{
		Username: input.Username,
		Email:    input.Email,
		Password: string(hashedPassword),
	}

	if input.Bio != "" {
		user.Bio = &input.Bio
	}

	result := config.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Username or email already exists",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user,
	})
}

// GetUser godoc
// @Summary      Get a user by ID
// @Tags         users
// @Produce      json
// @Param        id path int true "User ID"
// @Success      200 {object} models.User
// @Failure      404 {object} map[string]string
// @Router       /users/{id} [get]
func GetUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User

	result := config.DB.First(&user, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ListUsers godoc
// @Summary      List users
// @Tags         users
// @Produce      json
// @Param        page query int false "Page number"
// @Param        limit query int false "Page size (max 100)"
// @Param        active query bool false "Filter by active status"
// @Param        search query string false "Search by username"
// @Success      200 {object} map[string]interface{}
// @Router       /users [get]
func ListUsers(c *gin.Context) {
	var users []models.User

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")

	pageNum, err := strconv.Atoi(page)
	if err != nil || pageNum < 1 {
		pageNum = 1
	}

	limitNum, err := strconv.Atoi(limit)
	if err != nil || limitNum < 1 || limitNum > 100 {
		limitNum = 10
	}

	offset := (pageNum - 1) * limitNum

	query := config.DB.Model(&models.User{})

	if active := c.Query("active"); active != "" {
		query = query.Where("is_active = ?", active == "true")
	}

	if search := c.Query("search"); search != "" {
		// MariaDB's LIKE is case-insensitive under the default utf8mb4_unicode_ci
		// collation, so this matches the intended case-insensitive search without
		// relying on ILIKE (Postgres-only syntax MariaDB doesn't support).
		query = query.Where("username LIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	result := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limitNum).
		Find(&users)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch users",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"page":        pageNum,
			"limit":       limitNum,
			"total":       total,
			"total_pages": (total + int64(limitNum) - 1) / int64(limitNum),
		},
	})
}

// UpdateUserInput defines fields that can be updated
type UpdateUserInput struct {
	Username *string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    *string `json:"email" binding:"omitempty,email"`
	Bio      *string `json:"bio" binding:"omitempty,max=500"`
	IsActive *bool   `json:"is_active"`
}

// UpdateUser godoc
// @Summary      Partially update a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id path int true "User ID"
// @Param        input body UpdateUserInput true "Fields to update"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]string
// @Router       /users/{id} [patch]
func UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if result := config.DB.First(&user, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	var input UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	updates := make(map[string]interface{})

	if input.Username != nil {
		updates["username"] = *input.Username
	}
	if input.Email != nil {
		updates["email"] = *input.Email
	}
	if input.Bio != nil {
		updates["bio"] = *input.Bio
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	result := config.DB.Model(&user).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Update failed - username or email may already exist",
		})
		return
	}

	config.DB.First(&user, id)

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    user,
	})
}

// DeleteUser godoc
// @Summary      Delete a user
// @Tags         users
// @Produce      json
// @Param        id path int true "User ID"
// @Param        hard query bool false "Permanently delete instead of soft delete"
// @Success      200 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if result := config.DB.First(&user, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	hardDelete := c.Query("hard") == "true"

	if hardDelete {
		result := config.DB.Unscoped().Delete(&user)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete user",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "User permanently deleted",
		})
	} else {
		result := config.DB.Delete(&user)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete user",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "User deleted successfully",
		})
	}
}

// RestoreUser godoc
// @Summary      Restore a soft-deleted user
// @Tags         users
// @Produce      json
// @Param        id path int true "User ID"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} map[string]string
// @Router       /users/{id}/restore [post]
func RestoreUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	result := config.DB.Unscoped().First(&user, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	if user.DeletedAt.Time.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User is not deleted",
		})
		return
	}

	config.DB.Unscoped().Model(&user).Update("deleted_at", nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "User restored successfully",
		"user":    user,
	})
}
