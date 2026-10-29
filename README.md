# Crypto Rates Service

Сервис получает курсы криптовалют (BTC, ETH) из внешнего API, сохраняет историю
в PostgreSQL и отдаёт её через REST API и Telegram-бота.

## Возможности

- фоновое обновление курсов при старте и далее каждые 5 минут;
- статистика: текущая цена, min/max за сутки, изменение в % за последний час;
- REST: `GET /rates`, `GET /rates/{cryptocurrency}`;
- Telegram: `/start`, `/rates`, `/rates BTC`, `/start-auto 10`, `/stop-auto`.

## Архитектура

```
cmd/app                 composition root: сборка зависимостей, graceful shutdown
internal/domain         типы, ошибки и ПОРТЫ (интерфейсы). Ни от чего не зависит
internal/repository     адаптер PostgreSQL (реализация портов)
internal/fetcher        адаптер внешнего API + фоновый воркер
internal/service        бизнес-логика, общая для REST и бота
internal/transport/httpapi  REST-хендлеры (chi)
internal/bot            Telegram-хендлеры + шедулер автоотправки
migrations              SQL-схема
api/openapi.yaml        контракт REST API
```

Правило зависимостей: внешние слои знают о внутренних, обратное запрещено.
Контракты слоёв — интерфейсы в `internal/domain/interfaces.go` (`RateRepository`,
`RateService`, `RateProvider`, `SubscriptionRepository`). Реализации (`*RateRepo`,
`*RateService`, `*BinanceProvider`) к ним только подключаются; в тестах вместо
них моки.

## Запуск

```bash
cp .env.example .env      # заполнить TELEGRAM_TOKEN (можно оставить пустым)
docker compose up --build
```

API: http://localhost:8080/rates

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес REST API |
| `DATABASE_URL` | — (обязательно) | DSN PostgreSQL |
| `TELEGRAM_TOKEN` | пусто | пустой => бот не запускается |
| `FETCH_INTERVAL` | `5m` | период обновления курсов |
| `PROVIDER_BASE_URL` | `https://api.binance.com` | внешний источник курсов |
| `SHUTDOWN_TIMEOUT` | `10s` | таймаут graceful shutdown |

## Разработка

```bash
# go mod tidy
# go test ./... -race
# docker compose up --build
# golangci-lint
```
