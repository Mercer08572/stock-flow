-- name: GetAdminByUsername :one
SELECT id, username, password_hash, password_initialized, must_change_password, password_changed_at, status, created_at, updated_at
FROM admin_users
WHERE username = $1
  AND deleted_at IS NULL;

-- name: GetAdminByID :one
SELECT id, username, password_hash, password_initialized, must_change_password, password_changed_at, status, created_at, updated_at
FROM admin_users
WHERE id = $1
  AND deleted_at IS NULL;

-- 返回 updated 表示"本次是否真的写入口令"（不能用 password_initialized：写入成功与已初始化时它都是 TRUE）。
-- name: InitializeAdminPassword :one
WITH target AS (
    SELECT id, password_initialized
    FROM admin_users
    WHERE admin_users.username = sqlc.arg('username')
      AND admin_users.deleted_at IS NULL
    FOR UPDATE
),
updated AS (
    UPDATE admin_users
    SET password_hash = sqlc.arg('password_hash'),
        password_initialized = TRUE,
        must_change_password = TRUE,
        password_changed_at = NULL,
        updated_at = NOW()
    WHERE id IN (SELECT id FROM target WHERE NOT password_initialized)
    RETURNING id
)
SELECT
    target.id,
    (SELECT EXISTS (SELECT 1 FROM updated))::boolean AS updated
FROM target;

-- name: ChangeAdminPassword :execrows
UPDATE admin_users
SET password_hash = $2,
    must_change_password = FALSE,
    password_changed_at = $3,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListAPIApps :many
SELECT id, app_id, name, description, status, metadata, created_at, updated_at
FROM api_apps
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
ORDER BY id
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: CountAPIApps :one
-- 必须与 ListAPIApps 的过滤条件保持一致。
SELECT COUNT(*)
FROM api_apps
WHERE deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text);

-- name: GetAPIAppByID :one
SELECT id, app_id, name, description, status, metadata, created_at, updated_at
FROM api_apps
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetAPIAppByIdentifier :one
SELECT id, app_id, name, description, status, metadata, created_at, updated_at
FROM api_apps
WHERE app_id = $1
  AND deleted_at IS NULL;

-- name: CreateAPIApp :one
INSERT INTO api_apps (app_id, name, description, status, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, app_id, name, description, status, metadata, created_at, updated_at;

-- name: UpdateAPIApp :one
UPDATE api_apps
SET name = $2,
    description = $3,
    status = $4,
    metadata = $5,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, app_id, name, description, status, metadata, created_at, updated_at;

-- name: SoftDeleteAPIApp :execrows
UPDATE api_apps
SET deleted_at = NOW(),
    status = 'inactive',
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: CreateAPISecret :one
INSERT INTO api_secrets (
    api_app_id,
    secret_id,
    secret_hash,
    name,
    status,
    bound_metadata,
    expires_at
)
VALUES ($1, $2, $3, $4, 'active', $5, $6)
RETURNING id, api_app_id, secret_id, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at;

-- name: ListAPISecrets :many
SELECT id, api_app_id, secret_id, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at
FROM api_secrets
WHERE api_app_id = $1
  AND deleted_at IS NULL
ORDER BY id;

-- name: ListAPISecretsForAuthentication :many
SELECT id, api_app_id, secret_id, secret_hash, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at
FROM api_secrets
WHERE api_app_id = $1
  AND deleted_at IS NULL
ORDER BY id;

-- name: BlockAPISecret :one
UPDATE api_secrets
SET status = 'blocked',
    updated_at = NOW()
WHERE api_app_id = $1
  AND secret_id = $2
  AND deleted_at IS NULL
RETURNING id, api_app_id, secret_id, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at;

-- name: TouchAPISecretLastUsed :execrows
UPDATE api_secrets
SET last_used_at = $2,
    updated_at = NOW()
WHERE id = $1
  AND status = 'active'
  AND deleted_at IS NULL;

-- name: GetAPISecretForReissue :one
-- 只取重新签发所需的两列：状态用于判断是否仍可轮换，元数据用于让新密钥继承旧密钥的绑定信息。
SELECT status, bound_metadata
FROM api_secrets
WHERE api_app_id = $1
  AND secret_id = $2
  AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateAPISecret :one
UPDATE api_secrets
SET name = $3,
    expires_at = $4,
    updated_at = NOW()
WHERE api_app_id = $1
  AND secret_id = $2
  AND deleted_at IS NULL
RETURNING id, api_app_id, secret_id, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at;

-- name: UnblockAPISecret :one
UPDATE api_secrets
SET status = 'active',
    updated_at = NOW()
WHERE api_app_id = $1
  AND secret_id = $2
  AND deleted_at IS NULL
RETURNING id, api_app_id, secret_id, name, status, bound_metadata, expires_at, last_used_at, created_at, updated_at;

-- name: SoftDeleteAPISecret :execrows
UPDATE api_secrets
SET deleted_at = NOW(),
    status = 'blocked',
    updated_at = NOW()
WHERE api_app_id = $1
  AND secret_id = $2
  AND deleted_at IS NULL;
