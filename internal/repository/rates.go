// Package repository — адаптер к PostgreSQL (реализация портов из domain).
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type RateRepo struct {
	pool *pgxpool.Pool
}

func NewRateRepo(pool *pgxpool.Pool) *RateRepo { return &RateRepo{pool: pool} }

// ActiveSymbols — какие монеты нужно обновлять фоновому воркеру.
// Список берём из БД, а не из константы: добавить монету = INSERT, без релиза.
func (r *RateRepo) ActiveSymbols(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT symbol FROM currencies WHERE is_active ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("query active symbols: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SaveRates — пакетная вставка снимков цен в одной транзакции.
// Пишем все или ничего: частично сохранённый срез исказил бы статистику.
func (r *RateRepo) SaveRates(ctx context.Context, rates []domain.Rate) error {
	if len(rates) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // После успешного Commit откат вернёт ErrTxClosed, это ожидаемо.

	const q = `
		INSERT INTO rates (currency_id, price, fetched_at)
		SELECT c.id, $2, $3 FROM currencies c WHERE c.symbol = $1`

	batch := &pgx.Batch{}
	for _, rt := range rates {
		batch.Queue(q, rt.Symbol, rt.Price, rt.FetchedAt)
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("batch insert rates: %w", err)
	}
	return tx.Commit(ctx)
}

// statsQuery — вся витрина считается ОДНИМ запросом.
// Почему в SQL, а не в Go: иначе пришлось бы выкачивать все строки за сутки
// в память приложения. Агрегация рядом с данными — дешевле.
//
// last     — самая свежая цена
// day      — MIN/MAX за последние 24 часа
// hour_ago — последняя цена, которая старше часа (база для расчёта дельты)
const statsQuery = `
WITH cur AS (
    SELECT id, symbol FROM currencies WHERE symbol = $1
),
last AS (
    SELECT r.price, r.fetched_at
    FROM rates r JOIN cur ON r.currency_id = cur.id
    ORDER BY r.fetched_at DESC LIMIT 1
),
day AS (
    SELECT MIN(r.price) AS mn, MAX(r.price) AS mx
    FROM rates r JOIN cur ON r.currency_id = cur.id
    WHERE r.fetched_at >= now() - interval '24 hours'
),
hour_ago AS (
    SELECT r.price
    FROM rates r JOIN cur ON r.currency_id = cur.id
    WHERE r.fetched_at <= now() - interval '1 hour'
    ORDER BY r.fetched_at DESC LIMIT 1
)
SELECT cur.symbol,
       last.price,
       COALESCE(day.mn, last.price),
       COALESCE(day.mx, last.price),
       hour_ago.price,          -- может быть NULL
       last.fetched_at
FROM cur
JOIN last ON TRUE
LEFT JOIN day ON TRUE
LEFT JOIN hour_ago ON TRUE`

func (r *RateRepo) GetStats(ctx context.Context, symbol string) (domain.RateStats, error) {
	var (
		st      domain.RateStats
		hourAgo *float64
	)
	err := r.pool.QueryRow(ctx, statsQuery, symbol).
		Scan(&st.Symbol, &st.Price, &st.DayMin, &st.DayMax, &hourAgo, &st.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Либо такой валюты нет, либо по ней ещё нет ни одного снимка.
		var exists bool
		if e := r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM currencies WHERE symbol=$1)`, symbol).
			Scan(&exists); e != nil {
			return st, fmt.Errorf("check currency: %w", e)
		}
		if !exists {
			return st, domain.ErrCurrencyNotFound
		}
		return st, domain.ErrNoData
	}
	if err != nil {
		return st, fmt.Errorf("get stats %s: %w", symbol, err)
	}
	st.HourChangePct = PercentChange(hourAgo, st.Price)
	return st, nil
}

func (r *RateRepo) GetAllStats(ctx context.Context) ([]domain.RateStats, error) {
	symbols, err := r.ActiveSymbols(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RateStats, 0, len(symbols))
	for _, s := range symbols {
		st, err := r.GetStats(ctx, s)
		if errors.Is(err, domain.ErrNoData) {
			continue // монета добавлена, но ещё не опрошена — просто пропускаем
		}
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// PercentChange — вынесена отдельно, чтобы покрыть юнит-тестом.
// Возвращает nil, если базы для сравнения нет или она нулевая.
func PercentChange(base *float64, current float64) *float64 {
	if base == nil || *base == 0 {
		return nil
	}
	v := (current - *base) / *base * 100
	return &v
}
