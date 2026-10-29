// Composition root: единственное место, где конкретные реализации
// связываются друг с другом через интерфейсы из domain.
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

	app, err := setup(log)
	if err != nil {
		log.Error("setup", "err", err)
		os.Exit(1)
	}
	defer app.close()

	if err := run(app); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// app — собранные зависимости. Поля с интерфейсами, чтобы слои не знали конкретные типы.
type app struct {
	log             *slog.Logger
	shutdownTimeout time.Duration
	pool            *pgxpool.Pool
	worker          *fetcher.Worker
	tg              *bot.Bot
	sched           *bot.Scheduler
	http            *http.Server
}

// setup только инициализирует: конфиг, БД, репозитории, сервисы, транспорты.
func setup(log *slog.Logger) (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := waitForDB(ctx, pool, log); err != nil {
		pool.Close()
		return nil, err
	}

	rateRepo := repository.NewRateRepo(pool)
	subRepo := repository.NewSubscriptionRepo(pool)
	rateSvc := service.NewRateService(rateRepo)
	provider := fetcher.NewBinanceProvider(cfg.ProviderBaseURL)

	a := &app{
		log:             log,
		shutdownTimeout: cfg.ShutdownTimeout,
		pool:            pool,
		worker:          fetcher.NewWorker(provider, rateRepo, cfg.FetchInterval, log),
		http: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           httpapi.NewHandler(rateSvc, log).Router(),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	if cfg.TelegramToken == "" {
		log.Warn("TELEGRAM_TOKEN is empty, bot disabled")
		return a, nil
	}

	tg, err := bot.New(cfg.TelegramToken, rateSvc, subRepo, log)
	if err != nil {
		pool.Close()
		return nil, err
	}
	a.tg = tg
	a.sched = bot.NewScheduler(subRepo, rateSvc, tg, log)
	return a, nil
}

// run стартует фоновые процессы и HTTP, затем ждёт сигнала на остановку.
func run(a *app) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() { defer wg.Done(); a.worker.Run(ctx) }()

	if a.tg != nil {
		wg.Add(2)
		go func() { defer wg.Done(); a.tg.Run(ctx) }()
		go func() { defer wg.Done(); a.sched.Run(ctx) }()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.log.Info("http server started", "addr", a.http.Addr)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	a.log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	_ = a.http.Shutdown(shutdownCtx)

	wg.Wait()
	a.log.Info("stopped")
	return nil
}

func (a *app) close() {
	if a.pool != nil {
		a.pool.Close()
	}
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
		case <-time.After(time.Duration(i) * time.Second):
		}
	}
	return err
}
