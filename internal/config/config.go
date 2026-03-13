// Тут получение параметров для запуска сервисов из переменных окружения
package config

import (
	"os"
)

// Config - стуктура под настройки
type Config struct {
	DatabaseURL string
	BaseURL     string
	Port        string
}

// Load - получение настроек из переменных
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
