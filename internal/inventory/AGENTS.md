# 库存

库存模块负责库存状态、库存操作规则、库存预留、FIFO 层分配和库存变动记录。

库存被设计为独立的库存能力/服务。它向外部业务系统提供简单的库存操作 API，不负责采购、销售、入库单、出库单、审批、发运或其他工作流模块。

## 目的

库存模块回答两个问题：

- 系统当前有多少库存？
- 当外部业务操作增加、预留、释放或扣减库存时，库存应如何安全地变化？

## 范围

库存模块负责：

- 查询当前库存。
- 按 `warehouse + SKU` 跟踪库存。
- 存在批次信息时，按 `warehouse + SKU + batch` 跟踪库存。
- 增加库存。
- 预留库存。
- 释放预留库存。
- 直接从可用库存中扣减库存。
- 从之前预留的库存中扣减库存。
- 未指定 `batch_id` 时按 FIFO 分配库存层。
- 记录库存变动历史。
- 强制库存变更操作具备幂等性。
- 防止负库存、负预留和超额预留。

库存模块不负责：

- 物料主数据。
- SKU 定义。
- 物料级单位换算定义。
- 采购订单。
- 销售订单。
- 入库单工作流。
- 出库单工作流。
- 审批或发运工作流。

## 术语与账本模型

库存数据分为两条互不相同的分支：**数量分支**随后续操作变化，**属性分支**是主数据。

```text
数量分支                                        属性分支
inventory_stocks（库存余额）                    skus ──(sku_id)──> inventory_batches（批次）
  仓库 + SKU 汇总，唯一一行                        批号、生产日期、有效期、状态
      ↑ 恒等：Σ 层                                 没有数量字段，没有 warehouse_id
inventory_stock_layers（库存层）                         ↑
  仓库 + SKU + batch_id(可空) + received_at              └── batch_id 仅作为层的可选标签
  ← 数量、FIFO 分配、预留都发生在这里
```

术语：

- **库存层（stock layer）**：`inventory_stock_layers` 的一行，表示某个仓库中某个 SKU 某一次到货所形成的、可被独立消耗的一段库存。它是**数量的最小单位**，也是 FIFO 分配与预留的最小单位。
- **批次（batch）**：`inventory_batches` 的一行，是 **SKU 级主数据**（批号、生产日期、有效期、状态）。它没有数量字段，也没有 `warehouse_id`。
- **库存余额（stock balance）**：`inventory_stocks` 的一行，按仓库 + SKU 汇总，等于该仓库 + SKU 下所有未删除库存层的合计。
- **非批次库存**：`batch_id` 为 NULL 的库存层。

约定（违反即为缺陷）：

- 数量只存在于库存层与库存余额上；批次表不是数量来源。
- 批次与库存层是 N:M 关系，而不是包含关系：同一批次可以在多个仓库各有一层，也可以没有任何层（尚未收货或已清空）；同一仓库的同一 SKU 可以同时存在带批次的层与不带批次的层。
- FIFO 的顺序判定条件固定为 `(received_at, id)`；`batch_id` 不是排序键，也不参与顺序判定。
- `batch_id` 只用于两件事：把操作目标限定到某个批次的库存层，以及按批次归组查询库存层明细。
- "某仓库 + SKU + 批次现有多少库存"通过在库存层上按 `batch_id` 过滤得到，而不是在批次表上查数量。

## 库存维度

库存当前支持两个维度：

```text
warehouse + SKU
warehouse + SKU + batch
```

规则：

- 每次库存查询和变更都必须提供 `warehouse_id`。
- 每次库存查询和变更都必须提供 `sku_id`。
- `batch_id` 可选。
- 库存查询可以包含停用的仓库。
- 库存变更操作必须拒绝停用的仓库。
- 如果提供 `batch_id`，操作将以带该 `batch_id` 的库存层为目标（可能命中多层或跨仓库的多层）。此时有两种情况：若这些层的数量充足，则全部从这些层执行操作；若数量不足，剩余部分仍按 FIFO 处理。
- 出库操作未提供 `batch_id` 时，库存必须按 FIFO 分配。
- FIFO 分配必须基于稳定的库存层排序：先按 `received_at` 排序，再以 `id` 作为确定性的顺序判定条件。
- 可以按仓库 + SKU 汇总查询库存，也可以按仓库 + SKU + 批次查询明细。

