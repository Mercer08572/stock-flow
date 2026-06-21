MIGRATIONS_PATH := migrations
SCHEMA_PATH := sql/schema/schema.sql
SQLC_VERSION := v1.29.0
API_PACKAGE := ./cmd/api
CONFIG_PACKAGE := ./cmd/config
PG_DUMP ?= pg_dump

.PHONY: help fmt test run sqlc config-database-url schema-dump migrate-up migrate-down migrate-down-all migrate-version migrate-force

help:
	@echo "Available commands:"
	@echo ""
	@echo "Go:"
	@echo "  make fmt               - Format Go source files"
	@echo "  make test              - Run Go tests"
	@echo "  make run               - Run API server"
	@echo ""
	@echo "Code generation:"
	@echo "  make sqlc              - Generate sqlc Go code"
	@echo "  make schema-dump       - Dump current database schema for sqlc (requires pg_dump)"
	@echo "  make config-database-url - Print resolved database_url"
	@echo ""
	@echo "Database migrations:"
	@echo "  make migrate-up        - Apply all pending migrations"
	@echo "  make migrate-down      - Rollback the last migration"
	@echo "  make migrate-down-all  - Rollback all migrations (use with caution)"
	@echo "  make migrate-version   - Show current migration version"
	@echo "  make migrate-force     - Force set migration version (requires VERSION=xxx)"
	@echo ""
	@echo "Examples:"
	@echo "  make sqlc"
	@echo "  make schema-dump"
	@echo "  make migrate-force VERSION=202606120003"

fmt:
	go fmt ./...

test:
	go test ./...

run:
	go run $(API_PACKAGE)

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

config-database-url:
	go run $(CONFIG_PACKAGE) -key database_url

schema-dump:
	@mkdir -p $(dir $(SCHEMA_PATH))
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	tmp_file="$(SCHEMA_PATH).tmp"; \
	$(PG_DUMP) --schema-only --no-owner --no-privileges --no-comments --schema=public --file "$$tmp_file" "$$db_url"; \
	sed '/^\\/d' "$$tmp_file" > "$(SCHEMA_PATH)"; \
	rm -f "$$tmp_file"

migrate-up:
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	migrate -path $(MIGRATIONS_PATH) -database "$$db_url" up

migrate-down:
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	migrate -path $(MIGRATIONS_PATH) -database "$$db_url" down 1

migrate-down-all:
	@echo "WARNING: This will rollback ALL migrations!"
	@echo "Press Ctrl+C to cancel, or press Enter to continue..."
	@read confirm
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	migrate -path $(MIGRATIONS_PATH) -database "$$db_url" down

migrate-version:
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	migrate -path $(MIGRATIONS_PATH) -database "$$db_url" version

migrate-force:
ifndef VERSION
	$(error VERSION is required. Usage: make migrate-force VERSION=202606120003)
endif
	@db_url="$$(go run $(CONFIG_PACKAGE) -key database_url)"; \
	migrate -path $(MIGRATIONS_PATH) -database "$$db_url" force $(VERSION)
