# 数据库模式设计

本文档描述 Stock-Flow 的逻辑数据库模式。

该模式遵循当前模块边界：

```text
material -> sku -> inventory(warehouse, batch)
```

本服务不负责入库单或出库单模块。外部业务系统调用库存操作 API 来增加、预留、释放或扣减库存。

## 设计决策

- `available_qty` 不作为普通列存储，而是按 `on_hand_qty - reserved_qty` 计算。
- 批次建模为独立表。
- 预留使用主表和分配明细表。
- 在预留或扣减库存时记录 FIFO 分配。
- 幂等性使用 `operation_type + idempotency_key + request_hash`。
- 库存数量使用 `NUMERIC(20,6)`。
- 单位换算系数使用 `NUMERIC(24,10)`。
- 主键使用 `BIGSERIAL`。
- 外键使用 `BIGINT`。
- 类枚举值使用带 `CHECK` 约束的 `TEXT`。
- 软删除使用 `deleted_at TIMESTAMPTZ NULL`。

## 数值规则

库存和单位换算数据使用精确小数类型。

建议的类型：

- 库存数量：`NUMERIC(20,6)`
- 单位换算系数：`NUMERIC(24,10)`

库存数量或换算系数不得使用 `FLOAT`、`REAL` 或 `DOUBLE PRECISION`。

`available_qty` 应在 SQL 或应用代码中计算：

```sql
on_hand_qty - reserved_qty AS available_qty
```

如果以后出于性能考虑存储 `available_qty`，必须将其视为派生值，并在更新 `on_hand_qty` 和 `reserved_qty` 的同一事务中更新它。

## 表概览

建议的表：

- `units`
- `material_categories`
- `materials`
- `material_attribute_definitions`
- `material_attribute_values`
- `material_unit_conversions`
- `skus`
- `warehouses`
- `inventory_batches`
- `inventory_stocks`
- `inventory_stock_layers`
- `inventory_reservations`
- `inventory_reservation_items`
- `inventory_movements`
- `inventory_idempotency_keys`

## 单位

`units` 存储可复用的计量单位。

建议的列：

```text
id            BIGSERIAL PRIMARY KEY
code          TEXT NOT NULL
name          TEXT NOT NULL
symbol        TEXT NOT NULL
unit_type     TEXT NOT NULL
precision     INTEGER NOT NULL
status        TEXT NOT NULL DEFAULT 'active'
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at    TIMESTAMPTZ NULL
```

规则：

- `code` 在未删除的单位中必须唯一。
- `status` 应为 `active` 或 `inactive`。
- `precision` 控制允许的数量小数位数。
- 单位记录不得存储特定物料的换算规则。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `code` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `status` 创建索引。

## 物料分类

`material_categories` 存储物料分类主数据。

建议的列：

```text
id            BIGSERIAL PRIMARY KEY
code          TEXT NOT NULL
name          TEXT NOT NULL
parent_id     BIGINT NULL REFERENCES material_categories(id)
status        TEXT NOT NULL DEFAULT 'active'
remark        TEXT NULL
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at    TIMESTAMPTZ NULL
```

规则：

- `code` 在未删除的分类中必须唯一。
- 分类特有的物料属性必须通过属性定义表建模，不得向 `materials` 添加列。

## 物料

`materials` 存储物料主数据。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
code            TEXT NOT NULL
name            TEXT NOT NULL
category_id     BIGINT NOT NULL REFERENCES material_categories(id)
base_unit_id    BIGINT NOT NULL REFERENCES units(id)
status          TEXT NOT NULL DEFAULT 'active'
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `code` 在未删除的物料中必须唯一。
- `base_unit_id` 必须引用 `units`。
- 物料不负责 SKU 字段、库存字段、仓库字段或变动字段。
- 不得将分类特有字段直接添加到此表。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `code` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `category_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `base_unit_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `status` 创建索引。

## 物料属性定义

`material_attribute_definitions` 定义分类特有的物料属性。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
category_id     BIGINT NOT NULL REFERENCES material_categories(id)
code            TEXT NOT NULL
name            TEXT NOT NULL
data_type       TEXT NOT NULL
required        BOOLEAN NOT NULL DEFAULT FALSE
status          TEXT NOT NULL DEFAULT 'active'
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `data_type` 应限制为 `text`、`number`、`boolean`、`date` 或 `option` 等值。
- `(category_id, code)` 在未删除的定义中必须唯一。