如果同时存储库存余额和库存层，二者必须在同一事务中保持一致。

```text
warehouse_sku.on_hand_qty = sum(layer.on_hand_qty)
warehouse_sku.reserved_qty = sum(layer.reserved_qty)
```

非批次库存可以表示为没有 `batch_id` 的库存层。如果非批次库存参与 FIFO 分配，它也必须具有稳定的收货时间。

## 数量模型

库存不得只存储一个通用数量字段。

建议的数量字段：

- `on_hand_qty`：实物库存数量。
- `reserved_qty`：已被外部业务操作预留的数量。
- `available_qty`：按 `on_hand_qty - reserved_qty` 计算的数量。

默认应计算而非存储 `available_qty`。如果以后出于性能考虑存储该字段，服务层必须在更新 `on_hand_qty` 和 `reserved_qty` 的同一事务中保持其一致。

核心不变量：

- `on_hand_qty >= 0`
- `reserved_qty >= 0`
- `reserved_qty <= on_hand_qty`
- `available_qty >= 0`

## 库存操作

库存模块向外部业务系统提供简单的变更操作。

建议的服务方法：

```go
type InventoryService interface {
    IncreaseStock(ctx context.Context, req IncreaseStockRequest) error
    ReserveStock(ctx context.Context, req ReserveStockRequest) error
    ReleaseReservedStock(ctx context.Context, req ReleaseReservedStockRequest) error
    DecreaseAvailableStock(ctx context.Context, req DecreaseAvailableStockRequest) error
    DecreaseReservedStock(ctx context.Context, req DecreaseReservedStockRequest) error
    GetStock(ctx context.Context, query StockQuery) (*Stock, error)
}
```

### 增加库存

增加库存会增加实物库存。

```text
on_hand_qty += qty
reserved_qty 不变
```

规则：

- `qty` 必须大于零。
- `warehouse_id` 和 `sku_id` 为必填项。
- `batch_id` 可选。
- 如果提供 `batch_id`，则增加到该批次对应的库存层（不存在时创建）。
- 如果未提供 `batch_id`，则增加到 `batch_id` 为 NULL 的库存层，或由服务定义的默认库存层。

### 预留库存

预留库存会为外部业务操作锁定可用库存。

```text
reserved_qty += qty
available_qty = on_hand_qty - reserved_qty
```

规则：

- `qty` 必须大于零。
- 可用数量必须大于或等于 `qty`。
- 如果提供 `batch_id`，则从该批次的库存层预留。
- 如果未提供 `batch_id`，则跨符合条件的库存行按 FIFO 预留。
- 预留必须创建一条变动记录。

### 释放预留库存

释放预留库存会解锁之前预留的库存。

```text
reserved_qty -= qty
available_qty = on_hand_qty - reserved_qty
```

规则：

- `qty` 必须大于零。
- 预留数量必须大于或等于 `qty`。
- 当外部业务系统可以提供原始预留信息时，释放操作应引用原始预留。
- 如果预留分配到了多个库存层，释放操作必须一致地冲回各层已分配的数量。
- 释放操作必须创建一条变动记录。

### 扣减可用库存

扣减可用库存会直接消耗之前未预留的库存。

```text
on_hand_qty -= qty
reserved_qty 不变
```

规则：

- `qty` 必须大于零。
- 可用数量必须大于或等于 `qty`。
- 如果提供 `batch_id`，则从该批次的库存层扣减。
- 如果未提供 `batch_id`，则跨符合条件的库存行按 FIFO 扣减。
- 此操作用于直接消耗没有事先预留的库存。
- 扣减操作必须创建一条变动记录。

### 扣减预留库存

扣减预留库存用于确认消耗之前预留的库存。

```text
on_hand_qty -= qty
reserved_qty -= qty
```

规则：

- `qty` 必须大于零。
- 预留数量必须大于或等于 `qty`。
- 此操作应尽可能引用原始预留。
- 如果预留分配到了多个库存层，扣减必须遵循原始预留分配。
- 此操作用于成功预留后，外部业务操作得到确认的场景。
- 扣减操作必须创建一条变动记录。

