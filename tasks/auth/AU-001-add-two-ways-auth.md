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

It needs two simple authentication methods:

- Admin users log in to manage Stock-Flow data.
- External business systems call Stock-Flow APIs with an app ID and a secret.

The project does not have an auth module yet.

## Goal

Add an auth module for two request types.

The completed task should support:

- Admin login with username and password.
- Admin login state with session and cookie.
- Protected Stock-Flow APIs can be called by authenticated admins.
- Protected Stock-Flow APIs can be called by authenticated external business systems.
- API app registration by Stock-Flow.
- Secret issuing for an API app.
- API authentication with `app_id` and `secret`.
- Auth information in request context, so later operation logs can record the caller.

## Non-Goals

- Do not implement RBAC.
- Do not implement multiple admin roles.
- Do not implement OAuth, JWT, or third-party login.
- Do not build a frontend login page.
- Do not implement a full operation log module.
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
- The admin account has `username` and `password_hash`.
- The password must never be stored as plain text.
- The password hash must use an encoded Argon2id string.
- Admin login uses username and password.
- After login, the server creates a session and sends a cookie.
- The cookie must not contain the password or password hash.
- Session data does not need to be saved in the database.
- Session expiration is required.
- Logout must invalidate the current session.
- Admin session authentication is accepted by protected Stock-Flow APIs.
- `POST /auth/admin/login`, `POST /auth/admin/logout`, and `GET /auth/admin/me` are admin-session lifecycle APIs and do not require API app secret authentication.

API app auth:

- External systems must use an `app_id` and a `secret`.
- The `app_id` must be registered by Stock-Flow.
- The secret must be issued by Stock-Flow.
- The plain secret is shown only once when it is created.
- The database must store only the secret hash.
- An API app can be active or inactive.
- A secret can be active or blocked.
- Inactive apps and blocked secrets cannot authenticate requests.
- A secret can bind simple metadata for logs, such as system name, owner, or purpose.
- Auth middleware should put app information into request context.

Protected API auth:

- Except admin-session lifecycle APIs, every Stock-Flow API must support both admin session authentication and API app secret authentication.
- Admin callers authenticate through the admin session cookie created by `POST /auth/admin/login`.
- External business systems authenticate through `X-Stock-Flow-App-ID` and `X-Stock-Flow-Secret` headers.
- If both authentication methods are provided in one request, the handler must reject the request as ambiguous instead of choosing one silently.
- Auth middleware must put a unified caller identity into request context.
- The caller identity must distinguish at least `admin` and `api_app` caller types.
- Admin caller context should include the admin user id and username.
- API app caller context should include the API app id, app identifier, secret id, and bound metadata.
- This task only provides auth context for later operation logs; it does not persist operation logs.

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

Protected API authentication:

- All APIs except `POST /api/v1/auth/admin/login`, `POST /api/v1/auth/admin/logout`, and `GET /api/v1/auth/admin/me` must accept a valid admin session cookie.
- All APIs except `POST /api/v1/auth/admin/login`, `POST /api/v1/auth/admin/logout`, and `GET /api/v1/auth/admin/me` must also accept valid API app credentials through headers.
- A request with neither valid admin session authentication nor valid API app secret authentication must be rejected as unauthenticated.
- A request with both admin session authentication and API app secret authentication must be rejected as ambiguous.

All responses must use `pkg/response`.

## Data Model

New migration is expected for this task.

Expected tables:

- `admin_users`
- `admin_sessions`
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

`admin_sessions` should include:

- `id`
- `admin_user_id`
- `session_token_hash`
- `status`
- `expires_at`
- `last_used_at`
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
- Store only hashes for passwords, session tokens, and API secrets.
- Persist admin sessions in `admin_sessions` so session authentication works consistently across protected APIs.
- Add an admin auth middleware for admin-only routes.
- Add an API app auth middleware for business API routes.
- Add a protected API auth middleware that accepts either a valid admin session or valid API app secret credentials.
- Put unified caller information into request context.
- Reject requests that provide both admin session and API app secret credentials.
- Do not expose the plain secret after creation.
- Seed the first admin account through a new migration.
- The seeded admin password hash must be an Argon2id encoded string and must not contain a plain password in source code comments.
- This task only provides auth context for later operation logs; do not implement persistent auth or operation logs in this task.

## Acceptance Criteria

- Admin can log in with a valid username and password.
- Admin login sets a session cookie.
- Invalid admin credentials return an error.
- `GET /api/v1/auth/admin/me` returns the current admin when the session is valid.
- Admin logout invalidates the current session.
- Admin password is stored as an Argon2id encoded string.
- The first admin account is created by a new migration seed.
- Protected APIs accept valid admin session authentication.
- Protected APIs accept valid API app secret authentication.
- Protected APIs reject unauthenticated requests.
- Protected APIs reject requests that include both admin session and API app secret authentication.
- An admin can create, list, update, and soft delete API apps.
- An admin can issue a secret for an API app.
- The plain secret is returned only when it is created.
- The database stores only the secret hash.
- A blocked secret cannot authenticate a request.
- An inactive app cannot authenticate a request.
- API app auth middleware reads `X-Stock-Flow-App-ID` and `X-Stock-Flow-Secret`.
- Protected API auth middleware adds a unified caller identity to request context.
- API app caller context includes app identity and bound metadata.
- Admin caller context includes admin identity.
- Auth context is available for a later operation log task, but this task does not persist operation logs.
- Handler tests cover admin login and at least one auth error.
- Middleware or handler tests cover protected API authentication by admin session, API app secret, missing credentials, and ambiguous credentials.
- Service tests cover password verification, persisted session expiration, secret issuing, and blocked secret behavior.
- `go test ./...` passes.
- `make sqlc` generated files are committed with the implementation.

## Resolved Decisions

- The first admin account is created by a migration seed.
- Except `POST /api/v1/auth/admin/login`, `POST /api/v1/auth/admin/logout`, and `GET /api/v1/auth/admin/me`, every Stock-Flow API must support both admin session authentication and API app secret authentication.
- This task only provides auth context for a later operation log task. It must not persist operation logs.

## Open Questions

- None.
