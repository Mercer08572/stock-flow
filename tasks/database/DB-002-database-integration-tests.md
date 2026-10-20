# DB-002 Database Integration Tests

Status: Done
Owner: coding-agent
Module: database
Related:
- internal/shared/testdb/
- internal/auth/integration_test.go
- migrations/
- Makefile
- .github/workflows/ci.yml

## Background

现有测试全部基于 fake/mock，SQL 正确性、表约束、迁移与事务回滚没有自动化验证。
库存写操作的正确性高度依赖 SQL、行锁、唯一索引与 CHECK 约束，因此必须先建立可重复运行的数据库集成测试基础设施。

计划文档 `docs/development-plan.md` 的 D3 原建议使用 testcontainers-go。
当前项目会提供一个专用的远程测试库，因此改为：

- 本地：远程固定测试库 + `TEST_DATABASE_URL`
- CI：继续使用 `.github/workflows/ci.yml` 中的 PostgreSQL service 容器
- 不引入 testcontainers-go，不要求本机 Docker

## Goal

提供一套 `//go:build integration` 隔离的测试辅助包，使任何测试都可以：

- 连接专用测试库；
- 执行 `migrations/` 全量迁移；
- 在测试之间安全清理数据；
- 在未配置测试库时自动跳过，不影响 `go test ./...`。

## Non-Goals

- 不实现库存写操作本身（由 INV-002 ~ INV-005 承接）。
- 不引入 testcontainers-go。
- 不让集成测试读取开发用 `.env` 或 `DATABASE_URL`。
- 不修改现有 migration 的内容。

## Scope

允许修改或新增：

- `internal/shared/testdb/`
- `internal/inventory/integration_test.go`（示范测试）
- `internal/auth/integration_test.go`（可选复用辅助包）
- `Makefile`
- `.github/workflows/ci.yml`
- `docs/development.md`

不应修改：

- `internal/inventory/` 的生产代码
- 现有 migration 文件
- 前端项目

## Domain Rules

- 测试库 URL 从 `TEST_DATABASE_URL` 读取；文件位置为 `stock-flow/.env.test`（gitignored）。
- 已导出的环境变量优先于 `.env.test`。
- 未设置 URL 或缺少 `migrate` CLI 时，集成测试必须 `t.Skip`。
- 测试库必须是专用、可清空的库；必须启用安全闸门：
  库名不含 `test` 时，要求 `TEST_DATABASE_ALLOW_DESTRUCTIVE=1`。
- 测试进程必须把 `ENV_FILE` 指向空文件，避免 `config.Load` 读取开发用 `.env`。
- 迁移通过 `migrate -path migrations -database $TEST_DATABASE_URL up` 执行。
- 清理数据使用 `TRUNCATE ... RESTART IDENTITY CASCADE`，不清空 `schema_migrations`。
- 多个测试包不得同时对同一个测试库做迁移/清理；运行命令固定加 `-p 1`。

## API

无 HTTP API。测试辅助包导出：

- `testdb.Require(t) *DB`
- `(*DB).MigrateUp(t)`
- `(*DB).ResetSchema(t)`
- `(*DB).Truncate(t, tables ...string)`
- `testdb.DefaultTables`

## Data Model

不新增表、不修改迁移。测试库通过现有 `migrations/` 全量迁移创建。

## Implementation Notes

- 辅助包使用 `//go:build integration` 标签。
- 从当前包目录向上查找 `go.mod` 定位仓库根目录。
- 使用 `godotenv.Load(<root>/.env.test)` 加载测试配置，不覆盖已导出的环境变量。
- 通过 `pgxpool` 连接并 Ping。
- 使用 PostgreSQL advisory lock 串行化迁移与清理，降低多进程并发风险。
- 错误输出必须隐藏数据库 URL 中的凭据。
- 示例测试至少覆盖：迁移可执行、一次简单写入/读取、清理可重复运行。

## Acceptance Criteria

- `TEST_DATABASE_URL` 未设置时，`go test -tags=integration ./...` 全部跳过且不失败。
- 设置 `.env.test` 后，`make test-integration` 可重复运行。
- 集成测试可以执行迁移并清理数据。
- `go test ./...` 不编译集成测试辅助包。
- CI 的 integration job 使用 `-p 1 -count=1`，并继续通过 service container 运行。
- `docs/development.md` 说明 `.env.test` 的位置与运行方式。

## Open Questions

None.
