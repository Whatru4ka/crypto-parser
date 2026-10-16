package bot

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

// Бот отправляет сообщения с parse_mode = HTML.
// В HTML-режиме Telegram понимает теги <b>, <i>, <code>, <pre>, <a>,
// а символы < > & в обычном тексте ОБЯЗАНЫ быть экранированы.
// Поэтому любой текст, пришедший от пользователя, проходит через esc().
func esc(s string) string { return html.EscapeString(s) }

// FormatStats — блок по одной монете.
//
//	🟢 BTC  84 346.00 USDT
//	За час: ▲ +1.24%
//	За сутки: min 84 317.37 max 85 535.25
//	данные от 14:14 UTC
func FormatStats(s domain.RateStats) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s <b>%s</b>  <code>%s</code> USDT\n",
		trendIcon(s.HourChangePct), esc(s.Symbol), formatPrice(s.Price))
	fmt.Fprintf(&sb, "За час: %s\n", formatChange(s.HourChangePct))
	fmt.Fprintf(&sb, "За сутки: min <code>%s</code> · max <code>%s</code>\n",
		formatPrice(s.DayMin), formatPrice(s.DayMax))
	fmt.Fprintf(&sb, "<i>данные от %s UTC</i>", s.UpdatedAt.UTC().Format("15:04"))
	return sb.String()
}

// FormatRates — сообщение целиком: блоки по монетам + время формирования.
//
// Время передаётся параметром (а не time.Now() внутри) по двум причинам:
//  1. функцию можно проверить детерминированным тестом;
//  2. строка со временем с секундами делает текст уникальным при каждом
//     нажатии «Обновить», и Telegram не отвечает ошибкой
//     "message is not modified" на editMessageText с тем же текстом.
func FormatRates(stats []domain.RateStats, now time.Time) string {
	blocks := make([]string, 0, len(stats))
	for _, s := range stats {
		blocks = append(blocks, FormatStats(s))
	}
	return strings.Join(blocks, "\n\n") +
		"\n\n🕒 " + now.UTC().Format("15:04:05") + " UTC"
}

func trendIcon(pct *float64) string {
	switch {
	case pct == nil || roundPct(*pct) == 0:
		return "⚪"
	case *pct > 0:
		return "🟢"
	default:
		return "🔴"
	}
}

func formatChange(pct *float64) string {
	if pct == nil {
		return "н/д" // истории меньше часа — не путаем с 0%
	}
	v := roundPct(*pct)
	switch {
	case v > 0:
		return fmt.Sprintf("▲ +%.2f%%", v)
	case v < 0:
		return fmt.Sprintf("▼ %.2f%%", v)
	default:
		return "0.00%"
	}
}

// roundPct округляет до 2 знаков. Нужен, чтобы -0.004 не превращалось
// в строку "-0.00%" и не получало красную иконку.
func roundPct(v float64) float64 { return math.Round(v*100) / 100 }

// formatPrice: 84346 -> "84 346.00".
func formatPrice(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	intPart, frac, _ := strings.Cut(s, ".")

	sign := ""
	if strings.HasPrefix(intPart, "-") {
		sign, intPart = "-", intPart[1:]
	}

	var b strings.Builder
	for i, ch := range intPart {
		// пробел ставим перед каждой группой из 3 цифр, считая справа
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	return sign + b.String() + "." + frac
}
