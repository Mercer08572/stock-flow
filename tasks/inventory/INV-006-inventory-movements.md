# INV-006 Inventory Movements Query

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/handler_operations.go
- internal/inventory/operations.go
- sql/queries/inventory_operations.sql

## Background

每次库存变更都会写入 `inventory_movements`。需要只读查询接口用于审计与排障。

## Goal

`GET /api/v1/inventory/movements` 支持按仓库、SKU、操作类型、时间范围过滤与分页。

## Non-Goals

- 不提供修改或删除流水的接口。
- 不提供导出功能。

## Domain Rules

- 流水只读；不得通过更新流水修正库存。
- `operation_type` 仅允许 `increase`、`reserve`、`release_reserved`、`decrease_available`、`decrease_reserved`。
- `created_from` / `created_to` 使用 RFC3339；`created_to` 为开区间。
- 默认 `limit=20`，最大 100。
- 返回 actor 字段以支持审计。

## Acceptance Criteria

- [x] 支持仓库、SKU、操作类型、时间范围过滤。
- [x] 分页参数归一化。
- [x] 集成测试 `TestListMovementsFiltersAndPagination` 通过。
- [x] OpenAPI 已重新生成。
