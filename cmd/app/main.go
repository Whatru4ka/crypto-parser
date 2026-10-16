// Composition root: единственное место, где конкретные реализации
// связываются друг с другом. Все остальные пакеты знают только интерфейсы.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Whatru4ka/crypto-rates/internal/bot"
	"github.com/Whatru4ka/crypto-rates/internal/config"
	"github.com/Whatru4ka/crypto-rates/internal/fetcher"
	"github.com/Whatru4ka/crypto-rates/internal/repository"
	"github.com/Whatru4ka/crypto-rates/internal/service"
	"github.com/Whatru4ka/crypto-rates/internal/transport/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Контекст отменяется по SIGINT/SIGTERM — это и есть сигнал
	// всем фоновым горутинам начать graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Ждём готовности БД: в docker-compose Postgres может подняться позже нас.
	if err := waitForDB(ctx, pool, log); err != nil {
		return err
	}

	rateRepo := repository.NewRateRepo(pool)
	subRepo := repository.NewSubscriptionRepo(pool)
	rateSvc := service.NewRateService(rateRepo)
	provider := fetcher.NewBinanceProvider(cfg.ProviderBaseURL)

	var wg sync.WaitGroup

	// 1) Фоновое обновление курсов
	worker := fetcher.NewWorker(provider, rateRepo, cfg.FetchInterval, log)
	wg.Add(1)
	go func() { defer wg.Done(); worker.Run(ctx) }()

	// 2) Telegram-бот + шедулер автоотправки (только если задан токен)
	if cfg.TelegramToken != "" {
		tg, err := bot.New(cfg.TelegramToken, rateSvc, subRepo, log)
		if err != nil {
			return err
		}
		wg.Add(2)
		go func() { defer wg.Done(); tg.Run(ctx) }()
		sched := bot.NewScheduler(subRepo, rateSvc, tg, log)
		go func() { defer wg.Done(); sched.Run(ctx) }()
	} else {
		log.Warn("TELEGRAM_TOKEN is empty, bot disabled")
	}

	// 3) REST API
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewHandler(rateSvc, log).Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("http server started", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop() // роняем всё приложение, если HTTP не поднялся
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	// Даём серверу дошить текущие запросы.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	wg.Wait()
	log.Info("stopped")
	return nil
}

func waitForDB(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	const attempts = 10
	var err error
	for i := 1; i <= attempts; i++ {
		if err = pool.Ping(ctx); err == nil {
			return nil
		}
		log.Warn("database not ready, retrying", "attempt", i, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i) * time.Second): // линейный backoff
		}
	}
	return err
}
