# Database Migration Convention

> This file applies to the `migrations/` directory.
> When working in this directory, should read both the root `AGENTS.md` and this file.

## File Naming

```
<number>_<description>.up.sql # Forward migration (apply changes)
<number>_<description>.down.sql # Backward migration (revert changes)
```

- Number: 12 digits, the first eight digits represent year, month and day. The last 4 digits are the serial number. (`202601010001`)
- Description: snake_case.
- Next serial number = current max serial number + 1.


## Must include basic fields when creating a new table.

```sql
id         BIGSERIAL    PRIMARY KEY,
created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
deleted_at TIMESTAMPTZ  NULL      -- soft delete，NULL means not deleted
```

## PostgreSQL 类型规范

| Purpose | Use | Forbidden |
|---|---|---|
| Primary / Foreign Key | `BIGSERIAL` / `BIGINT` | `INT` / `SERIAL` |
| Text（name、code） | `TEXT` | `VARCHAR(n)` |
| Decimal / Amount | `NUMERIC(p, s)` | `FLOAT` / `REAL` |
| Timestamp | `TIMESTAMPTZ` | `TIMESTAMP`（without timezone）|
| Enum values | `TEXT` + `CHECK` constraint | PostgreSQL `ENUM` type |
| Boolean | `BOOLEAN` | |

## Example：20260101001_create_materials_table

**up.sql**

```sql
CREATE TABLE materials (
    id          BIGSERIAL    PRIMARY KEY,
    code        TEXT         NOT NULL UNIQUE,
    name        TEXT         NOT NULL,
    description TEXT,
    unit        TEXT         NOT NULL,
    category    TEXT,
    status      TEXT         NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'inactive')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ  NULL
);

CREATE INDEX idx_materials_code     ON materials (code)     WHERE deleted_at IS NULL;
CREATE INDEX idx_materials_status   ON materials (status)   WHERE deleted_at IS NULL;
CREATE INDEX idx_materials_category ON materials (category) WHERE deleted_at IS NULL;

COMMENT ON TABLE  materials           IS '物料主表';
COMMENT ON COLUMN materials.code      IS '物料编码，全局唯一';
COMMENT ON COLUMN materials.unit      IS '计量单位（pcs/kg/m 等）';
COMMENT ON COLUMN materials.status    IS '状态：active=启用, inactive=停用';
COMMENT ON COLUMN materials.deleted_at IS '软删除标记，NULL 表示未删除';
```

**down.sql**

```sql
DROP TABLE IF EXISTS materials;
```

## Rules
- When adding a foreign key constraint, you should to also add an index to the column of the foreign key constraint.