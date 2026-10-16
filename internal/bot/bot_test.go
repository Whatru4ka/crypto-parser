package bot_test

import (
	"testing"

	"github.com/Whatru4ka/crypto-rates/internal/bot"
)

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr bool
	}{
		{"валидный интервал", []string{"10"}, 10, false},
		{"нижняя граница", []string{"1"}, 1, false},
		{"верхняя граница", []string{"1440"}, 1440, false},
		{"без аргумента", nil, 0, true},
		{"не число", []string{"десять"}, 0, true},
		{"ноль", []string{"0"}, 0, true},
		{"отрицательное", []string{"-5"}, 0, true},
		{"больше суток", []string{"1441"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bot.ParseInterval(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
