-- name: ListUnits :many
SELECT id,
       code,
       name,
       symbol,
       unit_type,
       precision,
       status,
       created_at,
       updated_at
FROM units
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('unit_type')::text IS NULL OR unit_type = sqlc.narg('unit_type')::text)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetUnitByID :one
SELECT id,
       code,
       name,
       symbol,
       unit_type,
       precision,
       status,
       created_at,
       updated_at
FROM units
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateUnit :one
INSERT INTO units (code, name, symbol, unit_type, precision, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id,
          code,
          name,
          symbol,
          unit_type,
          precision,
          status,
          created_at,
          updated_at;

-- name: UpdateUnit :one
UPDATE units
SET code = $2,
    name = $3,
    symbol = $4,
    unit_type = $5,
    precision = $6,
    status = $7,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          code,
          name,
          symbol,
          unit_type,
          precision,
          status,
          created_at,
          updated_at;

-- name: SoftDeleteUnit :execrows
UPDATE units
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UnitCodeExists :one
SELECT EXISTS (
    SELECT 1
    FROM units
    WHERE code = $1
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;
