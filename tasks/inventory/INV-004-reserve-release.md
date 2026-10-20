# INV-004 Reserve and Release

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/operations.go
- internal/inventory/handler_operations.go
- sql/queries/inventory_operations.sql
- migrations/202608030007_simplify_inventory_reservation_status.up.sql

## Background

预留库存用于为外部业务操作锁定可用量；释放用于归还尚未消耗的预留。

## Goal

- `POST /api/v1/inventory/reserve` 创建预留单与 FIFO 分配明细。
- `POST /api/v1/inventory/release` 按原始预留明细释放，支持部分释放与全部释放。

## Domain Rules

- 预留只增加 `reserved_qty`，不改变 `on_hand_qty`。
- 预留不得超过可用量；并发预留不得超卖。
- 预留单状态遵循 `active | closed` + `close_reason`：
  - 全部释放 → `closed` + `released`
  - 部分释放后全部消耗 → `closed` + `mixed`
- 释放必须引用 `reservation_id`，使用原始明细，不重新计算分配顺序。
- 释放数量不得超过预留剩余量。
- 每次预留/释放按涉及库存层写流水，并记录 actor。

## API

`POST /api/v1/inventory/reserve`
请求字段：`warehouse_id`、`sku_id`、`batch_id?`、`qty`、`idempotency_key`、来源字段。

`POST /api/v1/inventory/release`
请求字段：`warehouse_id`、`sku_id`、`reservation_id`、`qty?`（省略表示释放全部剩余）、`idempotency_key`、来源字段。

## Acceptance Criteria

- [x] 预留后仅 reserved 增加。
- [x] 并发预留不超卖（`TestConcurrentReserveDoesNotOversell`）。
- [x] 部分释放、全部释放、超量释放被正确处理。
- [x] 预留单 actor 落库（`TestReservationRecordsActor`）。
- [x] 状态与 `close_reason` 符合迁移 0007。
