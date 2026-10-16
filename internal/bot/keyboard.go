package bot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

// Формат callback_data — компактная строка "действие:параметр".
// У Telegram лимит на callback_data — 64 байта, поэтому никакого JSON.
//
//	r:all   показать все монеты
//	r:BTC   показать одну монету
//	a:10    включить автоотправку каждые 10 минут
//	a:off   выключить автоотправку
const (
	actionRates = "r"
	actionAuto  = "a"
)

// Callback — разобранное нажатие на кнопку.
type Callback struct {
	Action  string // actionRates | actionAuto
	Symbol  string // для actionRates: "" означает "все монеты"
	Minutes int    // для actionAuto: 0 означает "выключить"
}

// Тот же принцип, что и у ParseInterval: чистая функция, которую легко тестировать.
// callback_data приходит от клиента и ей нельзя доверять — валидируем как любой ввод.
func ParseCallback(data string) (Callback, error) {
	action, param, ok := strings.Cut(data, ":")
	if !ok || param == "" {
		return Callback{}, errors.New("malformed callback data")
	}

	switch action {
	case actionRates:
		if param == "all" {
			return Callback{Action: actionRates}, nil
		}
		return Callback{Action: actionRates, Symbol: param}, nil

	case actionAuto:
		if param == "off" {
			return Callback{Action: actionAuto}, nil
		}
		n, err := strconv.Atoi(param)
		if err != nil || n < minInterval || n > maxInterval {
			return Callback{}, fmt.Errorf("bad auto interval %q", param)
		}
		return Callback{Action: actionAuto, Minutes: n}, nil
	}
	return Callback{}, fmt.Errorf("unknown callback action %q", action)
}

// autoIntervals — быстрые варианты интервалов на кнопках.
var autoIntervals = []int{5, 15, 60}

// RatesKeyboard строится из реального списка монет: добавили валюту в БД —
// кнопка появилась сама, без правки кода.
func RatesKeyboard(stats []domain.RateStats, current string) tgbotapi.InlineKeyboardMarkup {
	// Ряд 1: «Обновить» перерисовывает то же, что сейчас на экране.
	refreshData := actionRates + ":all"
	if current != "" {
		refreshData = actionRates + ":" + current
	}
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", refreshData)),
	}

	// Ряд 2: переключение между монетами.
	coins := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Все", actionRates+":all"),
	}
	for _, s := range stats {
		coins = append(coins,
			tgbotapi.NewInlineKeyboardButtonData(s.Symbol, actionRates+":"+s.Symbol))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(coins...))

	// Ряд 3: автоотправка.
	auto := make([]tgbotapi.InlineKeyboardButton, 0, len(autoIntervals)+1)
	for _, m := range autoIntervals {
		auto = append(auto, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("🔔 %s", intervalLabel(m)), actionAuto+":"+strconv.Itoa(m)))
	}
	auto = append(auto, tgbotapi.NewInlineKeyboardButtonData("🔕 Выкл", actionAuto+":off"))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(auto...))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func intervalLabel(minutes int) string {
	if minutes%60 == 0 {
		return fmt.Sprintf("%d ч", minutes/60)
	}
	return fmt.Sprintf("%d мин", minutes)
}
