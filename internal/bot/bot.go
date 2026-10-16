// Package bot — Telegram-транспорт. Как и HTTP, он только парсит ввод,
// зовёт тот же service и форматирует ответ.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Whatru4ka/crypto-rates/internal/domain"
)

type RateProvider interface {
	List(ctx context.Context) ([]domain.RateStats, error)
	Get(ctx context.Context, symbol string) (domain.RateStats, error)
}

const (
	minInterval = 1
	maxInterval = 1440
)

type Bot struct {
	api  *tgbotapi.BotAPI
	svc  RateProvider
	subs domain.SubscriptionRepository
	log  *slog.Logger
}

func New(token string, svc RateProvider, subs domain.SubscriptionRepository,
	log *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init telegram api: %w", err)
	}
	return &Bot{api: api, svc: svc, subs: subs, log: log}, nil
}

// Run читает апдейты long-polling'ом до отмены контекста.
func (b *Bot) Run(ctx context.Context) {
	b.setCommands()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			b.log.Info("telegram bot stopped")
			return
		case upd := <-updates:
			switch {
			// Нажатие на inline-кнопку приходит НЕ как Message,
			// а как отдельное поле апдейта — CallbackQuery.
			case upd.CallbackQuery != nil:
				go b.handleCallback(ctx, upd.CallbackQuery)
			case upd.Message != nil && upd.Message.IsCommand():
				// Каждое сообщение обрабатываем в отдельной горутине,
				// иначе медленный запрос к БД блокирует всю очередь апдейтов.
				go b.handle(ctx, upd.Message)
			}
		}
	}
}

// setCommands регистрирует меню команд (кнопка «Меню» рядом с полем ввода).
//
// Ограничение Bot API: имя команды — только a-z, 0-9 и "_", до 32 символов.
// Дефис запрещён, поэтому в меню попадает start_auto, а не start-auto.
// Обработчик ниже понимает оба написания.
func (b *Bot) setCommands() {
	cfg := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "rates", Description: "Курсы валют"},
		tgbotapi.BotCommand{Command: "start_auto", Description: "Автоотправка, например: /start_auto 10"},
		tgbotapi.BotCommand{Command: "stop_auto", Description: "Выключить автоотправку"},
		tgbotapi.BotCommand{Command: "start", Description: "Справка"},
	)
	// Ошибка меню не критична: бот прекрасно работает и без него.
	if _, err := b.api.Request(cfg); err != nil {
		b.log.Error("set bot commands", "err", err)
	}
}

func (b *Bot) handle(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	args := strings.Fields(msg.CommandArguments())

	switch msg.Command() {
	case "start":
		kb := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Показать курсы", actionRates+":all")))
		b.sendWithKeyboard(chatID, helpText, &kb)

	case "rates":
		symbol := ""
		if len(args) > 0 {
			symbol = args[0]
		}
		text, kb := b.ratesView(ctx, symbol)
		b.sendWithKeyboard(chatID, text, kb)

	case "start-auto", "start_auto":
		minutes, err := ParseInterval(args)
		if err != nil {
			b.send(chatID, err.Error())
			return
		}
		b.send(chatID, b.enableAuto(ctx, chatID, minutes))

	case "stop-auto", "stop_auto":
		b.send(chatID, b.disableAuto(ctx, chatID))

	default:
		b.send(chatID, "Неизвестная команда.\n\n"+helpText)
	}
}

// handleCallback — реакция на нажатие inline-кнопки.
func (b *Bot) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	// Telegram ждёт ответ на КАЖДЫЙ callback, иначе у кнопки
	// крутится индикатор загрузки. Отвечаем во всех ветках.
	cb, err := ParseCallback(cq.Data)
	if err != nil || cq.Message == nil {
		b.log.Warn("bad callback", "data", cq.Data, "err", err)
		b.answer(cq.ID, "", false)
		return
	}
	chatID, msgID := cq.Message.Chat.ID, cq.Message.MessageID

	switch cb.Action {
	case actionRates:
		text, kb := b.ratesView(ctx, cb.Symbol)
		if kb == nil {
			// Ошибка/пустой результат: старое сообщение с кнопками не трогаем,
			// а причину показываем всплывающим окном.
			b.answer(cq.ID, text, true)
			return
		}
		b.edit(chatID, msgID, text, kb)
		b.answer(cq.ID, "", false)

	case actionAuto:
		var toast string
		if cb.Minutes == 0 {
			toast = b.disableAuto(ctx, chatID)
		} else {
			toast = b.enableAuto(ctx, chatID, cb.Minutes)
		}
		b.answer(cq.ID, toast, false)
	}
}

