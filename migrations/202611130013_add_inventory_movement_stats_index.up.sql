-- 迁移元数据
-- status: pending
-- description: 为库存流水的按时间聚合统计补充 created_at 索引

-- 工作台的趋势统计要按 created_at 范围扫描全仓数据，再按 operation_type 分组。
-- 现有的 idx_inventory_movements_warehouse_sku_created_at 最左列是 warehouse_id，
-- 不带仓库条件的全仓查询用不上它，因此单独补一个时间索引。
CREATE INDEX idx_inventory_movements_created_at
    ON inventory_movements (created_at)
    WHERE deleted_at IS NULL;

COMMENT ON INDEX idx_inventory_movements_created_at IS
    '库存流水按时间范围聚合统计使用；支撑工作台趋势图的 created_at 范围扫描。';
