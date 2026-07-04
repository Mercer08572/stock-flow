-- name: ListMaterialUnitConversions :many
SELECT id,
       material_id,
       from_unit_id,
       to_unit_id,
       factor,
       created_at,
       updated_at
FROM material_unit_conversions
WHERE material_id = sqlc.arg('material_id')::bigint
  AND deleted_at IS NULL
  AND (sqlc.narg('from_unit_id')::bigint IS NULL OR from_unit_id = sqlc.narg('from_unit_id')::bigint)
  AND (sqlc.narg('to_unit_id')::bigint IS NULL OR to_unit_id = sqlc.narg('to_unit_id')::bigint)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetMaterialUnitConversionByID :one
SELECT id,
       material_id,
       from_unit_id,
       to_unit_id,
       factor,
       created_at,
       updated_at
FROM material_unit_conversions
WHERE id = sqlc.arg('id')::bigint
  AND material_id = sqlc.arg('material_id')::bigint
  AND deleted_at IS NULL;

-- name: CreateMaterialUnitConversion :one
INSERT INTO material_unit_conversions (material_id, from_unit_id, to_unit_id, factor)
VALUES ($1, $2, $3, $4)
RETURNING id,
          material_id,
          from_unit_id,
          to_unit_id,
          factor,
          created_at,
          updated_at;

-- name: UpdateMaterialUnitConversion :one
UPDATE material_unit_conversions
SET from_unit_id = $3,
    to_unit_id = $4,
    factor = $5,
    updated_at = NOW()
WHERE id = $1
  AND material_id = $2
  AND deleted_at IS NULL
RETURNING id,
          material_id,
          from_unit_id,
          to_unit_id,
          factor,
          created_at,
          updated_at;

-- name: SoftDeleteMaterialUnitConversion :execrows
UPDATE material_unit_conversions
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND material_id = sqlc.arg('material_id')::bigint
  AND deleted_at IS NULL;

-- name: MaterialUnitConversionExists :one
SELECT EXISTS (
    SELECT 1
    FROM material_unit_conversions
    WHERE material_id = sqlc.arg('material_id')::bigint
      AND from_unit_id = sqlc.arg('from_unit_id')::bigint
      AND to_unit_id = sqlc.arg('to_unit_id')::bigint
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;

-- name: ReverseMaterialUnitConversionExists :one
SELECT EXISTS (
    SELECT 1
    FROM material_unit_conversions
    WHERE material_id = sqlc.arg('material_id')::bigint
      AND from_unit_id = sqlc.arg('to_unit_id')::bigint
      AND to_unit_id = sqlc.arg('from_unit_id')::bigint
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;
