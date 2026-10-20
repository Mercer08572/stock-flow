-- Migration metadata
-- status: pending
-- description: drop actor audit fields from inventory movements and reservations

DROP INDEX IF EXISTS idx_inventory_reservations_actor;

ALTER TABLE inventory_reservations
    DROP CONSTRAINT IF EXISTS chk_inventory_reservations_actor_complete,
    DROP CONSTRAINT IF EXISTS chk_inventory_reservations_actor_type,
    DROP COLUMN IF EXISTS actor_label,
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS actor_type;

DROP INDEX IF EXISTS idx_inventory_movements_actor;

ALTER TABLE inventory_movements
    DROP CONSTRAINT IF EXISTS chk_inventory_movements_actor_complete,
    DROP CONSTRAINT IF EXISTS chk_inventory_movements_actor_type,
    DROP COLUMN IF EXISTS actor_label,
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS actor_type;
