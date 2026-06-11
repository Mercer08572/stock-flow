# Warehouse

The warehouse module owns warehouse master data.

Warehouse is a required logical dimension for inventory. It describes where stock can be stored, but it does not own stock quantity, batch quantity, reservations, FIFO allocation, or movement records.

## Scope

The warehouse module is responsible for:

- Warehouse basic information.
- Warehouse code and name rules.
- Warehouse type.
- Warehouse status.
- Warehouse location and contact information.
- Warehouse soft delete or disable rules.

The warehouse module is not responsible for:

- SKU definitions.
- Material master data.
- Stock quantities.
- Batch stock records.
- Reserved stock.
- Available stock calculation.
- Inventory movement records.
- FIFO allocation.
- Inventory increase, reserve, release, or decrease operations.

## Common Fields

Warehouse contains only warehouse master data fields.

Examples:

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

Warehouse must not contain inventory quantity fields.

Prohibited fields on warehouse records:

- `sku_id`
- `batch_id`
- `on_hand_qty`
- `reserved_qty`
- `available_qty`

## Relationship With Inventory

Inventory uses warehouse as a required stock dimension.

Inventory records are tracked by:

```text
warehouse + SKU
warehouse + SKU + batch
```

Rules:

- Inventory mutation operations must include `warehouse_id`.
- Inventory is responsible for stock state and stock operation rules.
- Warehouse is responsible only for validating whether a warehouse exists and whether it can participate in stock mutation operations.
- Inventory may depend on warehouse application services or stable warehouse application contracts.
- Warehouse must not update inventory tables directly.
- Warehouse must not calculate inventory quantities.

## Status Rules

Recommended warehouse statuses:

- `active`
- `inactive`

Rules:

- `active` warehouses can participate in inventory query and mutation operations.
- `inactive` warehouses can still be used for inventory queries.
- `inactive` warehouses must not be used for inventory mutation operations.
- Inventory mutation operations include increase, reserve, release, decrease available stock, and decrease reserved stock.
- Disabling a warehouse should not change existing inventory quantities or movement records.

## Delete And Disable Rules

Warehouses should be soft deleted or disabled, not hard deleted.

Rules:

- If a warehouse has any stock records, the warehouse must not be deleted.
- If a warehouse has any stock records, it can only be disabled.
- Stock records include warehouse + SKU stock, warehouse + SKU + batch stock, reservations, or movement records.
- A disabled warehouse may still be queried for historical inventory data.
- A warehouse without stock records may be soft deleted when business rules allow it.
- Hard delete is prohibited for warehouses referenced by inventory or movement records.

Checking whether a warehouse has stock must not be done through direct inventory repository access.

Allowed approaches:

- Use an inventory application service or stable inventory application contract.
- Use a higher-level application service to coordinate warehouse and inventory checks.
- Use database constraints to prevent deleting referenced warehouses.

Prohibited approaches:

- Warehouse repository directly querying inventory tables.
- Warehouse service directly using inventory repositories.
- Deleting a warehouse and leaving inventory records with a dangling `warehouse_id`.

## Boundary Rules

- Warehouse may be used by inventory as a validation dependency.
- Warehouse must not depend on material or SKU modules.
- Warehouse must not own inventory stock operation logic.
- Other modules must not access the warehouse repository directly.
- Cross-module validation, such as checking whether a warehouse exists or is active, must go through the warehouse service layer.

## Layer Rules

Warehouse must follow the project dependency direction:

```text
Handler -> Service -> Repository
```

- Handler parses HTTP input and returns unified responses.
- Service owns warehouse business rules and use cases.
- Repository owns persistence logic only.

Transactions, when needed, must be started and completed in the service layer.

## Business Rules

- `code` must uniquely identify a warehouse.
- `name` should be human-readable and should not be used as a unique business identifier.
- `type` should be kept simple unless warehouse operations require more detail.
- `status` controls whether the warehouse can be used for inventory mutation operations.
- Soft-deleted warehouses must not be returned by default list or detail queries.
- Warehouses referenced by inventory records, reservations, or movement records must not be hard deleted.

## Warehouse Type Rules

Warehouse type can start simple.

Recommended initial types:

- `normal`
- `virtual`

Future types may include:

- quality inspection warehouse
- defective goods warehouse
- return warehouse
- frozen warehouse

Do not introduce warehouse location or bin-level complexity until the business needs it.

If location/bin tracking is needed later, model it as a separate concept such as:

```text
warehouse -> location/bin
```

## API Rules

Warehouse APIs must use plural resource names under `/api/v1`.

Expected resource path:

```text
/api/v1/warehouses
```

Standard operations:

- `GET /api/v1/warehouses`: list warehouses.
- `GET /api/v1/warehouses/:id`: get warehouse detail.
- `POST /api/v1/warehouses`: create warehouse.
- `PUT /api/v1/warehouses/:id`: update warehouse.
- `DELETE /api/v1/warehouses/:id`: soft delete warehouse when allowed.
- `PUT /api/v1/warehouses/:id/disable`: disable warehouse.

Inventory mutation APIs must not be placed under warehouse resources. They belong to the inventory module.

All responses must use the `pkg/response` package.

## Prohibited Patterns

- Storing inventory quantity fields on warehouse records.
- Updating stock inside warehouse services.
- Reserving or releasing stock inside warehouse services.
- Running FIFO allocation inside warehouse services.
- Hard deleting a warehouse that has stock, reservations, or movement records.
- Blocking inventory queries only because a warehouse is inactive.
- Allowing inventory mutation operations against an inactive warehouse.
