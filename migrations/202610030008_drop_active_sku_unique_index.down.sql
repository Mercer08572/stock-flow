-- Migration metadata
-- status: applied
-- description: restore one-active-sku-per-material unique index

-- 回滚前必须保证数据满足「每个物料最多一条启用且未删除的 SKU」，
-- 否则重建唯一索引会失败（这正是该约束的语义，属于预期行为）。
CREATE UNIQUE INDEX IF NOT EXISTS ux_skus_active_material_current_stage
    ON skus (material_id)
    WHERE deleted_at IS NULL AND status = 'active';

COMMENT ON COLUMN skus.material_id IS 'Material id. Current stage allows at most one active SKU per material';
