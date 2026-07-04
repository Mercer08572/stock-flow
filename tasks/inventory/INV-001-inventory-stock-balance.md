# INV-001 Inventory Stock Balance

Status: Done
Owner: coding-agent
Module: inventory
Related:
- internal/inventory/
- internal/inventory/AGENTS.md
- internal/sku/
- internal/warehouse/
- internal/shared/http/router.go
- sqlc.yaml
- sql/queries/
- sql/schema/schema.sql
- migrations/202606120003_create_inventory_core_tables.up.sql
- docs/database/schema-design.md
- docs/architecture/module-boundaries.md
- docs/architecture/dependency-rules.md
- tasks/sku/SKU-001-sku-crud.md
- tasks/warehouse/WH-001-warehouse-crud.md

## Background

Inventory stock balance is the first inventory module slice. The database already has `inventory_stocks` for warehouse + SKU summary balances and `inventory_stock_layers` for FIFO/batch-level stock layers, but the project currently has no inventory application code, sqlc query file, repository, service, handler, or route registration.

This task establishes the read model and internal balance foundation for inventory. It should make current stock queryable without implementing stock mutation operations yet.

## Goal

Implement stock balance query APIs under `/api/v1/inventory/stocks`.

The completed task should support:

- Listing stock balances.
- Getting stock balance for one `warehouse_id + sku_id`.
- Getting stock balance with optional batch/layer details.
- Calculating `available_qty = on_hand_qty - reserved_qty`.
- Returning zero balance when a valid warehouse + SKU has no stock row yet, if the detail query is explicitly for that pair.

## Non-Goals

- Do not implement stock increase.
- Do not implement reservation, release, decrease available, or decrease reserved operations.
- Do not implement FIFO allocation.
- Do not create movement records.
- Do not implement idempotency records.
- Do not create or update stock rows through public CRUD APIs.
- Do not implement inventory batch CRUD.
- Do not modify existing migration files.
- Do not introduce an ORM or any repository pattern that bypasses sqlc.

## Scope

Allowed to modify or add:

- `internal/inventory/`
- stable application contracts needed for warehouse/SKU read validation, if they do not violate module boundaries
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `sql/queries/inventory_stocks.sql`
- `sqlc.yaml`
- generated sqlc files under `internal/inventory/db/`
- inventory stock balance focused tests

Should not modify:

- `internal/material/`
- `internal/warehouse/` repository internals
- `internal/sku/` repository internals
- existing migration files
- `pkg/response` response format

## Domain Rules

- `warehouse_id` and `sku_id` are required for detail stock queries.
- List queries may filter by `warehouse_id` and/or `sku_id`.
- `batch_id` may be used only for layer/detail filtering.
- Inventory query APIs may include inactive warehouses.
- Inventory query APIs may include inactive SKUs.
- Stock mutation rules about active warehouse/SKU status are out of scope for INV-001.
- `on_hand_qty` must be returned as an exact decimal string.
- `reserved_qty` must be returned as an exact decimal string.
- `available_qty` must be calculated as `on_hand_qty - reserved_qty` and returned as an exact decimal string.
- `available_qty` must not be stored by this task.
- Summary stock is represented by `inventory_stocks`.
- Layer stock is represented by `inventory_stock_layers`.
- Non-deleted stock rows only are returned.
- Non-deleted stock layers only are returned.
- When detail query targets a valid warehouse + SKU with no `inventory_stocks` row, return a zero balance instead of not found.
- When warehouse or SKU does not exist, return a validation/reference error.
- If `batch_id` is provided, it must reference a valid non-deleted batch for the same SKU.

## API

All endpoints are under `/api/v1`.

- `GET /inventory/stocks`
  - List stock balances.
  - Optional filters: `warehouse_id`, `sku_id`.
  - Pagination: `limit`, `offset`.
- `GET /inventory/stocks/:warehouse_id/:sku_id`
  - Get summary stock balance for one warehouse + SKU.
  - Optional query: `include_layers=true`.
- `GET /inventory/stocks/:warehouse_id/:sku_id/layers`
  - List stock layers for one warehouse + SKU.
  - Optional filters: `batch_id`.
  - Pagination: `limit`, `offset`.

All responses must use `pkg/response`.

## Response Shape

Summary stock should expose:

- `warehouse_id`
- `sku_id`
- `on_hand_qty`
- `reserved_qty`
- `available_qty`
- `updated_at`

Layer stock should expose:

- `id`
- `warehouse_id`
- `sku_id`
- `batch_id`
- `received_at`
- `on_hand_qty`
- `reserved_qty`
- `available_qty`
- `created_at`
- `updated_at`

Use string values for decimal quantities to preserve exact precision.

## Data Model

Use the existing tables from `migrations/202606120003_create_inventory_core_tables.up.sql`.

Summary table:

- `inventory_stocks`

Layer table:

- `inventory_stock_layers`

Batch reference table:

- `inventory_batches`

No new migration is expected for this task unless a schema mismatch is discovered.

Required sqlc work:

- Add `sql/queries/inventory_stocks.sql`.
- Add a new `sql:` block in `sqlc.yaml` for the inventory module.
- Generate code into `internal/inventory/db`.
- Use package name `inventorydb`.
- Keep generated files unedited after generation.

Expected queries:

- `ListInventoryStocks :many`
- `GetInventoryStock :one`
- `ListInventoryStockLayers :many`
- `WarehouseExistsForInventory :one`
- `SKUExistsForInventory :one`
- `InventoryBatchExistsForSKU :one`

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Keep sqlc row types inside the repository layer; do not expose them to handler or service.
- Service owns validation, reference checks, zero-balance behavior, and decimal output rules.
- Repository owns sqlc calls, row mapping, and PostgreSQL error mapping only.
- Handler owns HTTP parsing and response writing only.
- Add inventory route registration to `internal/shared/http/router.go`.
- Inventory must not access material repositories directly.
- Inventory should validate warehouse and SKU existence through stable application contracts where available.
- If warehouse/SKU stable validation contracts are not available, implement the smallest application-level contract needed without exposing cross-module repository access.
- Do not start transaction scaffolding in INV-001 unless a repository contract requires it. Mutation transaction rules belong to later inventory operation tasks.

## Acceptance Criteria

- `GET /api/v1/inventory/stocks` lists non-deleted summary stock rows.
- `GET /api/v1/inventory/stocks?warehouse_id=1&sku_id=2` filters summary stock rows.
- `GET /api/v1/inventory/stocks/:warehouse_id/:sku_id` returns summary stock with calculated `available_qty`.
- Detail query for a valid warehouse + SKU with no stock row returns zero quantities.
- Detail query with missing warehouse or SKU returns validation/reference error.
- `GET /api/v1/inventory/stocks/:warehouse_id/:sku_id/layers` lists non-deleted layers in FIFO order by `received_at`, then `id`.
- Layer response includes calculated `available_qty`.
- Invalid IDs, invalid pagination values, missing warehouse, missing SKU, and invalid batch for SKU return errors.
- Handler tests cover successful summary detail and at least one error mapping.
- Service tests cover zero-balance behavior, decimal available quantity calculation, list filter normalization, reference validation, and layer filtering.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Open Questions

- Decision: INV-001 is read/query only. Stock mutation APIs begin in later inventory tasks.
- Decision: Detail query for a valid warehouse + SKU with no stock row returns zero balance, because no stock is a valid current balance.
- Decision: Inactive warehouses and SKUs remain queryable. Mutation-time active checks belong to stock operation tasks.
- Decision: Decimal quantities are represented as strings in API responses to avoid precision loss.
