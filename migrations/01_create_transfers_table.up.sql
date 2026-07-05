CREATE TABLE transfers (
    id             UUID PRIMARY KEY,
    amount         INTEGER NOT NULL CHECK (amount > 0),
    comment        TEXT,
    sender         VARCHAR(255) NOT NULL,
    anonymous      BOOLEAN NOT NULL DEFAULT FALSE,
    receiver       VARCHAR(255),
    status         VARCHAR(32) NOT NULL DEFAULT 'created'
                   CHECK (status IN ('created', 'paid', 'user_selected', 'not_selected', 'not_selected', 'failed')),
    failure_reason TEXT,
    payment_code    TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    leased_until   TIMESTAMPTZ
);

CREATE INDEX idx_transfers_status_leased_until
    ON transfers (status, leased_until);

CREATE INDEX idx_transfers_sender
    ON transfers (sender);