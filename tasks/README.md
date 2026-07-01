# Tasks Guide for Coding Agents

本目录用于存放给 coding agent 读取和执行的任务文档。

在本项目中，`tasks/` 不是普通的项目管理目录，也不是给人类看的会议记录。它的主要目标是：让 agent 在开始编码前快速理解任务背景、业务边界、允许修改的范围、实现要求和验收标准。

任务文档应尽量写成“可执行规格”，而不是模糊想法。

## Agent 使用规则

coding agent 在实现任何任务前，应按以下顺序读取文档：

1. 根目录 `AGENTS.md`
2. 相关模块内的 `AGENTS.md`
3. 当前任务文档
4. 相关源代码、migration、sqlc query 和测试

如果文档之间存在冲突，优先级如下：

1. 根目录 `AGENTS.md`
2. 模块内 `AGENTS.md`
3. `tasks/` 中的任务文档
4. 源代码现状

任务文档不能覆盖 `AGENTS.md` 中的架构规则。比如三层架构、接口优先、构造函数注入、禁止 ORM、统一响应格式等规则仍然必须遵守。

## 目录组织方式

任务文档按业务模块或技术主题分类存放，不建议把所有 Markdown 文件都放在 `tasks/` 根目录。

推荐结构：

```text
tasks/
├── README.md
├── _template.md
├── material/
│   ├── MAT-001-material-crud.md
│   └── MAT-002-material-category.md
├── sku/
│   ├── SKU-001-sku-crud.md
│   └── SKU-002-sku-attribute.md
├── inventory/
│   ├── INV-001-inventory-balance.md
│   ├── INV-002-inventory-reservation.md
│   └── INV-003-inventory-movement.md
├── warehouse/
│   └── WH-001-warehouse-crud.md
├── shared/
│   ├── SHR-001-unified-response.md
│   └── SHR-002-trace-id-middleware.md
├── database/
│   └── DB-001-initial-schema.md
└── architecture/
    └── ARCH-001-module-boundary-rules.md
```

## 分类规则

优先按业务模块分类：

- `material/`：物料相关任务
- `sku/`：SKU 相关任务
- `inventory/`：库存、库存余额、预留、库存流水相关任务
- `warehouse/`：仓库、库区、库位相关任务

跨模块或基础能力按技术主题分类：

- `shared/`：公共包、中间件、统一响应、错误码、日志、trace id 等
- `database/`：数据库设计、迁移、索引、sqlc query 组织等
- `architecture/`：模块边界、分层规则、事务边界、反腐层等架构决策
- `devops/`：构建、部署、CI、环境配置等

如果一个任务同时涉及多个模块，优先放到它的主业务模块中。如果没有明确主模块，再放到 `shared/`、`database/` 或 `architecture/`。

## 文件命名规则

文件名格式：

```text
<MODULE>-<NUMBER>-<short-description>.md
```

示例：

```text
MAT-001-material-crud.md
SKU-001-sku-crud.md
INV-002-inventory-reservation.md
WH-001-warehouse-crud.md
SHR-001-unified-response.md
DB-001-initial-schema.md
ARCH-001-module-boundary-rules.md
```

命名要求：

- `<MODULE>` 使用大写模块前缀。
- `<NUMBER>` 使用三位数字，从 `001` 开始。
- `<short-description>` 使用小写英文和连字符 `-`。
- 文件名应简短表达任务主题，不要写成完整句子。
- 编号一旦创建，不要因为任务删除、取消或调整而重排。

## 模块前缀

推荐使用以下前缀：

| 前缀 | 目录 | 含义 |
| --- | --- | --- |
| `MAT` | `material/` | Material 物料模块 |
| `SKU` | `sku/` | SKU 模块 |
| `INV` | `inventory/` | Inventory 库存模块 |
| `WH` | `warehouse/` | Warehouse 仓库模块 |
| `SHR` | `shared/` | Shared 公共能力 |
| `DB` | `database/` | Database 数据库相关 |
| `ARCH` | `architecture/` | Architecture 架构相关 |
| `DEV` | `devops/` | DevOps、CI、部署相关 |

如果新增模块，应先在本 README 中登记模块前缀，再创建对应任务文档。

## 编号规则

任务编号按模块或目录分别递增，不使用全局连续编号。

例如：

```text
tasks/material/MAT-001-material-crud.md
tasks/material/MAT-002-material-category.md

tasks/inventory/INV-001-inventory-balance.md
tasks/inventory/INV-002-inventory-reservation.md

tasks/warehouse/WH-001-warehouse-crud.md
```

`MAT-001`、`INV-001`、`WH-001` 可以同时存在，因为模块前缀已经保证任务 ID 唯一。

不要使用 `MAT-001A`、`MAT-001-1` 这类变体。如果需要拆分任务，创建新的递增编号，例如 `MAT-003-material-import.md`。

