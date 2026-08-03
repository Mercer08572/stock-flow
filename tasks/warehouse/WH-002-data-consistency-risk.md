# WH-002 Inventory Reference Data Consistency

Status: Ready
Owner: coding-agent
Module: warehouse
Related:
- `internal/warehouse/`
- `internal/sku/`
- `internal/inventory/`
- `sql/queries/`
- `migrations/`

## Background

仓库删除目前直接执行软删除，没有检查库存引用：
[warehouse/service.go (line 95)](/Users/badbugu/workspace/myself_project/stock-flow/internal/warehouse/service.go:95)。

库存列表查询可以返回引用已软删除仓库或 SKU 的记录，但库存详情会校验仓库和 SKU 是否存在，因此可能查询失败：
[inventory/service.go (line 48)](/Users/badbugu/workspace/myself_project/stock-flow/internal/inventory/service.go:48)、
[inventory_stocks.sql (line 1)](/Users/badbugu/workspace/myself_project/stock-flow/sql/queries/inventory_stocks.sql:1)。

这会造成库存列表与详情行为不一致，也可能让库存、库存层、预留或流水引用已经删除的主数据。

## Goal

建立仓库和 SKU 的生命周期保护规则，保证主数据删除行为与库存历史数据保持一致。

任务完成后系统应具备以下能力：

- 有库存引用的仓库只能禁用，不能软删除。
- 有库存引用的 SKU 不能软删除。
- 历史库存查询可以正确处理已删除的仓库或 SKU 主数据。
- 数据库集成测试覆盖仓库和 SKU 删除保护以及历史库存查询场景。

## Non-Goals

- 不实现仓库、SKU 或库存的全新 CRUD API。
- 不修改库存数量、预留、释放、扣减或 FIFO 分配规则。
- 不物理删除仓库、SKU、库存、预留或流水记录。
- 不允许 warehouse 或 SKU 模块直接访问 inventory repository。
- 不重构与主数据生命周期保护无关的模块。

## Scope

允许修改：

- `internal/warehouse/`
- `internal/sku/`
- `internal/inventory/`
- 相关的 application service 接口或反腐层契约
- `sql/queries/` 下与引用检查和历史库存查询有关的 query
- 为该任务新增的 migration
- 仓库、SKU 和库存相关测试

不应修改：

- `internal/material/`
- 与本任务无关的库存操作逻辑
- `pkg/response` 的统一响应格式
- 已存在的 migration 文件

## Domain Rules

- 只要仓库被库存、库存层、预留或库存流水引用，就不能软删除。
- 有库存引用的仓库可以禁用；禁用不得修改现有库存数量、预留或流水。
- 只要 SKU 被库存、库存层、预留或库存流水引用，就不能软删除。
- 删除保护必须通过 inventory application service、稳定的 application contract 或更高层协调服务实现。
- warehouse service 和 SKU service 不得直接访问 inventory repository 或库存表。
- 历史库存查询必须允许读取引用已删除主数据的记录，不能仅因仓库或 SKU 已软删除而失败。
- 默认的仓库和 SKU 列表、详情查询仍然不应返回已软删除主数据。
- 禁止留下无法解释或无法查询的悬空库存引用。

## Implementation Notes

- 必须遵守 `Handler -> Service -> Repository -> PostgreSQL`。
- 先定义 application service 或稳定 contract 接口，再编写实现。
- 删除保护和生命周期规则放在 service 层。
- Repository 只负责持久化和引用存在性查询。
- 跨模块依赖通过 `NewXxx(dep)` 构造函数注入。
- 如果删除检查和软删除需要原子执行，事务必须由 application service 管理。
- 如果需要修改数据库结构，应新增 migration，不得修改已有 migration。
- HTTP 响应必须通过 `pkg/response` 返回。

## Acceptance Criteria

- 有库存记录的仓库不能软删除。
- 有库存层记录的仓库不能软删除。
- 有预留记录的仓库不能软删除。
- 有库存流水记录的仓库不能软删除。
- 有上述引用的仓库仍然可以被禁用。
- 有库存相关引用的 SKU 不能软删除。
- 没有库存相关引用的仓库和 SKU 可以按现有规则软删除。
- 历史库存列表和详情对已删除主数据的处理保持一致。
- 删除保护没有引入跨模块 repository 直接访问。
- 数据库集成测试覆盖允许删除、拒绝删除、允许禁用和历史查询场景。
- `go test ./...` 通过。

## Decisions

- 只要存在未软删除的库存余额、库存层、预留或库存流水记录，就禁止软删除仓库或 SKU；当前库存数量是否为零不影响判断。
- 所有状态的预留记录都属于库存历史引用，包括 `active` 和 `closed`，均阻止软删除仓库或 SKU。
- 如果预留记录已软删除但仍被库存流水引用，该库存流水仍然阻止软删除仓库或 SKU。
- 库存流水是不可变审计记录；只要存在相关库存流水，就始终阻止软删除仓库或 SKU。
- 历史库存查询返回仓库和 SKU 当前主数据的最小投影，不在本任务中引入主数据快照。
- 历史库存中的仓库和 SKU 最小投影至少包含 `id`、`code`、`name` 和 `deleted`。
- warehouse 和 SKU 模块应通过稳定的 application contract 提供包含已软删除记录的历史引用读取能力，例如 `GetReference` 或 `GetIncludingDeleted`；inventory 不得直接访问其 repository。
- 删除被库存引用的仓库时，返回独立的仓库领域冲突错误，例如 `ErrWarehouseReferencedByInventory`。
- 删除被库存引用的 SKU 时，返回独立的 SKU 领域冲突错误，例如 `ErrSKUReferencedByInventory`。
- 上述删除冲突统一映射为 HTTP `409 Conflict` 和 `response.CodeConflict`（`1009`）。
