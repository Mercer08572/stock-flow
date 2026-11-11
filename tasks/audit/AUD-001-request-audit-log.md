# AUD-001 Request Audit Log

Status: Done
Owner: coding-agent
Module: audit
Related:
- internal/audit/
- internal/shared/http/router.go
- internal/shared/config/
- internal/shared/database/
- internal/shared/httperr/
- cmd/api/main.go
- migrations/202611100011_create_audit_logs.up.sql
- openapi/

## Background

Stock-Flow 是对外提供库存能力的 API 服务。需要记录每一次后端接口调用，能按时间、调用方、
IP、状态码查询，至少包含：认证方式、调用时间、调用 IP。生产环境尚未上线，按日均 100 万
请求以内设计；热数据保留默认 180 天，可通过配置调整到 365 天。

## Goal

- 每一个 `/api/v1/**` 请求（包括登录、失败认证、401/400、panic 500）都产生一条审计记录。
- 记录认证方式（管理员会话 / API 密钥 / 未认证 / 认证冲突）、调用方、IP、状态、耗时、路径、
  trace_id、错误码。
- 异步批量写入 PostgreSQL，不阻塞请求；使用独立连接池，避免审计写入抢占业务连接。
- 提供管理员查询接口和页面，支持时间范围 + 调用方 + IP + 状态码精确过滤，游标分页。
- 按月分区，自动创建未来分区并按保留期删除整月旧分区。

## Non-Goals

- 不记录请求体、响应体、Cookie 值、`X-Stock-Flow-Secret` 或密码。
- 不做全文检索、复杂聚合报表、异地容灾。
- 暂不做不可篡改的 WORM 存储；表为追加写，应用不提供更新/删除接口。
- 不引入 ClickHouse / Kafka / Elasticsearch。日均 100 万以内 PostgreSQL 足够。
- 不记录静态资源、`/api-docs` 之外的框架内部请求。

## Domain Rules

- `auth_method` 取值：`admin_session`、`api_app`、`none`、`ambiguous`。
  - Cookie + API 请求头同时存在时记为 `ambiguous`，与认证中间件的拒绝行为一致。
- 只有认证成功才写入 `actor_type` / `actor_id` / `actor_label` / `credential_id`。
  - 管理员：`actor_label` 为 username。
  - API 应用：`actor_label` 为 app_id，`credential_id` 为 secret_id。
- 失败请求同样记录，`actor_*` 为空，`error_code` 记录业务错误码（如 `AUTH_REQUIRED`）。
- `client_ip` 使用 `INET`；来自可信代理的 `X-Forwarded-For` 才被 Gin 采用。
- 审计记录只追加；保留策略删除整月分区，不做行级 DELETE。
- 写入失败不能影响业务响应；队列满时丢弃并计数/告警，不阻塞请求。

## API

`GET /api/v1/audit/logs`（管理员会话）

查询参数：

- `from` / `to`：RFC3339；默认最近 24 小时，最大跨度 366 天。
- `auth_method`：`admin_session` / `api_app` / `none` / `ambiguous`。
- `actor_type`：`admin` / `api_app`。
- `actor_id`：管理员 ID 或 API 应用 ID。
- `actor_label`：username 或 app_id，精确匹配。
- `client_ip`：精确 IP。
- `status_code`：100–599。
- `path`：精确路径。
- `limit`：1–200，默认 50。
- `cursor`：上一页返回的 `next_cursor`。

响应：

```json
{
  "items": [/* AuditLog */],
  "next_cursor": "opaque-or-omitted"
}
```

## Data Model

新增 migration：`202611100011_create_audit_logs`。

- `audit_logs` 按 `occurred_at` 月度 RANGE 分区。
- 基础字段：`id`（显式序列）、`created_at`、`updated_at`、`deleted_at`（审计行不更新/不软删）。
- 业务字段：`occurred_at`、`trace_id`、`method`、`path`、`route`、`status_code`、`latency_ms`、
  `auth_method`、`actor_type`、`actor_id`、`actor_label`、`credential_id`、`client_ip`、
  `user_agent`、`error_code`、`request_bytes`、`response_bytes`。
- 分区由 `internal/audit.PartitionManager` 在启动时创建当前及未来两个月，并每 6 小时维护；
  过期整月分区通过 `DROP TABLE` 删除。

## Implementation Notes

### 异步批量写入

- `audit.Writer` 持有有界 channel（默认 8192）和后台 goroutine。
- 请求路径只做 channel send；队列满等待 100 ms 后丢弃并计数，绝不无限阻塞业务请求。
- 后台每 500 ms 或累计 500 条触发一次 `InsertBatch`。
- Repository 使用 `pgx.CopyFrom` 单次写入整批，减少 round trip 与 commit/fsync 次数。
- 关闭时 drain 队列并 flush；写入失败记录日志与失败计数。

### 独立连接池

- `cmd/api` 为审计单独创建 `pgxpool`：`application_name=stock-flow-audit`、`MaxConns=4`、
  `MinConns=1`。
- 业务请求使用主连接池；审计批量写入不会耗尽主池。慢查询/故障时通过
  `pg_stat_activity` 也能按 application_name 区分两类连接。
- 查询接口使用主连接池，避免审计查询和审计写入互相竞争。

### 其他

- 审计表使用原生 `pgx`：写入必须用 `CopyFrom`，查询使用参数化动态 SQL（按实际过滤条件拼接占位符）。
  因此没有加入 `sql/schema/schema.sql` / `sqlc.yaml`；分区表 DDL 由 migration 与 PartitionManager 维护。
- `httperr.Write` 把业务错误码写入 gin context，审计中间件据此记录 `error_code`。
- 审计中间件位于 TraceID 之后、Recovery 之外，保证 401/400/500 都有记录。
- `HTTP_TRUSTED_PROXIES` 未配置时只记录直连对端 IP；部署在反向代理后必须配置。

## Acceptance Criteria

- 启动时自动创建 `audit_logs` 当前及未来两个月分区；migration 未执行时启动失败并给出明确错误。
- 每个 API 请求产生一条记录，包含认证方式、时间、IP、状态码、耗时、trace_id。
- 管理员会话、API 密钥、未认证、认证冲突四种方式均能正确区分。
- 401/400/500（含 panic）也会被记录。
- 查询接口支持时间、调用方、IP、状态码过滤和游标分页。
- 审计写入使用独立连接池和批量 `CopyFrom`，不阻塞请求。
- 默认保留 180 天，`AUDIT_RETENTION_DAYS` 可配置到 365/3650。
- 单元测试覆盖 writer 批量/关闭 flush、中间件认证方式/错误码/panic、service 过滤与游标、
  handler 参数解析、分区命名。
- `go test ./...`、`go vet ./...` 通过。

## Verification

- `make migrate-up`（在目标库执行新 migration）
- `make test`
- `make swagger`
- 手工：登录管理端调用任一接口，再打开「系统设置 → 审计日志」确认记录；
  用 API 密钥调用同一接口，确认 `auth_method=api_app`；
  同时带 Cookie 与 API 头，确认 `auth_method=ambiguous` 且 `error_code=AUTH_AMBIGUOUS_METHOD`。

## Open Questions

None.
