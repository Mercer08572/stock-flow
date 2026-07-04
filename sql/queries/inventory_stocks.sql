-- name: ListInventoryStocks :many
SELECT warehouse_id,
       sku_id,
       on_hand_qty,
       reserved_qty,
       (on_hand_qty - reserved_qty)::numeric AS available_qty,
       updated_at
FROM inventory_stocks
WHERE deleted_at IS NULL
  AND (sqlc.narg('warehouse_id')::bigint IS NULL OR warehouse_id = sqlc.narg('warehouse_id')::bigint)
  AND (sqlc.narg('sku_id')::bigint IS NULL OR sku_id = sqlc.narg('sku_id')::bigint)
ORDER BY warehouse_id, sku_id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetInventoryStock :one
SELECT warehouse_id,
       sku_id,
       on_hand_qty,
       reserved_qty,
       (on_hand_qty - reserved_qty)::numeric AS available_qty,
       updated_at
FROM inventory_stocks
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL;

-- name: ListInventoryStockLayers :many
SELECT id,
       warehouse_id,
       sku_id,
       batch_id,
       received_at,
       on_hand_qty,
       reserved_qty,
       (on_hand_qty - reserved_qty)::numeric AS available_qty,
       created_at,
       updated_at
FROM inventory_stock_layers
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
  AND (sqlc.narg('batch_id')::bigint IS NULL OR batch_id = sqlc.narg('batch_id')::bigint)
ORDER BY received_at, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: WarehouseExistsForInventory :one
SELECT EXISTS (
    SELECT 1
    FROM warehouses
    WHERE id = $1
      AND deleted_at IS NULL
) AS exists;

-- name: SKUExistsForInventory :one
SELECT EXISTS (
    SELECT 1
    FROM skus
    WHERE id = $1
      AND deleted_at IS NULL
) AS exists;

-- name: InventoryBatchExistsForSKU :one
SELECT EXISTS (
    SELECT 1
    FROM inventory_batches
    WHERE id = sqlc.arg('batch_id')::bigint
      AND sku_id = sqlc.arg('sku_id')::bigint
      AND deleted_at IS NULL
) AS exists;
