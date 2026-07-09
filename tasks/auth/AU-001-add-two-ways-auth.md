# AU-001 Two Auth Methods

Status: Ready
Owner: coding-agent
Module: auth
Related:
- internal/auth/
- internal/shared/http/router.go
- internal/shared/http/middleware/
- internal/shared/config/
- pkg/response/
- sql/queries/
- sql/schema/schema.sql
- sqlc.yaml
- migrations/
- docs/architecture/dependency-rules.md
- docs/architecture/module-boundaries.md

## Background

Stock-Flow is an independent inventory service.

It needs two simple auth methods:

- Admin users log in to manage Stock-Flow data.
- External business systems call Stock-Flow APIs with an app ID and a secret.

The project does not have an auth module yet.

## Goal

Add an auth module for admin users and external API callers.

The completed task should support:

- Admin login with username and password.
- Admin session with cookie.
- API app registration by Stock-Flow.
- Secret issuing for an API app.
- API authentication with `app_id` and `secret`.
- A unified caller context for later operation logs.

## Non-Goals

- Do not implement RBAC.
- Do not implement multiple admin roles.
- Do not implement OAuth, JWT, or third-party login.
- Do not build a frontend login page.
- Do not implement persistent operation logs.
- Do not persist admin sessions in the database.
- Do not modify existing migration files.
- Do not change the unified response format.

## Scope

Allowed to modify or add:

- `internal/auth/`
- `internal/shared/http/router.go`
- `internal/shared/http/router_test.go`
- `internal/shared/http/middleware/`
- `internal/shared/config/`
- `sql/queries/auth.sql`
- `sql/schema/schema.sql`
- `sqlc.yaml`
- new migration files
- auth-focused tests

Should not modify:

- `internal/material/`
- `internal/sku/`
- `internal/inventory/`
- `internal/warehouse/`
- existing migration files
- `pkg/response` response format

## Domain Rules

Admin auth:

- This task only supports one admin account.
- The first admin account must be created by a migration seed.
- The admin account has `username` and `password_hash`.
- The password must never be stored as plain text.
- The password hash must use an encoded Argon2id string.
- Admin login uses username and password.
- After login, the server creates a session and sends a cookie.
- The cookie must not contain the password or password hash.
- Session data does not need to be saved in the database.
- Session expiration is required.
- Logout must invalidate the current session.

API app auth:

- External systems must use an `app_id` and a `secret`.
- Admin users must not use API app secrets.
- The `app_id` must be registered by Stock-Flow.
- The secret must be issued by Stock-Flow.
- The plain secret is shown only once when it is created.
- The database must store only the secret hash.
- An API app can be active or inactive.
- A secret can be active or blocked.
- Inactive apps and blocked secrets cannot authenticate requests.
- A secret can bind simple metadata, such as system name, owner, or purpose.

Protected API auth:

- Every protected Stock-Flow API must support two auth methods:
  - admin session cookie
  - API app secret headers
- Admin users should call protected APIs with the session cookie from login.
- External systems should call protected APIs with app secret headers.
- API app secret headers are only for external systems.
- A request must use only one auth method.
- If both auth methods are provided, reject the request as ambiguous.
- The login API is public.
- Logout and `me` APIs require a valid admin session, but they do not require an API app secret.
- Auth middleware must put a unified caller identity into request context.
- The caller identity must distinguish `admin` and `api_app`.
- Admin caller context should include admin user id and username.
- API app caller context should include app id, app identifier, secret id, and bound metadata.
- This task only provides auth context for a later log task.

## API

All endpoints are under `/api/v1`.

Admin auth:

- `POST /auth/admin/login`
  - Login with username and password.
- `POST /auth/admin/logout`
  - Logout the current admin session.
- `GET /auth/admin/me`
  - Return the current admin identity.

API app management:

- `GET /auth/apps`
  - List API apps.
- `POST /auth/apps`
  - Create an API app.
- `GET /auth/apps/:id`
  - Get API app detail.
- `PUT /auth/apps/:id`
  - Update API app basic information.
- `DELETE /auth/apps/:id`
  - Soft delete an API app.
