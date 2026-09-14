# DEV-003 配置来源统一为 .env（移除 configs/ 多环境模板）

Status: Done
Owner: coding-agent
Module: devops
Related:
- `internal/shared/config/config.go`
- `internal/shared/config/config_test.go`
- `cmd/config/main.go`
- `.env.example`
- `.gitignore`
- `README.md`
- `docs/development.md`
- 删除：`configs/`

## Background

DEV-001 已经把数据库口令从 `configs/development.yaml` 中移除，口令改由
`DATABASE_URL` 环境变量提供。但配置文件机制本身仍然保留，于是出现了**两套并行的
配置来源**：

```text
configs/development.yaml           <- YAML 配置文件
configs/development.example.yaml   <- 模板
configs/production.example.yaml    <- 模板
configs/test.example.yaml          <- 模板
.env.example                       <- 环境变量清单（DEV-001 新增）
```

问题：

- 「默认值 < 配置文件 < 环境变量」需要同时维护 YAML 解析与环境变量映射两套代码
  （`fileConfig` 结构体 + `applyConfigFile` + `applyEnv`），字段每加一个要改两处。
- 多环境模板（development / production / test）实际只有 `development` 一个被使用，
  其余是冗余模板。
- 配置文件与 `.env` 语义重复：两者最终都只是「给同一组配置项赋值」，
  而优先级更高的环境变量已经覆盖了配置文件的能力。
- 配置文件会跟随仓库被复制、备份、截图，是 DEV-001 中凭据泄露风险的来源，
  删除该机制可以从结构上消除这一类风险。

## Goal

让 `.env` 成为**唯一**的配置来源，配置文件机制整体移除：

- 使用 `godotenv` 读取 `.env`。
- 优先级维持：**export 导出的环境变量 > `.env` > 内置默认值**。
- 删除 `configs/` 目录及其全部 YAML 模板。
- 删除 YAML 解析代码与 `gopkg.in/yaml.v3` 直接依赖。

## Non-Goals

- 不修改任何配置**项**的名字、默认值或校验规则（`validate()` 保持不变）。
- 不修改 `internal/shared/config` 对外暴露的 `Config` 字段语义（`ConfigFile` 除外，
  见 Implementation Notes）。
- 不引入配置热加载、配置中心或其它基础设施。
- 不改动前端项目 `../stock-flow-admin/`。
- 不改动 `Makefile` 与 `script/dev.sh`（两者都通过 `cmd/config` 间接读取配置，
  无需感知来源变化）。

## Scope

允许修改：

- `internal/shared/config/`
- `cmd/config/main.go`
- `.env.example`
- `.gitignore`
- `README.md`
- `docs/development.md`

删除：

- `configs/`（整个目录：3 个已跟踪模板 + 本地 `development.yaml`）

不应修改：

- `internal/` 下除 `config` 外的任何模块
- `migrations/`
- `Makefile`、`script/dev.sh`
- `pkg/response`
- 前端项目 `../stock-flow-admin/`

## Domain Rules

- 优先级必须是：**非空**的导出环境变量 > `.env` > 内置默认值。
  - 「非空」的口径与既有 `env()` 辅助函数一致（`strings.TrimSpace` 后为空视为未设置）。
    这样 `.env` 可以填补一个被导出为空串的变量，且与旧 YAML 实现的行为一致。
- `.env` **缺失不是错误**：生产与 CI 应直接注入真实环境变量。
- `ENV_FILE` 显式指定的文件缺失**是错误**：显式意图必须被满足，不能静默忽略。
- `.env` 存在但解析失败**是错误**：不能静默使用默认值继续启动。
- 仓库中不得提交 `.env`；只能提交 `.env.example`。

## Implementation Notes

### 加载顺序

```text
defaultConfig()                    <- 内置默认值
   ↓ 被覆盖
.env（godotenv.Read）              <- 仅在对应变量「非空导出」时才被跳过
   ↓ 被覆盖
os.Getenv(...)（applyEnv）         <- 导出环境变量，最高优先级
```

