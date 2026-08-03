-- Migration metadata
-- status: applied
-- description: add admin login safety fields

ALTER TABLE admin_users
    ADD COLUMN password_initialized BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN password_changed_at TIMESTAMPTZ NULL;

COMMENT ON COLUMN admin_users.password_initialized IS 'Whether a deployment-specific administrator password has been initialized';
COMMENT ON COLUMN admin_users.must_change_password IS 'Whether normal administrator access is blocked until the password is changed';
COMMENT ON COLUMN admin_users.password_changed_at IS 'Time of the latest successful administrator password change';
