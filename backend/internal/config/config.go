// Package config загружает конфигурацию из переменных окружения
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config — конфигурация сервиса
type Config struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	MinIO    MinIOConfig
	App      AppConfig
	JWT      JWTConfig
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type AppConfig struct {
	BaseURL string
}

// JWTConfig — параметры JWT (UC-01)
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	PartialTTL time.Duration
}

// Load читает конфигурацию из переменных окружения.
func Load() (*Config, error) {
	dsn, err := buildDSN()
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTP: HTTPConfig{
			Addr:         getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:  parseDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: parseDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
		},
		Postgres: PostgresConfig{
			DSN: dsn,
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       parseInt("REDIS_DB", 0),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("MINIO_BUCKET", "borrowtime"),
			UseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		},
		App: AppConfig{
			BaseURL: getEnv("BASE_URL", "http://localhost:8080"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production-secret-32ch"),
			AccessTTL:  parseDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: parseDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			PartialTTL: parseDuration("JWT_PARTIAL_TTL", 5*time.Minute),
		},
	}, nil
}

func buildDSN() (string, error) {
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn, nil
	}

	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "borrowtime")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := getEnv("POSTGRES_DB", "borrowtime")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")

	if pass == "" {
		return "", fmt.Errorf("POSTGRES_PASSWORD or POSTGRES_DSN is required")
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, pass, host, port, db, sslmode), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return time.Duration(secs) * time.Second
}

func parseInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
