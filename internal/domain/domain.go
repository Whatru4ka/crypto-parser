// Package domain — ядро приложения: типы и ошибки предметной области.
// Здесь НЕТ зависимостей ни от БД, ни от HTTP, ни от Telegram.
// Это важное правило слоистой архитектуры: зависимости смотрят внутрь.
package domain

import (
	"errors"
	"time"
)

// Сентинельные ошибки — внешние слои сравнивают их через errors.Is
// и решают, какой HTTP-статус вернуть.
var (
	ErrCurrencyNotFound = errors.New("currency not found")
	ErrNoData           = errors.New("no rate data yet")
)

// Rate — один снимок цены.
type Rate struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	FetchedAt time.Time `json:"fetched_at"`
}

// RateStats — агрегированная витрина, которую отдаём клиентам.
// HourChangePct — указатель, т.к. "данных за час ещё нет" и "изменение 0%" —
// это РАЗНЫЕ состояния. nil сериализуется в JSON как null.
type RateStats struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	DayMin        float64   `json:"day_min"`
	DayMax        float64   `json:"day_max"`
	HourChangePct *float64  `json:"hour_change_pct"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Subscription — подписка чата на автоотправку.
type Subscription struct {
	ChatID          int64
	IntervalMinutes int
	IsActive        bool
	LastSentAt      *time.Time
}
