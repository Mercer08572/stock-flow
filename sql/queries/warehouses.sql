-- name: ListWarehouses :many
SELECT id,
       code,
       name,
       type,
       status,
       location,
       contact_name,
       contact_phone,
       remark,
       created_at,
       updated_at
FROM warehouses
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type')::text)
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetWarehouseByID :one
SELECT id,
       code,
       name,
       type,
       status,
       location,
       contact_name,
       contact_phone,
       remark,
       created_at,
       updated_at
FROM warehouses
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetWarehouseReference :one
SELECT id,
       code,
       name,
       deleted_at
FROM warehouses
WHERE id = $1;

-- name: CreateWarehouse :one
INSERT INTO warehouses (code, name, type, status, location, contact_name, contact_phone, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id,
          code,
          name,
          type,
          status,
          location,
          contact_name,
          contact_phone,
          remark,
          created_at,
          updated_at;

-- name: UpdateWarehouse :one
UPDATE warehouses
SET code = $2,
    name = $3,
    type = $4,
    status = $5,
    location = $6,
    contact_name = $7,
    contact_phone = $8,
    remark = $9,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          code,
          name,
          type,
          status,
          location,
          contact_name,
          contact_phone,
          remark,
          created_at,
          updated_at;

-- name: SoftDeleteWarehouse :execrows
UPDATE warehouses
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: DisableWarehouse :one
UPDATE warehouses
SET status = 'inactive',
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id,
          code,
          name,
          type,
          status,
          location,
          contact_name,
          contact_phone,
          remark,
          created_at,
          updated_at;

-- name: WarehouseCodeExists :one
SELECT EXISTS (
    SELECT 1
    FROM warehouses
    WHERE code = $1
      AND deleted_at IS NULL
      AND (sqlc.arg('exclude_id')::bigint = 0 OR id <> sqlc.arg('exclude_id')::bigint)
) AS exists;