## 物料属性值

`material_attribute_values` 存储物料在各属性定义下的具体值。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
definition_id   BIGINT NOT NULL REFERENCES material_attribute_definitions(id)
value_text      TEXT NULL
value_number    NUMERIC(20,6) NULL
value_boolean   BOOLEAN NULL
value_date      DATE NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `(material_id, definition_id)` 在未删除的属性值中必须唯一。
- 服务层必须校验值所在的列与定义的 `data_type` 匹配。

## 物料单位换算

`material_unit_conversions` 存储特定物料的单位换算规则。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
from_unit_id    BIGINT NOT NULL REFERENCES units(id)
to_unit_id      BIGINT NOT NULL REFERENCES units(id)
factor          NUMERIC(24,10) NOT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `factor` 必须大于零。
- `(material_id, from_unit_id, to_unit_id)` 在未删除的换算关系中必须唯一。
- 换算方向固定为「物料 `base_unit_id` → 另一单位」：提交反向对时会被规范化为该方向并取系数倒数，
  因此同一对单位只有一种存储方向。
- 两个单位的 `unit_type` 必须可公度：相同类型，或「包装 ↔ 计数」。
- 至少一端必须是物料的 `base_unit_id`，否则换算图无法保证能回到基本单位。
- `from_unit_id` 和 `to_unit_id` 必须不同。
- 换算逻辑属于物料服务层或物料领域辅助组件。

示例：

```text
物料 A：1 box = 12 pcs
物料 B：1 box = 24 pcs
```

## SKUs

`skus` 存储库存保有单位定义。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
code            TEXT NOT NULL
name            TEXT NOT NULL
unit_id         BIGINT NOT NULL REFERENCES units(id)
status          TEXT NOT NULL DEFAULT 'active'
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `code` 在未删除的 SKU 中必须唯一。
- 每种物料允许存在多个启用的 SKU（原「当前阶段最多一个启用 SKU」的约束已移除）。
- 多个启用 SKU 的前提是单位可公度：每个 SKU 的 `unit_id` 必须与物料 `base_unit_id` 同属一个 `unit_type`
  （「包装 ↔ 计数」例外），并已建立物料级换算规则。
- `unit_id` 必须是物料基本单位，或可通过物料单位换算进行转换的单位。
- SKU 不得存储库存数量字段。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `code` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `material_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `unit_id` 创建索引。
- 不再对 `(material_id)` 建「启用态唯一」的部分唯一索引（202610030008 已删除）。

## 仓库

`warehouses` 存储仓库主数据。

建议的列：

```text
id              BIGSERIAL PRIMARY KEY
code            TEXT NOT NULL
name            TEXT NOT NULL
type            TEXT NOT NULL DEFAULT 'normal'
status          TEXT NOT NULL DEFAULT 'active'
location        TEXT NULL
contact_name    TEXT NULL
contact_phone   TEXT NULL
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

规则：

- `code` 在未删除的仓库中必须唯一。
- `status` 应为 `active` 或 `inactive`。
- 可以查询 `inactive` 仓库的库存。
- `inactive` 仓库不得用于库存变更。
- 存在库存、预留或变动记录的仓库不得硬删除。
- 仓库存在任何库存记录时，必须将其停用而不是删除。
- 仓库不得存储库存数量字段。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `code` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `status` 创建索引。

## 库存批次

`inventory_batches` 存储批次主数据。

建议的列：

```text
id                BIGSERIAL PRIMARY KEY
sku_id            BIGINT NOT NULL REFERENCES skus(id)
batch_no          TEXT NOT NULL
first_received_at TIMESTAMPTZ NULL
production_date   DATE NULL
expiration_date   DATE NULL
status            TEXT NOT NULL DEFAULT 'active'
remark            TEXT NULL
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

规则：

- `(sku_id, batch_no)` 在未删除的批次中必须唯一。
- 批次元数据属于此表，而不属于库存余额表。
- `status` 应为 `active` 或 `inactive`。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `(sku_id, batch_no)` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `sku_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `first_received_at` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `expiration_date` 创建索引。

## 库存汇总

`inventory_stocks` 按仓库和 SKU 存储汇总库存。

此表支持快速查询当前库存。

建议的列：

