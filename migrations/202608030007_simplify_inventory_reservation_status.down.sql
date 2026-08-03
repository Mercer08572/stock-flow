-- Migration metadata
-- status: pending
-- description: revert reservation lifecycle status and closure reason

ALTER TABLE inventory_reservations
    DROP CONSTRAINT chk_inventory_reservations_status_matches_quantities,
    DROP CONSTRAINT chk_inventory_reservations_status_matches_close_reason,
    DROP CONSTRAINT chk_inventory_reservations_close_reason,
    DROP CONSTRAINT chk_inventory_reservations_status;

UPDATE inventory_reservations
SET status = CASE
    WHEN consumed_qty = total_qty THEN 'consumed'
    WHEN released_qty = total_qty THEN 'released'
    WHEN released_qty + consumed_qty = total_qty THEN 'cancelled'
    ELSE 'active'
END;

ALTER TABLE inventory_reservations
    ADD CONSTRAINT chk_inventory_reservations_status
        CHECK (status IN ('active', 'released', 'consumed', 'cancelled'));

ALTER TABLE inventory_reservations
    DROP COLUMN close_reason;
