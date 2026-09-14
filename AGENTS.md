# Stock-Flow — AGENTS.md

## 项目范围

本文件仅适用于 `stock-flow/` 后端项目。前端相关说明位于 `../stock-flow-admin/AGENTS.md`，不适用于本项目。

## 项目概述

Stock-Flow 是库存管理系统的后端 API 服务。

这是一个模块化单体库存系统，用于管理：

- 物料
- 产品
- SKU
- 库存
- 仓库
- 批次
- 库存预留
- 库存变动

未来可能包含以下模块：

- 采购管理
- 销售管理
- 审批工作流
- 库存估值
- 报表

## 后端技术栈

- **语言**：Go 1.25+
- **Web 框架**：Gin
- **数据库**：PostgreSQL（使用 `pgx` 驱动 + `sqlc`）
- **迁移工具**：golang-migrate

## 架构

- 模块化单体
- 轻量 DDD
- 整洁架构

### 模块边界

每个业务领域均作为独立模块实现。

示例：
- material
- sku
- inventory
- warehouse

模块之间通过应用服务通信。
跨模块访问仓储时使用防腐层，避免直接访问。
本服务不负责入库单或出库单模块。外部业务系统应调用库存操作 API 来变更库存。

## 仓库结构

```
stock-flow/
├── cmd/
│
├── internal/
│   ├── material/
│   │
│   ├── sku/
│   │
│   ├── inventory/
│   │
│   ├── warehouse/
│   │
│   └── shared/
│
├── pkg/
│
├── migrations/
│   └── AGENTS.md                    # 数据库迁移规范
│
├── sql/
│
├── tasks/
│
└── AGENTS.md                        # 本文件
```

## 开发原则

- 业务逻辑属于领域层。
- 仓储层仅包含持久化逻辑。
- 事务在应用服务层管理。
- 不使用 ORM。
- 优先使用显式代码，而非框架魔法。

### 三层架构（必须严格遵守）

```
HTTP 请求 → 处理器 → 服务 → 仓储 → PostgreSQL
```

### 接口优先原则

每一层都应先定义接口，再编写实现。这样便可在测试中将其替换为 mock。

```go
// 先定义接口
type MaterialService interface {
    List(ctx context.Context) ([]Material, error)
    Create(ctx context.Context, req CreateMaterialRequest) (*Material, error)
}

// 实现
type materialService struct {
    repo MaterialRepository
}

// 构造函数注入
func NewMaterialService(repo MaterialRepository) MaterialService {
    return &materialService{repo: repo}
}
```

所有依赖都应通过构造函数 `NewXxx(dep)` 注入。禁止使用全局变量传递依赖。

## API 规则

- 基础路径：`/api/v1`
- 资源名称应使用复数名词，例如 `/api/v1/materials`
- 标准 HTTP 操作：GET（列表/详情）、POST（创建）、PUT（更新）、DELETE（软删除）

### 统一响应格式

所有响应必须通过 `pkg/response` 包返回，格式固定如下：

```json
// 成功
{
    "code": 200,
    "message": "success",
    "data": {...},
    "trace_id": "req_abc123xyz",
    "timestamp": 1672531200000
}
// 失败
{
    "code": 1001,
    "message": "error msg",
    "data": null,
    "trace_id": "req_abc123xyz",
    "timestamp": 1672531200000
}
```

## Git 提交与推送规范

### Commit message 格式（必须严格遵守）

