package config

import (
	"log"

	"github.com/devancormick/billfold-gin-gorm-api/models"
)

// RunMigrations creates or updates database tables based on the model definitions
func RunMigrations() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Post{},
		&models.Tag{},
		&models.Wallet{},
		&models.Transaction{},
	)
	if err != nil {
		log.Fatal("Migration failed:", err)
	}

	log.Println("Database migration completed")
}
