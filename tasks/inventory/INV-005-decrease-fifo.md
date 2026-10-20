# INV-005 Decrease Stock and Allocation Order

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/operations.go
- sql/queries/inventory_operations.sql
- internal/inventory/AGENTS.md

## Background

扣减库存是出库的最终动作。未引用预留时只扣可用量；引用预留时按原始分配消耗预留。

## Goal

`POST /api/v1/inventory/decrease` 支持两种模式：

- 不带 `reservation_id`：扣减可用库存（`decrease_available`）。
- 带 `reservation_id`：消耗预留库存（`decrease_reserved`），同时减少 `on_hand_qty` 与 `reserved_qty`。

## Domain Rules

- 分配顺序固定为：目标批次优先（提供 `batch_id` 时），随后按批次 `dispatch_priority` 升序、`received_at`、`id`。
- 非批次层视为 `dispatch_priority = 10`。
- 目标批次数量不足时，剩余部分继续按上述顺序跨批次兜底（用户已确认）。
- 扣减可用库存不得吃掉 `reserved_qty`，因此必须满足 `on_hand_qty - reserved_qty >= take`。
- 消耗预留必须按原始预留明细，不重新计算 FIFO。
- 每次扣减按涉及库存层写流水并关联 `stock_layer_id`。
- 并发扣减通过余额行锁 + 条件更新防止超扣。

## Acceptance Criteria

- [x] 批次优先级与目标批次优先顺序正确（`TestAllocationOrderUsesDispatchPriorityAndTargetBatchFirst`）。
- [x] 可用扣减不消耗预留（`TestDecreaseAvailableDoesNotConsumeReserved`）。
- [x] 消耗预留同时减少 on_hand 与 reserved。
- [x] 并发扣减/预留不超扣、不超卖。
- [x] 余额与库存层合计保持一致。
