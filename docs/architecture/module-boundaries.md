# Module Boundaries

Stock-Flow is a modular monolith. Each business domain must be implemented as an independent module with clear ownership of its domain model, service logic, and persistence access.

## Main Modules

- `material`: owns material master data and material lifecycle rules.
- `sku`: owns SKU definitions derived from material information.
- `inventory`: owns stock state, quantities, batches, and inventory movements.
- `warehouse`: a logical inventory dimension. It identifies where stock is stored and must be included in inventory balance and movement operations.

## Business Relationship

The core dependency direction is:

```text
material -> sku -> inventory(warehouse, batch)
```

- `material` is the upstream source for material information.
- `sku` depends on material concepts to define sellable or stockable units.
- `inventory` depends on SKU concepts to track stock state.
- `warehouse` is a logical dimension inside inventory, not an independent upstream business module.
- Inventory quantity must be tracked by warehouse and SKU at minimum.
- Batch-level inventory is optional for a stock operation, but when batch information exists inventory must also support warehouse + SKU + batch tracking.
- External business systems call inventory application services to increase, reserve, release, or decrease stock.

## Boundary Rules

- A module must not access another module's repository directly.
- A module must not skip layers or bypass the dependency direction.
- Cross-module communication must happen through application services or an anti-corruption layer.
- Downstream modules may depend on upstream application contracts when needed.
- Upstream modules must not depend on downstream modules.
- Warehouse and batch stock rules must be accessed through inventory application services, not through direct repository access by external business code.
- This service does not own inbound or outbound order modules.
- Shared code belongs in `internal/shared` or `pkg` only when it is truly generic and has no business ownership.

## Prohibited Examples

- `inventory` directly updating `material` tables.
- External business code directly updating inventory, warehouse stock, or batch stock tables.
- Updating inventory quantity without a warehouse identifier.
- Duplicating stock increase, reserve, release, or decrease logic outside the inventory service.
- `handler` in one module calling a repository in another module.
- `repository` in any module calling another module's service.

## Allowed Examples

- External business services call inventory application services to increase stock.
- External business services call inventory application services to reserve, release, or decrease stock.
- External business services pass a warehouse identifier and SKU identifier to inventory application services.
- `inventory` service validates that a warehouse exists before changing stock.
- `inventory` service uses a `sku` application contract to validate SKU existence.
