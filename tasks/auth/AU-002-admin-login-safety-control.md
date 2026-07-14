# AU-002 Admin Login Safety Control

Status: Done
Owner: coding-agent
Module: auth
Related:
- internal/auth/
- internal/shared/config/
- internal/shared/http/router.go
- cmd/admin/
- migrations/
- sql/queries/auth.sql
- sql/schema/schema.sql
- tasks/auth/AU-001-add-two-ways-auth.md

## Background

The current auth migration seeds an administrator with a fixed password hash: [202607130005_create_auth_tables.up.sql (line 93)](/Users/badbugu/workspace/myself_project/stock-flow/migrations/202607130005_create_auth_tables.up.sql:93).

The auth module does not provide an initial-password setup flow, a password-change flow, or a way to force the administrator to change the seeded password after the first login. Login security settings and login attempt controls also need to be made explicit.

## Goal

Improve administrator login safety by adding:

- A mandatory password change for an administrator using an initial password.
- Configurable session TTL and cookie `SameSite` and `Secure` attributes.
- Login rate limiting.

## Non-Goals

- Do not implement RBAC or multiple administrator roles.
- Do not add OAuth, JWT, multi-factor authentication, or third-party login.
- Do not add password recovery by email, SMS, or another external channel.
- Do not change API app authentication or secret management.
- Do not build a frontend password-change page.
- Do not change the unified response format.
- Do not modify an existing migration file; add a new migration when schema or seed changes are required.

## Scope

Allowed to modify or add:

- `internal/auth/`
- `internal/shared/config/`
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `cmd/admin/`
- `sql/queries/auth.sql`
- `sql/schema/schema.sql`
- `sqlc.yaml`, if sqlc configuration changes are required
- new migration files
- auth-focused tests

Should not modify:

- `internal/material/`
- `internal/sku/`
- `internal/inventory/`
- `internal/warehouse/`
- existing migration files
- API app authentication behavior
- `pkg/response` response format

## Domain Rules

- An administrator using an initial password must be marked as requiring a password change.
- A successful login with an initial password must not grant normal access to protected business APIs until the password has been changed.
- The administrator must be allowed to access the password-change and logout operations while a password change is required.
- A password change must verify the current password before storing a new password.
- Passwords must never be stored or logged as plain text.
- New password hashes must use the existing encoded Argon2id format.
- After a successful password change, the administrator must no longer be marked as requiring a password change.
- Session expiration must use the configured session TTL.
- The admin session cookie must remain `HttpOnly` and use the configured `SameSite` and `Secure` attributes.
- Login rate limiting must apply to the admin login endpoint.
- Failed login responses must not reveal whether the username exists.
- Rate-limited responses must use the unified response format.

## API

All endpoints are under `/api/v1`.

Existing endpoint affected by this task:

- `POST /auth/admin/login`
  - Apply login rate limiting.
  - Indicate that a password change is required without exposing session credentials or password data.

New endpoint expected by this task:

- `PUT /auth/admin/password`
  - Require a valid admin session.
  - Accept the current password and a new password.
  - Clear the forced-password-change state after a successful update.

All responses must use `pkg/response`.

## Data Model

- Add a new migration if the administrator record needs a forced-password-change field or other persistent state.
- Update `sql/schema/schema.sql` to match the migration.
- Add or update sqlc queries needed to read the forced-password-change state and update the password hash.
- Generate auth sqlc code into `internal/auth/db` and do not manually edit generated files.
- Do not edit `202607130005_create_auth_tables.up.sql`; preserve migration history.

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Keep password validation, forced-password-change rules, session rules, and rate-limit decisions in the service or domain layer.
- Keep HTTP parsing, cookie writing, and response writing in the handler layer.
- Keep persistence and PostgreSQL error mapping in the repository layer.
- Load session TTL and cookie security settings through `internal/shared/config` and inject them through router dependencies.
- Keep rate-limit state behind an interface so it can be replaced in tests or by another implementation.
- Do not expose plain passwords, password hashes, or session tokens in responses or logs.
- Add tests for business rules and affected HTTP behavior.

## Acceptance Criteria

- An administrator marked as requiring a password change can log in but cannot access normal protected business APIs.
- The restricted administrator can change the password after providing the correct current password.
- A successful password change stores an Argon2id hash and clears the forced-password-change state.
- An incorrect current password does not change the stored password or forced-password-change state.
- Session expiration follows the configured session TTL.
- Login and logout cookies use the configured `SameSite` and `Secure` attributes and remain `HttpOnly`.
- Repeated admin login attempts are rate limited according to configuration.
- Login failures do not disclose whether an administrator username exists.
- Existing admin session and API app authentication behavior remains compatible except for the explicit forced-password-change restriction.
- Necessary service, handler, middleware, configuration, and repository tests are added or updated.
- `go test ./...` passes.

## Decisions

- Provide the initial password through a one-time `stock-flow admin init` command. Read it from a non-echoing terminal prompt, with a temporary environment variable supported for automated deployments. Do not store a reusable plain password or fixed usable password hash in source control.
- Require passwords to contain 12 to 128 characters. Do not require character-class composition. Reject a password that equals the username, ignoring case, or equals the current password.
- After a password change, invalidate all existing sessions for the administrator and issue a new session for the current client.
- Use an 8-hour default session TTL. Allow configured values from 15 minutes through 24 hours and reject invalid startup configuration.
- Default cookies to `SameSite=Lax`. Allow `Secure=false` in development and test, but require `Secure=true` in production. `HttpOnly` must always remain enabled.
- Rate limit by normalized username plus trusted client IP, and also enforce a client-IP-wide limit. Do not trust forwarding headers unless the proxy is configured as trusted.
- Allow five failed attempts per IP-and-username key in 15 minutes, followed by a 15-minute lockout. Allow 30 total login attempts per IP in 15 minutes. Return HTTP 429 with `Retry-After` when limited.
- Keep rate-limit state in memory behind an interface for this task. This provides single-instance consistency only; a shared implementation such as Redis requires a separate task before multi-instance deployment.
