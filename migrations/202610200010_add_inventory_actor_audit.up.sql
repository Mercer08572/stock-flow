-- Migration metadata
-- status: applied
-- description: add actor audit fields to inventory movements and reservations

ALTER TABLE inventory_movements
    ADD COLUMN actor_type  TEXT   NULL,
    ADD COLUMN actor_id    BIGINT NULL,
    ADD COLUMN actor_label TEXT   NULL,
    ADD CONSTRAINT chk_inventory_movements_actor_type
        CHECK (actor_type IS NULL OR actor_type IN ('admin', 'api_app')),
    ADD CONSTRAINT chk_inventory_movements_actor_complete
        CHECK (
            (actor_type IS NULL AND actor_id IS NULL AND actor_label IS NULL)
            OR (actor_type IS NOT NULL AND actor_id IS NOT NULL AND actor_label IS NOT NULL)
        );

CREATE INDEX idx_inventory_movements_actor
    ON inventory_movements (actor_type, actor_id, created_at)
    WHERE deleted_at IS NULL;

ALTER TABLE inventory_reservations
    ADD COLUMN actor_type  TEXT   NULL,
    ADD COLUMN actor_id    BIGINT NULL,
    ADD COLUMN actor_label TEXT   NULL,
    ADD CONSTRAINT chk_inventory_reservations_actor_type
        CHECK (actor_type IS NULL OR actor_type IN ('admin', 'api_app')),
    ADD CONSTRAINT chk_inventory_reservations_actor_complete
        CHECK (
            (actor_type IS NULL AND actor_id IS NULL AND actor_label IS NULL)
            OR (actor_type IS NOT NULL AND actor_id IS NOT NULL AND actor_label IS NOT NULL)
        );

CREATE INDEX idx_inventory_reservations_actor
    ON inventory_reservations (actor_type, actor_id, created_at)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN inventory_movements.actor_type IS
    'Caller type for the mutation: admin or api_app';
COMMENT ON COLUMN inventory_movements.actor_id IS
    'Admin user id or API application id';
COMMENT ON COLUMN inventory_movements.actor_label IS
    'Human readable actor label: admin username or API application app_id';
COMMENT ON COLUMN inventory_reservations.actor_type IS
    'Caller type that created the reservation: admin or api_app';
COMMENT ON COLUMN inventory_reservations.actor_id IS
    'Admin user id or API application id';
COMMENT ON COLUMN inventory_reservations.actor_label IS
    'Human readable actor label: admin username or API application app_id';
