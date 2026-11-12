# INV-007 Movement Trace ID

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/
- migrations/202611120012_add_inventory_movement_trace_id.up.sql
- sql/queries/inventory_operations.sql
- sql/schema/schema.sql
- internal/audit/

## Background

API 密钥管理上线后，库存流水已经通过 `actor_type/actor_id/actor_label` 记录了调用方
（管理员或 API 应用），但没有记录具体请求。审计日志按请求记录了 `app_id/secret_id/IP/trace_id`，
两者缺少稳定的关联键。本任务给库存流水增加 `trace_id`，让每条流水都能一跳关联到审计日志，
同时不把 `secret_id` 写入库存业务表。

## Goal

- `inventory_movements` 新增 nullable `trace_id`。
- 所有新的库存变更流水写入当前请求的 trace_id。
- 库存流水 API 返回 `trace_id`。
- 审计日志查询支持按 `trace_id` 精确过滤。
- 管理端流水列表可以点击 trace_id 打开对应审计日志。

## Non-Goals

- 不给 `inventory_reservations` 增加 trace_id（预留单的 `reserve` 流水已带 trace）。
- 不把 `secret_id`、`app_id` 写入库存流水表。
- 不按 trace_id 反查流水（本次只做流水 → 审计）。
- 不回填历史流水；历史行 `trace_id` 为 NULL。

## Domain Rules

- `trace_id` 是请求关联快照，不做外键；审计日志按保留期清理后 trace 仍保留。
- 一个请求可能产生多条流水，它们共享同一个 trace_id。
- 幂等重放不产生新流水，原流水保留第一次真正改变库存的请求的 trace_id。
- 历史数据不回填，API 返回 `null`。

## API

- `GET /api/v1/inventory/movements`：每个 item 增加 `trace_id`（string 或 null）。
- `GET /api/v1/audit/logs`：新增可选 `trace_id` 查询参数，精确匹配。
  - 当只传 `trace_id`、不传 `from`/`to` 时，后端默认查询整个保留期（最多 366 天），
    而不是默认的最近 24 小时。

## Data Model

- `inventory_movements.trace_id TEXT NULL`
- 部分索引：`(trace_id) WHERE trace_id IS NOT NULL AND deleted_at IS NULL`
- 不新增外键，不修改其他表。

## Implementation Notes

- `OperationContext` 增加 `TraceID`，由 `operationContextFromGin` 从 `TraceID` 中间件读取。
- 5 个库存变更路径（increase/reserve/release/decrease_available/decrease_reserved）的
  `MovementInput` 都传入 trace_id。
- Repository 写入 `trace_id`，列表查询映射回 `Movement.TraceID`。
- 审计查询增加 `trace_id` 条件；`trace_id` 走已有索引 `idx_audit_logs_trace`。

## Acceptance Criteria

- 用 `X-Trace-ID: req_demo` 调用库存变更接口，新流水返回 `trace_id=req_demo`。
- 不传 `X-Trace-ID` 时由中间件生成 trace，流水也有值。
- 历史流水返回 `trace_id=null`，接口不报错。
- `GET /api/v1/audit/logs?trace_id=req_demo` 返回对应请求记录。
- `go test ./...`、`go vet ./...` 通过；迁移 DDL 在开发库事务中验证通过。

## Verification

- `make sqlc`
- `make swagger`
- `go test ./...`
- `go vet ./...`
- 有 `TEST_DATABASE_URL` 时：`make test-integration`

## Open Questions

None.
