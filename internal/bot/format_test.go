package bot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/bot"
	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

func pct(v float64) *float64 { return &v }

func stats(sym string, price float64, change *float64) domain.RateStats {
	return domain.RateStats{
		Symbol: sym, Price: price, DayMin: price - 1000, DayMax: price + 1000,
		HourChangePct: change,
		UpdatedAt:     time.Date(2026, 1, 2, 14, 14, 6, 0, time.UTC),
	}
}

func TestFormatStats(t *testing.T) {
	tests := []struct {
		name  string
		stats domain.RateStats
		want  []string // подстроки, которые обязаны быть в выводе
		deny  []string // и которых быть не должно
	}{
		{
			name:  "рост",
			stats: stats("BTC", 84346, pct(1.236)),
			want: []string{"🟢", "<b>BTC</b>", "<code>84 346.00</code>", "▲ +1.24%",
				"min <code>83 346.00</code> · max <code>85 346.00</code>", "данные от 14:14 UTC"},
		},
		{
			name:  "падение",
			stats: stats("ETH", 2700.86, pct(-1.19)),
			want:  []string{"🔴", "<b>ETH</b>", "<code>2 700.86</code>", "▼ -1.19%"},
		},
		{
			name:  "нет истории за час",
			stats: stats("BTC", 100, nil),
			want:  []string{"⚪", "н/д"},
			deny:  []string{"🟢", "🔴", "▲", "▼"},
		},
		{
			name:  "микроскопическое падение не даёт «-0.00%»",
			stats: stats("BTC", 100, pct(-0.004)),
			want:  []string{"⚪", "0.00%"},
			deny:  []string{"-0.00", "🔴", "▼"},
		},
		{
			name:  "символ экранируется",
			stats: stats("<b>", 100, nil),
			want:  []string{"&lt;b&gt;"},
			deny:  []string{"<b><b>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := bot.FormatStats(tt.stats)
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("вывод %q не содержит %q", out, w)
				}
			}
			for _, d := range tt.deny {
				if strings.Contains(out, d) {
					t.Errorf("вывод %q не должен содержать %q", out, d)
				}
			}
		})
	}
}

func TestFormatRates_TimestampMakesTextUnique(t *testing.T) {
	list := []domain.RateStats{stats("BTC", 100, nil), stats("ETH", 50, nil)}

	t1 := time.Date(2026, 1, 2, 14, 20, 31, 0, time.UTC)
	t2 := t1.Add(time.Second)

	a, b := bot.FormatRates(list, t1), bot.FormatRates(list, t2)

	if !strings.HasSuffix(a, "🕒 14:20:31 UTC") {
		t.Fatalf("нет метки времени в конце: %q", a)
	}
	// Ключевое свойство: при тех же данных текст у разных нажатий разный,
	// поэтому editMessageText не вернёт "message is not modified".
	if a == b {
		t.Fatal("тексты с разным временем не должны совпадать")
	}
	// Оба блока на месте и разделены пустой строкой.
	if !strings.Contains(a, "BTC") || !strings.Contains(a, "ETH") || !strings.Contains(a, "\n\n") {
		t.Fatalf("неожиданный формат: %q", a)
	}
}

func TestFormatRates_TimezoneIsNormalizedToUTC(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*3600)
	now := time.Date(2026, 1, 2, 16, 0, 0, 0, loc) // это 14:00 UTC
	out := bot.FormatRates([]domain.RateStats{stats("BTC", 100, nil)}, now)
	if !strings.HasSuffix(out, "🕒 14:00:00 UTC") {
		t.Fatalf("время не приведено к UTC: %q", out)
	}
}
