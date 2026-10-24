ALTER TABLE issuance_requests DROP COLUMN IF EXISTS terminal_id;
DROP TRIGGER IF EXISTS terminals_sync_attribute ON terminals;
DROP FUNCTION IF EXISTS terminals_sync_attribute();
ALTER TABLE terminals DROP CONSTRAINT IF EXISTS terminals_attributes_check;
ALTER TABLE terminals DROP COLUMN IF EXISTS attributes;
