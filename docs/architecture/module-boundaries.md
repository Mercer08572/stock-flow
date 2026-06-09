# Module Boundaries

Stock-Flow is a modular monolith. Each business domain must be implemented as an independent module with clear ownership of its domain model, service logic, and persistence access.

## Main Modules

- `material`: owns material master data and material lifecycle rules.
- `sku`: owns SKU definitions derived from material information.
- `inventory`: owns stock state, quantities, batches, and inventory movements.
- `inbound`: owns inbound order workflows and receiving operations.
- `outbound`: owns outbound order workflows and shipping operations.

## Business Relationship

The core dependency direction is:

```text
material -> sku -> inventory -> inbound/outbound
```

- `material` is the upstream source for material information.
- `sku` depends on material concepts to define sellable or stockable units.
- `inventory` depends on SKU concepts to track stock state.
- `inbound` and `outbound` depend on inventory concepts to change stock through receiving and shipping workflows.

## Boundary Rules

- A module must not access another module's repository directly.
- A module must not skip layers or bypass the dependency direction.
- Cross-module communication must happen through application services or an anti-corruption layer.
- Downstream modules may depend on upstream application contracts when needed.
- Upstream modules must not depend on downstream modules.
- Shared code belongs in `internal/shared` or `pkg` only when it is truly generic and has no business ownership.

## Prohibited Examples

- `outbound` directly querying `sku` repository.
- `inventory` directly updating `material` tables.
- `handler` in one module calling a repository in another module.
- `repository` in any module calling another module's service.

## Allowed Examples

- `inbound` service calls an `inventory` application service to increase stock.
- `outbound` service calls an `inventory` application service to reserve or decrease stock.
- `inventory` service uses a `sku` application contract to validate SKU existence.
