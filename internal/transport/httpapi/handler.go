// Package httpapi — REST-транспорт. Задача слоя: распарсить запрос,
// вызвать сервис, замапить доменную ошибку на HTTP-статус. Логики здесь нет.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type Handler struct {
	svc domain.RateService
	log *slog.Logger
}

func NewHandler(svc domain.RateService, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	// Middleware-цепочка: request id -> логирование -> recover -> таймаут.
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer) // паника в хендлере => 500, а не падение процесса
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/rates", h.listRates)
	r.Get("/rates/{cryptocurrency}", h.getRate)

	return r
}

func (h *Handler) listRates(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.List(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rates": stats})
}

func (h *Handler) getRate(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "cryptocurrency")
	st, err := h.svc.Get(r.Context(), symbol)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// writeError — единственное место, где доменные ошибки превращаются в статусы.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrCurrencyNotFound):
		writeJSON(w, http.StatusNotFound, errBody{"currency not found"})
	case errors.Is(err, domain.ErrNoData):
		// 503: ресурс существует, но данных ещё нет — клиенту стоит повторить.
		writeJSON(w, http.StatusServiceUnavailable, errBody{"no data yet, try later"})
	default:
		h.log.Error("internal error", "err", err)
		// Наружу не отдаём подробности — это утечка внутренностей системы.
		writeJSON(w, http.StatusInternalServerError, errBody{"internal error"})
	}
}

type errBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
