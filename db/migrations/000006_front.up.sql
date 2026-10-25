-- Terminales con varios atributos y órdenes de emisión dirigidas a un terminal.
ALTER TABLE terminals ADD COLUMN attributes SMALLINT[] NOT NULL DEFAULT '{}';
UPDATE terminals SET attributes = ARRAY[attribute] WHERE attribute IS NOT NULL;
ALTER TABLE terminals ADD CONSTRAINT terminals_attributes_check CHECK (attributes <@ ARRAY[1,2,3,4,5,6]::smallint[]);

-- terminals.attribute queda como espejo del primer atributo.
CREATE FUNCTION terminals_sync_attribute() RETURNS trigger AS $$
BEGIN
    NEW.attribute := NEW.attributes[1];
    RETURN NEW;
END
$$ LANGUAGE plpgsql;
CREATE TRIGGER terminals_sync_attribute BEFORE INSERT OR UPDATE ON terminals
    FOR EACH ROW EXECUTE FUNCTION terminals_sync_attribute();

ALTER TABLE issuance_requests ADD COLUMN terminal_id BIGINT REFERENCES terminals(id) ON DELETE SET NULL;
