CREATE TABLE settlements (
    id                         UUID        PRIMARY KEY,
    user_id                    UUID        NOT NULL REFERENCES users(id)  ON DELETE RESTRICT,
    vault_id                   UUID        NOT NULL REFERENCES vaults(id) ON DELETE RESTRICT,
    amount                     NUMERIC(20, 8) NOT NULL CHECK (amount > 0),
    currency                   VARCHAR(10) NOT NULL,
    fiat_currency              VARCHAR(10) NOT NULL,
    fiat_amount                NUMERIC(20, 8) NOT NULL CHECK (fiat_amount > 0),
    exchange_rate              NUMERIC(20, 8) NOT NULL CHECK (exchange_rate > 0),
    destination_type           VARCHAR(50)  NOT NULL,
    destination_provider       VARCHAR(50)  NOT NULL,
    destination_account_number VARCHAR(100) NOT NULL,
    destination_account_name   VARCHAR(200) NOT NULL,
    destination_bank_code      VARCHAR(20)  NOT NULL DEFAULT '',
    status                     VARCHAR(30)  NOT NULL DEFAULT 'initiated'
                                    CHECK (status IN (
                                        'initiated',
                                        'liquidity_matched',
                                        'fiat_dispatched',
                                        'confirmed',
                                        'failed'
                                    )),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at               TIMESTAMPTZ
);

CREATE INDEX idx_settlements_user_id ON settlements(user_id);
CREATE INDEX idx_settlements_vault_id ON settlements(vault_id);
CREATE INDEX idx_settlements_status ON settlements(status);
