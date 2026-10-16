-- =========================================================
-- 001_init.up.sql — начальная схема БД (PostgreSQL)
-- =========================================================

-- Справочник отслеживаемых валют.
-- Вынесен в отдельную таблицу (3NF): символ монеты не дублируется
-- в миллионах строк истории, а хранится один раз.
CREATE TABLE IF NOT EXISTS currencies (
    id         SMALLSERIAL PRIMARY KEY,
    symbol     VARCHAR(16)  NOT NULL UNIQUE,   -- BTC, ETH
    name       VARCHAR(64)  NOT NULL,          -- Bitcoin, Ethereum
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- История котировок: одна строка = один снимок цены.
-- Храним именно историю, а не "текущую цену", иначе невозможно
-- посчитать min/max за сутки и изменение за час.
CREATE TABLE IF NOT EXISTS rates (
    id          BIGSERIAL     PRIMARY KEY,
    currency_id SMALLINT      NOT NULL REFERENCES currencies(id) ON DELETE CASCADE,
    price       NUMERIC(24,8) NOT NULL CHECK (price > 0),  -- NUMERIC, не FLOAT: деньги
    fetched_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- Главный индекс. Все три запроса (последняя цена, min/max за сутки,
-- цена час назад) фильтруют по currency_id и режут/сортируют по времени.
-- Составной (currency_id, fetched_at DESC) покрывает все три.
CREATE INDEX IF NOT EXISTS idx_rates_currency_fetched
    ON rates (currency_id, fetched_at DESC);

-- Подписки Telegram на автоотправку.
-- chat_id — естественный первичный ключ: у одного чата одна подписка.
CREATE TABLE IF NOT EXISTS subscriptions (
    chat_id          BIGINT      PRIMARY KEY,
    interval_minutes INTEGER     NOT NULL CHECK (interval_minutes BETWEEN 1 AND 1440),
    is_active        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_sent_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Частичный индекс: шедулер читает только активные подписки.
-- Индексировать неактивные бессмысленно — экономим размер индекса.
CREATE INDEX IF NOT EXISTS idx_subscriptions_active
    ON subscriptions (is_active) WHERE is_active;

-- Стартовые данные
INSERT INTO currencies (symbol, name) VALUES
    ('BTC', 'Bitcoin'),
    ('ETH', 'Ethereum')
ON CONFLICT (symbol) DO NOTHING;
