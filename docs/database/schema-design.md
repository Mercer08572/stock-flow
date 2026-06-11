# Database Schema Design

This document describes the logical database schema for Stock-Flow.

The schema follows the current module boundaries:

```text
material -> sku -> inventory(warehouse, batch)
```

This service does not own inbound or outbound order modules. External business systems call inventory operation APIs to increase, reserve, release, or decrease stock.

## Design Decisions

- `available_qty` is not stored as a normal column. It is calculated as `on_hand_qty - reserved_qty`.
- Batch is modeled as an independent table.
- Reservation uses a header table and allocation item table.
- FIFO allocation is recorded during reservation or stock decrease.
- Idempotency uses `operation_type + idempotency_key + request_hash`.
- Stock quantities use `NUMERIC(20,6)`.
- Unit conversion factors use `NUMERIC(24,10)`.
- Primary keys use `BIGSERIAL`.
- Foreign keys use `BIGINT`.
- Enum-like values use `TEXT` with `CHECK` constraints.
- Soft delete uses `deleted_at TIMESTAMPTZ NULL`.

## Numeric Rules

Use exact decimal types for inventory and unit conversion data.

Recommended types:

- Stock quantity: `NUMERIC(20,6)`
- Unit conversion factor: `NUMERIC(24,10)`

Do not use `FLOAT`, `REAL`, or `DOUBLE PRECISION` for stock quantities or conversion factors.

`available_qty` should be calculated in SQL or application code:

```sql
on_hand_qty - reserved_qty AS available_qty
```

If `available_qty` is ever stored later for performance, it must be treated as a derived value and updated in the same transaction as `on_hand_qty` and `reserved_qty`.

## Table Overview

Recommended tables:

- `units`
- `material_categories`
- `materials`
- `material_attribute_definitions`
- `material_attribute_values`
- `material_unit_conversions`
- `skus`
- `warehouses`
- `inventory_batches`
- `inventory_stocks`
- `inventory_stock_layers`
- `inventory_reservations`
- `inventory_reservation_items`
- `inventory_movements`
- `inventory_idempotency_keys`

## Units

`units` stores reusable measurement units.

Recommended columns:

```text
id            BIGSERIAL PRIMARY KEY
code          TEXT NOT NULL
name          TEXT NOT NULL
symbol        TEXT NOT NULL
unit_type     TEXT NOT NULL
precision     INTEGER NOT NULL
status        TEXT NOT NULL DEFAULT 'active'
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at    TIMESTAMPTZ NULL
```

Rules:

- `code` must be unique among non-deleted units.
- `status` should be `active` or `inactive`.
- `precision` controls allowed quantity decimal places.
- Unit records must not store material-specific conversion rules.

Recommended indexes:

- Unique partial index on `code` where `deleted_at IS NULL`.
- Index on `status` where `deleted_at IS NULL`.

## Material Categories

`material_categories` stores material category master data.

Recommended columns:

```text
id            BIGSERIAL PRIMARY KEY
code          TEXT NOT NULL
name          TEXT NOT NULL
parent_id     BIGINT NULL REFERENCES material_categories(id)
status        TEXT NOT NULL DEFAULT 'active'
remark        TEXT NULL
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at    TIMESTAMPTZ NULL
```

Rules:

- `code` must be unique among non-deleted categories.
- Category-specific material attributes must be modeled through attribute definition tables, not by adding columns to `materials`.

## Materials

`materials` stores material master data.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
code            TEXT NOT NULL
name            TEXT NOT NULL
category_id     BIGINT NOT NULL REFERENCES material_categories(id)
base_unit_id    BIGINT NOT NULL REFERENCES units(id)
status          TEXT NOT NULL DEFAULT 'active'
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `code` must be unique among non-deleted materials.
- `base_unit_id` must reference `units`.
- Material does not own SKU fields, stock fields, warehouse fields, or movement fields.
- Category-specific fields must not be added directly to this table.

Recommended indexes:

- Unique partial index on `code` where `deleted_at IS NULL`.
- Index on `category_id` where `deleted_at IS NULL`.
- Index on `base_unit_id` where `deleted_at IS NULL`.
- Index on `status` where `deleted_at IS NULL`.

## Material Attribute Definitions

