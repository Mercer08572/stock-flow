# Inventory

The inventory module owns stock state, stock operation rules, stock reservations, batch allocation, and stock movement records.

Inventory is designed as an independent inventory capability/service. It exposes simple stock operation APIs to external business systems and does not own purchase, sales, inbound order, outbound order, approval, shipping, or other workflow modules.

## Purpose

Inventory answers two questions:

- How much stock exists in the system right now?
- How should stock change safely when an external business operation increases, reserves, releases, or decreases stock?

## Scope

The inventory module is responsible for:

- Querying current stock.
- Tracking stock by `warehouse + SKU`.
- Tracking stock by `warehouse + SKU + batch` when batch information exists.
- Increasing stock.
- Reserving stock.
- Releasing reserved stock.
- Decreasing stock directly from available stock.
- Decreasing stock from previously reserved stock.
- FIFO batch allocation when no batch is specified.
- Recording inventory movement history.
- Enforcing idempotency for mutation operations.
- Preventing negative stock, negative reservations, and over-reservation.

The inventory module is not responsible for:

- Material master data.
- SKU definitions.
- Material-level unit conversion definitions.
- Purchase orders.
- Sales orders.
- Inbound order workflows.
- Outbound order workflows.
- Approval or shipping workflows.

## Inventory Dimensions

Inventory currently supports two dimensions:

```text
warehouse + SKU
warehouse + SKU + batch
```

Rules:

- `warehouse_id` is required for every inventory query and mutation.
- `sku_id` is required for every inventory query and mutation.
- `batch_id` is optional.
- If `batch_id` is provided, the operation targets that batch.
- If `batch_id` is not provided for stock-out operations, inventory must allocate stock by FIFO.
- FIFO allocation must be based on stable batch ordering, such as `received_at`, then `batch_id` as a deterministic tiebreaker.
- Stock may be queried as a warehouse + SKU summary or as warehouse + SKU + batch details.

If both summary stock and batch stock are stored, they must stay consistent in the same transaction.

```text
warehouse_sku.on_hand_qty = sum(batch.on_hand_qty)
warehouse_sku.reserved_qty = sum(batch.reserved_qty)
```

Non-batch stock may be represented as a stock row with no `batch_id`. If non-batch stock participates in FIFO allocation, it must also have a stable received time.

## Quantity Model

Inventory must not store only one generic quantity field.

Recommended quantity fields:

- `on_hand_qty`: physical stock quantity.
- `reserved_qty`: quantity already reserved by external business operations.
- `available_qty`: calculated as `on_hand_qty - reserved_qty`.

`available_qty` should be calculated by default instead of stored. If it is stored later for performance, the service layer must keep it consistent in the same transaction as `on_hand_qty` and `reserved_qty`.

Core invariants:

- `on_hand_qty >= 0`
- `reserved_qty >= 0`
- `reserved_qty <= on_hand_qty`
- `available_qty >= 0`

## Stock Operations

Inventory exposes simple mutation operations to external business systems.

Recommended service methods:

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

### Increase Stock

Increase stock adds physical stock.

```text
on_hand_qty += qty
reserved_qty unchanged
```

Rules:

- `qty` must be greater than zero.
- `warehouse_id` and `sku_id` are required.
- `batch_id` is optional.
- If `batch_id` is provided, increase that batch stock.
- If `batch_id` is not provided, increase non-batch stock or the service-defined default stock bucket.

### Reserve Stock

Reserve stock locks available stock for an external business operation.

```text
reserved_qty += qty
available_qty = on_hand_qty - reserved_qty
```

Rules:

- `qty` must be greater than zero.
- Available quantity must be greater than or equal to `qty`.
- If `batch_id` is provided, reserve from that batch.
- If `batch_id` is not provided, reserve by FIFO across eligible stock rows.
- Reservation must create a movement record.

### Release Reserved Stock

Release reserved stock unlocks stock that was previously reserved.

```text
reserved_qty -= qty
available_qty = on_hand_qty - reserved_qty
```

Rules:

- `qty` must be greater than zero.
- Reserved quantity must be greater than or equal to `qty`.
- Release should reference the original reservation when the external business system can provide it.
- If the reservation was allocated across multiple batches, release must reverse the allocated batch quantities consistently.
- Release must create a movement record.

