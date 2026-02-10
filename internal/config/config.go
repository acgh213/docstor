package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                   string
	Port                  string
	DatabaseURL           string
	SessionKey            string
	CSRFKey               string
	AttachmentStoragePath string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:                   getEnv("APP_ENV", "development"),
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           getEnv("DATABASE_URL", ""),
		SessionKey:            getEnv("SESSION_KEY", ""),
		CSRFKey:               getEnv("CSRF_KEY", ""),
		AttachmentStoragePath: getEnv("ATTACHMENT_STORAGE_PATH", "/tmp/docstor-attachments"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.SessionKey == "" {
		return nil, fmt.Errorf("SESSION_KEY is required")
	}

	if cfg.CSRFKey == "" {
		return nil, fmt.Errorf("CSRF_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}
