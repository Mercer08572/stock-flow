-- Migration metadata
-- status: pending
-- description: add request trace id to inventory movements

ALTER TABLE inventory_movements
    ADD COLUMN trace_id TEXT NULL;

CREATE INDEX idx_inventory_movements_trace_id
    ON inventory_movements (trace_id)
    WHERE trace_id IS NOT NULL
      AND deleted_at IS NULL;

COMMENT ON COLUMN inventory_movements.trace_id IS
    'Request trace id from the TraceID middleware; links the movement to audit_logs. Nullable for historical rows.';
