ALTER TABLE admin_users
    DROP COLUMN IF EXISTS password_changed_at,
    DROP COLUMN IF EXISTS must_change_password,
    DROP COLUMN IF EXISTS password_initialized;
