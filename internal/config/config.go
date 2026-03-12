package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	BaseURL     string
	Port        string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/shortener?sslmode=disable"),
		BaseURL:     getEnv("BASE_URL", "https://short.io"),
		Port:        getEnv("PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
