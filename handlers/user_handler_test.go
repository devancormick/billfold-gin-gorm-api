package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devancormick/billfold-gin-gorm-api/config"
	"github.com/devancormick/billfold-gin-gorm-api/handlers"
	"github.com/devancormick/billfold-gin-gorm-api/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestDB() {
	config.ConnectSQLite()
	config.DB.AutoMigrate(&models.User{}, &models.Post{}, &models.Tag{})
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	return router
}

func TestCreateUser(t *testing.T) {
	setupTestDB()
	router := setupRouter()
	router.POST("/users", handlers.CreateUser)

	t.Run("valid user", func(t *testing.T) {
		payload := map[string]string{
			"username": "testuser",
			"email":    "test@example.com",
			"password": "securepassword123",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "User created successfully", response["message"])
	})

	t.Run("duplicate username", func(t *testing.T) {
		payload := map[string]string{
			"username": "testuser",
			"email":    "test2@example.com",
			"password": "securepassword123",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("invalid email", func(t *testing.T) {
		payload := map[string]string{
			"username": "newuser",
			"email":    "invalid-email",
			"password": "securepassword123",
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	config.DB.Exec("DELETE FROM users")
}
