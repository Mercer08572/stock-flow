# AU-003 Admin Bootstrap

Status: Done
Owner: coding-agent
Module: auth
Related:
- migrations/202607130005_create_auth_tables.up.sql
- migrations/202607140006_add_admin_login_safety_fields.up.sql
- internal/auth/
- cmd/admin/
- sql/queries/auth.sql
- internal/auth/integration_test.go
- .github/workflows/ci.yml

## Background

管理员种子行在 `a3fcea1` 被从 `202607130005_create_auth_tables.up.sql` 中删除，而 `cmd/admin init`
只执行 `UPDATE ... WHERE password_initialized = FALSE`，没有 INSERT 路径。结果是：

- 任何全新数据库执行 `make migrate-up` 后 `admin_users` 表为空；
- `stock-flow-admin init` 必然返回 `admin user not found`；
- 新环境无法完成「建库 → 迁移 → 创建管理员 → 登录」流程（计划文档 M1 阻断项、风险 R1）。

同时「重复执行初始化」的语义不正确：旧查询把「已初始化」也归入 0 行，于是仓库层统一返回
`ErrAdminNotFound`，操作者看到的是"账号不存在"，而不是"已初始化、拒绝覆盖"。

## Goal

- 空库迁移后存在一个可被 `stock-flow-admin init` 初始化的引导管理员。
- 引导态口令不可登录，且不引入任何可复用的默认弱口令。
- 初始化命令可重复执行时给出明确错误，绝不覆盖已有口令。
- 用单元测试与数据库集成测试锁定以上行为。

## Non-Goals

- 不做 RBAC、多管理员角色或管理员管理界面。
- 不做口令找回、邮件/SMS 重置。
- 不引入默认弱口令、固定可用口令或口令明文。
- 不改动登录、改密、会话与限流业务规则（属于 AU-002）。
- 不为已应用旧迁移的数据库提供回填迁移：本次经用户确认，种子集中保留在 `202607130005`，
  旧库若缺该行需手工 INSERT 或重建库（见 `migrations/AGENTS.md`「唯一例外」）。

## Scope

允许修改：

- `migrations/202607130005_create_auth_tables.up.sql`（**用户显式批准的例外**：仅追加引导 INSERT，不改 DDL）
- `sql/queries/auth.sql`
- `internal/auth/`
- `cmd/admin/`
- `.github/workflows/ci.yml`（新增集成测试 job）
- 认证相关测试

不应修改：

- 其它已应用迁移
- `internal/` 下与认证无关的模块
- `pkg/response` 的统一响应格式
- 登录 / 改密 / 会话 / 限流的既有行为

## Domain Rules

- `admin_users.password_hash` 必须满足 `chk_admin_users_password_hash_argon2id`，因此占位值必须是
  格式合法的 Argon2id 编码；但它对应的口令必须是**未被记录的随机值**，任何人（包括作者）都无法用它登录。
- 引导态由 `password_initialized = FALSE` 表达，该值由 `202607140006` 的列默认值提供，
  `202607130005` 不显式写入（那时列还不存在）。
- 只要 `password_initialized = FALSE`，登录必须失败，且失败信息与"用户名不存在"无法区分。
- 初始化只能作用于 `password_initialized = FALSE` 的账号；已初始化账号返回
  `ErrAdminAlreadyInitialized`，且**存储的哈希保持原样**。
- 初始化成功后 `must_change_password` 必须为 `TRUE`，首次登录强制改密。
- 该写操作必须在单条语句内完成"判定 + 可能更新"，避免并发下的重复初始化。

## Implementation Notes

- 占位哈希：`$argon2id$v=19$m=65536,t=3,p=2$kh1tRCiets3++0uR69wK7Q$P3YXeq+qkOyBuldryA72SBa8YkKFtvPM/5B1K1j6uQU`
  （salt 是公开的占位标签哈希，口令是随机 32 字节且未记录）。
- `InitializeAdminPassword` 改为 CTE：`target`（`SELECT ... FOR UPDATE`）+ `updated`
  （仅当 `NOT password_initialized` 时更新）。返回行表示账号存在；第二个返回值必须表达
  **"本次调用是否真的写入了口令"**（列名 `updated`），而不是"账号当前是否已初始化"：
  - 无返回行（`pgx.ErrNoRows`）→ `ErrAdminNotFound`
  - 返回行且 `updated = false` → `ErrAdminAlreadyInitialized`（本次未写入任何数据）
  - 返回行且 `updated = true` → 成功
- **踩过的坑（务必保留这条约束）**：最初的实现返回
  `(target.password_initialized OR EXISTS (SELECT 1 FROM updated))`，而这个表达式在
  「写入成功」与「已初始化、未写入」两种情形下**都是 `TRUE`**，导致 `cmd/admin init`
  在任何新库上第一次执行就被判成"已初始化"（CI 报
  `admin password is already initialized`）。SQL 的写入逻辑本身是对的，错的是返回的结果契约：
  两个互斥结果共用一个布尔值，调用方无法区分。修复方式就是改为只返回 `updated`。
  这一条必须保留，避免后人"简化"回原来的写法。
- 仓库层映射 PG 错误，服务层保持业务规则，CLI 只负责把错误翻译成可执行提示。
- CLI 拆出 `runWithDeps(args, stdin, stdout, serviceFactory)` 以便在不连库的情况下测试参数、
  口令读取与错误文案；`main()` 行为不变。
- 集成测试必须由 `TEST_DATABASE_URL` 显式指定一次性数据库，**禁止回退到 `DATABASE_URL` 或 `.env`**，
  因为它会 drop 整个 schema；通过 `ENV_FILE` 指向空文件隔离配置文件。
- 集成测试用 `migrate` CLI 执行 `migrations/`，与应用部署使用同一套迁移工具。

