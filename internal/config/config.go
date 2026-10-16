// Package config — загрузка конфигурации из переменных окружения.
// Требование Docker "каждый может указать настройки" реализуется именно так:
// 12-factor app, никаких хардкод-значений в коде.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string        // адрес REST API, напр. ":8080"
	DatabaseURL     string        // postgres://user:pass@host:5432/db?sslmode=disable
	TelegramToken   string        // пустой => бот не запускается
	FetchInterval   time.Duration // 5m по ТЗ
	ProviderBaseURL string        // базовый URL внешнего API
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		TelegramToken:   getEnv("TELEGRAM_TOKEN", ""),
		FetchInterval:   getDuration("FETCH_INTERVAL", 5*time.Minute),
		ProviderBaseURL: getEnv("PROVIDER_BASE_URL", "https://api.binance.com"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	// Поддерживаем и "5m", и голое число минут ("5")
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Minute
	}
	return def
}
