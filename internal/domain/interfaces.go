package domain

import (
	"context"
	"time"
)

// Интерфейсы — контракт между слоями. Структуры (*RateRepo, *RateService, *BinanceProvider)
// зависят от них, а не друг от друга: в тестах вместо БД и Binance подставляется мок.

// RateRepository — хранилище курсов. Реализация: repository.RateRepo.
type RateRepository interface {
	ActiveSymbols(ctx context.Context) ([]string, error)
	SaveRates(ctx context.Context, rates []Rate) error
	GetStats(ctx context.Context, symbol string) (RateStats, error)
	GetAllStats(ctx context.Context) ([]RateStats, error)
}

// SubscriptionRepository — хранилище подписок Telegram. Реализация: repository.SubscriptionRepo.
type SubscriptionRepository interface {
	Upsert(ctx context.Context, chatID int64, minutes int) error
	Deactivate(ctx context.Context, chatID int64) error
	DueSubscriptions(ctx context.Context, now time.Time) ([]Subscription, error)
	MarkSent(ctx context.Context, chatID int64, at time.Time) error
}

// RateProvider — внешний источник котировок. Реализация: fetcher.BinanceProvider.
type RateProvider interface {
	FetchRates(ctx context.Context, symbols []string) ([]Rate, error)
}

// RateService — прикладной сервис для REST и бота.
// Транспорты зависят от этого интерфейса, а не от *service.RateService.
type RateService interface {
	List(ctx context.Context) ([]RateStats, error)
	Get(ctx context.Context, symbol string) (RateStats, error)
}
