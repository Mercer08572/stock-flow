-- name: ListMaterialCategories :many
SELECT id,
       code,
       name,
       parent_id,
       status,
       remark,
       created_at,
       updated_at
FROM material_categories
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('parent_id')::bigint IS NULL OR parent_id = sqlc.narg('parent_id')::bigint)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetMaterialCategoryByID :one
SELECT id,
       code,
       name,
       parent_id,
       status,
       remark,
       created_at,
       updated_at
FROM material_categories
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateMaterialCategory :one
INSERT INTO material_categories (code, name, parent_id, status, remark)
VALUES ($1, $2, $3, $4, $5)
RETURNING id,
          code,
          name,
          parent_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: UpdateMaterialCategory :one
UPDATE material_categories
SET code = $2,
    name = $3,
    parent_id = $4,
    status = $5,
    remark = $6,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          code,
          name,
          parent_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: SoftDeleteMaterialCategory :execrows
UPDATE material_categories
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: MaterialCategoryCodeExists :one
SELECT EXISTS (
    SELECT 1
    FROM material_categories
    WHERE code = $1
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;
