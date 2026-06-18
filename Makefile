# Load environment variables from .env file when present.
-include .env

MIGRATIONS_PATH := migrations
SQLC_VERSION := v1.29.0
API_PACKAGE := ./cmd/api

.PHONY: help fmt test run sqlc migrate-up migrate-down migrate-down-all migrate-version migrate-force require-database-url

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
	@echo "  make migrate-force VERSION=202606120003"

fmt:
	go fmt ./...

test:
	go test ./...

run:
	go run $(API_PACKAGE)

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

migrate-up: require-database-url
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

migrate-down: require-database-url
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down 1

migrate-down-all: require-database-url
	@echo "WARNING: This will rollback ALL migrations!"
	@echo "Press Ctrl+C to cancel, or press Enter to continue..."
	@read confirm
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down

migrate-version: require-database-url
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version

migrate-force: require-database-url
ifndef VERSION
	$(error VERSION is required. Usage: make migrate-force VERSION=202606120003)
endif
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" force $(VERSION)

require-database-url:
ifndef DATABASE_URL
	$(error DATABASE_URL is not set. Please create .env from .env.example)
endif
