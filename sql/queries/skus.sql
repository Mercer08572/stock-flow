-- name: ListSKUs :many
SELECT id,
       material_id,
       code,
       name,
       unit_id,
       status,
       remark,
       created_at,
       updated_at
FROM skus
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('material_id')::bigint IS NULL OR material_id = sqlc.narg('material_id')::bigint)
  AND (sqlc.narg('unit_id')::bigint IS NULL OR unit_id = sqlc.narg('unit_id')::bigint)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetSKUByID :one
SELECT id,
       material_id,
       code,
       name,
       unit_id,
       status,
       remark,
       created_at,
       updated_at
FROM skus
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateSKU :one
INSERT INTO skus (material_id, code, name, unit_id, status, remark)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id,
          material_id,
          code,
          name,
          unit_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: UpdateSKU :one
UPDATE skus
SET material_id = $2,
    code = $3,
    name = $4,
    unit_id = $5,
    status = $6,
    remark = $7,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          material_id,
          code,
          name,
          unit_id,
          status,
          remark,
          created_at,
          updated_at;

-- name: SoftDeleteSKU :execrows
UPDATE skus
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: SKUCodeExists :one
SELECT EXISTS (
    SELECT 1
    FROM skus
    WHERE code = $1
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;

-- name: ActiveSKUExistsForMaterial :one
SELECT EXISTS (
    SELECT 1
    FROM skus
    WHERE material_id = $1
      AND status = 'active'
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;
