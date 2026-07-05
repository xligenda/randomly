CREATE OR REPLACE FUNCTION notify_transfer_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status IN ('PAID', 'SELECTED_RECEIVER', 'NOT_SELECTED') THEN
        PERFORM pg_notify('transfer_event', '');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER transfer_changed_trigger
AFTER INSERT OR UPDATE ON transfers
FOR EACH ROW
EXECUTE FUNCTION notify_transfer_change();