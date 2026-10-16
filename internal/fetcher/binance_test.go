package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Whatru4ka/crypto-rates/internal/fetcher"
)

// Тест на внешний адаптер без настоящей сети: поднимаем httptest-сервер.
func TestBinanceProvider_FetchRates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","price":"65000.50"},
		                        {"symbol":"ETHUSDT","price":"3200.10"}]`))
	}))
	defer srv.Close()

	p := fetcher.NewBinanceProvider(srv.URL)
	rates, err := p.FetchRates(context.Background(), []string{"BTC", "ETH"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("got %d rates, want 2", len(rates))
	}
	// Проверяем, что "BTCUSDT" корректно смаплен обратно в "BTC"
	if rates[0].Symbol != "BTC" || rates[0].Price != 65000.50 {
		t.Fatalf("unexpected first rate: %+v", rates[0])
	}
}

func TestBinanceProvider_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := fetcher.NewBinanceProvider(srv.URL)
	if _, err := p.FetchRates(context.Background(), []string{"BTC"}); err == nil {
		t.Fatal("expected error on non-200 status")
	}
}
