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

COMMENT ON TABLE inventory_reservations IS 'Inventory reservation headers';
COMMENT ON COLUMN inventory_reservations.total_qty IS 'Total reserved quantity';
COMMENT ON COLUMN inventory_reservations.released_qty IS 'Released reserved quantity';
COMMENT ON COLUMN inventory_reservations.consumed_qty IS 'Consumed reserved quantity';
COMMENT ON COLUMN inventory_reservations.idempotency_key IS 'Idempotency key for the reservation operation';
COMMENT ON COLUMN inventory_reservations.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE inventory_reservation_items IS 'FIFO allocation details for inventory reservations';
COMMENT ON COLUMN inventory_reservation_items.stock_layer_id IS 'Stock layer selected during FIFO allocation';
COMMENT ON COLUMN inventory_reservation_items.reserved_qty IS 'Quantity reserved from this stock layer';
COMMENT ON COLUMN inventory_reservation_items.released_qty IS 'Quantity released from this reservation item';
COMMENT ON COLUMN inventory_reservation_items.consumed_qty IS 'Quantity consumed from this reservation item';
COMMENT ON COLUMN inventory_reservation_items.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE inventory_movements IS 'Immutable inventory movement audit records';
COMMENT ON COLUMN inventory_movements.operation_type IS 'Operation type: increase, reserve, release_reserved, decrease_available, decrease_reserved';
COMMENT ON COLUMN inventory_movements.qty IS 'Movement quantity. Direction is determined by operation_type';
COMMENT ON COLUMN inventory_movements.idempotency_key IS 'Idempotency key for the mutation operation';
COMMENT ON COLUMN inventory_movements.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE inventory_idempotency_keys IS 'Inventory mutation idempotency records';
COMMENT ON COLUMN inventory_idempotency_keys.request_hash IS 'SHA-256 hash of the canonical mutation request payload';
COMMENT ON COLUMN inventory_idempotency_keys.request_payload IS 'Canonical mutation request payload';
COMMENT ON COLUMN inventory_idempotency_keys.response_ref_id IS 'Optional reference id for the result entity';
COMMENT ON COLUMN inventory_idempotency_keys.deleted_at IS 'Soft delete marker. NULL means not deleted';
