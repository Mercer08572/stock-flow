# SKU-001 SKU CRUD

Status: Done
Owner: coding-agent
Module: sku
Related:
- internal/sku/
- internal/sku/AGENTS.md
- internal/material/
- internal/shared/http/router.go
- sqlc.yaml
- sql/queries/
- sql/schema/schema.sql
- migrations/202606120002_create_skus_and_warehouses_tables.up.sql
- docs/database/schema-design.md
- docs/architecture/module-boundaries.md
- docs/architecture/dependency-rules.md

## Background

SKU master data is required before inventory stock operations can be implemented. Inventory records must include `sku_id`, but the project currently only has SKU schema and module rules. There is no SKU application code, sqlc query file, repository, service, handler, or route registration yet.

This task implements the first SKU application slice using the same architecture and testing style already used by the material and warehouse modules.

## Goal

Implement SKU master data management under `/api/v1/skus`.

The completed task should support:

- Listing SKUs.
- Getting a SKU by ID.
- Creating a SKU.
- Updating a SKU.
- Soft deleting a SKU.

## Non-Goals

- Do not implement inventory stock, reservation, movement, FIFO, or idempotency logic.
- Do not implement warehouse APIs.
- Do not implement material CRUD changes.
- Do not implement material attribute APIs or material unit conversion CRUD unless a stable material validation contract requires a small addition.
- Do not add inventory quantity fields to SKU records.
- Do not hard delete SKU records.
- Do not modify existing migration files.
- Do not introduce an ORM or any repository pattern that bypasses sqlc.

## Scope

Allowed to modify or add:

- `internal/sku/`
- material application contracts needed by SKU validation, if they do not violate module boundaries
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `sql/queries/skus.sql`
- `sqlc.yaml`
- generated sqlc files under `internal/sku/db/`
- SKU-focused tests

Should not modify:

- `internal/inventory/`
- `internal/warehouse/`
- existing migration files
- `pkg/response` response format

## Domain Rules

- `code` must uniquely identify a non-deleted SKU.
- `code` is required and must be trimmed by the service layer.
- `name` is required and must be trimmed by the service layer.
- `material_id` is required and must reference a valid non-deleted material.
- `unit_id` is required and must reference a valid non-deleted unit.
- `status` must be `active` or `inactive`.
- If `status` is omitted during create, default to `active`.
- Optional text fields should be trimmed; blank optional text should be stored as `nil`.
- Current business stage must enforce at most one active SKU per material.
- Future one-material-to-many-SKUs must remain possible; do not design APIs or service names around a permanent one-to-one assumption.
- `unit_id` must be the material's `base_unit_id` or a unit that can be converted through material-level unit conversion.
- Cross-module validation must go through material application services or stable material application contracts; SKU must not access material repositories directly.
- Soft-deleted SKUs must not be returned by default list or detail queries.
- Delete must be implemented as soft delete by setting `deleted_at`.
- SKU records must not contain `warehouse_id`, `batch_id`, `on_hand_qty`, `reserved_qty`, or `available_qty`.

## API

All endpoints are under `/api/v1`.

- `GET /skus`
  - List SKUs.
  - Optional filters: `status`, `material_id`, `unit_id`.
  - Pagination: `limit`, `offset`.
- `GET /skus/:id`
  - Get SKU detail.
- `POST /skus`
  - Create SKU.
- `PUT /skus/:id`
  - Update SKU.
- `DELETE /skus/:id`
  - Soft delete SKU.

All responses must use `pkg/response`.

## Data Model

Use the existing `skus` table from `migrations/202606120002_create_skus_and_warehouses_tables.up.sql`.

Expected columns:

- `id`
- `material_id`
- `code`
- `name`
- `unit_id`
- `status`
- `remark`
- `created_at`
- `updated_at`
- `deleted_at`

No new migration is expected for this task unless a schema mismatch is discovered.

Required sqlc work:

- Add `sql/queries/skus.sql`.
- Add a new `sql:` block in `sqlc.yaml` for the SKU module.
- Generate code into `internal/sku/db`.
- Use package name `skudb`.
- Keep generated files unedited after generation.

Expected queries:

- `ListSKUs :many`
- `GetSKUByID :one`
- `CreateSKU :one`
- `UpdateSKU :one`
- `SoftDeleteSKU :execrows`
- `SKUCodeExists :one`
- `ActiveSKUExistsForMaterial :one`

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Service owns validation, trimming, default values, duplicate checks, reference checks, and business rules.
- Repository owns sqlc calls, row mapping, and PostgreSQL error mapping only.
- Handler owns HTTP parsing and response writing only.
- Add SKU route registration to `internal/shared/http/router.go`.
- Follow the existing style of `internal/material/material`, `internal/material/unit`, `internal/material/category`, and `internal/warehouse`.
- Keep sqlc row types inside the repository layer; do not expose them to handler or service.
- Map duplicate SKU code to a domain error and HTTP conflict.
- Map active-SKU-for-material conflicts to a domain error and HTTP conflict.
- Map missing material or missing unit references to domain errors and HTTP bad request.
- Map not found to HTTP not found.
- Map validation errors to HTTP bad request.
- Do not call material repositories from `internal/sku`; create or use material service contracts for cross-module validation.

## Acceptance Criteria

- `GET /api/v1/skus` lists non-deleted SKUs.
- `GET /api/v1/skus/:id` returns SKU detail or not found.
- `POST /api/v1/skus` creates a SKU with normalized input.
- `PUT /api/v1/skus/:id` updates a SKU with normalized input.
- `DELETE /api/v1/skus/:id` soft deletes a SKU.
- Duplicate non-deleted `code` cannot be created or updated.
- More than one active SKU for the same material cannot be created or updated in the current stage.
- Invalid `status`, blank `code`, blank `name`, invalid IDs, missing material, missing unit, and invalid SKU unit rules return errors.
- Handler tests cover successful create and at least one error mapping.
- Service tests cover create normalization, duplicate code, active-SKU-for-material conflict, required field validation, reference validation, list filter normalization, and delete validation.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Open Questions

- Decision: SKU-001 enforces the current-stage rule of at most one active SKU per material, matching the existing partial unique index.
- Decision: SKU-001 does not implement inventory-aware delete protection. Inventory-aware delete protection will be implemented later through an inventory application service after inventory module APIs exist.
- Decision: SKU unit validation must go through material application contracts. If material unit conversion contracts are not ready, implement the smallest stable contract needed for validating material existence, material base unit, and allowed SKU unit.