`material_attribute_definitions` defines category-specific material attributes.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
category_id     BIGINT NOT NULL REFERENCES material_categories(id)
code            TEXT NOT NULL
name            TEXT NOT NULL
data_type       TEXT NOT NULL
required        BOOLEAN NOT NULL DEFAULT FALSE
status          TEXT NOT NULL DEFAULT 'active'
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `data_type` should be constrained to values such as `text`, `number`, `boolean`, `date`, or `option`.
- `(category_id, code)` must be unique among non-deleted definitions.

## Material Attribute Values

`material_attribute_values` stores material-specific values for attribute definitions.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
definition_id   BIGINT NOT NULL REFERENCES material_attribute_definitions(id)
value_text      TEXT NULL
value_number    NUMERIC(20,6) NULL
value_boolean   BOOLEAN NULL
value_date      DATE NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `(material_id, definition_id)` must be unique among non-deleted values.
- The service layer must validate that the value column matches the definition `data_type`.

## Material Unit Conversions

`material_unit_conversions` stores material-specific unit conversion rules.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
from_unit_id    BIGINT NOT NULL REFERENCES units(id)
to_unit_id      BIGINT NOT NULL REFERENCES units(id)
factor          NUMERIC(24,10) NOT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `factor` must be greater than zero.
- `(material_id, from_unit_id, to_unit_id)` must be unique among non-deleted conversions.
- `from_unit_id` and `to_unit_id` must be different.
- Conversion logic belongs in the material service layer or material domain helper.

Example:

```text
Material A: 1 box = 12 pcs
Material B: 1 box = 24 pcs
```

## SKUs

`skus` stores stock keeping unit definitions.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
material_id     BIGINT NOT NULL REFERENCES materials(id)
code            TEXT NOT NULL
name            TEXT NOT NULL
unit_id         BIGINT NOT NULL REFERENCES units(id)
status          TEXT NOT NULL DEFAULT 'active'
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `code` must be unique among non-deleted SKUs.
- Current business stage should enforce at most one active SKU per material.
- The schema must still allow future one-material-to-many-SKUs expansion.
- `unit_id` must be the material base unit or a unit convertible through material unit conversions.
- SKU must not store inventory quantity fields.

Recommended indexes:

- Unique partial index on `code` where `deleted_at IS NULL`.
- Index on `material_id` where `deleted_at IS NULL`.
- Index on `unit_id` where `deleted_at IS NULL`.
- Partial unique index for current stage: one active non-deleted SKU per material.

## Warehouses

`warehouses` stores warehouse master data.

Recommended columns:

```text
id              BIGSERIAL PRIMARY KEY
code            TEXT NOT NULL
name            TEXT NOT NULL
type            TEXT NOT NULL DEFAULT 'normal'
status          TEXT NOT NULL DEFAULT 'active'
location        TEXT NULL
contact_name    TEXT NULL
contact_phone   TEXT NULL
remark          TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at      TIMESTAMPTZ NULL
```

Rules:

- `code` must be unique among non-deleted warehouses.
- `status` should be `active` or `inactive`.
- `inactive` warehouses can be queried for inventory.
- `inactive` warehouses must not be used for inventory mutations.
- Warehouses with stock, reservations, or movement records must not be hard deleted.
- If a warehouse has any stock records, it must be disabled instead of deleted.
- Warehouse must not store stock quantity fields.

Recommended indexes:

- Unique partial index on `code` where `deleted_at IS NULL`.
- Index on `status` where `deleted_at IS NULL`.

## Inventory Batches

`inventory_batches` stores batch master data.

Recommended columns:

