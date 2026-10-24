CREATE TABLE citizens (
    id             BIGSERIAL PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'ciudadano' CHECK (role IN ('ciudadano', 'emisor', 'comercio')),
    name           TEXT NOT NULL DEFAULT '',
    whatsapp_phone TEXT,
    notifications  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cards (
    handle     BYTEA PRIMARY KEY CHECK (octet_length(handle) = 8),
    uid        BYTEA NOT NULL CHECK (octet_length(uid) IN (4, 7, 10)),
    citizen_id BIGINT REFERENCES citizens(id) ON DELETE SET NULL,
    status     TEXT NOT NULL DEFAULT 'activa' CHECK (status IN ('activa', 'revocada')),
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE attributes (
    id         BIGSERIAL PRIMARY KEY,
    handle     BYTEA NOT NULL REFERENCES cards(handle) ON DELETE CASCADE,
    code       SMALLINT NOT NULL CHECK (code BETWEEN 1 AND 6),
    issuer     SMALLINT NOT NULL CHECK (issuer IN (1, 2)),
    value      BYTEA NOT NULL CHECK (octet_length(value) <= 64),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (handle, code)
);

CREATE TABLE devices (
    id         BIGSERIAL PRIMARY KEY,
    citizen_id BIGINT NOT NULL REFERENCES citizens(id) ON DELETE CASCADE,
    handle     BYTEA NOT NULL REFERENCES cards(handle) ON DELETE CASCADE,
    public_key BYTEA NOT NULL UNIQUE CHECK (octet_length(public_key) = 33),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE wallet_credentials (
    id         BIGSERIAL PRIMARY KEY,
    device_id  BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    handle     BYTEA NOT NULL REFERENCES cards(handle) ON DELETE CASCADE,
    code       SMALLINT NOT NULL CHECK (code BETWEEN 1 AND 6),
    blob       BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (device_id, code)
);

CREATE TABLE terminals (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('comercio', 'registro', 'salud', 'transito', 'emergencia')),
    attribute  SMALLINT CHECK (attribute BETWEEN 1 AND 6),
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE events (
    id          BIGSERIAL PRIMARY KEY,
    handle      BYTEA NOT NULL CHECK (octet_length(handle) = 8),
    terminal_id BIGINT REFERENCES terminals(id) ON DELETE SET NULL,
    attribute   SMALLINT NOT NULL CHECK (attribute BETWEEN 1 AND 6),
    result      SMALLINT NOT NULL CHECK (result BETWEEN 1 AND 6),
    emergency   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE link_codes (
    code       TEXT PRIMARY KEY CHECK (code ~ '^[0-9]{6}$'),
    handle     BYTEA NOT NULL REFERENCES cards(handle) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX events_handle_created_idx   ON events (handle, created_at DESC);
CREATE INDEX events_terminal_created_idx ON events (terminal_id, created_at DESC);
CREATE INDEX events_created_idx          ON events (created_at);
CREATE INDEX cards_revoked_idx           ON cards (handle) WHERE status = 'revocada';
CREATE INDEX cards_citizen_idx           ON cards (citizen_id);
CREATE INDEX link_codes_expires_idx      ON link_codes (expires_at);
