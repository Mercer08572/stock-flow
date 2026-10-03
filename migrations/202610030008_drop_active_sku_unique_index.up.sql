-- Migration metadata
-- status: applied
-- description: allow multiple active skus per material

-- 放开「一个物料最多一条启用 SKU」。原有部分唯一索引是当前阶段的口径护栏，
-- 现在按新规则（SKU 单位必须与物料基础单位可公度）允许多条启用 SKU。
--
-- 注意：应用层 internal/sku/service.go 的同名预检必须在同一次发布中一并删除，
-- 否则并发下仍会被索引拒绝，表现为偶发的唯一约束冲突。
DROP INDEX IF EXISTS ux_skus_active_material_current_stage;

COMMENT ON COLUMN skus.material_id IS 'Material id. Current stage allows multiple active SKUs per material';
