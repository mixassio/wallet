-- Схема и сид игроков. Файл монтируется в /docker-entrypoint-initdb.d Postgres
-- и выполняется автоматически при первой инициализации тома данных.

CREATE TABLE IF NOT EXISTS players (
    id       UUID PRIMARY KEY,
    currency CHAR(3)       NOT NULL,
    balance  NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (balance >= 0)
);

INSERT INTO players (id, currency, balance) VALUES
    ('11111111-1111-1111-1111-111111111111', 'USD', 100.00),
    ('22222222-2222-2222-2222-222222222222', 'EUR', 50.50)
ON CONFLICT (id) DO NOTHING;
