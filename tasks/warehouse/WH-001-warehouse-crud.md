# WH-001 Warehouse CRUD

Status: Done
Owner: coding-agent
Module: warehouse
Related:
- internal/warehouse/
- internal/shared/http/router.go
- sqlc.yaml
- sql/queries/
- sql/schema/schema.sql
- migrations/202606120002_create_skus_and_warehouses_tables.up.sql
- docs/database/schema-design.md

## Background

Warehouse master data is required before inventory stock operations can be implemented. Inventory records must include `warehouse_id`, but the project currently only has warehouse schema and module rules. There is no warehouse application code, sqlc query file, repository, service, handler, or route registration yet.

This task implements the first warehouse application slice using the same architecture and testing style already used by the material module.

## Goal

Implement warehouse master data management under `/api/v1/warehouses`.

The completed task should support:

- Listing warehouses.
- Getting a warehouse by ID.
- Creating a warehouse.
- Updating a warehouse.
- Soft deleting a warehouse.
- Disabling a warehouse.

## Non-Goals

- Do not implement inventory stock, reservation, movement, FIFO, or idempotency logic.
- Do not implement SKU APIs.
- Do not add warehouse stock quantity fields.
- Do not implement warehouse location/bin hierarchy.
- Do not check whether a warehouse has existing stock before delete through direct inventory repository access.
- Do not modify existing migration files.
- Do not introduce an ORM or any repository pattern that bypasses sqlc.

## Scope

Allowed to modify or add:

- `internal/warehouse/`
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `sql/queries/warehouses.sql`
- `sqlc.yaml`
- generated sqlc files under `internal/warehouse/db/`
- warehouse-focused tests

Should not modify:

- `internal/material/`
- `internal/sku/`
- `internal/inventory/`
- existing migration files
- `pkg/response` response format

## Domain Rules

- `code` must uniquely identify a non-deleted warehouse.
- `code` is required and must be trimmed by the service layer.
- `name` is required and must be trimmed by the service layer.
- `type` must be `normal` or `virtual`.
- If `type` is omitted during create, default to `normal`.
- `status` must be `active` or `inactive`.
- If `status` is omitted during create, default to `active`.
- Optional text fields should be trimmed; blank optional text should be stored as `nil`.
- Soft-deleted warehouses must not be returned by default list or detail queries.
- Delete must be implemented as soft delete by setting `deleted_at`.
- Disable must set `status` to `inactive`; it must not change inventory quantities.
- Warehouse records must not contain `sku_id`, `batch_id`, `on_hand_qty`, `reserved_qty`, or `available_qty`.

## API

All endpoints are under `/api/v1`.

- `GET /warehouses`
  - List warehouses.
  - Optional filters: `status`, `type`.
  - Pagination: `limit`, `offset`.
- `GET /warehouses/:id`
  - Get warehouse detail.
- `POST /warehouses`
  - Create warehouse.
- `PUT /warehouses/:id`
  - Update warehouse.
- `DELETE /warehouses/:id`
  - Soft delete warehouse.
- `PUT /warehouses/:id/disable`
  - Disable warehouse by setting `status = inactive`.

All responses must use `pkg/response`.

## Data Model

Use the existing `warehouses` table from `migrations/202606120002_create_skus_and_warehouses_tables.up.sql`.

Expected columns:

- `id`
- `code`
- `name`
- `type`
- `status`
- `location`
- `contact_name`
- `contact_phone`
- `remark`
- `created_at`
- `updated_at`
- `deleted_at`

No new migration is expected for this task unless a schema mismatch is discovered.

Required sqlc work:

- Add `sql/queries/warehouses.sql`.
- Add a new `sql:` block in `sqlc.yaml` for the warehouse module.
- Generate code into `internal/warehouse/db`.
- Use package name `warehousedb`.
- Keep generated files unedited after generation.

Expected queries:

- `ListWarehouses :many`
- `GetWarehouseByID :one`
- `CreateWarehouse :one`
- `UpdateWarehouse :one`
- `SoftDeleteWarehouse :execrows`
- `DisableWarehouse :one`
- `WarehouseCodeExists :one`

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Service owns validation, trimming, default values, duplicate checks, and business rules.
- Repository owns sqlc calls, row mapping, and PostgreSQL error mapping only.
- Handler owns HTTP parsing and response writing only.
- Add warehouse route registration to `internal/shared/http/router.go`.
- Follow the existing style of `internal/material/material`, `internal/material/unit`, and `internal/material/category`.
- Keep sqlc row types inside the repository layer; do not expose them to handler or service.
- Map duplicate warehouse code to a domain error and HTTP conflict.
- Map not found to HTTP not found.
- Map validation errors to HTTP bad request.

## Acceptance Criteria

- `GET /api/v1/warehouses` lists non-deleted warehouses.
- `GET /api/v1/warehouses/:id` returns warehouse detail or not found.
- `POST /api/v1/warehouses` creates a warehouse with normalized input.
- `PUT /api/v1/warehouses/:id` updates a warehouse with normalized input.
- `DELETE /api/v1/warehouses/:id` soft deletes a warehouse.
- `PUT /api/v1/warehouses/:id/disable` sets warehouse status to inactive.
- Duplicate non-deleted `code` cannot be created or updated.
- Invalid `type`, invalid `status`, blank `code`, blank `name`, and invalid IDs return validation errors.
- Handler tests cover successful create and at least one error mapping.
- Service tests cover create normalization, duplicate code, required field validation, list filter normalization, and disable behavior.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Open Questions

- Decision: WH-001 does not check inventory references during delete. Inventory-aware delete protection will be implemented later through an inventory application service after the inventory module exists.
- Decision: `PUT /api/v1/warehouses/:id/disable` returns the updated warehouse record.