// ratesView — ЕДИНСТВЕННОЕ место, где собирается экран с курсами.
// Его используют и команда /rates, и кнопки: дублировать нельзя,
// иначе два пути рано или поздно покажут разное.
//
// symbol == "" — все монеты. Возвращает (текст, клавиатура).
// kb == nil означает, что показывать нечего: text — это пояснение для пользователя.
// В пояснения НЕ подставляется пользовательский ввод — они безопасны и для
// HTML-сообщения, и для всплывающего окна (где HTML не работает).
func (b *Bot) ratesView(ctx context.Context, symbol string) (string, *tgbotapi.InlineKeyboardMarkup) {
	all, err := b.svc.List(ctx)
	if err != nil {
		b.log.Error("bot list rates", "err", err)
		return "Сервис временно недоступен.", nil
	}
	if len(all) == 0 {
		return "Данных пока нет, курсы ещё не загружены.", nil
	}

	shown, current := all, ""
	if symbol != "" {
		st, err := b.svc.Get(ctx, symbol)
		switch {
		case errors.Is(err, domain.ErrCurrencyNotFound):
			return "Такой валюты нет. Доступные: " + joinSymbols(all), nil
		case errors.Is(err, domain.ErrNoData):
			return "Данных по этой валюте пока нет.", nil
		case err != nil:
			b.log.Error("bot get rate", "err", err)
			return "Сервис временно недоступен.", nil
		}
		shown, current = []domain.RateStats{st}, st.Symbol
	}

	kb := RatesKeyboard(all, current)
	return FormatRates(shown, time.Now()), &kb
}

func joinSymbols(stats []domain.RateStats) string {
	names := make([]string, len(stats))
	for i, s := range stats {
		names[i] = s.Symbol
	}
	return strings.Join(names, ", ")
}

// enableAuto / disableAuto общие для команд и кнопок.
// Возвращают короткий текст-результат (в чат или во всплывающее окно).
func (b *Bot) enableAuto(ctx context.Context, chatID int64, minutes int) string {
	if err := b.subs.Upsert(ctx, chatID, minutes); err != nil {
		b.log.Error("enable auto", "err", err)
		return "Не удалось включить автоотправку, попробуйте позже."
	}
	return fmt.Sprintf("Автоотправка включена: каждые %d мин.", minutes)
}

func (b *Bot) disableAuto(ctx context.Context, chatID int64) string {
	if err := b.subs.Deactivate(ctx, chatID); err != nil {
		b.log.Error("disable auto", "err", err)
		return "Не удалось выключить автоотправку, попробуйте позже."
	}
	return "Автоотправка выключена."
}

// ParseInterval — чистая функция без зависимостей => идеальна для юнит-теста.
func ParseInterval(args []string) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("Укажите интервал в минутах, например: /start-auto 10")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, errors.New("Интервал должен быть целым числом минут.")
	}
	if n < minInterval || n > maxInterval {
		return 0, fmt.Errorf("Интервал должен быть от %d до %d минут.", minInterval, maxInterval)
	}
	return n, nil
}

// ---- низкоуровневые обёртки над Telegram API ----

func (b *Bot) send(chatID int64, text string) { b.sendWithKeyboard(chatID, text, nil) }

func (b *Bot) sendWithKeyboard(chatID int64, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML // см. format.go: ввод пользователя экранируем
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send telegram message", "chat_id", chatID, "err", err)
	}
}

// edit перерисовывает существующее сообщение (кнопка «Обновить»),
// а не шлёт новое — чат не засоряется.
func (b *Bot) edit(chatID int64, msgID int, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	cfg := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, text, *kb)
	cfg.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(cfg); err != nil && !isNotModified(err) {
		b.log.Error("edit telegram message", "chat_id", chatID, "err", err)
	}
}

// isNotModified: Telegram отвечает 400 "message is not modified", если новый
// текст и разметка совпали со старыми. Строка со временем с секундами в
// FormatRates почти исключает это, а здесь — страховка для редкого случая
// двух нажатий в одну секунду. Это не ошибка, логировать её не нужно.
func isNotModified(err error) bool {
	return err != nil && strings.Contains(err.Error(), "message is not modified")
}

// answer подтверждает нажатие кнопки. text != "" — всплывающее уведомление
// (alert=true — модальное окно с кнопкой OK, иначе короткий тост сверху).
func (b *Bot) answer(callbackID, text string, alert bool) {
	cfg := tgbotapi.NewCallback(callbackID, text)
	cfg.ShowAlert = alert
	if _, err := b.api.Request(cfg); err != nil {
		b.log.Error("answer callback", "err", err)
	}
}

// SendText — публичный метод для шедулера автоотправки.
func (b *Bot) SendText(chatID int64, text string) { b.send(chatID, text) }

const helpText = `<b>Курсы криптовалют</b>

/rates — курсы по всем монетам
/rates BTC — курс конкретной монеты
/start-auto 10 — присылать курсы каждые 10 минут
/stop-auto — выключить автоотправку`