## 任务状态

任务文档顶部应声明状态，方便 agent 判断是否可以执行。

推荐状态：

- `Draft`：草稿，信息可能不完整，不建议直接实现。
- `Ready`：已准备好，可以由 agent 实现。
- `In Progress`：正在实现中。
- `Blocked`：被外部条件阻塞。
- `Done`：已完成。
- `Canceled`：已取消，不应实现。

如果任务状态不是 `Ready`，agent 不应默认开始大范围编码。可以先补充问题、做小范围调研，或等待用户确认。

## 任务文档模板

每个任务文档建议使用以下结构：

```md
# MAT-001 Material CRUD

Status: Ready
Owner: coding-agent
Module: material
Related:
- internal/material/
- migrations/
- sql/

## Background

说明为什么需要这个任务，以及它解决什么业务问题。

## Goal

说明任务完成后系统应具备什么能力。

## Non-Goals

明确本任务不处理什么，防止 agent 扩大范围。

## Scope

允许修改：

- internal/material/
- sql/material.sql
- migrations/

不应修改：

- internal/inventory/
- internal/warehouse/

## Domain Rules

- 写清楚核心业务规则。
- 写清楚字段约束、状态流转、唯一性规则和边界条件。

## API

- GET /api/v1/materials
- GET /api/v1/materials/:id
- POST /api/v1/materials
- PUT /api/v1/materials/:id
- DELETE /api/v1/materials/:id

## Data Model

- 涉及哪些表。
- 是否需要 migration。
- 是否需要新增 sqlc query。
- 是否需要唯一索引、普通索引或软删除字段。

## Implementation Notes

- 必须遵守 Handler -> Service -> Repository -> PostgreSQL。
- 先定义接口，再写实现。
- 业务逻辑放在 service 或 domain 层。
- Repository 只处理持久化。
- 依赖通过 NewXxx(dep) 构造函数注入。
- 所有 HTTP 响应通过 pkg/response 返回。

## Acceptance Criteria

- 支持创建、查询、更新、软删除物料。
- 重复编码不能创建成功。
- 删除使用 soft delete。
- 添加必要的 service 层测试。
- go test ./... 通过。

## Open Questions

- 如果有不确定点，写在这里。
- 没有则写：None.
```

实际任务可以按复杂度裁剪，但至少应包含：

- `Status`
- `Background`
- `Goal`
- `Non-Goals`
- `Scope`
- `Domain Rules`
- `Implementation Notes`
- `Acceptance Criteria`

## 编写任务时的建议

面向 coding agent 的任务文档应尽量具体：

- 写“允许改哪些文件或目录”，也写“不应该改哪些文件或目录”。
- 写清楚业务规则，不只写“实现 CRUD”。
- 写清楚边界条件，例如重复编码、软删除、分页、状态变更、库存不能为负等。
- 写清楚是否需要 migration、sqlc query、测试和接口路由。
- 写清楚验收标准，让 agent 可以自己验证结果。
- 对不确定事项使用 `Open Questions`，不要把猜测写成确定规则。

避免使用过于模糊的描述：

```md
实现物料功能。
```

更推荐：

```md
实现物料基础管理能力，包括创建、列表、详情、更新和软删除。
物料编码在未删除数据中必须唯一。
删除为 soft delete，不物理删除数据库记录。
HTTP 响应必须通过 pkg/response 返回。
```

## 与代码结构的关系

任务目录应和项目模块边界保持一致：

```text
tasks/material/     -> internal/material/
tasks/sku/          -> internal/sku/
tasks/inventory/    -> internal/inventory/
tasks/warehouse/    -> internal/warehouse/
tasks/shared/       -> internal/shared/ 或 pkg/
tasks/database/     -> migrations/ 和 sql/
```

任务文档只描述需求和约束，不应复制大量实现代码。具体实现以源代码为准。

## Agent 执行约束

coding agent 执行任务时应遵守以下约束：

- 不要跨模块直接访问其他模块 repository。
- 不要绕过 service 层直接在 handler 中写业务逻辑。
- 不要引入 ORM。
- 不要通过全局变量传递依赖。
- 不要为了完成一个任务重构无关模块。
- 不要在任务没有要求时修改 API 统一响应格式。
- 不要在任务没有要求时修改已有 migration 文件；应新增 migration。
- 如果业务规则不明确，应优先在任务文档的 `Open Questions` 中记录，并向用户确认。

## 什么时候需要新建任务文档

以下情况建议创建任务文档：

- 新增一个业务能力。
- 修改核心业务规则。
- 涉及数据库 schema 或 migration。
- 跨多个模块协作。
- 需要 agent 多轮实现。
- 有明确验收标准的重构。

以下情况通常不需要创建任务文档：

- 修复一个很小的拼写错误。
- 调整 README 中的一句话。
- 单文件内非常明确的小修复。

