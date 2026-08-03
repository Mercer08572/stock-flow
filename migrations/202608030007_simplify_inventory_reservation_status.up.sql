-- Migration metadata
-- status: pending
-- description: simplify reservation lifecycle status and add closure reason

ALTER TABLE inventory_reservations
    DROP CONSTRAINT chk_inventory_reservations_status,
    ADD COLUMN close_reason TEXT NULL;

UPDATE inventory_reservations
SET
    status = CASE
        WHEN released_qty + consumed_qty = total_qty THEN 'closed'
        ELSE 'active'
    END,
    close_reason = CASE
        WHEN released_qty + consumed_qty < total_qty THEN NULL
        WHEN consumed_qty = total_qty THEN 'consumed'
        WHEN released_qty = total_qty THEN 'released'
        ELSE 'mixed'
    END;

ALTER TABLE inventory_reservations
    ADD CONSTRAINT chk_inventory_reservations_status
        CHECK (status IN ('active', 'closed')),
    ADD CONSTRAINT chk_inventory_reservations_close_reason
        CHECK (close_reason IS NULL OR close_reason IN ('consumed', 'released', 'mixed', 'cancelled', 'expired')),
    ADD CONSTRAINT chk_inventory_reservations_status_matches_close_reason
        CHECK (
            (status = 'active' AND close_reason IS NULL)
            OR
            (status = 'closed' AND close_reason IS NOT NULL)
        ),
    ADD CONSTRAINT chk_inventory_reservations_status_matches_quantities
        CHECK (
            (status = 'active' AND released_qty + consumed_qty < total_qty)
            OR
            (status = 'closed' AND released_qty + consumed_qty = total_qty)
        );

COMMENT ON COLUMN inventory_reservations.status IS
    'Reservation lifecycle status: active while quantity remains, closed when fully consumed or released';
COMMENT ON COLUMN inventory_reservations.close_reason IS
    'Closure reason: consumed, released, mixed, cancelled, or expired';
