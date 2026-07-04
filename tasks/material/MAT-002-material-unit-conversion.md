# MAT-002 Material Unit Conversion

Status: Done
Owner: coding-agent
Module: material
Related:
- internal/material/
- internal/material/AGENTS.md
- internal/shared/http/router.go
- sqlc.yaml
- sql/queries/
- sql/schema/schema.sql
- migrations/202606110001_create_basic_master_data_tables.up.sql
- docs/database/schema-design.md
- docs/architecture/module-boundaries.md
- docs/architecture/dependency-rules.md
- tasks/sku/SKU-001-sku-crud.md

## Background

SKU unit rules require a SKU `unit_id` to be either the material's `base_unit_id` or a unit that can be converted through material-level unit conversion. The database already has the `material_unit_conversions` table, and the SKU module already depends on material application contracts for SKU unit validation.

However, the material module currently does not expose APIs for maintaining material-specific unit conversions. Without this slice, users can create SKUs in a material's base unit, but cannot maintain package units such as `box`, `pack`, `bag`, or material-specific alternate units.

This task implements material unit conversion management inside the material module.

## Goal

Implement material-specific unit conversion management under `/api/v1/materials/:material_id/unit-conversions`.

The completed task should support:

- Listing conversions for a material.
- Getting a conversion by ID.
- Creating a conversion.
- Updating a conversion.
- Soft deleting a conversion.

## Non-Goals

- Do not implement global unit conversion rules.
- Do not implement SKU APIs.
- Do not implement inventory quantity calculation, stock operation, reservation, or movement logic.
- Do not update existing SKU records when conversions change.
- Do not add conversion fields directly to `materials`, `units`, or `skus`.
- Do not implement conversion graph search or multi-hop conversion.
- Do not modify existing migration files.
- Do not introduce an ORM or any repository pattern that bypasses sqlc.

## Scope

Allowed to modify or add:

- `internal/material/`
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `sql/queries/material_unit_conversions.sql` or an equivalent material query file
- `sqlc.yaml`, if a new query file needs to be included
- generated sqlc files under `internal/material/db/`
- material unit conversion focused tests

Should not modify:

- `internal/sku/`, except if a compile-only interface adjustment is strictly required
- `internal/inventory/`
- `internal/warehouse/`
- existing migration files
- `pkg/response` response format

## Domain Rules

- `material_id` is required and must reference a valid non-deleted material.
- `from_unit_id` is required and must reference a valid non-deleted unit.
- `to_unit_id` is required and must reference a valid non-deleted unit.
- `from_unit_id` and `to_unit_id` must be different.
- `factor` is required and must be greater than zero.
- `factor` represents conversion from `from_unit_id` to `to_unit_id`.
- A conversion pair `(material_id, from_unit_id, to_unit_id)` must be unique among non-deleted conversions.
- The service layer must prevent creating the exact reverse pair unless reverse conversion is explicitly needed by a later task.
- The material's `base_unit_id` should normally be one side of the conversion pair for SKU validation to remain simple and predictable.
- Conversion validation belongs in the material service layer or material-owned domain helpers.
- Repository code must only persist/query conversion records and map database errors.
- Soft-deleted conversions must not be returned by default list or detail queries.
- Delete must be implemented as soft delete by setting `deleted_at`.
- Use exact decimal handling. Do not use `float32` or `float64` for `factor`.

## API

All endpoints are under `/api/v1`.

- `GET /materials/:material_id/unit-conversions`
  - List conversions for one material.
  - Optional filters: `from_unit_id`, `to_unit_id`.
  - Pagination: `limit`, `offset`.
- `GET /materials/:material_id/unit-conversions/:id`
  - Get conversion detail.
- `POST /materials/:material_id/unit-conversions`
  - Create conversion.
- `PUT /materials/:material_id/unit-conversions/:id`
  - Update conversion.
- `DELETE /materials/:material_id/unit-conversions/:id`
  - Soft delete conversion.

All responses must use `pkg/response`.

## Data Model

Use the existing `material_unit_conversions` table from `migrations/202606110001_create_basic_master_data_tables.up.sql`.

Expected columns:

- `id`
- `material_id`
- `from_unit_id`
- `to_unit_id`
- `factor`
- `created_at`
- `updated_at`
- `deleted_at`

No new migration is expected for this task unless a schema mismatch is discovered.

Required sqlc work:

- Add `sql/queries/material_unit_conversions.sql`, or extend the existing material query organization if that better matches the codebase.
- Ensure `sqlc.yaml` includes the conversion query file in the material sql block.
- Generate code into `internal/material/db`.
- Keep generated files unedited after generation.

Expected queries:

- `ListMaterialUnitConversions :many`
- `GetMaterialUnitConversionByID :one`
- `CreateMaterialUnitConversion :one`
- `UpdateMaterialUnitConversion :one`
- `SoftDeleteMaterialUnitConversion :execrows`
- `MaterialUnitConversionExists :one`
- `ReverseMaterialUnitConversionExists :one`

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Keep this slice inside the material module, preferably as a focused subpackage such as `internal/material/conversion` if that matches the existing `category` and `unit` organization.
- Service owns validation, reference checks, duplicate checks, reverse-pair checks, decimal normalization, and business rules.
- Repository owns sqlc calls, row mapping, and PostgreSQL error mapping only.
- Handler owns HTTP parsing and response writing only.
- Do not expose sqlc row types to handler or service callers.
- Map duplicate conversion pair to a domain error and HTTP conflict.
- Map reverse conversion pair conflict to a domain error and HTTP conflict.
- Map missing material or missing unit references to domain errors and HTTP bad request.
- Map not found to HTTP not found.
- Map validation errors to HTTP bad request.
- Preserve the SKU-facing material validation contract that checks whether a SKU unit is allowed for a material.

## Acceptance Criteria

- `GET /api/v1/materials/:material_id/unit-conversions` lists non-deleted conversions for the material.
- `GET /api/v1/materials/:material_id/unit-conversions/:id` returns conversion detail or not found.
- `POST /api/v1/materials/:material_id/unit-conversions` creates a conversion with validated material, units, and factor.
- `PUT /api/v1/materials/:material_id/unit-conversions/:id` updates a conversion with validated material, units, and factor.
- `DELETE /api/v1/materials/:material_id/unit-conversions/:id` soft deletes a conversion.
- Duplicate non-deleted `(material_id, from_unit_id, to_unit_id)` cannot be created or updated.
- Reverse conversion pairs cannot be created or updated in this task.
- Invalid material ID, invalid unit IDs, same from/to unit, non-positive factor, duplicate pair, and reverse pair return errors.
- The SKU unit validation contract can recognize conversions created by this API.
- Handler tests cover successful create and at least one error mapping.
- Service tests cover create validation, duplicate pair, reverse pair, required references, list filter normalization, decimal factor handling, and delete validation.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Open Questions

- Decision: MAT-002 treats conversions as directional and does not auto-create reverse records.
- Decision: MAT-002 blocks reverse pairs for now to avoid ambiguous conversion ownership and rounding behavior.
- Decision: MAT-002 does not update existing SKUs or inventory data when a conversion is deleted; inventory-aware protection can be added later through inventory or SKU application services.
