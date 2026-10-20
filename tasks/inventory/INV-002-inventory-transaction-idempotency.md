# INV-002 Inventory Transaction and Idempotency

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/unit_of_work.go
- internal/inventory/idempotency.go
- internal/inventory/repository_operations.go
- migrations/202610200010_add_inventory_actor_audit.up.sql
- docs/architecture/transaction-rules.md

## Background

库存写操作必须原子执行，并通过幂等键避免外部系统重试导致重复扣减。
此前项目没有任何应用层事务代码，`inventory_idempotency_keys` 表也未被使用。

## Goal

- 在服务层建立事务边界，仓储只使用事务绑定的 Querier。
- 实现统一的幂等占位、重放与冲突检测。
- 把 `auth.Caller` 记录到库存流水和预留单，补齐审计身份。

## Non-Goals

- 不实现具体库存业务动作（由 INV-003 ~ INV-005 承接）。
- 不引入分布式事务或消息队列。

## Scope

允许修改：

- `internal/inventory/`
- `internal/shared/http/router.go`
- `pkg/apperr/codes.go`
- `sql/queries/`
- `migrations/`

## Domain Rules

- 事务必须由服务层开启/提交/回滚，仓储不得管理事务。
- 幂等键必填，长度 1–128。
- 相同 `operation_type + idempotency_key` 且请求哈希相同：返回首次结果。
- 相同键但请求哈希不同：409 `INVENTORY_IDEMPOTENCY_CONFLICT`。
- 幂等记录、库存更新、预留更新、流水必须在同一事务提交。
- 操作人写入 `inventory_movements` 与 `inventory_reservations` 的 `actor_type/actor_id/actor_label`。

## Data Model

新增迁移 `202610200010_add_inventory_actor_audit`：

- `inventory_movements.actor_type/actor_id/actor_label`
- `inventory_reservations.actor_type/actor_id/actor_label`
- 对应 actor 索引与 CHECK 约束

## Implementation Notes

- `UnitOfWork.WithinTx` 使用 `pgxpool.Begin`，并传入事务绑定的 `Repository`。
- 事务顺序：幂等占位 → 锁库存余额行 → 预留/库存层 → 更新 → 流水 → 幂等完成。
- 请求哈希使用规范化结构体序列化后 SHA-256；数量固定 6 位小数。

## Acceptance Criteria

- [x] 事务失败可完整回滚（集成测试 `TestTransactionRollsBackOnFailure`）。
- [x] 同幂等键并发重复提交只生效一次（集成测试覆盖重放）。
- [x] 同键不同载荷返回冲突。
- [x] 流水与预留单记录 actor。
- [x] `go test ./...` 与 `make test-integration` 通过。
