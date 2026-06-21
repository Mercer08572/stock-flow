# Development Guide

This document records the local development workflow for Stock-Flow.

## Commands

Use the project Makefile as the main command entrypoint.

```bash
make help
make fmt
make test
make run
make sqlc
```

## Runtime Configuration

The API loads configuration with this priority:

```text
default values < config file < environment variables
```

`APP_ENV` selects an implicit config file:

```bash
APP_ENV=development go run ./cmd/api
```

This looks for:

```text
configs/development.yaml
```

`CONFIG_FILE` can be used to explicitly choose a config file:

```bash
CONFIG_FILE=configs/development.yaml go run ./cmd/api
```

Environment variables override config file values:

```bash
APP_ENV=production DATABASE_URL=postgres://... ./stock-flow
```

Supported config keys:

```yaml
app_env: development
gin_mode: debug
http_addr: ":8080"
database_url: "postgres://postgres:postgres@localhost:5432/stock_flow_dev?sslmode=disable"
shutdown_timeout: "10s"
```

The equivalent environment variables are:

```text
APP_ENV
CONFIG_FILE
GIN_MODE
HTTP_ADDR
PORT
DATABASE_URL
SHUTDOWN_TIMEOUT
```

Real `configs/*.yaml` files are ignored by git. Commit only `*.example.yaml` templates.

Database migration commands resolve `database_url` through the same Go config loader used by the API, so `APP_ENV`, `CONFIG_FILE`, and `DATABASE_URL` work consistently for Makefile migration commands too.

```bash
make migrate-up
make migrate-down
make migrate-version
```

You can inspect the resolved migration database URL with:

```bash
make config-database-url
```

## Database Schema Sources

The project keeps two database schema artifacts with different purposes.

`migrations/` is the historical database change log. It is used to upgrade or roll back real databases.

`sql/schema/schema.sql` is the current schema snapshot for sqlc code generation. It should describe the database after all intended migrations have been applied.

In short:

```text
migrations = history
sql/schema/schema.sql = current shape for code generation
```

## Updating Schema And sqlc Code

When a database table changes:

1. Add a new migration under `migrations/`.
2. Apply migrations to the development database.
3. Refresh `sql/schema/schema.sql`.
4. Update query files under `sql/queries/`.
5. Regenerate sqlc code.
6. Run tests.

The usual flow is:

```bash
make migrate-up
make schema-dump
make sqlc
make test
```

## Dumping The Current PostgreSQL Schema

PostgreSQL provides `pg_dump --schema-only` for exporting database structure without table data.

The optional Makefile target is:

```bash
make schema-dump
```

This target requires PostgreSQL client tools because it calls `pg_dump`.

It writes the current database schema to:

```text
sql/schema/schema.sql
```

The command uses these options:

```bash
pg_dump --schema-only --no-owner --no-privileges --no-comments --schema=public
```

Notes:

- `pg_dump` must be installed locally through PostgreSQL client tools.
- Override the binary path with `PG_DUMP=/path/to/pg_dump make schema-dump` when needed.
- Make sure the target database has already been migrated to the intended version before dumping.
- The Makefile filters psql meta-command lines such as `\restrict` because sqlc expects regular SQL input.
- Always review the `sql/schema/schema.sql` diff after dumping.

## sqlc Layout

sqlc reads:

```text
sqlc.yaml
sql/schema/schema.sql
sql/queries/*.sql
```

Generated code is placed inside each module's `db` subpackage.

Example:

```text
sql/queries/materials.sql -> internal/material/db
```

Generated `db` packages are persistence adapters. Do not edit generated files manually.

The normal dependency flow remains:

```text
Handler -> Service -> Repository -> sqlc db package -> PostgreSQL
```

The service layer should use module-owned business types, not sqlc row types directly.

## Adding A New sqlc Module

For a new module such as `warehouse`:

1. Add query SQL under `sql/queries/warehouses.sql`.
2. Add a new `sql:` block in `sqlc.yaml`.
3. Set `out` to `internal/warehouse/db`.
4. Set `package` to a module-specific package name such as `warehousedb`.
5. Run `make sqlc`.
6. Keep repository code responsible for mapping sqlc rows to module business types.

Each module should have its own generated db package. Avoid sharing one generated db package across business modules unless there is a deliberate shared persistence boundary.

## Tests

Run all tests with:

```bash
make test
```

For sandboxed environments where Go cannot write to the default build cache, use a local cache path:

```bash
GOCACHE=/private/tmp/stock-flow-go-build-cache make test
```
