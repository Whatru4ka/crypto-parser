package bot

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type stubRates struct {
	stats []domain.RateStats
	err   error
}

func (s stubRates) List(context.Context) ([]domain.RateStats, error) { return s.stats, s.err }
func (s stubRates) Get(context.Context, string) (domain.RateStats, error) {
	return domain.RateStats{}, nil
}

type stubSubs struct {
	due     []domain.Subscription
	marked  []int64
	listErr error
}

func (s *stubSubs) Upsert(context.Context, int64, int) error         { return nil }
func (s *stubSubs) Deactivate(context.Context, int64) error          { return nil }
func (s *stubSubs) MarkSent(_ context.Context, chatID int64, _ time.Time) error {
	s.marked = append(s.marked, chatID)
	return nil
}
func (s *stubSubs) DueSubscriptions(context.Context, time.Time) ([]domain.Subscription, error) {
	return s.due, s.listErr
}

type captureSender struct{ chats []int64 }

func (c *captureSender) SendText(chatID int64, _ string) { c.chats = append(c.chats, chatID) }

func TestScheduler_dispatchSendsToDueChats(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	subs := &stubSubs{due: []domain.Subscription{{ChatID: 11}, {ChatID: 22}}}
	sender := &captureSender{}
	s := &Scheduler{
		subs:   subs,
		svc:    stubRates{stats: []domain.RateStats{{Symbol: "BTC", Price: 1}}},
		sender: sender,
		log:    log,
	}

	s.dispatch(context.Background(), time.Now().UTC())

	if len(sender.chats) != 2 {
		t.Fatalf("sends = %v, want 2 chats", sender.chats)
	}
	if len(subs.marked) != 2 {
		t.Fatalf("marked = %v, want 2", subs.marked)
	}
}

func TestScheduler_dispatchSkipsWhenNoDue(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := &captureSender{}
	s := &Scheduler{
		subs:   &stubSubs{},
		svc:    stubRates{stats: []domain.RateStats{{Symbol: "BTC"}}},
		sender: sender,
		log:    log,
	}

	s.dispatch(context.Background(), time.Now().UTC())

	if len(sender.chats) != 0 {
		t.Fatalf("unexpected sends: %v", sender.chats)
	}
}
