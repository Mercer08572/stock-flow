# WH-003 Close WH-002

Status: Done
Owner: coding-agent
Module: warehouse
Related:
- tasks/warehouse/WH-002-data-consistency-risk.md
- internal/warehouse/integration_test.go
- internal/inventory/

## Background

WH-002 唯一未完成的验收项是「数据库集成测试覆盖允许删除、拒绝删除、允许禁用和历史查询」，
该项依赖 DB-002 的集成测试基础设施。

## Goal

用真实 PostgreSQL 验证仓库与库存引用的交互：

- 无库存引用：允许软删除。
- 有库存引用：拒绝删除，返回 `WAREHOUSE_REFERENCED_BY_INVENTORY`。
- 有库存引用：允许停用。
- 停用仓库：仍可查询历史库存。

## Acceptance Criteria

- [x] 集成测试 `TestWarehouseInventoryReferenceProtection` 通过。
- [x] WH-002 的遗留验收项已闭环。
- [x] `make test-integration` 通过。
