package config

import (
	"fmt"
	"os"
)

type DBConfig struct {
	DSN string // строка подключения к Postgres
}

type Config struct {
	HTTPAddr string // адрес сервера
	DB       DBConfig
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load загружает конфиг из переменных окружения.
// HTTP_PORT порт HTTP (по умолчанию 8080)
// DB_DSN строка подключения к Postgres
func Load() *Config {
	port := getenv("HTTP_PORT", "8080")

	dsn := getenv("DB_DSN", "postgres://postgres:postgres@db:5432/postgres?sslmode=disable")

	return &Config{
		HTTPAddr: fmt.Sprintf(":%s", port),
		DB: DBConfig{
			DSN: dsn,
		},
	}
}
