-- Eventos de tamper (sin handle, atributo ni resultado) y modo de cada terminal.
ALTER TABLE events
    ALTER COLUMN handle DROP NOT NULL,
    ALTER COLUMN attribute DROP NOT NULL,
    ALTER COLUMN result DROP NOT NULL,
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'verif' CHECK (kind IN ('verif', 'tamper')),
    ADD CONSTRAINT events_verif_fields CHECK (
        kind = 'tamper' OR (handle IS NOT NULL AND attribute IS NOT NULL AND result IS NOT NULL)
    );

ALTER TABLE terminals
    ADD COLUMN modo TEXT NOT NULL DEFAULT 'verificar'
    CHECK (modo IN ('verificar', 'emergencia', 'emitir', 'vincular'));
