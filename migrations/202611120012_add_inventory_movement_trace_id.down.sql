DROP INDEX IF EXISTS idx_inventory_movements_trace_id;

ALTER TABLE inventory_movements
    DROP COLUMN IF EXISTS trace_id;