```text
id                BIGSERIAL PRIMARY KEY
sku_id            BIGINT NOT NULL REFERENCES skus(id)
batch_no          TEXT NOT NULL
first_received_at TIMESTAMPTZ NULL
production_date   DATE NULL
expiration_date   DATE NULL
status            TEXT NOT NULL DEFAULT 'active'
remark            TEXT NULL
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

Rules:

- `(sku_id, batch_no)` must be unique among non-deleted batches.
- Batch metadata belongs here, not in stock balance tables.
- `status` should be `active` or `inactive`.

Recommended indexes:

- Unique partial index on `(sku_id, batch_no)` where `deleted_at IS NULL`.
- Index on `sku_id` where `deleted_at IS NULL`.
- Index on `first_received_at` where `deleted_at IS NULL`.
- Index on `expiration_date` where `deleted_at IS NULL`.

## Inventory Stocks

`inventory_stocks` stores summary stock by warehouse and SKU.

This table supports fast current stock queries.

Recommended columns:

```text
id                BIGSERIAL PRIMARY KEY
warehouse_id      BIGINT NOT NULL REFERENCES warehouses(id)
sku_id            BIGINT NOT NULL REFERENCES skus(id)
on_hand_qty       NUMERIC(20,6) NOT NULL DEFAULT 0
reserved_qty      NUMERIC(20,6) NOT NULL DEFAULT 0
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

Calculated value:

```text
available_qty = on_hand_qty - reserved_qty
```

Rules:

- `(warehouse_id, sku_id)` must be unique among non-deleted stock rows.
- `on_hand_qty` must be greater than or equal to zero.
- `reserved_qty` must be greater than or equal to zero.
- `reserved_qty` must be less than or equal to `on_hand_qty`.
- All stock mutations must update this summary table and stock layers in the same transaction.

Recommended indexes:

- Unique partial index on `(warehouse_id, sku_id)` where `deleted_at IS NULL`.
- Index on `sku_id` where `deleted_at IS NULL`.
- Index on `warehouse_id` where `deleted_at IS NULL`.

## Inventory Stock Layers

`inventory_stock_layers` stores FIFO allocation units.

A stock layer represents a quantity received into a warehouse for a SKU. `batch_id` is optional.

Recommended columns:

```text
id                BIGSERIAL PRIMARY KEY
warehouse_id      BIGINT NOT NULL REFERENCES warehouses(id)
sku_id            BIGINT NOT NULL REFERENCES skus(id)
batch_id          BIGINT NULL REFERENCES inventory_batches(id)
received_at       TIMESTAMPTZ NOT NULL
on_hand_qty       NUMERIC(20,6) NOT NULL DEFAULT 0
reserved_qty      NUMERIC(20,6) NOT NULL DEFAULT 0
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at        TIMESTAMPTZ NULL
```

Calculated value:

```text
available_qty = on_hand_qty - reserved_qty
```

Rules:

- `batch_id` may be null for non-batch stock.
- FIFO allocation uses `received_at`, then `id` as a deterministic tiebreaker.
- `on_hand_qty` must be greater than or equal to zero.
- `reserved_qty` must be greater than or equal to zero.
- `reserved_qty` must be less than or equal to `on_hand_qty`.
- The service layer must ensure `batch_id`, when present, belongs to the same `sku_id`.
- Warehouse + SKU + batch stock is queried by grouping layers.
- Warehouse + SKU summary stock must remain consistent with layer totals.

Recommended indexes:

- Index on `(warehouse_id, sku_id, received_at, id)` where `deleted_at IS NULL`.
- Index on `(warehouse_id, sku_id, batch_id)` where `deleted_at IS NULL`.
- Index on `batch_id` where `deleted_at IS NULL`.

## Inventory Reservations

`inventory_reservations` stores reservation headers.

Recommended columns:

```text
id                  BIGSERIAL PRIMARY KEY
warehouse_id        BIGINT NOT NULL REFERENCES warehouses(id)
sku_id              BIGINT NOT NULL REFERENCES skus(id)
total_qty           NUMERIC(20,6) NOT NULL
released_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
consumed_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
status              TEXT NOT NULL DEFAULT 'active'
idempotency_key     TEXT NOT NULL
source_type         TEXT NULL
source_id           TEXT NULL
source_line_id      TEXT NULL
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at          TIMESTAMPTZ NULL
```

Rules:

- `total_qty` must be greater than zero.
- `released_qty` and `consumed_qty` must be greater than or equal to zero.
- `released_qty + consumed_qty` must be less than or equal to `total_qty`.
- `status` should be `active`, `released`, `consumed`, or `cancelled`.
- Reservation must be allocated to stock layers by FIFO at reservation time.
- Release and reserved decrease operations must use reservation allocation items instead of recalculating FIFO.

Recommended indexes:

- Index on `(warehouse_id, sku_id, status)` where `deleted_at IS NULL`.
- Index on `(source_type, source_id, source_line_id)` where `deleted_at IS NULL`.
- Index on `idempotency_key` where `deleted_at IS NULL`.

