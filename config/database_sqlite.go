package config

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ConnectSQLite sets up a SQLite database for local development
func ConnectSQLite() {
	db, err := gorm.Open(sqlite.Open("app.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to SQLite database:", err)
	}

	DB = db
	log.Println("SQLite database connected")
}
