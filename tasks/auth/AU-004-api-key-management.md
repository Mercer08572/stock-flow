# AU-004 API Key Management

Status: Done
Owner: coding-agent
Module: auth
Related:
- internal/auth/
- sql/queries/auth.sql
- sqlc.yaml
- openapi/
- pkg/apperr/codes.go

## Background

Stock-Flow 已经支持「API 应用 + 密钥」两级认证：外部系统用 `X-Stock-Flow-App-ID` 与
`X-Stock-Flow-Secret` 调用受保护接口。现有管理接口只能创建应用、签发密钥、列出密钥和禁用密钥，
缺少前端页面需要的「修改密钥」「启用密钥」「删除密钥」「重新签发」能力。

本次按「沿用应用 + 密钥两级模型、仅认证不做权限范围」的决策补齐密钥生命周期管理。
「再次查看明文」已被明确放弃：数据库中只保存 SHA-256 哈希，需要再次获取时改为重新签发，
旧密钥在同一事务内立即禁用。

## Goal

管理端能够完整管理 API 应用与密钥：

- 应用的列表、创建、更新、停用/启用、软删除。
- 密钥的列表、签发（明文只展示一次）、修改名称/过期时间、禁用/启用、软删除。
- 重新签发：生成新密钥并禁用旧密钥，明文只展示一次，且两步在同一事务内完成。

## Non-Goals

- 不做明文可逆存储，不提供「再次查看」。
- 不做 scopes / RBAC / IP 白名单。
- 不做密钥管理操作审计日志。
- 不修改已有 migration 或数据库结构。
- 不修改对外鉴权请求头与统一响应格式。

## Domain Rules

- 应用状态为 `active` / `inactive`；密钥状态为 `active` / `blocked`。
- 停用应用后，其下所有密钥都不能通过认证；删除应用为软删除，效果相同。
- 密钥明文只在签发/重新签发的响应中出现一次；数据库只保存 `SHA-256` 哈希。
- 修改密钥只允许改 `name` 与 `expires_at`，不改变状态，不覆盖 `bound_metadata`。
- 删除密钥为软删除，同时置为 `blocked`；删除后不再出现在列表，也不能认证。
- 重新签发要求旧密钥处于 `active` 且未删除；新密钥继承旧密钥的 `bound_metadata`。
- 重新签发在单个事务中执行：`SELECT ... FOR UPDATE` 旧行 → 插入新行 → 禁用旧行。
  并发第二次重新签发返回 `409 AUTH_API_SECRET_NOT_ACTIVE`，不会产生两把启用密钥。
- `expires_at` 只允许未来时间或 `null`（永不过期）。

## API

- `PUT /api/v1/auth/apps/{id}/secrets/{secret_id}`
  - 请求：`{ "name": string, "expires_at": string|null }`
  - 响应：`APISecret`
- `DELETE /api/v1/auth/apps/{id}/secrets/{secret_id}`
  - 响应：统一成功封装，`data=null`
- `POST /api/v1/auth/apps/{id}/secrets/{secret_id}/unblock`
  - 响应：`APISecret`
- `POST /api/v1/auth/apps/{id}/secrets/{secret_id}/reissue`
  - 请求：`{ "name": string, "expires_at": string|null }`
  - 响应：`IssuedSecret`（含一次性明文 `secret`）

所有接口都要求管理员会话，沿用 `AdminSession` 中间件。

## Data Model

不新增 migration。使用现有字段：

- `api_secrets.name`
- `api_secrets.expires_at`
- `api_secrets.status`
- `api_secrets.deleted_at`

新增 sqlc query：

- `GetAPISecretForReissue`
- `UpdateAPISecret`
- `UnblockAPISecret`
- `SoftDeleteAPISecret`

## Implementation Notes

- 遵守 Handler → Service → Repository → PostgreSQL。
- Repository 只做 SQL、行映射与 PostgreSQL 错误映射；事务在 Repository 的重新签发方法内完成。
- 所有响应通过 `pkg/response`，错误通过 `internal/shared/httperr.Write`。
- 新增错误码 `AUTH_API_SECRET_NOT_ACTIVE`（409）。
- 不返回 `secret_hash`；`IssuedSecret.secret` 只在签发/重新签发响应中出现。

## Acceptance Criteria

- 管理员可以修改密钥名称与过期时间，状态和 `bound_metadata` 不变。
- 管理员可以禁用密钥、再启用；禁用期间认证失败，启用后恢复。
- 管理员可以软删除密钥；删除后列表不可见且认证失败。
- 管理员可以重新签发密钥；旧密钥立即禁用，新密钥可用，明文只出现一次。
- 对已禁用/已删除密钥重新签发返回 404 或 409，不产生新密钥。
- 非管理员请求返回 401。
- `go test ./...` 通过。
- `make sqlc`、`make swagger` 生成物与实现一致。

## Verification

- `make sqlc`
- `make test`
- `make swagger`
- 有 `TEST_DATABASE_URL` 时：`make test-integration`

## Open Questions

None.
