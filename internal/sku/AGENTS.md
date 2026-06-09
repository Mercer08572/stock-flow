# SKU

The SKU module owns stock keeping unit definitions.

In Stock-Flow, SKU is the smallest unit that can be stored and managed in warehouse inventory. SKU is derived from material, but it is not the same concept as material.

Material describes what an item is. SKU describes the specific stockable form used by inventory operations.

## Scope

The SKU module is responsible for:

- SKU basic information.
- The relationship between SKU and material.
- SKU code and name rules.
- SKU status.
- SKU storage unit rules.
- Preparing the model for future one-material-to-many-SKUs support.
- SKU soft delete rules.

The SKU module is not responsible for:

- Material master data.
- Material category attributes.
- Material-level unit conversion definitions.
- Warehouse inventory quantities.
- Batch stock records.
- External business workflows.
- Inventory movement records.

## Relationship With Material

SKU depends on material.

The dependency direction is:

```text
material -> sku -> inventory
```

Current business rule:

- One material maps to one SKU.

Future extension rule:

- One material may map to multiple SKUs.

The data model and service API should not make one-material-to-one-SKU impossible to evolve later.

For example, prefer a SKU table with `material_id` on each SKU record:

```text
skus.material_id -> materials.id
```

Do not design the material table as if it owns a single fixed `sku_id`.

## Common Fields

SKU contains only fields that describe the stockable unit.

Examples:

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

`unit_id` represents the unit used by this SKU for stock operations. It must reference a valid unit record.

## Unit Rules

SKU unit rules must respect material unit rules.

- A SKU must be linked to one material.
- A SKU `unit_id` must be either the material's `base_unit_id` or a unit that can be converted through material-level unit conversion.
- Inventory quantity calculations must use SKU unit rules consistently.
- Unit conversion validation must go through material application services or stable material application contracts.
- SKU must not duplicate material-level unit conversion definitions.

Examples:

- Material A base unit is `pcs`; SKU A can also use `box` only if material A has a conversion such as `1 box = 12 pcs`.
- Material B base unit is `pcs`; SKU B can use `box` with a different conversion such as `1 box = 24 pcs`.

## Boundary Rules

- SKU may depend on material through material application services or stable application contracts.
- SKU must not access the material repository directly.
- SKU must not depend on inventory or warehouse modules.
- Inventory may depend on SKU through SKU application services or stable application contracts.
- Other modules must not access the SKU repository directly.
- Cross-module validation, such as checking whether a SKU exists or is active, must go through the SKU service layer.

## Layer Rules

SKU must follow the project dependency direction:

```text
Handler -> Service -> Repository
```

- Handler parses HTTP input and returns unified responses.
- Service owns SKU business rules and use cases.
- Repository owns persistence logic only.

Transactions, when needed, must be started and completed in the service layer.

## Business Rules

- `code` must uniquely identify a SKU.
- `material_id` is required.
- `unit_id` is required.
- Current implementation should enforce at most one active SKU per material.
- Future implementation may allow multiple active SKUs per material.
- `name` should be human-readable and should not be used as a unique business identifier.
- `status` controls whether a SKU can be used by downstream inventory operations.
- Soft-deleted SKUs must not be returned by default list or detail queries.
- A SKU that is already used by inventory or movement records should not be hard deleted.

## Extension Rules

Because one material may support multiple SKUs in the future:

- Do not assume `material_id` is globally unique forever unless the constraint is clearly marked as a current-stage business rule.
- Keep service methods named around SKU behavior, not only material behavior.
- Avoid APIs that imply a material can never have more than one SKU.
- If a current API creates a default SKU for a material, name and document it as a current shortcut.

## API Rules

SKU APIs must use plural resource names under `/api/v1`.

Expected resource path:

```text
/api/v1/skus
```

Standard operations:

- `GET /api/v1/skus`: list SKUs.
- `GET /api/v1/skus/:id`: get SKU detail.
- `POST /api/v1/skus`: create SKU.
- `PUT /api/v1/skus/:id`: update SKU.
- `DELETE /api/v1/skus/:id`: soft delete SKU.

All responses must use the `pkg/response` package.

## Prohibited Patterns

- Storing SKU fields directly on the `materials` table.
- Accessing the material repository from the SKU module.
- Accessing inventory repositories from the SKU module.
- Creating inventory quantity records inside SKU services.
- Designing APIs or database ownership around permanent one-material-to-one-SKU assumptions.
