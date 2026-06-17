# Load environment variables from .env file
include .env

# Migration path
MIGRATIONS_PATH := migrations

# Check if DATABASE_URL is set
ifndef DATABASE_URL
$(error DATABASE_URL is not set. Please create .env from .env.example)
endif

.PHONY: help migrate-up migrate-down migrate-down-all migrate-version migrate-force

# Default target: show help
help:
	@echo "Available migration commands:"
	@echo "  make migrate-up          - Apply all pending migrations"
	@echo "  make migrate-down        - Rollback the last migration"
	@echo "  make migrate-down-all    - Rollback all migrations (use with caution)"
	@echo "  make migrate-version     - Show current migration version"
	@echo "  make migrate-force       - Force set migration version (requires VERSION=xxx)"
	@echo ""
	@echo "Examples:"
	@echo "  make migrate-force VERSION=202606120003"

# Apply all pending migrations
migrate-up:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

# Rollback the last migration
migrate-down:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down 1

# Rollback all migrations (dangerous, use with caution)
migrate-down-all:
	@echo "WARNING: This will rollback ALL migrations!"
	@echo "Press Ctrl+C to cancel, or press Enter to continue..."
	@read confirm
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down

# Show current migration version
migrate-version:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version

# Force set migration version (use when database is in dirty state)
# Usage: make migrate-force VERSION=202606120003
migrate-force:
ifndef VERSION
	$(error VERSION is required. Usage: make migrate-force VERSION=202606120003)
endif
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" force $(VERSION)
