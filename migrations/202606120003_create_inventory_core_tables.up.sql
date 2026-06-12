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

COMMENT ON TABLE inventory_batches IS 'Inventory batch master data';
COMMENT ON COLUMN inventory_batches.sku_id IS 'SKU id for the batch';
COMMENT ON COLUMN inventory_batches.batch_no IS 'Batch number, unique per SKU among non-deleted batches';
COMMENT ON COLUMN inventory_batches.first_received_at IS 'First received time of the batch';
COMMENT ON COLUMN inventory_batches.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE inventory_stocks IS 'Summary stock by warehouse and SKU';
COMMENT ON COLUMN inventory_stocks.on_hand_qty IS 'Physical stock quantity';
COMMENT ON COLUMN inventory_stocks.reserved_qty IS 'Reserved stock quantity';
COMMENT ON COLUMN inventory_stocks.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE inventory_stock_layers IS 'FIFO stock layers for warehouse, SKU, and optional batch';
COMMENT ON COLUMN inventory_stock_layers.batch_id IS 'Optional inventory batch id. NULL means non-batch stock';
COMMENT ON COLUMN inventory_stock_layers.received_at IS 'Received time used for FIFO allocation';
COMMENT ON COLUMN inventory_stock_layers.on_hand_qty IS 'Physical stock quantity in this layer';
COMMENT ON COLUMN inventory_stock_layers.reserved_qty IS 'Reserved stock quantity in this layer';
COMMENT ON COLUMN inventory_stock_layers.deleted_at IS 'Soft delete marker. NULL means not deleted';