- `POST /auth/apps/:id/secrets`
  - Issue a new secret for an API app.
- `GET /auth/apps/:id/secrets`
  - List secret records without plain secret values.
- `POST /auth/apps/:id/secrets/:secret_id/block`
  - Block a secret.

External systems should send credentials by headers:

- `X-Stock-Flow-App-ID`
- `X-Stock-Flow-Secret`

All responses must use `pkg/response`.

## Data Model

New migration is expected for this task.

Expected tables:

- `admin_users`
- `api_apps`
- `api_secrets`

`admin_users` should include:

- `id`
- `username`
- `password_hash`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

`api_apps` should include:

- `id`
- `app_id`
- `name`
- `description`
- `status`
- `metadata`
- `created_at`
- `updated_at`
- `deleted_at`

`api_secrets` should include:

- `id`
- `api_app_id`
- `secret_id`
- `secret_hash`
- `name`
- `status`
- `bound_metadata`
- `expires_at`
- `last_used_at`
- `created_at`
- `updated_at`
- `deleted_at`

Required sqlc work:

- Add `sql/queries/auth.sql`.
- Add a new `sql:` block in `sqlc.yaml` for the auth module.
- Generate code into `internal/auth/db`.
- Use package name `authdb`.
- Keep generated files unedited after generation.

## Implementation Notes

- Follow `Handler -> Service -> Repository -> PostgreSQL`.
- Define interfaces before implementations.
- Use constructor injection with `NewXxx(dep)`.
- Service owns validation, password verification, session rules, and secret rules.
- Repository owns sqlc calls, row mapping, and PostgreSQL error mapping only.
- Handler owns HTTP parsing and response writing only.
- Use `crypto/rand` for session token and secret generation.
- Use constant-time comparison for secret verification.
- Store only hashes for passwords and API secrets.
- Keep admin session storage simple and in memory for this task.
- Add an admin session middleware.
- Add an API app secret middleware for external business systems.
- Add a protected API middleware that accepts either admin session or API app secret.
- Put unified caller information into request context.
- Reject requests that provide both auth methods.
- Do not expose the plain secret after creation.
- Seed the first admin account through a new migration.
- The seeded admin password hash must be an Argon2id encoded string.
- The source code and comments must not contain the plain admin password.
- Do not implement persistent operation logs in this task.

## Acceptance Criteria

- The first admin account is created by a new migration seed.
- Admin can log in with a valid username and password.
- Admin login sets a session cookie.
- Invalid admin credentials return an error.
- `GET /api/v1/auth/admin/me` returns the current admin when the session is valid.
- Admin logout invalidates the current session.
- Admin password is stored as an Argon2id encoded string.
- Protected APIs accept valid admin session authentication.
- Protected APIs accept valid API app secret authentication.
- Protected APIs reject unauthenticated requests.
- Protected APIs reject requests that include both auth methods.
- An admin can create, list, update, and soft delete API apps.
- An admin can issue a secret for an API app.
- Admin users do not receive or use API app secrets for their own requests.
- The plain secret is returned only when it is created.
- The database stores only the secret hash.
- A blocked secret cannot authenticate a request.
- An inactive app cannot authenticate a request.
- API app auth middleware reads `X-Stock-Flow-App-ID` and `X-Stock-Flow-Secret`.
- Protected API auth middleware adds a unified caller identity to request context.
- API app caller context includes app identity and bound metadata.
- Admin caller context includes admin identity.
- Auth context is available for a later operation log task.
- This task does not persist operation logs.
- Handler tests cover admin login and at least one auth error.
- Middleware or handler tests cover admin session auth, API app secret auth, missing credentials, and ambiguous credentials.
- Service tests cover password verification, session expiration, secret issuing, and blocked secret behavior.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Resolved Decisions

- The first admin account is created by a migration seed.
- Protected APIs support two auth methods: admin session cookie or API app secret headers.
- A request must use only one auth method.
- Admin users use session only.
- API app secrets are only for external business systems.
- This task only provides auth context for a later operation log task.

## Open Questions

- None.
