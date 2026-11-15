-- name: ListSKUs :many
-- 引用快照（物料 / 单位）随行内联，避免客户端再按 ID 二次查询后自行拼接。
-- JOIN 不带 status / deleted_at 过滤：停用或软删除的引用也要给出名字，
-- 由 reference_deleted_at / reference_status 说明其当前状态。
SELECT s.id,
       s.material_id,
       s.code,
       s.name,
       s.unit_id,
       s.status,
       s.remark,
       s.created_at,
       s.updated_at,
       m.code       AS material_code,
       m.name       AS material_name,
       m.status     AS material_status,
       m.deleted_at AS material_deleted_at,
       u.code       AS unit_code,
       u.name       AS unit_name,
       u.status     AS unit_status,
       u.deleted_at AS unit_deleted_at
FROM skus s
LEFT JOIN materials m ON m.id = s.material_id
LEFT JOIN units u ON u.id = s.unit_id
WHERE s.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text)
  AND (sqlc.narg('material_id')::bigint IS NULL OR s.material_id = sqlc.narg('material_id')::bigint)
  AND (sqlc.narg('unit_id')::bigint IS NULL OR s.unit_id = sqlc.narg('unit_id')::bigint)
  AND (sqlc.narg('keyword')::text IS NULL
       OR strpos(lower(s.code), lower(sqlc.narg('keyword')::text)) > 0
       OR strpos(lower(s.name), lower(sqlc.narg('keyword')::text)) > 0)
ORDER BY s.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountSKUs :one
-- 必须与 ListSKUs 的过滤条件保持一致。
SELECT COUNT(*)
FROM skus
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('material_id')::bigint IS NULL OR material_id = sqlc.narg('material_id')::bigint)
  AND (sqlc.narg('unit_id')::bigint IS NULL OR unit_id = sqlc.narg('unit_id')::bigint)
  AND (sqlc.narg('keyword')::text IS NULL
       OR strpos(lower(code), lower(sqlc.narg('keyword')::text)) > 0
       OR strpos(lower(name), lower(sqlc.narg('keyword')::text)) > 0);

-- name: GetSKUByID :one
SELECT s.id,
       s.material_id,
       s.code,
       s.name,
       s.unit_id,
       s.status,
       s.remark,
       s.created_at,
       s.updated_at,
       m.code       AS material_code,
       m.name       AS material_name,
       m.status     AS material_status,
       m.deleted_at AS material_deleted_at,
       u.code       AS unit_code,
       u.name       AS unit_name,
       u.status     AS unit_status,
       u.deleted_at AS unit_deleted_at
FROM skus s
LEFT JOIN materials m ON m.id = s.material_id
LEFT JOIN units u ON u.id = s.unit_id
WHERE s.id = $1
  AND s.deleted_at IS NULL;

-- name: GetSKUReference :one
SELECT id,
       code,
       name,
       deleted_at
FROM skus
WHERE id = $1;

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
