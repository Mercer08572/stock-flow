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

COMMENT ON TABLE units IS 'Reusable measurement units';
COMMENT ON COLUMN units.code IS 'Unique unit code among non-deleted units';
COMMENT ON COLUMN units.unit_type IS 'Unit type: count, weight, length, area, volume, package, time, other';
COMMENT ON COLUMN units.precision IS 'Allowed quantity decimal places';
COMMENT ON COLUMN units.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE material_categories IS 'Material category master data';
COMMENT ON COLUMN material_categories.code IS 'Unique category code among non-deleted categories';
COMMENT ON COLUMN material_categories.parent_id IS 'Parent material category id';
COMMENT ON COLUMN material_categories.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE materials IS 'Material master data';
COMMENT ON COLUMN materials.code IS 'Unique material code among non-deleted materials';
COMMENT ON COLUMN materials.category_id IS 'Material category id';
COMMENT ON COLUMN materials.base_unit_id IS 'Material base unit id';
COMMENT ON COLUMN materials.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE material_attribute_definitions IS 'Category-specific material attribute definitions';
COMMENT ON COLUMN material_attribute_definitions.data_type IS 'Attribute data type: text, number, boolean, date, option';
COMMENT ON COLUMN material_attribute_definitions.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE material_attribute_values IS 'Material attribute values';
COMMENT ON COLUMN material_attribute_values.definition_id IS 'Material attribute definition id';
COMMENT ON COLUMN material_attribute_values.deleted_at IS 'Soft delete marker. NULL means not deleted';

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

COMMENT ON TABLE material_unit_conversions IS 'Material-specific unit conversion rules';
COMMENT ON COLUMN material_unit_conversions.factor IS 'Conversion factor from source unit to target unit';
COMMENT ON COLUMN material_unit_conversions.deleted_at IS 'Soft delete marker. NULL means not deleted';