实现上用 `godotenv.Read` 读取成 `map[string]string` 后自行合并，而不是直接用
`godotenv.Load`。原因：`Load` 以「变量是否**存在于** environ」判断，导出为空串的
变量会阻止 `.env` 生效；而本项目的既有口径是「空白即未设置」。自行合并可以让
文件层与环境层的「已设置」判定保持一致，也便于用 `t.Setenv(key, "")` 写测试。

### 字段变更

- `Config.ConfigFile` → `Config.EnvFile`：记录实际加载的 dotenv 文件路径，
  未加载时为 `""`。用于排查「到底读到了哪个文件」。
- `cmd/config` 的 `-key config_file` → `-key env_file`。
  （无其它调用方：`Makefile` 只用 `-key database_url`，`script/dev.sh` 只用 `-key http_addr`。）

### 其它

- `go.mod`：新增 `github.com/joho/godotenv`，移除 `gopkg.in/yaml.v3`
  （已核实该依赖仅被 `config.go` 使用）。
- `.gitignore`：删除 `configs/*.yaml` 相关规则；`.env` 的忽略规则扩展为
  `.env` 与 `.env.*`（并保留 `!.env.example`），避免 `.env.production`
  这类文件被误提交。
- `HTTP_ADDR` 在 `.env.example` 中给 `:8181`，与前端
  `VITE_API_PROXY_TARGET` 及 `script/dev.sh` 的既有约定一致；代码默认值仍为 `:8080`。

## Acceptance Criteria

- [x] `configs/` 目录已删除，仓库内无任何 YAML 配置文件。（跟踪的 YAML 仅剩 `openapi/swagger.yaml`、`sqlc.yaml`、`.github/workflows/ci.yml`，均非配置来源文件）
- [x] `.env` 存在时其值生效；对应变量被导出时以导出值为准（含导出为空串时 `.env` 生效）。（`TestLoadDiscoversDefaultEnvFile` / `TestExportedEnvOverridesEnvFile` / `TestBlankExportedEnvFallsBackToEnvFile`）
- [x] `.env` 不存在时不报错，使用默认值；`DATABASE_URL` 缺失仍然启动失败。（`TestLoadWithoutEnvFileSucceeds` / `TestLoadRequiresDatabaseURL`；2026-09-14 实测退出码 1 并提示 `DATABASE_URL is required`）
- [x] `ENV_FILE` 指向不存在的文件时报错。（`TestLoadRequiresExplicitEnvFile`；实测提示 `read env file "...": no such file or directory`）
- [x] `.env` 内容非法时报错。（`TestLoadRejectsMalformedEnvFile`）
- [x] `go build ./... && go vet ./... && go test ./...` 全部通过。（2026-09-14 实测全绿）
- [x] `gofmt -l .` 无输出。（2026-09-14 实测）
- [x] `go run ./cmd/config -key database_url` 能读取 `.env` 中的值。（2026-09-14 实测）
- [x] `README.md` 与 `docs/development.md` 不再出现 `configs/` 与 `CONFIG_FILE`。（实测 grep 无命中；本次已修正 `README.md` 第 106 行遗留的「配置文件中也没有 `database_url`」表述）
- [x] 未在浏览器/仓库中提交任何真实凭据。（`.env` 被 `.gitignore` 排除且未跟踪；跟踪文件中仅有占位假值）

## 完成记录（2026-09-14）

- 实现提交：`081f082`（`refactor(config): 将配置系统从 YAML 文件迁移至 .env`）。
- 依赖变更已核实：`github.com/joho/godotenv v1.5.1` 为直接依赖，`gopkg.in/yaml.v2/v3` 均降为 `// indirect`。
- 唯一未闭环项：`make migrate-version` 走环境变量的验证需要可达的 PostgreSQL；本机 5432 端口无实例，未能实测。

## Open Questions

- ~~`docs/development-plan.md` 中 P0-4 与 M0 里程碑仍引用 `configs/development.yaml`~~
  **已解决**：工作区级计划文档已于 2026-09-14 更新——P0-4 增加「现状更新」说明（`configs/` 已整体删除），
  M0 清单该项标记为已完成并注明「文件已不存在」。