- 始终使用**中文**编写 commit message。
- 严格遵循 Conventional Commits 规范，格式为：`<type>(<scope>): <subject>`。
- `type` 必须是以下之一：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`。
- `subject` 需用祈使句简要概括核心改动，不超过 72 字符，确保能清晰体现修改内容和意图。

### Push 授权（硬性约束）

- **绝对不可擅自执行 `git push`**。
- 只有当用户在**当次对话中明确要求**推送时，才可以执行推送。
- 在本机执行 `git commit` 属于正常操作；推送是唯一必须获得显式授权的远程操作。

## 沙箱环境构建缓存约定

Agent 运行在沙箱中，通常只允许写入项目工作区与系统临时目录（/tmp）。因此**所有缓存、
临时文件与临时下载的工具都必须放在系统临时目录下**，禁止在项目仓库内创建任何缓存目录
（如 `.pnpm-store`、`.tools`、`.goroot_tmp`、`.gocache` 等），确保缓存不跟随项目。

### 通用约定

- 统一缓存根目录：`/tmp/agent-cache`（macOS 上 `/tmp` 即 `/private/tmp`）。
  该目录可跨项目共享：pnpm store 与 Go 模块缓存都是内容寻址的，跨项目共享安全且能提高缓存命中率。
- 禁止在项目内创建缓存/依赖目录；需要临时文件时使用 `mktemp -d`（默认位于系统临时目录）。
- 若项目内发现历史遗留的缓存目录（如 `.pnpm-store`、`.tools`），直接删除即可：
  它们是可再生内容，不得提交、不得保留。

### 各工具约定

- Go：执行任何 `go` 命令前，先导出（或写入 Makefile 默认值，可用环境变量覆盖）：
  - `GOCACHE=/tmp/agent-cache/go-build`
  - `GOMODCACHE=/tmp/agent-cache/go-mod`
  - `GOTMPDIR=/tmp/agent-cache/go-tmp`（使用前确保目录存在：`mkdir -p`）
  - `GOPATH=/tmp/agent-cache/go-path`（仅需要时）
  - `GOBIN=/tmp/agent-cache/bin`（仅 `go install` 工具时）
- pnpm / npm：
  - 统一使用 store：`pnpm --config.store-dir=/tmp/agent-cache/pnpm-store <命令>`
    （这是**唯一实测有效**的写法，见下）。
  - 注意：pnpm 10 及以上**忽略项目 `.npmrc` 中的 `store-dir`**（该配置仅在 pnpm ≤9 有效）。
  - 实测结论（2026-09-14，pnpm 10.15.1）：
    - 未指定 store 时，pnpm 会把 store 解析到**工作区根的 `.pnpm-store/v11`**；
      `pnpm run <脚本>`（包括 `pnpm check` 这类内部再次调用 pnpm 的脚本）都会重建该目录。
    - ✅ `pnpm --config.store-dir=/tmp/agent-cache/pnpm-store <命令>`：对 `run <脚本>`、
      自带子命令、以及脚本内嵌套的 pnpm 调用均生效（实测完整 `pnpm check` 退出码 0，
      且未生成项目内 store）。
    - ❌ `pnpm --store-dir=... run <脚本>`：直接报 `Unknown option: 'store-dir'`
      （该标志只被部分子命令接受，如 `pnpm store path`，不能用于脚本调用）。
    - ❌ 手工设置 `npm_config_store_dir=...`：被 pnpm 10 **静默忽略**，store 仍落在项目内。
    - ❌ `XDG_DATA_HOME` 指向空目录：pnpm 会因 `packageManager` 固定版本切换失败
      （`Failed to switch pnpm to v10.15.1 ... ENOENT`）。
  - npm 对应 `cache=/tmp/agent-cache/npm-cache`（npm 仍可从 `.npmrc` 读取）。
  - 不得让 store 落到项目内路径（如项目根的 `.pnpm-store`）；跑完 pnpm 后应用
    `ls -d .pnpm-store` 复核，发现即删除。
- 需要固定版本的独立工具（如 sqlc）：
  - 优先用 `go run <module>@<version>`（编译产物进 GOCACHE），
    或 `GOBIN=/tmp/agent-cache/bin go install <module>@<version>`。
  - 禁止把工具二进制下载到项目内（如项目内 `.tools/bin`）。

### 例外与权衡

- `node_modules/`、`dist/`、编译产物等属于**构建产物**而非缓存，仍按项目约定放在项目内
  并由 `.gitignore` 排除，不受本约束限制。
- `/tmp` 会被系统清理（重启必清空；macOS 约 3 天未访问即清理）。缓存被清理后重新下载即可，
  属正常现象；不得因此把缓存改回项目内。
- 本条款的意图是约束"缓存与临时文件"的位置，不限制业务数据、源码或文档在项目内的正常存放。

## 实现任何功能之前

1. 阅读 `AGENTS.md`。
2. 阅读相关文档/领域文档。
3. 阅读相关任务文档。
4. 遵守模块边界。
5. 业务逻辑发生变更时添加测试。

## 文档优先级顺序

0. 用户本次明确提出的要求。
1. `AGENTS.md`
2. 包内的子级 `AGENTS.md`
3. `tasks`
5. 源代码

若存在冲突，以优先级更高的文档为准。
