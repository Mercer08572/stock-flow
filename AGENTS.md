# Stock-Flow — AGENTS.md

## Project Scope

This file applies only to the `stock-flow/` backend project. Frontend-specific instructions belong in `../stock-flow-admin/AGENTS.md` and do not apply here.

## Project Overview

Stock—Flow is a back-end API service for an inventory management system.



A modular monolith inventory system for managing:

- Materials
- Products
- SKU
- Inventory
- Warehouses
- Batches
- Inventory Reservations
- Inventory Movements

Future modules may include:

- Purchase Management
- Sales Management
- Approval Workflow
- Inventory Valuation
- Reporting


## Backend Tech Stack

- **Language**：Go 1.25+
- **Web framework**：Gin
- **Database**：PostgreSQL（use `pgx` driver + `sqlc`）
- **Migration tool**：golang-migrate

## Architecture 

- Modular Monolith
- DDD Lite
- Clean Architecture

### Modular Boundaries

Each business domain is implemented as an independent module.

Examples:
- material
- sku
- inventory
- warehouse

Modules communicate through application services.
Cross-module repository access by anti-corruption layer, avoid direct access.
This service does not own inbound or outbound order modules. External business systems should call inventory operation APIs for stock changes.

## Repository Structure

```
stock-flow/
├── cmd/
│   
├── internal/
│   ├── material/
│   │   
│   ├── sku/
│   │   
│   ├── inventory/
│   │   
│   ├── warehouse/
│   │   
│   └── shared/
│       
├── pkg/
│
├── migrations/
│   └── AGENTS.md                    # 数据库迁移规范
│
├── sql/
│
├── tasks/
│
└── AGENTS.md                        # 本文件
```

## Development Principles

- Business logic belongs in domain layer.
- Repository layer contains persistence logic only.
- Transactions are managed at application service level.
- No ORM.
- Prefer explicit code over framework magic.

### Three layer architecture (Must be strictly adhered to)

```
HTTP Request → Handler → Service → Repository → PostgreSQL
```


### Interface-first principle

For each layer, define the interface first, then write the implementation. This allows it to be replaced with a mock in tests.

```go
// Define the interface first
type MaterialService interface {
    List(ctx context.Context) ([]Material, error)
    Create(ctx context.Context, req CreateMaterialRequest) (*Material, error)
}

// implementation
type materialService struct {
    repo MaterialRepository
}

// Constructor injection
func NewMaterialService(repo MaterialRepository) MaterialService {
    return &materialService{repo: repo}
}
```

All dependencies should be injected via the constructor NewXxx(dep). Global variables are prohibited for passing dependencies.

## API Rules

- Base path：`/api/v1`
- Resource names should be plural nouns `/api/v1/materials`
- Standard HTTP function：GET（list/detail）、POST（create）、PUT（update）、DELETE（soft delete）

### Unified response format

All responses must be returned through the `pkg/response` package, with a fixed format:

```json
// success
{
    "code": 200,
    "message": "success",
    "data": {...},
    "trace_id": "req_abc123xyz",
    "timestamp": 1672531200000
}
// failure
{
    "code": 1001,
    "message": "error msg",
    "data": null,
    "trace_id": "req_abc123xyz",
    "timestamp": 1672531200000
}
```

## Before Implementing Any Feature
1. Read AGENTS.md
2. Read related docs/domain documents.
3. Read related tasks documents.
4. Follow module boundaries.
5. Add tests when business logic changes.


## Documentation Priority order:
0. The user explicitly requested this time.
1. AGENTS.md
2. Sub AGENTS.md in package
3. tasks
5. Source Code

If conflicts exist, higher priority documents win.
