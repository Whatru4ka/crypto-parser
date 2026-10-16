package bot_test

import (
	"strings"
	"testing"

	"github.com/Whatru4ka/crypto-rates/internal/bot"
	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

func TestParseCallback(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    bot.Callback
		wantErr bool
	}{
		{"все монеты", "r:all", bot.Callback{Action: "r"}, false},
		{"одна монета", "r:BTC", bot.Callback{Action: "r", Symbol: "BTC"}, false},
		{"автоотправка 15", "a:15", bot.Callback{Action: "a", Minutes: 15}, false},
		{"автоотправка выкл", "a:off", bot.Callback{Action: "a"}, false},
		{"пустая строка", "", bot.Callback{}, true},
		{"нет параметра", "r:", bot.Callback{}, true},
		{"нет разделителя", "rBTC", bot.Callback{}, true},
		{"неизвестное действие", "x:1", bot.Callback{}, true},
		{"интервал не число", "a:abc", bot.Callback{}, true},
		{"интервал вне диапазона", "a:5000", bot.Callback{}, true},
		{"нулевой интервал", "a:0", bot.Callback{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bot.ParseCallback(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Каждая кнопка обязана укладываться в лимит Telegram (64 байта)
// и обязана разбираться нашим же парсером — иначе нажатие «в никуда».
func TestRatesKeyboard_CallbackDataIsValid(t *testing.T) {
	list := []domain.RateStats{{Symbol: "BTC"}, {Symbol: "ETH"}}

	for _, current := range []string{"", "BTC"} {
		kb := bot.RatesKeyboard(list, current)
		buttons := 0
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				buttons++
				if btn.CallbackData == nil {
					t.Fatalf("у кнопки %q нет callback_data", btn.Text)
				}
				data := *btn.CallbackData
				if len(data) > 64 {
					t.Errorf("callback_data %q длиннее 64 байт", data)
				}
				if _, err := bot.ParseCallback(data); err != nil {
					t.Errorf("кнопка %q: парсер не понимает %q: %v", btn.Text, data, err)
				}
			}
		}
		if buttons == 0 {
			t.Fatal("клавиатура пуста")
		}
	}
}

func TestRatesKeyboard_RefreshKeepsCurrentView(t *testing.T) {
	list := []domain.RateStats{{Symbol: "BTC"}, {Symbol: "ETH"}}

	refresh := func(current string) string {
		kb := bot.RatesKeyboard(list, current)
		btn := kb.InlineKeyboard[0][0]
		if !strings.Contains(btn.Text, "Обновить") {
			t.Fatalf("первая кнопка — не «Обновить»: %q", btn.Text)
		}
		return *btn.CallbackData
	}

	if got := refresh(""); got != "r:all" {
		t.Errorf("обновление общего экрана: got %q, want r:all", got)
	}
	if got := refresh("ETH"); got != "r:ETH" {
		t.Errorf("обновление экрана ETH: got %q, want r:ETH", got)
	}
}
