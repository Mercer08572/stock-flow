# DEV-001 凭据治理：明文数据库口令下线

Status: Done（文件层部分已被 [DEV-003](./DEV-003-dotenv-configuration.md) 取代；`DEV-003` 亦已完成。
唯一未闭环项为**远端数据库口令轮换**，需用户在数据库侧执行，见 Open Questions）

> **后续变更**：DEV-003 已把配置来源统一为 `.env`，并删除整个 `configs/` 目录。
> 因此本任务中「修改 `configs/development.example.yaml`」「保留 `configs/*.yaml`
> 的 gitignore 规则」等描述已不再反映当前实现——相关目标由 DEV-003 以更彻底的方式
> 达成（配置文件机制整体移除，凭据只能来自环境变量）。
> 本任务中**仍未关闭**的一项：远端数据库口令的轮换（见 Open Questions）。

Owner: coding-agent
Module: devops
Related:
- `configs/development.yaml`
- `configs/*.example.yaml`
- `.env.example`
- `internal/shared/config/config.go`
- `cmd/config/main.go`
- `README.md`

## Background

`configs/development.yaml` 是**未跟踪**的本地配置文件（`.gitignore` 已排除 `configs/*.yaml`），
其中 `database_url` 含明文远端数据库口令，指向公网可达、`sslmode=disable` 的主机：

```text
postgres://<user>:<明文口令>@<公网IP>:5432/stock_flow_test?sslmode=disable
```

已核实的事实：

- 该口令**从未进入 git 历史**（`git log --all -S '<口令>'` 无命中），因此无需重写历史。
- 该口令**只出现在这一个文件中**，未在仓库其它位置复用。
- 该文件随时可能被误提交（例如 `git add -f`、或 `.gitignore` 被改动），而一旦提交即为
  公网可达库的凭据泄露。

`internal/shared/config/config.go` 的加载顺序已经是「默认值 < 配置文件 < 环境变量」
（`applyConfigFile` 先于 `applyEnv`，见 `config.go:65-74`），且 `DATABASE_URL` 环境变量
已被 `applyEnv` 支持（`config.go:193-195`）。因此**不需要改动任何 Go 代码**，
只需让本地配置文件不再承载口令。

## Goal

让开发环境的数据库连接串完全由环境变量提供，配置文件中不残留任何明文口令，
同时保持现有配置加载优先级与 `make migrate-*` 工作流不变。

任务完成后：

- `configs/development.yaml` 不含任何明文口令。
- 本地开发者通过 `DATABASE_URL` 环境变量提供连接串。
- `.env.example` 文档化所有受支持的 `DATABASE_URL` / `AUTH_*` 等环境变量。
- 缺失 `DATABASE_URL` 时启动**明确失败**（而不是静默回落到某个地址）。

## Non-Goals

- **不轮换远端数据库口令**（需要用户在数据库侧操作，见 Open Questions）。
- 不修改 `internal/shared/config/config.go` 的加载逻辑或优先级。
- 不改动任何 migration 文件。
- 不引入 `godotenv` 之类的依赖来做 `.env` 自动加载。
- 不删除 `.gitignore` 中对 `configs/*.yaml` 的排除规则。

## Scope

允许修改：

- `configs/development.yaml`（移除口令）
- `configs/development.example.yaml`（补充环境变量说明注释）
- `.env.example`（新增）
- `README.md`（补充环境变量使用说明）

不应修改：

- `internal/` 下的任何 Go 源码
- `migrations/`
- 前端项目 `../stock-flow-admin/`

## Domain Rules

- 仓库中**不得存在任何明文凭据**，包括示例文件（示例只能用明显的假值）。
- 配置优先级保持：默认值 < 配置文件 < 环境变量。
- `DATABASE_URL` 缺失时 `config.Load()` 必须返回错误（现有 `validate()` 已保证），
  不允许提供指向真实主机的默认值。

## Implementation Notes

- 这是配置与文档变更，不涉及三层架构。
- 修改 `.gitignore` 的检查：`.env` 被忽略，`.env.example` **不在**忽略范围内，可正常入库。
- 验证 `DATABASE_URL` 确实覆盖配置文件：

```bash
cd stock-flow
export GOCACHE=/tmp/agent-cache/go-build GOMODCACHE=/tmp/agent-cache/go-mod GOTMPDIR=/tmp/agent-cache/go-tmp
DATABASE_URL='postgres://user:pass@localhost:5432/db?sslmode=disable' go run ./cmd/config -key database_url
```

## Acceptance Criteria

- [x] `configs/development.yaml` 不含任何明文口令（`grep -i password` 无命中）。
      **已由 DEV-003 以更强的方式满足**：整个 `configs/` 目录已删除，该文件不复存在。
- [x] 不设置 `DATABASE_URL` 时，`go run ./cmd/config -key database_url` 明确失败并提示
      `DATABASE_URL is required`。（2026-09-14 实测：退出码 1，输出 `load config: DATABASE_URL is required`）
- [ ] 设置 `DATABASE_URL` 后，`go run ./cmd/config -key database_url` 输出该值（**已实测通过**），
      且 `make migrate-version` 能通过环境变量正常工作（**未验证**：本机 5432 端口无 PostgreSQL 实例）。
- [x] `.env.example` 列出所有受支持的环境变量，值全部为占位假值。（`DATABASE_URL=postgres://postgres:postgres@localhost:5432/stock_flow_dev` 等）
- [x] `go build ./... && go vet ./... && go test ./...` 通过。（2026-09-14 实测全绿）
- [x] `README.md` 说明「配置文件不含凭据，口令走环境变量」。（README「凭据只通过环境变量提供」一节；本次修正了同文件中遗留的配置文件表述）

## Open Questions

1. **远端口令轮换未完成**：原口令曾以明文形式存在于本地磁盘（且该库对公网开放、
   关闭 TLS）。是否已在数据库侧轮换该口令，需用户确认与执行——本地文件清理
   **不能**替代轮换。
2. 按计划决策点 **D7**，该远端测试库建议迁移到内网或改用容器化本地库；
   在迁移完成前，应继续保证连接串只走环境变量。

## Follow-up（不在本任务范围）

移除配置文件的 `database_url` 后，`make migrate-*` / `make schema-dump` 在
**未设置 `DATABASE_URL`** 时会输出两条信息：

```text
load config: DATABASE_URL is required     <- 根因，清晰
error: failed to parse scheme from database URL: URL cannot be empty   <- 误导
```

原因是 `Makefile` 中各 migrate 目标没有 `set -e`：`go run ./cmd/config` 失败后
`db_url` 为空，脚本继续把空串交给 `migrate`，于是抛出第二条与真实原因无关的错误。
（退出码仍为非 0，**行为正确，只是信息噪声**。）

本任务按 Scope 约定**不修改 `Makefile`**（该改动属独立主题）。建议在后续任务中
为各 migrate 目标补一个解析失败即退出的守卫，例如：

```make
db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)" || { \
	echo "ERROR: 无法解析 database_url，请设置 DATABASE_URL（见 .env.example）。" >&2; exit 1; }; \
```

在此之前，使用方式为：`export DATABASE_URL='postgres://...'` 后再执行 make 目标。