## Inventory Reservation Items

`inventory_reservation_items` stores FIFO allocation details for a reservation.

Recommended columns:

```text
id                  BIGSERIAL PRIMARY KEY
reservation_id      BIGINT NOT NULL REFERENCES inventory_reservations(id)
stock_layer_id      BIGINT NOT NULL REFERENCES inventory_stock_layers(id)
reserved_qty        NUMERIC(20,6) NOT NULL
released_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
consumed_qty        NUMERIC(20,6) NOT NULL DEFAULT 0
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at          TIMESTAMPTZ NULL
```

Rules:

- `reserved_qty` must be greater than zero.
- `released_qty` and `consumed_qty` must be greater than or equal to zero.
- `released_qty + consumed_qty` must be less than or equal to `reserved_qty`.
- Each item points to the exact stock layer selected during FIFO allocation.
- Release reserved stock reduces `reserved_qty` on these stock layers.
- Decrease reserved stock reduces both `reserved_qty` and `on_hand_qty` on these stock layers.

Recommended indexes:

- Index on `reservation_id` where `deleted_at IS NULL`.
- Index on `stock_layer_id` where `deleted_at IS NULL`.

## Inventory Movements

`inventory_movements` stores immutable inventory audit records.

Recommended columns:

```text
id                    BIGSERIAL PRIMARY KEY
operation_type        TEXT NOT NULL
warehouse_id          BIGINT NOT NULL REFERENCES warehouses(id)
sku_id                BIGINT NOT NULL REFERENCES skus(id)
batch_id              BIGINT NULL REFERENCES inventory_batches(id)
stock_layer_id        BIGINT NULL REFERENCES inventory_stock_layers(id)
reservation_id        BIGINT NULL REFERENCES inventory_reservations(id)
reservation_item_id   BIGINT NULL REFERENCES inventory_reservation_items(id)
qty                   NUMERIC(20,6) NOT NULL
idempotency_key       TEXT NOT NULL
source_type           TEXT NULL
source_id             TEXT NULL
source_line_id        TEXT NULL
created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at            TIMESTAMPTZ NULL
```

Recommended `operation_type` values:

- `increase`
- `reserve`
- `release_reserved`
- `decrease_available`
- `decrease_reserved`

Rules:

- Movement records are immutable audit records.
- Corrections must be new movement records.
- `qty` must be greater than zero.
- For FIFO operations that touch multiple stock layers, create one movement record per touched stock layer.
- Movement direction is determined by `operation_type`, not by negative quantities.

Recommended indexes:

- Index on `(warehouse_id, sku_id, created_at)`.
- Index on `batch_id`.
- Index on `stock_layer_id`.
- Index on `reservation_id`.
- Index on `(source_type, source_id, source_line_id)`.
- Index on `(operation_type, idempotency_key)`.

## Inventory Idempotency Keys

`inventory_idempotency_keys` stores mutation request idempotency records.

Recommended columns:

```text
id                    BIGSERIAL PRIMARY KEY
operation_type        TEXT NOT NULL
idempotency_key       TEXT NOT NULL
request_hash          TEXT NOT NULL
request_payload       JSONB NOT NULL
status                TEXT NOT NULL DEFAULT 'processing'
response_ref_type     TEXT NULL
response_ref_id       BIGINT NULL
response_payload      JSONB NULL
error_code            TEXT NULL
created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
deleted_at            TIMESTAMPTZ NULL
```

Recommended `status` values:

- `processing`
- `succeeded`
- `failed`

Rules:

- `(operation_type, idempotency_key)` must be unique among non-deleted rows.
- `request_hash` is a SHA-256 hash of the canonical mutation request payload.
- Retrying the same operation with the same key and same hash must not apply stock changes again.
- Retrying the same operation with the same key but a different hash must return an idempotency conflict error.
- The idempotency record, stock updates, reservation updates, and movement records must be committed in the same transaction.

Recommended indexes:

- Unique partial index on `(operation_type, idempotency_key)` where `deleted_at IS NULL`.
- Index on `status` where `deleted_at IS NULL`.

## Idempotency Payload Hash

The service should build a canonical payload before hashing.

