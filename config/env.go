package config

import (
	"log"
	"os"
)

// RequireEnv validates required environment variables are set before the
// server starts accepting traffic. Failing fast here beats failing on the
// first real request (e.g. signing a JWT with an empty secret).
func RequireEnv() {
	required := []string{
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"JWT_SECRET",
	}

	var missing []string
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		log.Fatalf("Missing required environment variables: %v", missing)
	}
}
