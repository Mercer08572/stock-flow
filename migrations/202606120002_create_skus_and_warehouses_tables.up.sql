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

COMMENT ON TABLE skus IS 'Stock keeping unit definitions';
COMMENT ON COLUMN skus.material_id IS 'Material id. Current stage allows at most one active SKU per material';
COMMENT ON COLUMN skus.code IS 'Unique SKU code among non-deleted SKUs';
COMMENT ON COLUMN skus.unit_id IS 'SKU stock operation unit id';
COMMENT ON COLUMN skus.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE warehouses IS 'Warehouse master data';
COMMENT ON COLUMN warehouses.code IS 'Unique warehouse code among non-deleted warehouses';
COMMENT ON COLUMN warehouses.type IS 'Warehouse type: normal, virtual';
COMMENT ON COLUMN warehouses.status IS 'Status: active warehouses allow inventory mutation; inactive warehouses allow inventory query only';
COMMENT ON COLUMN warehouses.deleted_at IS 'Soft delete marker. NULL means not deleted';
