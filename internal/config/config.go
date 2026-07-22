package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	FranchiseServiceURL string
	LogLevel            string
}

// Load reads typed configuration from environment variables only.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                getEnv("PORT", "8085"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		FranchiseServiceURL: getEnv("FRANCHISE_SERVICE_URL", "http://franchise-service:8089"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
