package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents the users table in the database
type User struct {
	gorm.Model

	Username string `gorm:"uniqueIndex;not null;size:50" json:"username"`
	Email    string `gorm:"uniqueIndex;not null;size:100" json:"email"`
	Password string `gorm:"not null" json:"-"`

	Bio *string `gorm:"size:500" json:"bio,omitempty"`

	IsActive bool `gorm:"default:true" json:"is_active"`

	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// TableName overrides the default table name
func (User) TableName() string {
	return "users"
}
