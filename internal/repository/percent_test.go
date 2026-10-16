package repository_test

import (
	"math"
	"testing"

	"github.com/Whatru4ka/crypto-rates/internal/repository"
)

func ptr(f float64) *float64 { return &f }

func TestPercentChange(t *testing.T) {
	tests := []struct {
		name    string
		base    *float64
		current float64
		want    *float64
	}{
		{"рост на 10%", ptr(100), 110, ptr(10)},
		{"падение на 25%", ptr(200), 150, ptr(-25)},
		{"без изменений", ptr(50), 50, ptr(0)},
		{"нет базы (мало истории)", nil, 100, nil},
		{"нулевая база — деления на ноль быть не должно", ptr(0), 100, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repository.PercentChange(tt.base, tt.current)
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("got %v, want nil", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("got nil, want %v", *tt.want)
			case tt.want != nil && math.Abs(*got-*tt.want) > 1e-9:
				t.Fatalf("got %v, want %v", *got, *tt.want)
			}
		})
	}
}
