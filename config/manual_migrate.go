package config

import (
	"log"
	"strings"
)

// RunManualMigrations executes raw SQL for precise schema control not
// covered by AutoMigrate (indexes, check constraints).
func RunManualMigrations() {
	// MariaDB has no partial/filtered index syntax, so this indexes the full
	// table instead of just non-deleted rows — still covers the query pattern
	// (published posts ordered by recency), just with a slightly larger index.
	if err := DB.Exec(`
		CREATE INDEX idx_posts_published_created
		ON posts (published, created_at DESC, deleted_at)
	`).Error; err != nil && !strings.Contains(err.Error(), "Duplicate key name") {
		log.Println("Warning: Could not create index:", err)
	}

	// MariaDB 10.2+ supports CHECK constraints directly.
	if err := DB.Exec(`
		ALTER TABLE users
		ADD CONSTRAINT chk_email_format
		CHECK (email REGEXP '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}$')
	`).Error; err != nil && !strings.Contains(err.Error(), "Duplicate check constraint") {
		log.Println("Warning: Could not create email constraint:", err)
	}
}
