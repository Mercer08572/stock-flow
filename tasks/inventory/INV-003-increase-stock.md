# INV-003 Increase Stock

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/handler_operations.go
- internal/inventory/operations.go
- sql/queries/inventory_operations.sql

## Background

P3-3 需要第一个可用的库存写操作：增加库存，并写入不可变流水。

## Goal

`POST /api/v1/inventory/increase` 支持按 `warehouse + SKU + batch` 增加库存。

## Non-Goals

- 不实现预留、释放、扣减（INV-004 / INV-005）。
- 不实现批次 CRUD。

## Domain Rules

- `warehouse_id`、`sku_id` 必填；仓库必须启用且未删除；SKU 必须存在且未删除。
- `batch_id` 可选；提供时必须属于该 SKU、未删除、状态为 `active`。
- 数量为正的十进制字符串，最多 14 位整数、6 位小数。
- 每次增加创建一条新的库存层，`received_at = NOW()`。
- `on_hand_qty += qty`，`reserved_qty` 不变。
- 每次变更写一条 `increase` 流水并记录 actor。

## API

`POST /api/v1/inventory/increase`

请求字段：`warehouse_id`、`sku_id`、`batch_id?`、`qty`、`idempotency_key`、`source_type?`、`source_id?`、`source_line_id?`。

## Acceptance Criteria

- [x] 增加后余额、库存层、流水一致。
- [x] 幂等重放不重复增加。
- [x] 停用仓库、无效批次被拒绝。
- [x] 集成测试 `TestIncreaseStockWritesStockLayerMovementAndIsIdempotent` 通过。
