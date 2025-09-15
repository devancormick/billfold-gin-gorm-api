package models

import (
	"gorm.io/gorm"
)

// Post represents a blog post with author relationship
type Post struct {
	gorm.Model

	Title     string `gorm:"not null;size:200" json:"title"`
	Content   string `gorm:"type:text" json:"content"`
	Published bool   `gorm:"default:false" json:"published"`

	UserID uint `gorm:"not null" json:"user_id"`
	User   User `gorm:"foreignKey:UserID" json:"author,omitempty"`

	ParentID *uint  `json:"parent_id,omitempty"`
	Parent   *Post  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies  []Post `gorm:"foreignKey:ParentID" json:"replies,omitempty"`

	Tags []Tag `gorm:"many2many:post_tags;" json:"tags,omitempty"`
}

// Tag represents a categorization label for posts
type Tag struct {
	gorm.Model

	Name  string `gorm:"uniqueIndex;not null;size:50" json:"name"`
	Posts []Post `gorm:"many2many:post_tags;" json:"posts,omitempty"`
}
