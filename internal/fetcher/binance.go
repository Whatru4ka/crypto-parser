// Package fetcher — адаптер к внешнему источнику курсов + фоновый воркер.
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

// BinanceProvider реализует domain.RateProvider.
// baseURL вынесен в поле — в тестах подставляем httptest.Server.
type BinanceProvider struct {
	baseURL string
	client  *http.Client
	quote   string // котировочная валюта: BTCUSDT
}

var _ domain.RateProvider = (*BinanceProvider)(nil)

func NewBinanceProvider(baseURL string) *BinanceProvider {
	return &BinanceProvider{
		baseURL: baseURL,
		quote:   "USDT",
		// Таймаут обязателен: без него горутина воркера может зависнуть навсегда.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type binanceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"` // Binance отдаёт цену строкой
}

// FetchRates забирает цены одним запросом на все символы сразу,
// чтобы не упираться в rate limit внешнего API.
func (p *BinanceProvider) FetchRates(ctx context.Context, symbols []string) ([]domain.Rate, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	// Binance принимает symbols=["BTCUSDT","ETHUSDT"] в query-параметре.
	pairs := make([]string, 0, len(symbols))
	back := make(map[string]string, len(symbols)) // BTCUSDT -> BTC
	for _, s := range symbols {
		pair := s + p.quote
		pairs = append(pairs, `"`+pair+`"`)
		back[pair] = s
	}
	list := "[" + join(pairs, ",") + "]"

	endpoint := fmt.Sprintf("%s/api/v3/ticker/price?symbols=%s",
		p.baseURL, url.QueryEscape(list))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call provider: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // ошибка закрытия тела после чтения не влияет на результат

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	var tickers []binanceTicker
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}

	now := time.Now().UTC()
	out := make([]domain.Rate, 0, len(tickers))
	for _, t := range tickers {
		sym, ok := back[t.Symbol]
		if !ok {
			continue // пришло что-то лишнее — игнорируем
		}
		price, err := strconv.ParseFloat(t.Price, 64)
		if err != nil {
			return nil, fmt.Errorf("parse price for %s: %w", t.Symbol, err)
		}
		out = append(out, domain.Rate{Symbol: sym, Price: price, FetchedAt: now})
	}
	return out, nil
}

func join(parts []string, sep string) string {
	res := ""
	for i, p := range parts {
		if i > 0 {
			res += sep
		}
		res += p
	}
	return res
}
