# Dependency Rules

Stock-Flow follows a strict three-layer request flow:

```text
HTTP Request -> Handler -> Service -> Repository -> PostgreSQL
```

Dependencies must point in one direction only:

```text
handler -> service -> repository
```

## Handler Layer

- Handles HTTP requests and responses.
- Parses path, query, and request body data.
- Performs request-level validation only.
- Calls service interfaces.
- Returns responses through `pkg/response`.
- Must not contain business logic.
- Must not call repositories directly.

## Service Layer

- Owns application use cases and business orchestration.
- Calls repository interfaces for persistence.
- Coordinates cross-module application service calls.
- Enforces inventory dimensions such as warehouse when changing stock.
- Enforces idempotency for inventory mutation operations.
- Manages transactions when a use case requires atomic changes.
- Must not depend on Gin or HTTP-specific types.

## Repository Layer

- Owns persistence logic only.
- Uses `pgx` and `sqlc` generated queries.
- Maps database records to domain or application data structures.
- Persists warehouse identifiers as part of inventory records and movement records when required by the service layer.
- Persists idempotency keys and movement records when required by the service layer.
- Must not contain business rules.
- Must not call handlers or services.
- Must not manage application workflows.

## Interface-First Rule

Each layer must expose an interface before its implementation.

Dependencies must be injected through constructors:

```go
func NewMaterialService(repo MaterialRepository) MaterialService {
    return &materialService{repo: repo}
}
```

Global variables must not be used to pass dependencies between layers.
