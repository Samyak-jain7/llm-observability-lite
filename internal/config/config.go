package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	LogLevel           string
	DatabaseURL        string
	RedisURL           string
	JWTSecret          string
	StripeWebhookSecret string
	StripeSecretKey    string
	Environment        string // "development" or "production"
}

func Load() (*Config, error) {
	godotenvErr := godotenv.Load()
	_ = godotenvErr // ok if missing; env vars must be set externally

	return &Config{
		Port:               getEnv("PORT", "8080"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/observability?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeSecretKey:    getEnv("STRIPE_SECRET_KEY", ""),
		Environment:        getEnv("ENVIRONMENT", "development"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
