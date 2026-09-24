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

### 提交与推送授权（硬性约束）

- **绝对不可擅自执行 `git commit`**。
- **绝对不可擅自执行 `git push`**。
- 只有当用户在**当次对话中明确要求**提交或推送时，才可以执行对应的操作。
- **禁止丢弃用户改动**：不得擅自执行 `git restore`、`git checkout -- <path>`、`git reset --hard`、`git stash`、`git clean`；需要清理未提交改动时必须先取得用户明确同意。

### Commit message 格式（必须严格遵守）

- 始终使用**中文**编写 commit message。
- 严格遵循 Conventional Commits 规范，格式为：`<type>(<scope>): <subject>`。
- `type` 必须是以下之一：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`。
- `subject` 需用祈使句简要概括核心改动，不超过 72 字符，确保能清晰体现修改内容和意图。

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
4. 源代码

若存在冲突，以优先级更高的文档为准。
