DELETE FROM wallet_credentials WHERE issuer <> 1;
ALTER TABLE wallet_credentials DROP CONSTRAINT IF EXISTS wallet_credentials_device_code_issuer_key;
ALTER TABLE wallet_credentials ADD CONSTRAINT wallet_credentials_device_id_code_key UNIQUE (device_id, code);
ALTER TABLE wallet_credentials DROP COLUMN IF EXISTS issuer;
