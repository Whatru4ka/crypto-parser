package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	// Без DSN сервис не стартует: пустые значения остальных переменных
	// имитируют «чистое» окружение, как у нового docker compose.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("TELEGRAM_TOKEN", "")
	t.Setenv("FETCH_INTERVAL", "")
	t.Setenv("PROVIDER_BASE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is empty")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want mention of DATABASE_URL", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Задан только обязательный DATABASE_URL — остальное берётся из дефолтов ТЗ.
	t.Setenv("DATABASE_URL", "postgres://rates:rates@localhost:5432/rates?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("TELEGRAM_TOKEN", "")
	t.Setenv("FETCH_INTERVAL", "")
	t.Setenv("PROVIDER_BASE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.TelegramToken != "" {
		t.Errorf("TelegramToken = %q, want empty", cfg.TelegramToken)
	}
	if cfg.FetchInterval != 5*time.Minute {
		t.Errorf("FetchInterval = %v, want 5m", cfg.FetchInterval)
	}
	if cfg.ProviderBaseURL != "https://api.binance.com" {
		t.Errorf("ProviderBaseURL = %q", cfg.ProviderBaseURL)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	// Каждая переменная из .env должна перекрыть значение по умолчанию.
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/rates")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("TELEGRAM_TOKEN", "token-123")
	t.Setenv("FETCH_INTERVAL", "2m")
	t.Setenv("PROVIDER_BASE_URL", "http://localhost:9999")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.TelegramToken != "token-123" {
		t.Errorf("TelegramToken = %q", cfg.TelegramToken)
	}
	if cfg.FetchInterval != 2*time.Minute {
		t.Errorf("FetchInterval = %v, want 2m", cfg.FetchInterval)
	}
	if cfg.ProviderBaseURL != "http://localhost:9999" {
		t.Errorf("ProviderBaseURL = %q", cfg.ProviderBaseURL)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 3s", cfg.ShutdownTimeout)
	}
}

func TestGetDuration(t *testing.T) {
	// FETCH_INTERVAL принимают и как "90s", и как голое число минут ("10").
	tests := []struct {
		name string
		val  string
		def  time.Duration
		want time.Duration
	}{
		{"пусто — дефолт", "", 5 * time.Minute, 5 * time.Minute},
		{"Go duration", "90s", time.Minute, 90 * time.Second},
		{"голое число минут", "10", time.Minute, 10 * time.Minute},
		{"мусор — дефолт", "abc", 7 * time.Second, 7 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_DURATION", tt.val)
			got := getDuration("TEST_DURATION", tt.def)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Пустая строка в env равносильна «переменная не задана».
	t.Setenv("TEST_ENV", "")
	if got := getEnv("TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("empty env: got %q, want fallback", got)
	}

	t.Setenv("TEST_ENV", "set")
	if got := getEnv("TEST_ENV", "fallback"); got != "set" {
		t.Fatalf("set env: got %q, want set", got)
	}
}
