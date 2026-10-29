package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
	"github.com/Whatru4ka/crypto-rates/internal/transport/httpapi"
)

// stubSvc — мок domain.RateService: хендлер тестируем без БД и Binance.
type stubSvc struct {
	stats domain.RateStats
	err   error
}

func (s stubSvc) List(context.Context) ([]domain.RateStats, error) {
	return []domain.RateStats{s.stats}, s.err
}
func (s stubSvc) Get(context.Context, string) (domain.RateStats, error) {
	return s.stats, s.err
}

func newRouter(svc domain.RateService) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpapi.NewHandler(svc, log).Router()
}

func TestGetRate(t *testing.T) {
	// Три ответа сервиса → три HTTP-статуса: 200 / 404 / 503.
	pct := 1.5
	ok := domain.RateStats{Symbol: "BTC", Price: 65000, DayMin: 64000,
		DayMax: 66000, HourChangePct: &pct, UpdatedAt: time.Now()}

	tests := []struct {
		name       string
		svc        stubSvc
		path       string
		wantStatus int
		wantBody   string
	}{
		{"ok", stubSvc{stats: ok}, "/rates/BTC", http.StatusOK, `"symbol":"BTC"`},
		{"not found", stubSvc{err: domain.ErrCurrencyNotFound}, "/rates/DOGE",
			http.StatusNotFound, "currency not found"},
		{"no data", stubSvc{err: domain.ErrNoData}, "/rates/BTC",
			http.StatusServiceUnavailable, "no data yet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			newRouter(tt.svc).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %s, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestListRates(t *testing.T) {
	// GET /rates оборачивает список в {"rates": [...]}.
	req := httptest.NewRequest(http.MethodGet, "/rates", nil)
	rec := httptest.NewRecorder()
	newRouter(stubSvc{stats: domain.RateStats{Symbol: "ETH"}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"rates"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
