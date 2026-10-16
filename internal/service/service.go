// Package service — бизнес-логика. Единая точка правды для ОБОИХ клиентов
// (REST и Telegram): дублировать логику в двух транспортах нельзя.
package service

import (
	"context"
	"strings"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type RateService struct {
	repo domain.RateRepository
}

func NewRateService(repo domain.RateRepository) *RateService {
	return &RateService{repo: repo}
}

// Normalize — нормализация пользовательского ввода.
// "btc", " Btc " и "BTC" должны означать одно и то же.
func Normalize(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func (s *RateService) List(ctx context.Context) ([]domain.RateStats, error) {
	return s.repo.GetAllStats(ctx)
}

func (s *RateService) Get(ctx context.Context, symbol string) (domain.RateStats, error) {
	sym := Normalize(symbol)
	if sym == "" {
		return domain.RateStats{}, domain.ErrCurrencyNotFound
	}
	return s.repo.GetStats(ctx, sym)
}
