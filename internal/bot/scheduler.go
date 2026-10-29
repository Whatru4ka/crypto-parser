package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

// Sender — что шедулеру нужно от бота (снова интерфейс у потребителя).
type Sender interface {
	SendText(chatID int64, text string)
}

// Scheduler — автоотправка подписчикам.
//
// Решение: ОДИН тикер на всё приложение, который раз в минуту спрашивает у БД
// "кому пора слать", вместо горутины-таймера на каждого пользователя.
// Плюсы: состояние в БД переживает рестарт, число горутин не растёт с числом
// подписчиков, работает при нескольких репликах.
type Scheduler struct {
	subs   domain.SubscriptionRepository
	svc    domain.RateService
	sender Sender
	log    *slog.Logger
	tick   time.Duration
}

func NewScheduler(subs domain.SubscriptionRepository, svc domain.RateService,
	sender Sender, log *slog.Logger) *Scheduler {
	return &Scheduler{subs: subs, svc: svc, sender: sender, log: log, tick: time.Minute}
}

func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.tick)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("auto-send scheduler stopped")
			return
		case now := <-t.C:
			s.dispatch(ctx, now.UTC())
		}
	}
}

func (s *Scheduler) dispatch(ctx context.Context, now time.Time) {
	due, err := s.subs.DueSubscriptions(ctx, now)
	if err != nil {
		s.log.Error("load due subscriptions", "err", err)
		return
	}
	if len(due) == 0 {
		return
	}

	stats, err := s.svc.List(ctx)
	if err != nil || len(stats) == 0 {
		s.log.Error("auto-send: no stats", "err", err)
		return
	}

	// Текст формируем один раз на всех — это дешевле, чем на каждого.
	text := "🔔 <b>Автообновление курсов</b>\n\n" + FormatRates(stats, now)

	for _, sub := range due {
		s.sender.SendText(sub.ChatID, text)
		if err := s.subs.MarkSent(ctx, sub.ChatID, now); err != nil {
			s.log.Error("mark sent", "chat_id", sub.ChatID, "err", err)
		}
	}
}