## 幂等性规则

所有库存变更操作都必须是幂等的。

变更操作包括：

- 增加库存。
- 预留库存。
- 释放预留库存。
- 扣减可用库存。
- 扣减预留库存。

每个变更请求都必须包含幂等键。

建议的字段：

- `idempotency_key`
- `operation_type`
- `source_type`
- `source_id`
- `source_line_id`

规则：

- 相同的 `idempotency_key` 和 `operation_type` 不得多次应用同一次库存变更。
- 使用相同幂等键和相同请求载荷重试时，服务应返回原始结果，或将其视为空操作并返回成功。
- 使用相同幂等键但不同请求载荷重试时，服务必须返回幂等冲突错误。
- 幂等记录、库存余额更新和变动记录必须在同一事务中写入。

## 变动记录

每次库存变更都必须创建库存变动记录。

建议的变动记录字段：

- `id`
- `operation_type`
- `warehouse_id`
- `sku_id`
- `batch_id`
- `qty`
- `idempotency_key`
- `source_type`
- `source_id`
- `source_line_id`
- `created_at`

变动记录是审计记录。不得通过更新变动记录来修正库存状态。修正必须通过新的变动操作表示。

## 边界规则

- 库存可以通过 SKU 应用服务或稳定的应用契约依赖 SKU。
- 存在仓库模块时，库存可以通过仓库应用服务或稳定的应用契约校验仓库是否存在。
- 库存变更操作前，库存模块必须校验仓库处于启用状态。
- 库存可以查询停用仓库的库存。
- 库存不得直接访问物料仓储。
- 外部业务系统不得直接写入库存仓储或库存表。
- 外部业务系统必须使用库存应用服务或 HTTP API 变更库存。
- 增加、预留、释放和扣减库存的逻辑不得在库存模块之外重复实现。

## 分层规则

库存模块必须遵循项目的依赖方向：

```text
处理器 -> 服务 -> 仓储
```

- 处理器解析 HTTP 输入并返回统一响应。
- 服务负责库存业务规则、FIFO 分配、幂等性检查和库存操作编排。
- 仓储仅负责持久化逻辑。

事务必须在服务层启动并完成。

## 事务与并发规则

库存变更必须具备原子性。

服务层必须将以下变更包含在同一个事务中：

- 幂等性检查或插入。
- 选择并锁定库存行。
- 更新库存余额。
- 适用时更新库存层。
- 创建变动记录。

并发规则：

- 对选中进行变更的库存行，必须加锁或使用原子 SQL 条件更新。
- 服务必须防止 `on_hand_qty`、`reserved_qty` 和 `available_qty` 出现负数。
- FIFO 分配必须在应用更新前锁定所有选中的库存层行。
- 仓储方法不得启动、提交或回滚事务。

## API 规则

库存 API 必须在 `/api/v1` 下使用复数资源名称或语义清晰的动作资源。

预期查询资源：

```text
/api/v1/inventory/stocks
```

预期变更资源：

```text
POST /api/v1/inventory/increase
POST /api/v1/inventory/reserve
POST /api/v1/inventory/release
POST /api/v1/inventory/decrease-available
POST /api/v1/inventory/decrease-reserved
```

所有响应必须使用 `pkg/response` 包。

## 禁止模式

- 在没有 `warehouse_id` 的情况下更新库存。
- 在没有 `sku_id` 的情况下更新库存。
- 在没有幂等键的情况下应用变更。
- 允许 `reserved_qty` 超过 `on_hand_qty`。
- 库存此前已被预留时，从可用数量中扣减库存。
- 从预留数量中扣减库存时不减少 `reserved_qty`。
- 在仓储中脱离服务层规则重新计算 FIFO 顺序。
- 从物料、SKU、仓库或外部业务模块直接更新库存表。
- 把 `batch_id` 当作 FIFO 排序键，或依赖批号字典序决定分配顺序。
- 把 `inventory_batches` 当作数量来源（该表没有数量字段，也没有 `warehouse_id`）。
