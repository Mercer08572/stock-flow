-- Migration metadata
-- status: applied
-- description: drop dispatch priority from inventory batches

-- 回滚会丢弃所有已设置的批次优先级（列被删除），这是预期行为。
ALTER TABLE inventory_batches
    DROP CONSTRAINT IF EXISTS chk_inventory_batches_dispatch_priority_non_negative,
    DROP COLUMN IF EXISTS dispatch_priority;
