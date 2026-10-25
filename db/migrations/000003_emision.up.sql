-- Datos declarados del ciudadano (de ellos salen los atributos que se firman)
-- y solicitudes de emisión que el emisor deja pendientes para el terminal.
ALTER TABLE citizens ADD COLUMN profile JSONB NOT NULL DEFAULT '{}';

CREATE TABLE issuance_requests (
    id           BIGSERIAL PRIMARY KEY,
    citizen_id   BIGINT NOT NULL REFERENCES citizens(id) ON DELETE CASCADE,
    requested_by BIGINT REFERENCES citizens(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at      TIMESTAMPTZ
);

CREATE INDEX issuance_pending_idx ON issuance_requests (created_at) WHERE used_at IS NULL;
