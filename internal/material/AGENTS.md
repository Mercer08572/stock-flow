# Material

The material module owns material master data.

Material is the upstream source for SKU and inventory. It describes what an item is, but it must not describe how a specific SKU is packaged, stocked, reserved, or consumed.

## Scope

The material module is responsible for:

- Material basic information.
- Material category relationship.
- Material base unit.
- Material-level unit conversions.
- Material status.
- Material extensible attributes.
- Material soft delete rules.

The material module is not responsible for:

- SKU definitions.
- Warehouse inventory quantities.
- Batch stock records.
- External business workflows.
- Inventory movement records.

## Common Fields

Material contains only common inventory fields.

Examples:

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

## Extensible Attributes

Category-specific attributes must not be added directly to the `materials` table.

Use material attribute definitions and values for extensibility.

Examples:

- A steel material may need `thickness`, `grade`, or `surface_treatment`.
- A chemical material may need `concentration`, `hazard_level`, or `storage_condition`.
- These fields belong in material attribute definitions and material attribute values, not in the base material table.

## Units and Conversions

`base_unit_id` must reference a unit record. Do not store material units as free text on the `materials` table.

Recommended unit fields:

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

`base_unit_id` represents the material's base measurement unit. Inventory quantities should be normalized to this unit when the business operation requires quantity calculation or comparison.

Material-level unit conversion is required because package units are often material-specific.

Recommended material unit conversion fields:

- `id`
- `material_id`
- `from_unit_id`
- `to_unit_id`
- `factor`
- `created_at`
- `updated_at`
- `deleted_at`

Examples:

- For material A, `1 box = 12 pcs`.
- For material B, `1 box = 24 pcs`.
- Global conversions such as `1 kg = 1000 g` may be shared, but material-specific conversions must be stored at material level.

Conversion rules:

- `from_unit_id` and `to_unit_id` must reference valid unit records.
- `factor` must be greater than zero.
- A conversion pair must be unique per material.
- Conversions must not be duplicated in opposite directions unless the reverse conversion is explicitly needed by the service layer.
- Unit conversion calculation belongs in the material service layer or a domain helper owned by the material module.
- Repositories must only persist and query unit conversion records.

## Boundary Rules

- Material must not depend on SKU, inventory, or warehouse modules.
- SKU may depend on material through material application services or stable application contracts.
- Other modules must not access the material repository directly.
- Cross-module validation, such as checking whether a material exists, must go through the material service layer.

## Layer Rules

Material must follow the project dependency direction:

```text
Handler -> Service -> Repository
```

- Handler parses HTTP input and returns unified responses.
- Service owns material business rules and use cases.
- Repository owns persistence logic only.

Transactions, when needed, must be started and completed in the service layer.

## Business Rules

- `code` must uniquely identify a material.
- `name` should be human-readable and should not be used as a unique business identifier.
- `category_id` is required when category-based attribute definitions are used.
- `base_unit_id` represents the default unit for material-level measurement and must reference the unit table.
- Material-level unit conversions must be maintained when a material can be operated in units other than its base unit.
- `status` controls whether a material can be used by downstream modules.
- Soft-deleted materials must not be returned by default list or detail queries.
- A material that is already used by downstream SKU or inventory data should not be hard deleted.

## API Rules

Material APIs must use plural resource names under `/api/v1`.

Expected resource path:

```text
/api/v1/materials
```

Standard operations:

- `GET /api/v1/materials`: list materials.
- `GET /api/v1/materials/:id`: get material detail.
- `POST /api/v1/materials`: create material.
- `PUT /api/v1/materials/:id`: update material.
- `DELETE /api/v1/materials/:id`: soft delete material.

All responses must use the `pkg/response` package.
