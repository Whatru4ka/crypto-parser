package fetcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

// Worker — фоновое обновление курсов.
// Классический паттерн: time.Ticker + select по ctx.Done() для graceful shutdown.
type Worker struct {
	provider domain.RateProvider
	repo     domain.RateRepository
	interval time.Duration
	log      *slog.Logger
}

func NewWorker(p domain.RateProvider, r domain.RateRepository,
	interval time.Duration, log *slog.Logger) *Worker {
	return &Worker{provider: p, repo: r, interval: interval, log: log}
}

// Run блокируется до отмены контекста.
func (w *Worker) Run(ctx context.Context) {
	// Требование ТЗ: обновить сразу при запуске, а потом по расписанию.
	w.tick(ctx)

	t := time.NewTicker(w.interval)
	defer t.Stop() // без Stop тикер продолжит держать ресурсы

	for {
		select {
		case <-ctx.Done():
			w.log.Info("rate worker stopped")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// tick — один цикл обновления. Ошибку логируем, но НЕ роняем воркер:
// внешний API может быть временно недоступен, следующий тик попробует снова.
func (w *Worker) tick(ctx context.Context) {
	// Отдельный таймаут на итерацию, чтобы зависший запрос
	// не съел весь интервал до следующего тика.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	symbols, err := w.repo.ActiveSymbols(ctx)
	if err != nil {
		w.log.Error("load active symbols", "err", err)
		return
	}
	rates, err := w.provider.FetchRates(ctx, symbols)
	if err != nil {
		w.log.Error("fetch rates", "err", err)
		return
	}
	if err := w.repo.SaveRates(ctx, rates); err != nil {
		w.log.Error("save rates", "err", err)
		return
	}
	w.log.Info("rates updated", "count", len(rates))
}
