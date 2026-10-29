package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type SubscriptionRepo struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepo(pool *pgxpool.Pool) *SubscriptionRepo {
	return &SubscriptionRepo{pool: pool}
}

var _ domain.SubscriptionRepository = (*SubscriptionRepo)(nil)

// Upsert — /start-auto N. Повторный вызов меняет интервал, а не плодит строки.
func (r *SubscriptionRepo) Upsert(ctx context.Context, chatID int64, minutes int) error {
	const q = `
		INSERT INTO subscriptions (chat_id, interval_minutes, is_active, last_sent_at)
		VALUES ($1, $2, TRUE, NULL)
		ON CONFLICT (chat_id) DO UPDATE
		SET interval_minutes = EXCLUDED.interval_minutes,
		    is_active        = TRUE,
		    last_sent_at     = NULL,
		    updated_at       = now()`
	if _, err := r.pool.Exec(ctx, q, chatID, minutes); err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

// Deactivate — /stop-auto. Мягкое выключение: историю подписки сохраняем.
func (r *SubscriptionRepo) Deactivate(ctx context.Context, chatID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE subscriptions SET is_active=FALSE, updated_at=now() WHERE chat_id=$1`, chatID)
	if err != nil {
		return fmt.Errorf("deactivate subscription: %w", err)
	}
	return nil
}

// DueSubscriptions — кому пора слать.
// Условие: ни разу не слали ИЛИ прошло >= interval_minutes.
// Вся выборка делается в БД — приложение не хранит таймеры в памяти,
// поэтому переживает рестарт и масштабируется на несколько реплик.
func (r *SubscriptionRepo) DueSubscriptions(ctx context.Context, now time.Time) ([]domain.Subscription, error) {
	const q = `
		SELECT chat_id, interval_minutes, is_active, last_sent_at
		FROM subscriptions
		WHERE is_active
		  AND (last_sent_at IS NULL
		       OR last_sent_at + (interval_minutes * interval '1 minute') <= $1)`
	rows, err := r.pool.Query(ctx, q, now)
	if err != nil {
		return nil, fmt.Errorf("query due subscriptions: %w", err)
	}
	defer rows.Close()

	var out []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(&s.ChatID, &s.IntervalMinutes, &s.IsActive, &s.LastSentAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SubscriptionRepo) MarkSent(ctx context.Context, chatID int64, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE subscriptions SET last_sent_at=$2, updated_at=now() WHERE chat_id=$1`, chatID, at)
	return err
}
