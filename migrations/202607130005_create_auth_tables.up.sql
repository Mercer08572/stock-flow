-- Migration metadata
-- status: applied
-- description: create admin and API app authentication tables

CREATE TABLE admin_users (
    id            BIGSERIAL   PRIMARY KEY,
    username      TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ NULL,

    CONSTRAINT chk_admin_users_username_not_blank
        CHECK (btrim(username) <> ''),
    CONSTRAINT chk_admin_users_password_hash_argon2id
        CHECK (password_hash LIKE '$argon2id$%'),
    CONSTRAINT chk_admin_users_status
        CHECK (status IN ('active', 'inactive'))
);

CREATE UNIQUE INDEX ux_admin_users_username
    ON admin_users (username)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_admin_users_status
    ON admin_users (status)
    WHERE deleted_at IS NULL;

CREATE TABLE api_apps (
    id          BIGSERIAL   PRIMARY KEY,
    app_id      TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NULL,
    status      TEXT        NOT NULL DEFAULT 'active',
    metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL,

    CONSTRAINT chk_api_apps_app_id_not_blank
        CHECK (btrim(app_id) <> ''),
    CONSTRAINT chk_api_apps_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_api_apps_status
        CHECK (status IN ('active', 'inactive')),
    CONSTRAINT chk_api_apps_metadata_object
        CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE UNIQUE INDEX ux_api_apps_app_id
    ON api_apps (app_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_api_apps_status
    ON api_apps (status)
    WHERE deleted_at IS NULL;

CREATE TABLE api_secrets (
    id             BIGSERIAL   PRIMARY KEY,
    api_app_id     BIGINT      NOT NULL REFERENCES api_apps(id),
    secret_id      TEXT        NOT NULL,
    secret_hash    TEXT        NOT NULL,
    name           TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'active',
    bound_metadata JSONB       NOT NULL DEFAULT '{}'::jsonb,
    expires_at     TIMESTAMPTZ NULL,
    last_used_at   TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ NULL,

    CONSTRAINT chk_api_secrets_secret_id_not_blank
        CHECK (btrim(secret_id) <> ''),
    CONSTRAINT chk_api_secrets_secret_hash_not_blank
        CHECK (btrim(secret_hash) <> ''),
    CONSTRAINT chk_api_secrets_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT chk_api_secrets_status
        CHECK (status IN ('active', 'blocked')),
    CONSTRAINT chk_api_secrets_bound_metadata_object
        CHECK (jsonb_typeof(bound_metadata) = 'object')
);

CREATE UNIQUE INDEX ux_api_secrets_secret_id
    ON api_secrets (secret_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_api_secrets_api_app_id
    ON api_secrets (api_app_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_api_secrets_status
    ON api_secrets (status)
    WHERE deleted_at IS NULL;

-- Bootstrap administrator row.
--
-- A brand-new database must contain exactly one administrator account, otherwise
-- `stock-flow-admin init` has nothing to update and can never provision a deployment.
--
-- password_hash below is an Argon2id hash of a random value that is not recorded anywhere,
-- so it matches chk_admin_users_password_hash_argon2id while remaining impossible to log in with.
-- The real initial password is written by `stock-flow-admin init`; until then the row keeps
-- password_initialized = FALSE / must_change_password = TRUE
-- (defaults added by 202607140006), which internal/auth uses to reject every login attempt.
-- Do not replace it with a reusable plain password or a fixed usable hash.
INSERT INTO admin_users (username, password_hash, status)
VALUES (
    'admin',
    '$argon2id$v=19$m=65536,t=3,p=2$kh1tRCiets3++0uR69wK7Q$P3YXeq+qkOyBuldryA72SBa8YkKFtvPM/5B1K1j6uQU',
    'active'
);

COMMENT ON TABLE admin_users IS 'Stock-Flow administrator accounts';
COMMENT ON COLUMN admin_users.password_hash IS 'Encoded Argon2id password hash';
COMMENT ON TABLE api_apps IS 'Registered external systems allowed to call Stock-Flow APIs';
COMMENT ON COLUMN api_apps.app_id IS 'Public application identifier issued by Stock-Flow';
COMMENT ON TABLE api_secrets IS 'Hashed credentials issued to API applications';
COMMENT ON COLUMN api_secrets.secret_hash IS 'One-way hash of a secret whose plain value is shown only at issuance';