### Decrease Available Stock

Decrease available stock directly consumes stock that was not previously reserved.

```text
on_hand_qty -= qty
reserved_qty unchanged
```

Rules:

- `qty` must be greater than zero.
- Available quantity must be greater than or equal to `qty`.
- If `batch_id` is provided, decrease from that batch.
- If `batch_id` is not provided, decrease by FIFO across eligible stock rows.
- This operation is used for direct stock consumption without a prior reservation.
- Decrease must create a movement record.

### Decrease Reserved Stock

Decrease reserved stock confirms consumption of stock that was previously reserved.

```text
on_hand_qty -= qty
reserved_qty -= qty
```

Rules:

- `qty` must be greater than zero.
- Reserved quantity must be greater than or equal to `qty`.
- This operation should reference the original reservation when possible.
- If the reservation was allocated across multiple batches, decrease must follow the original reservation allocation.
- This operation is used after a successful reservation when the external business operation is confirmed.
- Decrease must create a movement record.

## Idempotency Rules

All inventory mutation operations must be idempotent.

Mutation operations include:

- Increase stock.
- Reserve stock.
- Release reserved stock.
- Decrease available stock.
- Decrease reserved stock.

Each mutation request must include an idempotency key.

Recommended fields:

- `idempotency_key`
- `operation_type`
- `source_type`
- `source_id`
- `source_line_id`

Rules:

- The same `idempotency_key` and `operation_type` must not apply the same stock change more than once.
- If the same idempotency key is retried with the same request payload, the service should return the original result or treat it as a no-op success.
- If the same idempotency key is retried with a different request payload, the service must return an idempotency conflict error.
- Idempotency records, stock balance updates, and movement records must be written in the same transaction.

## Movement Records

Every inventory mutation must create an inventory movement record.

Recommended movement fields:

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

Movement records are audit records. They must not be updated to correct stock state. Corrections must be represented by a new movement operation.

## Boundary Rules

- Inventory may depend on SKU through SKU application services or stable application contracts.
- Inventory may validate warehouse existence through a warehouse application service or stable application contract when a warehouse module exists.
- Inventory must not access material repositories directly.
- External business systems must not write inventory repositories or stock tables directly.
- External business systems must use inventory application services or HTTP APIs for stock changes.
- Stock increase, reserve, release, and decrease logic must not be duplicated outside the inventory module.

## Layer Rules

Inventory must follow the project dependency direction:

```text
Handler -> Service -> Repository
```

- Handler parses HTTP input and returns unified responses.
- Service owns inventory business rules, FIFO allocation, idempotency checks, and stock operation orchestration.
- Repository owns persistence logic only.

Transactions must be started and completed in the service layer.

## Transaction And Concurrency Rules

Inventory mutations must be atomic.

The service layer must include these changes in one transaction:

- Idempotency check or insert.
- Stock row selection and locking.
- Stock balance update.
- Batch allocation update when applicable.
- Movement record creation.

Concurrency rules:

- Stock rows selected for mutation must be locked or updated using atomic SQL conditions.
- The service must prevent negative `on_hand_qty`, negative `reserved_qty`, and negative `available_qty`.
- FIFO allocation must lock all selected batch rows before applying updates.
- Repository methods must not start, commit, or roll back transactions.

## API Rules

Inventory APIs must use plural resource names or clear action resources under `/api/v1`.

Expected query resource:

```text
/api/v1/inventory/stocks
```

Expected mutation resources:

```text
POST /api/v1/inventory/increase
POST /api/v1/inventory/reserve
POST /api/v1/inventory/release
POST /api/v1/inventory/decrease-available
POST /api/v1/inventory/decrease-reserved
```

All responses must use the `pkg/response` package.

## Prohibited Patterns

- Updating stock without `warehouse_id`.
- Updating stock without `sku_id`.
- Applying a mutation without an idempotency key.
- Allowing `reserved_qty` to exceed `on_hand_qty`.
- Decreasing stock from available quantity when the stock was previously reserved.
- Decreasing stock from reserved quantity without reducing `reserved_qty`.
- Recomputing FIFO order in a repository without service-layer rules.
- Updating inventory tables directly from material, SKU, warehouse, or external business modules.
