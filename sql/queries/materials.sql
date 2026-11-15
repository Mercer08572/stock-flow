-- name: ListMaterials :many
-- 分类与基础单位的引用快照随行内联（JOIN 不过滤 status / deleted_at，见 ListSKUs 说明）。
SELECT m.id,
       m.code,
       m.name,
       m.category_id,
       m.base_unit_id,
       m.status,
       m.remark,
       m.created_at,
       m.updated_at,
       c.code       AS category_code,
       c.name       AS category_name,
       c.status     AS category_status,
       c.deleted_at AS category_deleted_at,
       u.code       AS base_unit_code,
       u.name       AS base_unit_name,
       u.status     AS base_unit_status,
       u.deleted_at AS base_unit_deleted_at
FROM materials m
LEFT JOIN material_categories c ON c.id = m.category_id
LEFT JOIN units u ON u.id = m.base_unit_id
WHERE m.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR m.status = sqlc.narg('status')::text)
  AND (sqlc.narg('category_id')::bigint IS NULL OR m.category_id = sqlc.narg('category_id')::bigint)
ORDER BY m.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMaterials :one
-- 必须与 ListMaterials 的过滤条件保持一致。
SELECT COUNT(*)
FROM materials
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('category_id')::bigint IS NULL OR category_id = sqlc.narg('category_id')::bigint);

-- name: GetMaterialByID :one
SELECT m.id,
       m.code,
       m.name,
       m.category_id,
       m.base_unit_id,
       m.status,
       m.remark,
       m.created_at,
       m.updated_at,
       c.code       AS category_code,
       c.name       AS category_name,
       c.status     AS category_status,
       c.deleted_at AS category_deleted_at,
       u.code       AS base_unit_code,
       u.name       AS base_unit_name,
       u.status     AS base_unit_status,
       u.deleted_at AS base_unit_deleted_at
FROM materials m
LEFT JOIN material_categories c ON c.id = m.category_id
LEFT JOIN units u ON u.id = m.base_unit_id
WHERE m.id = $1
  AND m.deleted_at IS NULL;

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

-- name: MaterialExists :one
SELECT EXISTS (
    SELECT 1
    FROM materials
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

-- name: MaterialSKUUnitAllowed :one
SELECT EXISTS (
    SELECT 1
    FROM material_unit_conversions
    WHERE material_id = sqlc.arg('material_id')::bigint
      AND deleted_at IS NULL
      AND (from_unit_id = sqlc.arg('unit_id')::bigint OR to_unit_id = sqlc.arg('unit_id')::bigint)
) AS exists;

-- name: GetMaterialBaseUnit :one
SELECT base_unit_id
FROM materials
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetUnitsByIDs :many
SELECT id,
       unit_type
FROM units
WHERE id = ANY(sqlc.arg('unit_ids')::bigint[])
  AND deleted_at IS NULL;
