package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
	"github.com/Whatru4ka/crypto-rates/internal/service"
)

// mockRepo — ручной мок порта domain.RateRepository.
// Именно ради такой подменяемости интерфейс объявлен в domain, а не в repository.
type mockRepo struct {
	gotSymbol string
	stats     domain.RateStats
	err       error
}

func (m *mockRepo) ActiveSymbols(context.Context) ([]string, error) { return nil, nil }
func (m *mockRepo) SaveRates(context.Context, []domain.Rate) error  { return nil }
func (m *mockRepo) GetAllStats(context.Context) ([]domain.RateStats, error) {
	return []domain.RateStats{m.stats}, m.err
}
func (m *mockRepo) GetStats(_ context.Context, symbol string) (domain.RateStats, error) {
	m.gotSymbol = symbol
	return m.stats, m.err
}

func TestRateService_Get(t *testing.T) {
	// Табличный тест — стандартный для Go стиль.
	tests := []struct {
		name       string
		input      string
		repoErr    error
		wantSymbol string
		wantErr    error
	}{
		{name: "uppercase passthrough", input: "BTC", wantSymbol: "BTC"},
		{name: "lowercase normalized", input: "btc", wantSymbol: "BTC"},
		{name: "spaces trimmed", input: "  eth ", wantSymbol: "ETH"},
		{name: "empty input", input: "   ", wantErr: domain.ErrCurrencyNotFound},
		{name: "repo error propagated", input: "DOGE",
			repoErr: domain.ErrCurrencyNotFound, wantSymbol: "DOGE",
			wantErr: domain.ErrCurrencyNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepo{err: tt.repoErr}
			svc := service.NewRateService(repo)

			_, err := svc.Get(context.Background(), tt.input)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantSymbol != "" && repo.gotSymbol != tt.wantSymbol {
				t.Fatalf("repo got %q, want %q", repo.gotSymbol, tt.wantSymbol)
			}
		})
	}
}
