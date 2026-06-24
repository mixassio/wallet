CREATE TABLE IF NOT EXISTS transactions (
    id         UUID          PRIMARY KEY,
    player_id  UUID          NOT NULL REFERENCES players(id),
    type       VARCHAR(10)   NOT NULL CHECK (type IN ('deposit', 'withdraw')),
    amount     NUMERIC(20,2) NOT NULL CHECK (amount >= 0),
    currency   CHAR(3)       NOT NULL,
    status     VARCHAR(10)   NOT NULL DEFAULT 'applied' CHECK (status IN ('applied', 'cancelled')),
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transactions_player_id ON transactions(player_id);
