CREATE TABLE units (
    id         BIGSERIAL   PRIMARY KEY,
    code       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    symbol     TEXT        NOT NULL,
    unit_type  TEXT        NOT NULL,
    precision  INTEGER     NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,

    CONSTRAINT chk_units_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_units_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_units_symbol_not_blank
        CHECK (btrim(symbol) <> ''),
    CONSTRAINT chk_units_type
        CHECK (unit_type IN ('count', 'weight', 'length', 'area', 'volume', 'package', 'time', 'other')),
    CONSTRAINT chk_units_precision
        CHECK (precision >= 0 AND precision <= 6),
    CONSTRAINT chk_units_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_units_code
    ON units (code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_units_status
    ON units (status)
    WHERE deleted_at IS NULL;

CREATE TABLE material_categories (
    id         BIGSERIAL   PRIMARY KEY,
    code       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    parent_id  BIGINT      NULL REFERENCES material_categories(id),
    status     TEXT        NOT NULL DEFAULT 'active',
    remark     TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,

    CONSTRAINT chk_material_categories_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_material_categories_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_material_categories_parent_not_self
        CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT chk_material_categories_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_material_categories_code
    ON material_categories (code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_categories_parent_id
    ON material_categories (parent_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_categories_status
    ON material_categories (status)
    WHERE deleted_at IS NULL;

CREATE TABLE materials (
    id           BIGSERIAL   PRIMARY KEY,
    code         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    category_id  BIGINT      NOT NULL REFERENCES material_categories(id),
    base_unit_id BIGINT      NOT NULL REFERENCES units(id),
    status       TEXT        NOT NULL DEFAULT 'active',
    remark       TEXT        NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ NULL,

    CONSTRAINT chk_materials_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_materials_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_materials_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_materials_code
    ON materials (code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_materials_category_id
    ON materials (category_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_materials_base_unit_id
    ON materials (base_unit_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_materials_status
    ON materials (status)
    WHERE deleted_at IS NULL;

CREATE TABLE material_attribute_definitions (
    id          BIGSERIAL   PRIMARY KEY,
    category_id BIGINT      NOT NULL REFERENCES material_categories(id),
    code        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    data_type   TEXT        NOT NULL,
    required    BOOLEAN     NOT NULL DEFAULT FALSE,
    status      TEXT        NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL,

    CONSTRAINT chk_material_attribute_definitions_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_material_attribute_definitions_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_material_attribute_definitions_data_type
        CHECK (data_type IN ('text', 'number', 'boolean', 'date', 'option')),
    CONSTRAINT chk_material_attribute_definitions_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_material_attribute_definitions_category_code
    ON material_attribute_definitions (category_id, code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_attribute_definitions_category_id
    ON material_attribute_definitions (category_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_attribute_definitions_status
    ON material_attribute_definitions (status)
    WHERE deleted_at IS NULL;

CREATE TABLE material_attribute_values (
    id            BIGSERIAL     PRIMARY KEY,
    material_id   BIGINT        NOT NULL REFERENCES materials(id),
    definition_id BIGINT        NOT NULL REFERENCES material_attribute_definitions(id),
    value_text    TEXT          NULL,
    value_number  NUMERIC(20,6) NULL,
    value_boolean BOOLEAN       NULL,
    value_date    DATE          NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ   NULL,

    CONSTRAINT chk_material_attribute_values_single_value
        CHECK (num_nonnulls(value_text, value_number, value_boolean, value_date) = 1)
);

CREATE UNIQUE INDEX ux_material_attribute_values_material_definition
    ON material_attribute_values (material_id, definition_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_attribute_values_material_id
    ON material_attribute_values (material_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_attribute_values_definition_id
    ON material_attribute_values (definition_id)
    WHERE deleted_at IS NULL;

CREATE TABLE material_unit_conversions (
    id           BIGSERIAL      PRIMARY KEY,
    material_id  BIGINT         NOT NULL REFERENCES materials(id),
    from_unit_id BIGINT         NOT NULL REFERENCES units(id),
    to_unit_id   BIGINT         NOT NULL REFERENCES units(id),
    factor       NUMERIC(24,10) NOT NULL,
    created_at   TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ    NULL,

    CONSTRAINT chk_material_unit_conversions_factor_positive
        CHECK (factor > 0),
    CONSTRAINT chk_material_unit_conversions_different_units
        CHECK (from_unit_id <> to_unit_id)
);

CREATE UNIQUE INDEX ux_material_unit_conversions_material_units
    ON material_unit_conversions (material_id, from_unit_id, to_unit_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_unit_conversions_material_id
    ON material_unit_conversions (material_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_unit_conversions_from_unit_id
    ON material_unit_conversions (from_unit_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_material_unit_conversions_to_unit_id
    ON material_unit_conversions (to_unit_id)
    WHERE deleted_at IS NULL;

CREATE TABLE skus (
    id          BIGSERIAL   PRIMARY KEY,
    material_id BIGINT      NOT NULL REFERENCES materials(id),
    code        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    unit_id     BIGINT      NOT NULL REFERENCES units(id),
    status      TEXT        NOT NULL DEFAULT 'active',
    remark      TEXT        NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL,

    CONSTRAINT chk_skus_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_skus_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_skus_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_skus_code
    ON skus (code)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ux_skus_active_material_current_stage
    ON skus (material_id)
    WHERE deleted_at IS NULL AND status = 'active';

CREATE INDEX idx_skus_material_id
    ON skus (material_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_skus_unit_id
    ON skus (unit_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_skus_status
    ON skus (status)
    WHERE deleted_at IS NULL;

CREATE TABLE warehouses (
    id            BIGSERIAL   PRIMARY KEY,
    code          TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    type          TEXT        NOT NULL DEFAULT 'normal',
    status        TEXT        NOT NULL DEFAULT 'active',
    location      TEXT        NULL,
    contact_name  TEXT        NULL,
    contact_phone TEXT        NULL,
    remark        TEXT        NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ NULL,

    CONSTRAINT chk_warehouses_code_not_blank
        CHECK (btrim(code) <> ''),
    CONSTRAINT chk_warehouses_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_warehouses_type
        CHECK (type IN ('normal', 'virtual')),
    CONSTRAINT chk_warehouses_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_warehouses_code
    ON warehouses (code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_warehouses_status
    ON warehouses (status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_warehouses_type
    ON warehouses (type)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_batches (
    id                BIGSERIAL   PRIMARY KEY,
    sku_id            BIGINT      NOT NULL REFERENCES skus(id),
    batch_no          TEXT        NOT NULL,
    first_received_at TIMESTAMPTZ NULL,
    production_date   DATE        NULL,
    expiration_date   DATE        NULL,
    status            TEXT        NOT NULL DEFAULT 'active',
    remark            TEXT        NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ NULL,

    CONSTRAINT chk_inventory_batches_batch_no_not_blank
        CHECK (btrim(batch_no) <> ''),
    CONSTRAINT chk_inventory_batches_date_range
        CHECK (production_date IS NULL OR expiration_date IS NULL OR expiration_date >= production_date),
    CONSTRAINT chk_inventory_batches_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_inventory_batches_sku_batch_no
    ON inventory_batches (sku_id, batch_no)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_batches_sku_id
    ON inventory_batches (sku_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_batches_first_received_at
    ON inventory_batches (first_received_at)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_batches_expiration_date
    ON inventory_batches (expiration_date)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_batches_status
    ON inventory_batches (status)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_stocks (
    id           BIGSERIAL     PRIMARY KEY,
    warehouse_id BIGINT        NOT NULL REFERENCES warehouses(id),
    sku_id       BIGINT        NOT NULL REFERENCES skus(id),
    on_hand_qty  NUMERIC(20,6) NOT NULL DEFAULT 0,
    reserved_qty NUMERIC(20,6) NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ   NULL,

    CONSTRAINT chk_inventory_stocks_on_hand_qty_non_negative
        CHECK (on_hand_qty >= 0),
    CONSTRAINT chk_inventory_stocks_reserved_qty_non_negative
        CHECK (reserved_qty >= 0),
    CONSTRAINT chk_inventory_stocks_reserved_qty_not_exceed_on_hand
        CHECK (reserved_qty <= on_hand_qty)
);

CREATE UNIQUE INDEX ux_inventory_stocks_warehouse_sku
    ON inventory_stocks (warehouse_id, sku_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stocks_warehouse_id
    ON inventory_stocks (warehouse_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stocks_sku_id
    ON inventory_stocks (sku_id)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_stock_layers (
    id           BIGSERIAL     PRIMARY KEY,
    warehouse_id BIGINT        NOT NULL REFERENCES warehouses(id),
    sku_id       BIGINT        NOT NULL REFERENCES skus(id),
    batch_id     BIGINT        NULL REFERENCES inventory_batches(id),
    received_at  TIMESTAMPTZ   NOT NULL,
    on_hand_qty  NUMERIC(20,6) NOT NULL DEFAULT 0,
    reserved_qty NUMERIC(20,6) NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ   NULL,

    CONSTRAINT chk_inventory_stock_layers_on_hand_qty_non_negative
        CHECK (on_hand_qty >= 0),
    CONSTRAINT chk_inventory_stock_layers_reserved_qty_non_negative
        CHECK (reserved_qty >= 0),
    CONSTRAINT chk_inventory_stock_layers_reserved_qty_not_exceed_on_hand
        CHECK (reserved_qty <= on_hand_qty)
);

CREATE INDEX idx_inventory_stock_layers_fifo
    ON inventory_stock_layers (warehouse_id, sku_id, received_at, id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stock_layers_warehouse_sku_batch
    ON inventory_stock_layers (warehouse_id, sku_id, batch_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stock_layers_warehouse_id
    ON inventory_stock_layers (warehouse_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stock_layers_sku_id
    ON inventory_stock_layers (sku_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_stock_layers_batch_id
    ON inventory_stock_layers (batch_id)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_reservations (
    id              BIGSERIAL     PRIMARY KEY,
    warehouse_id    BIGINT        NOT NULL REFERENCES warehouses(id),
    sku_id          BIGINT        NOT NULL REFERENCES skus(id),
    total_qty       NUMERIC(20,6) NOT NULL,
    released_qty    NUMERIC(20,6) NOT NULL DEFAULT 0,
    consumed_qty    NUMERIC(20,6) NOT NULL DEFAULT 0,
    status          TEXT          NOT NULL DEFAULT 'active',
    idempotency_key TEXT          NOT NULL,
    source_type     TEXT          NULL,
    source_id       TEXT          NULL,
    source_line_id  TEXT          NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ   NULL,

    CONSTRAINT chk_inventory_reservations_total_qty_positive
        CHECK (total_qty > 0),
    CONSTRAINT chk_inventory_reservations_released_qty_non_negative
        CHECK (released_qty >= 0),
    CONSTRAINT chk_inventory_reservations_consumed_qty_non_negative
        CHECK (consumed_qty >= 0),
    CONSTRAINT chk_inventory_reservations_qty_not_exceed_total
        CHECK (released_qty + consumed_qty <= total_qty),
    CONSTRAINT chk_inventory_reservations_status
        CHECK (status IN ('active', 'released', 'consumed', 'cancelled')),
    CONSTRAINT chk_inventory_reservations_idempotency_key_not_blank
        CHECK (btrim(idempotency_key) <> '')
);

CREATE INDEX idx_inventory_reservations_warehouse_sku_status
    ON inventory_reservations (warehouse_id, sku_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_reservations_warehouse_id
    ON inventory_reservations (warehouse_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_reservations_sku_id
    ON inventory_reservations (sku_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_reservations_source
    ON inventory_reservations (source_type, source_id, source_line_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_reservations_idempotency_key
    ON inventory_reservations (idempotency_key)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_reservation_items (
    id             BIGSERIAL     PRIMARY KEY,
    reservation_id BIGINT        NOT NULL REFERENCES inventory_reservations(id),
    stock_layer_id BIGINT        NOT NULL REFERENCES inventory_stock_layers(id),
    reserved_qty   NUMERIC(20,6) NOT NULL,
    released_qty   NUMERIC(20,6) NOT NULL DEFAULT 0,
    consumed_qty   NUMERIC(20,6) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ   NULL,

    CONSTRAINT chk_inventory_reservation_items_reserved_qty_positive
        CHECK (reserved_qty > 0),
    CONSTRAINT chk_inventory_reservation_items_released_qty_non_negative
        CHECK (released_qty >= 0),
    CONSTRAINT chk_inventory_reservation_items_consumed_qty_non_negative
        CHECK (consumed_qty >= 0),
    CONSTRAINT chk_inventory_reservation_items_qty_not_exceed_reserved
        CHECK (released_qty + consumed_qty <= reserved_qty)
);

CREATE INDEX idx_inventory_reservation_items_reservation_id
    ON inventory_reservation_items (reservation_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_reservation_items_stock_layer_id
    ON inventory_reservation_items (stock_layer_id)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_movements (
    id                  BIGSERIAL     PRIMARY KEY,
    operation_type      TEXT          NOT NULL,
    warehouse_id        BIGINT        NOT NULL REFERENCES warehouses(id),
    sku_id              BIGINT        NOT NULL REFERENCES skus(id),
    batch_id            BIGINT        NULL REFERENCES inventory_batches(id),
    stock_layer_id      BIGINT        NULL REFERENCES inventory_stock_layers(id),
    reservation_id      BIGINT        NULL REFERENCES inventory_reservations(id),
    reservation_item_id BIGINT        NULL REFERENCES inventory_reservation_items(id),
    qty                 NUMERIC(20,6) NOT NULL,
    idempotency_key     TEXT          NOT NULL,
    source_type         TEXT          NULL,
    source_id           TEXT          NULL,
    source_line_id      TEXT          NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ   NULL,

    CONSTRAINT chk_inventory_movements_operation_type
        CHECK (operation_type IN ('increase', 'reserve', 'release_reserved', 'decrease_available', 'decrease_reserved')),
    CONSTRAINT chk_inventory_movements_qty_positive
        CHECK (qty > 0),
    CONSTRAINT chk_inventory_movements_idempotency_key_not_blank
        CHECK (btrim(idempotency_key) <> '')
);

CREATE INDEX idx_inventory_movements_warehouse_sku_created_at
    ON inventory_movements (warehouse_id, sku_id, created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_warehouse_id
    ON inventory_movements (warehouse_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_sku_id
    ON inventory_movements (sku_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_batch_id
    ON inventory_movements (batch_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_stock_layer_id
    ON inventory_movements (stock_layer_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_reservation_id
    ON inventory_movements (reservation_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_reservation_item_id
    ON inventory_movements (reservation_item_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_source
    ON inventory_movements (source_type, source_id, source_line_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_movements_operation_idempotency
    ON inventory_movements (operation_type, idempotency_key)
    WHERE deleted_at IS NULL;

CREATE TABLE inventory_idempotency_keys (
    id                BIGSERIAL   PRIMARY KEY,
    operation_type    TEXT        NOT NULL,
    idempotency_key   TEXT        NOT NULL,
    request_hash      TEXT        NOT NULL,
    request_payload   JSONB       NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'processing',
    response_ref_type TEXT        NULL,
    response_ref_id   BIGINT      NULL,
    response_payload  JSONB       NULL,
    error_code        TEXT        NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ NULL,

    CONSTRAINT chk_inventory_idempotency_keys_operation_type
        CHECK (operation_type IN ('increase', 'reserve', 'release_reserved', 'decrease_available', 'decrease_reserved')),
    CONSTRAINT chk_inventory_idempotency_keys_idempotency_key_not_blank
        CHECK (btrim(idempotency_key) <> ''),
    CONSTRAINT chk_inventory_idempotency_keys_request_hash_not_blank
        CHECK (btrim(request_hash) <> ''),
    CONSTRAINT chk_inventory_idempotency_keys_request_payload_object
        CHECK (jsonb_typeof(request_payload) = 'object'),
    CONSTRAINT chk_inventory_idempotency_keys_response_payload_object
        CHECK (response_payload IS NULL OR jsonb_typeof(response_payload) = 'object'),
    CONSTRAINT chk_inventory_idempotency_keys_status
        CHECK (status IN ('processing', 'succeeded', 'failed'))
);

CREATE UNIQUE INDEX ux_inventory_idempotency_keys_operation_key
    ON inventory_idempotency_keys (operation_type, idempotency_key)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_inventory_idempotency_keys_status
    ON inventory_idempotency_keys (status)
    WHERE deleted_at IS NULL;