```text
id                BIGSERIAL PRIMARY KEY
warehouse_id      BIGINT NOT NULL REFERENCES warehouses(id)
sku_id            BIGINT NOT NULL REFERENCES skus(id)
on_hand_qty       NUMERIC(20,6) NOT NULL DEFAULT 0
reserved_qty      NUMERIC(20,6) NOT NULL DEFAULT 0
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

计算值：

```text
available_qty = on_hand_qty - reserved_qty
```

规则：

- `(warehouse_id, sku_id)` 在未删除的库存行中必须唯一。
- `on_hand_qty` 必须大于或等于零。
- `reserved_qty` 必须大于或等于零。
- `reserved_qty` 必须小于或等于 `on_hand_qty`。
- 所有库存变更都必须在同一事务中更新此汇总表和库存层。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `(warehouse_id, sku_id)` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `sku_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `warehouse_id` 创建索引。

## 库存层

`inventory_stock_layers` 存储 FIFO 分配单元。

一个库存层表示某个 SKU 收入仓库的一批数量。`batch_id` 可选。

建议的列：

```text
id                BIGSERIAL PRIMARY KEY
warehouse_id      BIGINT NOT NULL REFERENCES warehouses(id)
sku_id            BIGINT NOT NULL REFERENCES skus(id)
batch_id          BIGINT NULL REFERENCES inventory_batches(id)
received_at       TIMESTAMPTZ NOT NULL
on_hand_qty       NUMERIC(20,6) NOT NULL DEFAULT 0
reserved_qty      NUMERIC(20,6) NOT NULL DEFAULT 0
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

计算值：

```text
available_qty = on_hand_qty - reserved_qty
```

规则：

- 对于非批次库存，`batch_id` 可以为 null。
- FIFO 分配先使用 `received_at` 排序，再以 `id` 作为确定性的顺序判定条件。
- `on_hand_qty` 必须大于或等于零。
- `reserved_qty` 必须大于或等于零。
- `reserved_qty` 必须小于或等于 `on_hand_qty`。
- 存在 `batch_id` 时，服务层必须确保它属于同一个 `sku_id`。
- 通过对库存层分组来查询仓库 + SKU + 批次库存。
- 仓库 + SKU 汇总库存必须与库存层合计保持一致。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `(warehouse_id, sku_id, received_at, id)` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `(warehouse_id, sku_id, batch_id)` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `batch_id` 创建索引。

## 库存预留

`inventory_reservations` 存储预留主记录。

建议的列：

```text
id                  BIGSERIAL PRIMARY KEY
warehouse_id        BIGINT NOT NULL REFERENCES warehouses(id)
sku_id              BIGINT NOT NULL REFERENCES skus(id)
total_qty           NUMERIC(20,6) NOT NULL
released_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
consumed_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
status              TEXT NOT NULL DEFAULT 'active'
close_reason        TEXT NULL
idempotency_key     TEXT NOT NULL
source_type         TEXT NULL
source_id           TEXT NULL
source_line_id      TEXT NULL
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at          TIMESTAMPTZ NULL
```

规则：

- `total_qty` 必须大于零。
- `released_qty` 和 `consumed_qty` 必须大于或等于零。
- `released_qty + consumed_qty` 必须小于或等于 `total_qty`。
- `status` 是生命周期状态：仍有预留数量时为 `active`，预留处理完毕时为 `closed`。
- `active` 要求 `released_qty + consumed_qty < total_qty`。
- `closed` 要求 `released_qty + consumed_qty = total_qty`。
- 已关闭的预留是全部消耗、全部释放还是混合处理，应从 `consumed_qty` 和 `released_qty` 推导，而不是编码在 `status` 中。
- 外部工作流取消通过释放剩余预留数量表示；它不是库存预留状态。
- 预留为 `active` 时 `close_reason` 为 NULL；预留为 `closed` 时该字段必填。
- `close_reason` 应为 `consumed`、`released`、`mixed`、`cancelled` 或 `expired`；数量字段仍是所发生情况的权威记录。
- 预留时必须按 FIFO 将数量分配到库存层。
- 释放和扣减预留库存操作必须使用预留分配明细，而不是重新计算 FIFO。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `(warehouse_id, sku_id, status)` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `(source_type, source_id, source_line_id)` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `idempotency_key` 创建索引。

## 库存预留明细

`inventory_reservation_items` 存储预留的 FIFO 分配明细。

建议的列：

```text
id                  BIGSERIAL PRIMARY KEY
reservation_id      BIGINT NOT NULL REFERENCES inventory_reservations(id)
stock_layer_id      BIGINT NOT NULL REFERENCES inventory_stock_layers(id)
reserved_qty        NUMERIC(20,6) NOT NULL
released_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
consumed_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at          TIMESTAMPTZ NULL
```

规则：

- `reserved_qty` 必须大于零。
- `released_qty` 和 `consumed_qty` 必须大于或等于零。
- `released_qty + consumed_qty` 必须小于或等于 `reserved_qty`。
- 每条明细指向 FIFO 分配期间选中的具体库存层。
- 释放预留库存会减少这些库存层上的 `reserved_qty`。
- 扣减预留库存会同时减少这些库存层上的 `reserved_qty` 和 `on_hand_qty`。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `reservation_id` 创建索引。
- 在 `deleted_at IS NULL` 条件下为 `stock_layer_id` 创建索引。

## 库存变动

`inventory_movements` 存储不可变的库存审计记录。

建议的列：

```text
id                    BIGSERIAL PRIMARY KEY
operation_type        TEXT NOT NULL
warehouse_id          BIGINT NOT NULL REFERENCES warehouses(id)
sku_id                BIGINT NOT NULL REFERENCES skus(id)
batch_id              BIGINT NULL REFERENCES inventory_batches(id)
stock_layer_id        BIGINT NULL REFERENCES inventory_stock_layers(id)
reservation_id        BIGINT NULL REFERENCES inventory_reservations(id)
reservation_item_id   BIGINT NULL REFERENCES inventory_reservation_items(id)
qty                   NUMERIC(20,6) NOT NULL
idempotency_key       TEXT NOT NULL
source_type           TEXT NULL
source_id             TEXT NULL
source_line_id        TEXT NULL
created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at            TIMESTAMPTZ NULL
```

建议的 `operation_type` 值：

- `increase`
- `reserve`
- `release_reserved`
- `decrease_available`
- `decrease_reserved`

规则：

- 变动记录是不可变的审计记录。
- 修正必须使用新的变动记录。
- `qty` 必须大于零。
- 对于涉及多个库存层的 FIFO 操作，每个涉及的库存层都要创建一条变动记录。
- 变动方向由 `operation_type` 决定，而不是由负数数量决定。

建议的索引：

- 为 `(warehouse_id, sku_id, created_at)` 创建索引。
- 为 `batch_id` 创建索引。
- 为 `stock_layer_id` 创建索引。
- 为 `reservation_id` 创建索引。
- 为 `(source_type, source_id, source_line_id)` 创建索引。
- 为 `(operation_type, idempotency_key)` 创建索引。

## 库存幂等键

`inventory_idempotency_keys` 存储变更请求的幂等记录。

建议的列：

```text
id                    BIGSERIAL PRIMARY KEY
operation_type        TEXT NOT NULL
idempotency_key       TEXT NOT NULL
request_hash          TEXT NOT NULL
request_payload       JSONB NOT NULL
status                TEXT NOT NULL DEFAULT 'processing'
response_ref_type     TEXT NULL
response_ref_id       BIGINT NULL
response_payload      JSONB NULL
error_code            TEXT NULL
created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at            TIMESTAMPTZ NULL
```

建议的 `status` 值：

- `processing`
- `succeeded`
- `failed`

规则：

- `(operation_type, idempotency_key)` 在未删除的行中必须唯一。
- `request_hash` 是规范化变更请求载荷的 SHA-256 哈希值。
- 使用相同键和相同哈希值重试同一操作时，不得再次应用库存变更。
- 使用相同键但不同哈希值重试同一操作时，必须返回幂等冲突错误。
- 幂等记录、库存更新、预留更新和变动记录必须在同一事务中提交。

建议的索引：

- 在 `deleted_at IS NULL` 条件下为 `(operation_type, idempotency_key)` 创建唯一部分索引。
- 在 `deleted_at IS NULL` 条件下为 `status` 创建索引。

## 幂等载荷哈希

服务应在计算哈希前构建规范化载荷。

规范化载荷规则：

- 仅包含具有业务意义的请求字段。
- 包含 `operation_type`。
- 包含 `warehouse_id`、`sku_id`、可选的 `batch_id` 和数量。
- 提供时包含 `source_type`、`source_id` 和 `source_line_id`。
- 操作释放或消耗预留库存时，包含预留引用。
- 将数量规范化为服务使用的精度，例如 `10.000000`。
- 对缺失的可选值使用 `null`，而不是省略字段。
- 使用稳定的字段名和稳定的字段顺序。

规范化载荷示例：

```json
{
  "operation_type": "reserve",
  "warehouse_id": 1,
  "sku_id": 10,
  "batch_id": null,
  "qty": "10.000000",
  "source_type": "sales_order",
  "source_id": "SO001",
  "source_line_id": "1"
}
```

哈希规则：

```text
request_hash = hex(sha256(canonical_payload_bytes))
```

服务应使用有类型的请求结构体和专用的规范化辅助组件。不得对原始 HTTP 请求字节计算哈希，因为重试之间的字段顺序、空白和省略的 null 可能发生变化。

## 库存操作规则

### 增加库存

影响：

```text
summary.on_hand_qty += qty
layer.on_hand_qty += qty
```

规则：

- 创建新的库存层，或更新由服务定义的库存层。
- 创建变动记录。
- 必须具备幂等性。

### 预留库存

影响：

```text
summary.reserved_qty += qty
layer.reserved_qty += allocated_qty
```

规则：

- 检查可用数量。
- 未提供 `batch_id` 时，按 FIFO 分配库存层。
- 创建 `inventory_reservations`。
- 创建 `inventory_reservation_items`。
- 创建变动记录。
- 必须具备幂等性。

### 释放预留库存

影响：

```text
summary.reserved_qty -= qty
layer.reserved_qty -= released_qty
reservation_item.released_qty += released_qty
```

规则：

- 使用现有预留明细。
- 不重新计算 FIFO。
- 创建变动记录。
- 必须具备幂等性。

### 扣减可用库存

影响：

```text
summary.on_hand_qty -= qty
layer.on_hand_qty -= allocated_qty
```

规则：

- 检查可用数量。
- 未提供 `batch_id` 时，按 FIFO 分配库存层。
- 不更改 `reserved_qty`。
- 创建变动记录。
- 必须具备幂等性。

### 扣减预留库存

影响：

```text
summary.on_hand_qty -= qty
summary.reserved_qty -= qty
layer.on_hand_qty -= consumed_qty
layer.reserved_qty -= consumed_qty
reservation_item.consumed_qty += consumed_qty
```

规则：

- 使用现有预留明细。
- 不重新计算 FIFO。
- 创建变动记录。
- 必须具备幂等性。

## 事务规则

库存变更操作必须在应用服务层事务中运行。

事务必须包含：

- 幂等性检查或插入。
- 锁定库存汇总行或执行原子更新。
- 选择并锁定库存层。
- 更新库存汇总。
- 更新库存层。
- 适用时更新预留主记录和明细。
- 创建变动记录。
- 将幂等记录更新为成功。

仓储不得启动、提交或回滚事务。

## 并发规则

服务必须防止并发请求导致负库存。

建议的数据库处理方式：

- 变更前锁定库存汇总行。
- 变更前锁定选中的库存层。
- 使用能够防止无效数量的 SQL 条件。
- 在同一事务中执行 FIFO 分配。

重要不变量：

```text
on_hand_qty >= 0
reserved_qty >= 0
reserved_qty <= on_hand_qty
```

## 创建顺序

建议的迁移创建顺序：

1. `units`
2. `material_categories`
3. `materials`
4. `material_attribute_definitions`
5. `material_attribute_values`
6. `material_unit_conversions`
7. `warehouses`
8. `skus`
9. `inventory_batches`
10. `inventory_stocks`
11. `inventory_stock_layers`
12. `inventory_reservations`
13. `inventory_reservation_items`
14. `inventory_movements`
15. `inventory_idempotency_keys`

## 待实现事项

- 对状态和操作类型值使用 `CHECK` 约束。
- 为每个外键列添加索引。
- 使用带 `deleted_at IS NULL` 条件的部分唯一索引，实现考虑软删除的唯一性。
- 即使存在数据库约束，业务校验仍应保留在服务层。
- 数据库约束用于保护数据完整性，不能替代领域规则。
