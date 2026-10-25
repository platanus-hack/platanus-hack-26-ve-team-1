-- El carnet universitario es una credencial de billetera firmada por el emisor 0x02:
-- un mismo dispositivo puede tener A3 del emisor nacional y A3 de la universidad.
ALTER TABLE wallet_credentials
    ADD COLUMN issuer SMALLINT NOT NULL DEFAULT 1 CHECK (issuer IN (1, 2));
ALTER TABLE wallet_credentials DROP CONSTRAINT wallet_credentials_device_id_code_key;
ALTER TABLE wallet_credentials ADD CONSTRAINT wallet_credentials_device_code_issuer_key UNIQUE (device_id, code, issuer);
