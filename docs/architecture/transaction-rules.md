# Transaction Rules

Transactions are managed only in the application service layer.

## Core Rule

Only services may start, commit, or roll back database transactions.

```text
Handler -> Service transaction boundary -> Repository
```

## Service Layer Responsibilities

- Start a transaction when one use case changes multiple records that must succeed or fail together.
- Pass the transaction context or transaction-bound query object to repositories.
- Commit the transaction only after the full use case succeeds.
- Roll back the transaction when any step in the use case fails.
- Keep transaction scope as small as possible.
- Keep inventory changes by warehouse atomic with their related movement or order records.

## Handler Layer Rules

- Handlers must not start transactions.
- Handlers must not commit or roll back transactions.
- Handlers should only call service methods and return unified responses.

## Repository Layer Rules

- Repositories must not start transactions.
- Repositories must not commit or roll back transactions.
- Repositories should execute persistence operations using the database executor provided by the service layer.
- Repository methods should remain focused on a single persistence operation.

## Cross-Module Transaction Rule

When one use case coordinates multiple modules, the owning application service is responsible for the transaction boundary.

For inventory operations, the inventory service owns the transaction boundary.

The transaction boundary still belongs to the application service layer.

## Warehouse Inventory Transaction Rule

Inventory changes involving warehouses, SKUs, batches, reservations, idempotency keys, and movement records must be committed atomically.

Examples:

- Increasing stock must update stock balance and create a movement record in one transaction.
- Reserving stock must check available quantity, update reserved quantity, store the idempotency key, and create a movement record in one transaction.
- Releasing reserved stock must reduce reserved quantity and create a movement record in one transaction.
- Decreasing stock from available stock must check available quantity, reduce on-hand quantity, and create a movement record in one transaction.
- Decreasing stock from reserved stock must reduce both on-hand quantity and reserved quantity, and create a movement record in one transaction.
- FIFO batch allocation must lock and update all selected batch stock rows in one transaction.
