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
-- 分配顺序：先按批次 dispatch_priority（0-9 提升、10 默认、11+ 延后；非批次层视为 10），
-- 再按 received_at, id 即 FIFO。
-- 注意：左侧不过滤批次 deleted_at，否则批次被软删除时其库存层会从结果中消失。
SELECT l.id,
       l.warehouse_id,
       l.sku_id,
       l.batch_id,
       l.received_at,
       l.on_hand_qty,
       l.reserved_qty,
       (l.on_hand_qty - l.reserved_qty)::numeric AS available_qty,
       l.created_at,
       l.updated_at
FROM inventory_stock_layers AS l
LEFT JOIN inventory_batches AS b ON b.id = l.batch_id
WHERE l.warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND l.sku_id = sqlc.arg('sku_id')::bigint
  AND l.deleted_at IS NULL
  AND (sqlc.narg('batch_id')::bigint IS NULL OR l.batch_id = sqlc.narg('batch_id')::bigint)
ORDER BY COALESCE(b.dispatch_priority, 10), l.received_at, l.id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: InventoryBatchExistsForSKU :one
SELECT EXISTS (
    SELECT 1
    FROM inventory_batches
    WHERE id = sqlc.arg('batch_id')::bigint
      AND sku_id = sqlc.arg('sku_id')::bigint
      AND deleted_at IS NULL
) AS exists;

-- name: HasWarehouseInventoryReferences :one
SELECT EXISTS (
    SELECT 1 FROM inventory_stocks AS stocks
    WHERE stocks.warehouse_id = sqlc.arg('warehouse_id')::bigint
      AND stocks.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_stock_layers AS layers
    WHERE layers.warehouse_id = sqlc.arg('warehouse_id')::bigint
      AND layers.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_reservations AS reservations
    WHERE reservations.warehouse_id = sqlc.arg('warehouse_id')::bigint
      AND reservations.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_movements AS movements
    WHERE movements.warehouse_id = sqlc.arg('warehouse_id')::bigint
) AS exists;

-- name: HasSKUInventoryReferences :one
SELECT EXISTS (
    SELECT 1 FROM inventory_batches AS batches
    WHERE batches.sku_id = sqlc.arg('sku_id')::bigint
      AND batches.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_stocks AS stocks
    WHERE stocks.sku_id = sqlc.arg('sku_id')::bigint
      AND stocks.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_stock_layers AS layers
    WHERE layers.sku_id = sqlc.arg('sku_id')::bigint
      AND layers.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_reservations AS reservations
    WHERE reservations.sku_id = sqlc.arg('sku_id')::bigint
      AND reservations.deleted_at IS NULL
    UNION ALL
    SELECT 1 FROM inventory_movements AS movements
    WHERE movements.sku_id = sqlc.arg('sku_id')::bigint
) AS exists;
