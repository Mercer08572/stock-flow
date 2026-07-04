# MAT-001 Material CRUD

Status: Done
Owner: coding-agent
Module: material
Related:
- internal/material/
- internal/material/AGENTS.md
- internal/shared/http/router.go
- sqlc.yaml
- sql/queries/materials.sql
- sql/queries/material_categories.sql
- sql/queries/units.sql
- sql/schema/schema.sql
- migrations/202606110001_create_basic_master_data_tables.up.sql
- docs/database/schema-design.md
- docs/architecture/module-boundaries.md
- docs/architecture/dependency-rules.md
- tasks/material/MAT-002-material-unit-conversion.md

## Background

This document is a historical baseline task. The material master data implementation was completed before the `tasks/` documentation workflow was introduced, so this file records the completed scope instead of requesting new implementation work.

Material is the upstream master data module for SKU and inventory. It owns material definitions, reusable units, and material categories. SKU and inventory depend on this module through application services or stable application contracts.

## Goal

Provide basic material master data management under `/api/v1`.

The completed baseline supports:

- Material CRUD.
- Unit CRUD.
- Material category CRUD.
- Soft delete for material master data records.
- Basic reference validation from material to category and base unit.
- Unified API responses through `pkg/response`.

## Non-Goals

- Do not implement material unit conversion CRUD in MAT-001. That belongs to MAT-002.
- Do not implement material attribute definition or value APIs.
- Do not implement SKU APIs.
- Do not implement inventory stock, reservation, movement, FIFO, or idempotency logic.
- Do not store SKU, warehouse, batch, or inventory quantity fields on material records.
- Do not introduce an ORM or any repository pattern that bypasses sqlc.

## Completed Scope

Implemented areas:

- `internal/material/material/`
- `internal/material/unit/`
- `internal/material/category/`
- `internal/material/db/`
- `internal/shared/http/router.go`
- `sql/queries/materials.sql`
- `sql/queries/material_categories.sql`
- `sql/queries/units.sql`
- generated sqlc files under `internal/material/db/`
- material, unit, and category focused tests

Current database tables:

- `materials`
- `units`
- `material_categories`

## Domain Rules

Material rules:

- `code` must uniquely identify a non-deleted material.
- `code` is required and trimmed by the service layer.
- `name` is required and trimmed by the service layer.
- `category_id` is required and must reference a valid non-deleted material category.
- `base_unit_id` is required and must reference a valid non-deleted unit.
- `status` must be `active` or `inactive`.
- If `status` is omitted during create, default to `active`.
- Optional text fields are trimmed; blank optional text is stored as `nil`.
- Soft-deleted materials are not returned by default list or detail queries.

Unit rules:

- `code` must uniquely identify a non-deleted unit.
- `code`, `name`, and `symbol` are required and trimmed by the service layer.
- `unit_type` must be one of the supported unit types.
- `precision` must be between `0` and `6`.
- `status` must be `active` or `inactive`.
- If `status` is omitted during create, default to `active`.
- Unit records must not store material-specific conversion rules.

Material category rules:

- `code` must uniquely identify a non-deleted category.
- `code` and `name` are required and trimmed by the service layer.
- `parent_id` is optional, but when present must reference a valid category.
- A category cannot use itself as its parent.
- `status` must be `active` or `inactive`.
- If `status` is omitted during create, default to `active`.
- Optional text fields are trimmed; blank optional text is stored as `nil`.

## API

All endpoints are under `/api/v1`.

Materials:

- `GET /materials`
  - Optional filters: `status`, `category_id`.
  - Pagination: `limit`, `offset`.
- `GET /materials/:id`
- `POST /materials`
- `PUT /materials/:id`
- `DELETE /materials/:id`

Units:

- `GET /units`
  - Optional filters: `status`, `unit_type`.
  - Pagination: `limit`, `offset`.
- `GET /units/:id`
- `POST /units`
- `PUT /units/:id`
- `DELETE /units/:id`

Material categories:

- `GET /material-categories`
  - Optional filters: `status`, `parent_id`.
  - Pagination: `limit`, `offset`.
- `GET /material-categories/:id`
- `POST /material-categories`
- `PUT /material-categories/:id`
- `DELETE /material-categories/:id`

All responses use `pkg/response`.

## Data Model

MAT-001 uses the existing basic master data tables from `migrations/202606110001_create_basic_master_data_tables.up.sql`.

Material columns:

- `id`
- `code`
- `name`
- `category_id`
- `base_unit_id`
- `status`
- `remark`
- `created_at`
- `updated_at`
- `deleted_at`

Unit columns:

- `id`
- `code`
- `name`
- `symbol`
- `unit_type`
- `precision`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

Material category columns:

- `id`
- `code`
- `name`
- `parent_id`
- `status`
- `remark`
- `created_at`
- `updated_at`
- `deleted_at`

## Implementation Notes

- The implemented request flow follows `Handler -> Service -> Repository -> PostgreSQL`.
- Layer interfaces are defined before implementations.
- Dependencies are injected through constructors such as `NewService(repo)`.
- Service layer owns validation, trimming, default values, duplicate checks, reference checks, and business rules.
- Repository layer owns sqlc calls, row mapping, and PostgreSQL error mapping.
- Handler layer owns HTTP parsing and response writing.
- sqlc row types stay inside repository code.
- Duplicate code errors map to HTTP conflict.
- Validation and missing reference errors map to HTTP bad request.
- Not found errors map to HTTP not found.

## Acceptance Criteria

- `GET /api/v1/materials` lists non-deleted materials.
- `GET /api/v1/materials/:id` returns material detail or not found.
- `POST /api/v1/materials` creates a material with normalized input.
- `PUT /api/v1/materials/:id` updates a material with normalized input.
- `DELETE /api/v1/materials/:id` soft deletes a material.
- `GET /api/v1/units` lists non-deleted units.
- `POST /api/v1/units` creates a unit with normalized input.
- `DELETE /api/v1/units/:id` soft deletes a unit.
- `GET /api/v1/material-categories` lists non-deleted categories.
- `POST /api/v1/material-categories` creates a category with normalized input.
- `DELETE /api/v1/material-categories/:id` soft deletes a category.
- Duplicate non-deleted `code` cannot be created or updated for materials, units, or categories.
- Invalid IDs, invalid status, blank required fields, and missing references return errors.
- Handler and service tests cover the main success and error paths.
- `go test ./...` passed for the completed implementation at the time this baseline was recorded.

## Open Questions

- Decision: MAT-001 is a historical baseline document. It records completed implementation and should not be executed as a new task.
- Decision: Material unit conversion CRUD is intentionally excluded and tracked by MAT-002.
- Decision: Material attributes are intentionally excluded and should be tracked by a later material task.
