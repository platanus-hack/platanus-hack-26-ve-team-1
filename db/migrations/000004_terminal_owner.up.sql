-- Dueño del terminal (comercio): recibe por WhatsApp la alerta de manipulación.
ALTER TABLE terminals ADD COLUMN owner_id BIGINT REFERENCES citizens(id) ON DELETE SET NULL;
CREATE INDEX terminals_owner_idx ON terminals (owner_id);
