DROP INDEX IF EXISTS terminals_owner_idx;
ALTER TABLE terminals DROP COLUMN IF EXISTS owner_id;
