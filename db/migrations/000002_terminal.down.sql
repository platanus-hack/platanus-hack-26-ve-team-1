ALTER TABLE terminals DROP COLUMN IF EXISTS modo;

DELETE FROM events WHERE kind = 'tamper';
ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_verif_fields,
    DROP COLUMN IF EXISTS kind,
    ALTER COLUMN result SET NOT NULL,
    ALTER COLUMN attribute SET NOT NULL,
    ALTER COLUMN handle SET NOT NULL;