## Acceptance Criteria

- [x] 空库执行全量迁移后 `admin_users` 恰好有 1 行 `username = 'admin'`，`password_initialized = FALSE`，
      `must_change_password = TRUE`（集成测试断言，已由 CI 真库执行通过）
- [x] 占位哈希不能被任意口令验证通过（集成测试断言，已由 CI 真库执行通过）
- [x] 首次 `InitializeAdminPassword` 成功，口令可用新哈希验证通过（集成测试断言，已由 CI 真库执行通过）
- [x] 第二次执行返回 `ErrAdminAlreadyInitialized`，且存储哈希不变（单元测试 + 集成测试）
- [x] 未知用户名返回 `ErrAdminNotFound`，CLI 提示先执行迁移（单元测试）
- [x] CLI 成功路径打印 `administrator "admin" initialized; ...`（单元测试）
- [x] 错误信息不回显口令（单元测试）
- [x] `gofmt -l .`、`go vet ./...`、`go build ./...`、`go test ./...` 通过（本地实测）
- [x] CI 新增 postgres service 的集成测试 job（2026-09-24 核查：run `35815838642` 的 `integration` job success）
- [x] 任务文档与 `migrations/AGENTS.md` 记录本次对已应用迁移的例外

## 验证记录（2026-09-16）

```bash
cd stock-flow
export GOCACHE=/tmp/agent-cache/go-build GOTMPDIR=/tmp/agent-cache/go-tmp \
       GOMODCACHE=/Users/badbugu/workspace/Environment/go/pkg/mod
gofmt -l . && go vet ./... && go build ./... && go test ./...
make sqlc          # 无额外差异
go test -tags=integration ./internal/auth/...   # 无 TEST_DATABASE_URL 时自动 SKIP
```

- 本机无 Docker、无本地 PostgreSQL（`.env` 指向 `localhost:5432`，不通），
  因此 `TEST_DATABASE_URL` 路径**未在本地执行**，由 CI 的 `integration` job 覆盖。
- 空库全流程演练（迁移 → init → 登录 → 强制改密）同样依赖可用数据库，本地未执行。

### CI 复跑确认（2026-09-24 核查）

- `stock-flow` 的 GitHub Actions run **#2**（`3a1fcab`，2026-09-23）结论 **success**，
  其中 `integration` job 在 `postgres:16-alpine` service 容器上执行：`migrate` CLI 安装 → 空库全量迁移 →
  `go test -tags=integration -count=1 ./...`，全部步骤成功。因此上文第一条「由 CI 覆盖」的验收项
  已获得**真库证据**（本机仍未执行，因为本机无 PostgreSQL 与 Docker）。
- 同一次 run 的 `quality` job（gofmt / vet / build / test）同样 success。

### 覆盖缺口（2026-09-24 核查，如实标注）

- 集成测试的边界是「迁移 → 初始化」：它断言了种子行、占位哈希不可登录、初始化成功、
  重复执行被拒且不覆盖、未知用户报 `ErrAdminNotFound`。
- **未覆盖**：P1-1 验收项 ② 的后半段——用初始化后的口令**登录成功**、会话被要求强制改密、
  改密后放行、旧会话失效。这一链路目前只有 `service_test.go` / `middleware_test.go` 用
  fake repository 覆盖，没有真库端到端断言。
- 2026-09-24 已提出补测方案（在 `integration_test.go` 末尾续接 `Login` → `ChangePassword` →
  旧 token 失效 → 新口令再登录），**经用户决定暂不做**，留给 P3-1（数据库集成测试基础设施）一并处理。

### 首轮 CI 事故与修复（2026-09-17）

- **现象**：后端首次 CI 的 `integration` job 在
  `integration_test.go:84 initialize administrator password: admin password is already initialized`
  失败；`quality` job（普通单测）全绿。
- **根因**：见 Implementation Notes 中的"踩过的坑"——返回值把两种互斥结果塌缩为同一个布尔值。
  真正执行写入的 `updated` CTE 逻辑没有错，SQL 在真库上的写入/跳过行为均正确。
- **为什么单测没拦住**：`fakeRepository` 按"正确意图"实现（已初始化才报错），与真 SQL 的
  错误信号不一致；该不一致恰好只能由真库暴露。这正是 P3-1 集成测试基础设施的价值所在。
- **修复验证（本机，无 Docker 环境下的替代验证）**：用 PGlite（PostgreSQL 16 的 WASM 构建，
  与 CI 的 `postgres:16-alpine` 同大版本）跑项目**真实 migrations**，并直接从
  `internal/auth/db/auth.sql.go` 抽取生成代码里逐字符相同的 SQL：

  | 场景 | 修复前返回 | 修复后返回 | 判定 |
  | --- | --- | --- | --- |
  | 全新库首次初始化 | `password_initialized = true` | `updated = true` | 写入成功 |
  | 已初始化后重复执行 | `password_initialized = true`（与上一行相同，无法区分） | `updated = false` | 已初始化，拒绝且不覆盖 |
  | 账号不存在 | 0 行 | 0 行 | `ErrAdminNotFound` |

  另用 pglite 复现了修复前的歧义（两种情形返回值完全相同），确认这不是环境差异所致。
- **已确认（2026-09-24 核查）**：修复后的真库集成测试已由 CI 复跑通过——
  run `35815838642`（`3a1fcab`，2026-09-23）的 `integration` job **success**，
  失败日志中的 `initialize administrator password: admin password is already initialized` 已不再出现。

## Open Questions

- 已应用旧 `202607130005` 的部署（本地开发库）需要手工补齐管理员行；是否需要为它们补一个
  显式回填迁移，取决于是否还存在需要保留数据的老库。
