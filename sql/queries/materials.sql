-- name: ListMaterials :many
SELECT id,
       code,
       name,
       category_id,
       base_unit_id,
       status,
       remark,
       created_at,
       updated_at
FROM materials
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('category_id')::bigint IS NULL OR category_id = sqlc.narg('category_id')::bigint)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetMaterialByID :one
SELECT id,
       code,
       name,
       category_id,
       base_unit_id,
       status,
       remark,
       created_at,
       updated_at
FROM materials
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateMaterial :one
INSERT INTO materials (code, name, category_id, base_unit_id, status, remark)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id,
          code,
          name,
          category_id,
          base_unit_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: UpdateMaterial :one
UPDATE materials
SET code = $2,
    name = $3,
    category_id = $4,
    base_unit_id = $5,
    status = $6,
    remark = $7,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          code,
          name,
          category_id,
          base_unit_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: SoftDeleteMaterial :execrows
UPDATE materials
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: MaterialCodeExists :one
SELECT EXISTS (
    SELECT 1
    FROM materials
    WHERE code = $1
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;

-- name: MaterialCategoryExists :one
SELECT EXISTS (
    SELECT 1
    FROM material_categories
    WHERE id = $1
      AND deleted_at IS NULL
) AS exists;

-- name: UnitExists :one
SELECT EXISTS (
    SELECT 1
    FROM units
    WHERE id = $1
      AND deleted_at IS NULL
) AS exists;
