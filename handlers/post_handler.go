package handlers

import (
	"net/http"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
)

// CreatePostInput defines the request body for creating posts
type CreatePostInput struct {
	Title   string   `json:"title" binding:"required,max=200"`
	Content string   `json:"content" binding:"required"`
	UserID  uint     `json:"user_id" binding:"required"`
	Tags    []string `json:"tags"`
}

// CreatePost godoc
// @Summary      Create a post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        input body CreatePostInput true "New post"
// @Success      201 {object} map[string]interface{}
// @Failure      400 {object} map[string]string
// @Router       /posts [post]
func CreatePost(c *gin.Context) {
	var input CreatePostInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	var user models.User
	if result := config.DB.First(&user, input.UserID); result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User not found",
		})
		return
	}

	var tags []models.Tag
	for _, tagName := range input.Tags {
		var tag models.Tag
		config.DB.FirstOrCreate(&tag, models.Tag{Name: tagName})
		tags = append(tags, tag)
	}

	post := models.Post{
		Title:   input.Title,
		Content: input.Content,
		UserID:  input.UserID,
		Tags:    tags,
	}

	result := config.DB.Create(&post)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create post",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Post created successfully",
		"post":    post,
	})
}

// GetPost godoc
// @Summary      Get a post by ID
// @Tags         posts
// @Produce      json
// @Param        id path int true "Post ID"
// @Success      200 {object} models.Post
// @Failure      404 {object} map[string]string
// @Router       /posts/{id} [get]
func GetPost(c *gin.Context) {
	id := c.Param("id")

	var post models.Post

	result := config.DB.
		Preload("User").
		Preload("Tags").
		Preload("Replies").
		Preload("Replies.User").
		First(&post, id)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Post not found",
		})
		return
	}

	c.JSON(http.StatusOK, post)
}

// ListPosts godoc
// @Summary      List posts
// @Tags         posts
// @Produce      json
// @Param        published query bool false "Filter by published status"
// @Param        author_id query int false "Filter by author"
// @Param        tag query string false "Filter by tag name"
// @Success      200 {array} models.Post
// @Router       /posts [get]
func ListPosts(c *gin.Context) {
	var posts []models.Post

	query := config.DB.Model(&models.Post{})

	if published := c.Query("published"); published != "" {
		query = query.Where("published = ?", published == "true")
	}

	if authorID := c.Query("author_id"); authorID != "" {
		query = query.Where("user_id = ?", authorID)
	}

	if tagName := c.Query("tag"); tagName != "" {
		query = query.
			Joins("JOIN post_tags ON post_tags.post_id = posts.id").
			Joins("JOIN tags ON tags.id = post_tags.tag_id").
			Where("tags.name = ?", tagName)
	}

	result := query.
		Preload("User").
		Preload("Tags").
		Order("created_at DESC").
		Find(&posts)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch posts",
		})
		return
	}

	c.JSON(http.StatusOK, posts)
}