Canonical payload rules:

- Include only business-significant request fields.
- Include `operation_type`.
- Include `warehouse_id`, `sku_id`, optional `batch_id`, and quantity.
- Include `source_type`, `source_id`, and `source_line_id` when provided.
- Include reservation references when the operation releases or consumes reserved stock.
- Normalize quantity to the service scale, such as `10.000000`.
- Use `null` for absent optional values instead of omitting fields.
- Use stable field names and stable ordering.

Example canonical payload:

```json
{
  "operation_type": "reserve",
  "warehouse_id": 1,
  "sku_id": 10,
  "batch_id": null,
  "qty": "10.000000",
  "source_type": "sales_order",
  "source_id": "SO001",
  "source_line_id": "1"
}
```

Hash rule:

```text
request_hash = hex(sha256(canonical_payload_bytes))
```

The service should use a typed request struct and a dedicated canonicalization helper. Do not hash raw HTTP request bytes, because field ordering, whitespace, and omitted nulls can change between retries.

## Stock Operation Rules

### Increase Stock

Effects:

```text
summary.on_hand_qty += qty
layer.on_hand_qty += qty
```

Rules:

- Creates a new stock layer or updates a service-defined layer.
- Creates movement records.
- Requires idempotency.

### Reserve Stock

Effects:

```text
summary.reserved_qty += qty
layer.reserved_qty += allocated_qty
```

Rules:

- Checks available quantity.
- Allocates stock layers by FIFO when `batch_id` is not provided.
- Creates `inventory_reservations`.
- Creates `inventory_reservation_items`.
- Creates movement records.
- Requires idempotency.

### Release Reserved Stock

Effects:

```text
summary.reserved_qty -= qty
layer.reserved_qty -= released_qty
reservation_item.released_qty += released_qty
```

Rules:

- Uses existing reservation items.
- Does not recalculate FIFO.
- Creates movement records.
- Requires idempotency.

### Decrease Available Stock

Effects:

```text
summary.on_hand_qty -= qty
layer.on_hand_qty -= allocated_qty
```

Rules:

- Checks available quantity.
- Allocates stock layers by FIFO when `batch_id` is not provided.
- Does not change `reserved_qty`.
- Creates movement records.
- Requires idempotency.

### Decrease Reserved Stock

Effects:

```text
summary.on_hand_qty -= qty
summary.reserved_qty -= qty
layer.on_hand_qty -= consumed_qty
layer.reserved_qty -= consumed_qty
reservation_item.consumed_qty += consumed_qty
```

Rules:

- Uses existing reservation items.
- Does not recalculate FIFO.
- Creates movement records.
- Requires idempotency.

## Transaction Rules

Inventory mutation operations must run in the application service layer transaction.

The transaction must include:

- Idempotency check or insert.
- Stock summary row lock or atomic update.
- Stock layer selection and lock.
- Stock summary updates.
- Stock layer updates.
- Reservation header and item updates when applicable.
- Movement record creation.
- Idempotency success update.

Repositories must not start, commit, or roll back transactions.

## Concurrency Rules

The service must prevent negative stock under concurrent requests.

Recommended database approach:

- Lock stock summary rows before mutation.
- Lock selected stock layers before mutation.
- Use SQL conditions that prevent invalid quantities.
- Apply FIFO allocation inside the same transaction.

Important invariants:

```text
on_hand_qty >= 0
reserved_qty >= 0
reserved_qty <= on_hand_qty
```

## Creation Order

Recommended migration creation order:

1. `units`
2. `material_categories`
3. `materials`
4. `material_attribute_definitions`
5. `material_attribute_values`
6. `material_unit_conversions`
7. `warehouses`
8. `skus`
9. `inventory_batches`
10. `inventory_stocks`
11. `inventory_stock_layers`
12. `inventory_reservations`
13. `inventory_reservation_items`
14. `inventory_movements`
15. `inventory_idempotency_keys`

## Open Implementation Notes

- Use `CHECK` constraints for status and operation type values.
- Add indexes for every foreign key column.
- Use partial unique indexes with `deleted_at IS NULL` for soft-delete-aware uniqueness.
- Keep business validation in the service layer even when database constraints exist.
- Database constraints protect data integrity; they do not replace domain rules.
