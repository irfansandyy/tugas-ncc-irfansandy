package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppPort                string
	DatabaseURL            string
	JWTSecret              string
	TokenTTL               time.Duration
	AllowedOrigin          string
	LLMBaseURL             string
	LLMModel               string
	LLMCtxSize             int
	LLMTimeout             time.Duration
	DBMaxOpenConns         int
	DBMaxIdleConns         int
	DBConnMaxLifetime      time.Duration
	RateLimitRequestsPerS  float64
	RateLimitBurstRequests int
}

func Load() Config {
	return Config{
		AppPort:                getEnv("APP_PORT", "8000"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/chatdb?sslmode=disable"),
		JWTSecret:              getEnv("JWT_SECRET", "change-me-in-production"),
		TokenTTL:               time.Duration(getEnvInt("JWT_TTL_MINUTES", 60*24)) * time.Minute,
		AllowedOrigin:          getEnv("ALLOWED_ORIGIN", "http://localhost:3000"),
		LLMBaseURL:             getEnv("LLM_BASE_URL", "http://localhost:8081"),
		LLMModel:               getEnv("LLM_MODEL", "hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K"),
		LLMCtxSize:             getEnvInt("LLM_CTX_SIZE", 4096),
		LLMTimeout:             time.Duration(getEnvInt("LLM_TIMEOUT_SECONDS", 60)) * time.Second,
		DBMaxOpenConns:         getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:         getEnvInt("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetime:      time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 5)) * time.Minute,
		RateLimitRequestsPerS:  getEnvFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurstRequests: getEnvInt("RATE_LIMIT_BURST", 10),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
