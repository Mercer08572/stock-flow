-- Migration metadata
-- status: applied
-- description: add dispatch priority to inventory batches

-- 出库优先级分带：0-9 = 提升（数值越小越先出），10 = 默认，11+ = 延后。
-- 全部相等时退化为纯 FIFO；非批次库存层（batch_id 为 NULL）在排序时按 10 参与。
ALTER TABLE inventory_batches
    ADD COLUMN dispatch_priority INTEGER NOT NULL DEFAULT 10;

ALTER TABLE inventory_batches
    ADD CONSTRAINT chk_inventory_batches_dispatch_priority_non_negative
        CHECK (dispatch_priority >= 0);

COMMENT ON COLUMN inventory_batches.dispatch_priority IS
    'Dispatch priority. Lower value is allocated first; 0-9 boosts, 10 is default, 11+ delays. Equal values fall back to FIFO by (received_at, id)';
