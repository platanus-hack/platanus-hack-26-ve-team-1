CREATE TABLE consent_requests (
    id          BIGSERIAL PRIMARY KEY,
    handle      BYTEA NOT NULL REFERENCES cards(handle) ON DELETE CASCADE,
    terminal_id BIGINT NOT NULL REFERENCES terminals(id) ON DELETE CASCADE,
    attribute   SMALLINT NOT NULL CHECK (attribute BETWEEN 1 AND 6),
    phone       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pendiente'
                CHECK (status IN ('pendiente', 'aprobado', 'rechazado')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    answered_at TIMESTAMPTZ
);
CREATE INDEX consent_requests_pending_idx ON consent_requests (phone, created_at DESC)
    WHERE status = 'pendiente';
