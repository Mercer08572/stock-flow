-- name: EnsureInventoryStockRow :exec
INSERT INTO inventory_stocks (warehouse_id, sku_id)
VALUES (sqlc.arg('warehouse_id')::bigint, sqlc.arg('sku_id')::bigint)
ON CONFLICT DO NOTHING;

-- name: LockInventoryStockRow :one
SELECT id,
       warehouse_id,
       sku_id,
       on_hand_qty,
       reserved_qty,
       updated_at
FROM inventory_stocks
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
FOR UPDATE;

-- name: AddInventoryStockOnHand :execrows
UPDATE inventory_stocks
SET on_hand_qty = on_hand_qty + sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL;

-- name: ReserveInventoryStock :execrows
UPDATE inventory_stocks
SET reserved_qty = reserved_qty + sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty + sqlc.arg('qty')::numeric <= on_hand_qty;

-- name: ReleaseInventoryStockReserved :execrows
UPDATE inventory_stocks
SET reserved_qty = reserved_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty >= sqlc.arg('qty')::numeric;

-- name: DecreaseInventoryStockAvailable :execrows
UPDATE inventory_stocks
SET on_hand_qty = on_hand_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
  AND on_hand_qty - reserved_qty >= sqlc.arg('qty')::numeric;

-- name: ConsumeInventoryStockReserved :execrows
UPDATE inventory_stocks
SET on_hand_qty = on_hand_qty - sqlc.arg('qty')::numeric,
    reserved_qty = reserved_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty >= sqlc.arg('qty')::numeric
  AND on_hand_qty >= sqlc.arg('qty')::numeric;

-- name: InsertInventoryStockLayer :one
INSERT INTO inventory_stock_layers (warehouse_id, sku_id, batch_id, received_at, on_hand_qty, reserved_qty)
VALUES (sqlc.arg('warehouse_id')::bigint,
        sqlc.arg('sku_id')::bigint,
        sqlc.narg('batch_id')::bigint,
        NOW(),
        sqlc.arg('qty')::numeric,
        0)
RETURNING id,
          warehouse_id,
          sku_id,
          batch_id,
          received_at,
          on_hand_qty,
          reserved_qty,
          created_at,
          updated_at;

-- name: ListInventoryStockLayersForUpdate :many
-- 分配顺序：指定 batch_id 时目标批次优先，随后按 dispatch_priority、received_at、id。
-- 非批次层视为 dispatch_priority = 10。锁定库存层行，防止并发分配超扣。
SELECT l.id,
       l.warehouse_id,
       l.sku_id,
       l.batch_id,
       l.received_at,
       l.on_hand_qty,
       l.reserved_qty
FROM inventory_stock_layers l
LEFT JOIN inventory_batches b
       ON b.id = l.batch_id
      AND b.deleted_at IS NULL
WHERE l.warehouse_id = sqlc.arg('warehouse_id')::bigint
  AND l.sku_id = sqlc.arg('sku_id')::bigint
  AND l.deleted_at IS NULL
  AND l.on_hand_qty - l.reserved_qty > 0
ORDER BY CASE
             WHEN sqlc.narg('batch_id')::bigint IS NOT NULL
                  AND l.batch_id = sqlc.narg('batch_id')::bigint THEN 0
             ELSE 1
         END,
         COALESCE(b.dispatch_priority, 10),
         l.received_at,
         l.id
FOR UPDATE OF l;

-- name: AddInventoryStockLayerReserved :execrows
UPDATE inventory_stock_layers
SET reserved_qty = reserved_qty + sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty + sqlc.arg('qty')::numeric <= on_hand_qty;

-- name: DecreaseInventoryStockLayerAvailable :execrows
UPDATE inventory_stock_layers
SET on_hand_qty = on_hand_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND on_hand_qty - reserved_qty >= sqlc.arg('qty')::numeric;

-- name: ReleaseInventoryStockLayerReserved :execrows
UPDATE inventory_stock_layers
SET reserved_qty = reserved_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty >= sqlc.arg('qty')::numeric;

-- name: ConsumeInventoryStockLayerReserved :execrows
UPDATE inventory_stock_layers
SET on_hand_qty = on_hand_qty - sqlc.arg('qty')::numeric,
    reserved_qty = reserved_qty - sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND reserved_qty >= sqlc.arg('qty')::numeric
  AND on_hand_qty >= sqlc.arg('qty')::numeric;

-- name: InsertInventoryReservation :one
INSERT INTO inventory_reservations (warehouse_id,
                                    sku_id,
                                    total_qty,
                                    idempotency_key,
                                    source_type,
                                    source_id,
                                    source_line_id,
                                    actor_type,
                                    actor_id,
                                    actor_label)
VALUES (sqlc.arg('warehouse_id')::bigint,
        sqlc.arg('sku_id')::bigint,
        sqlc.arg('total_qty')::numeric,
        sqlc.arg('idempotency_key')::text,
        sqlc.narg('source_type')::text,
        sqlc.narg('source_id')::text,
        sqlc.narg('source_line_id')::text,
        sqlc.narg('actor_type')::text,
        sqlc.narg('actor_id')::bigint,
        sqlc.narg('actor_label')::text)
RETURNING id,
          warehouse_id,
          sku_id,
          total_qty,
          released_qty,
          consumed_qty,
          status,
          close_reason,
          idempotency_key,
          source_type,
          source_id,
          source_line_id,
          actor_type,
          actor_id,
          actor_label,
          created_at,
          updated_at;

-- name: LockInventoryReservation :one
SELECT id,
       warehouse_id,
       sku_id,
       total_qty,
       released_qty,
       consumed_qty,
       status,
       close_reason,
       idempotency_key,
       source_type,
       source_id,
       source_line_id,
       actor_type,
       actor_id,
       actor_label,
       created_at,
       updated_at
FROM inventory_reservations
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
FOR UPDATE;

-- name: ListInventoryReservations :many
SELECT id,
       warehouse_id,
       sku_id,
       total_qty,
       released_qty,
       consumed_qty,
       status,
       close_reason,
       idempotency_key,
       source_type,
       source_id,
       source_line_id,
       actor_type,
       actor_id,
       actor_label,
       created_at,
       updated_at
FROM inventory_reservations
WHERE deleted_at IS NULL
  AND (sqlc.narg('warehouse_id')::bigint IS NULL OR warehouse_id = sqlc.narg('warehouse_id')::bigint)
  AND (sqlc.narg('sku_id')::bigint IS NULL OR sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountInventoryReservations :one
-- 必须与 ListInventoryReservations 的过滤条件保持一致。
SELECT COUNT(*)
FROM inventory_reservations
WHERE deleted_at IS NULL
  AND (sqlc.narg('warehouse_id')::bigint IS NULL OR warehouse_id = sqlc.narg('warehouse_id')::bigint)
  AND (sqlc.narg('sku_id')::bigint IS NULL OR sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: GetInventoryReservation :one
SELECT id,
       warehouse_id,
       sku_id,
       total_qty,
       released_qty,
       consumed_qty,
       status,
       close_reason,
       idempotency_key,
       source_type,
       source_id,
       source_line_id,
       actor_type,
       actor_id,
       actor_label,
       created_at,
       updated_at
FROM inventory_reservations
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL;

-- name: AddInventoryReservationReleased :one
UPDATE inventory_reservations
SET released_qty = released_qty + sqlc.arg('qty')::numeric,
    status = CASE
                 WHEN released_qty + consumed_qty + sqlc.arg('qty')::numeric = total_qty THEN 'closed'
                 ELSE 'active'
             END,
    close_reason = CASE
                       WHEN released_qty + consumed_qty + sqlc.arg('qty')::numeric = total_qty
                           THEN CASE WHEN consumed_qty = 0 THEN 'released' ELSE 'mixed' END
                       ELSE NULL
                   END,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND status = 'active'
  AND released_qty + consumed_qty + sqlc.arg('qty')::numeric <= total_qty
RETURNING id,
          warehouse_id,
          sku_id,
          total_qty,
          released_qty,
          consumed_qty,
          status,
          close_reason,
          idempotency_key,
          source_type,
          source_id,
          source_line_id,
          actor_type,
          actor_id,
          actor_label,
          created_at,
          updated_at;

-- name: AddInventoryReservationConsumed :one
UPDATE inventory_reservations
SET consumed_qty = consumed_qty + sqlc.arg('qty')::numeric,
    status = CASE
                 WHEN released_qty + consumed_qty + sqlc.arg('qty')::numeric = total_qty THEN 'closed'
                 ELSE 'active'
             END,
    close_reason = CASE
                       WHEN released_qty + consumed_qty + sqlc.arg('qty')::numeric = total_qty
                           THEN CASE WHEN released_qty = 0 THEN 'consumed' ELSE 'mixed' END
                       ELSE NULL
                   END,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND status = 'active'
  AND released_qty + consumed_qty + sqlc.arg('qty')::numeric <= total_qty
RETURNING id,
          warehouse_id,
          sku_id,
          total_qty,
          released_qty,
          consumed_qty,
          status,
          close_reason,
          idempotency_key,
          source_type,
          source_id,
          source_line_id,
          actor_type,
          actor_id,
          actor_label,
          created_at,
          updated_at;

-- name: InsertInventoryReservationItem :one
INSERT INTO inventory_reservation_items (reservation_id, stock_layer_id, reserved_qty)
VALUES (sqlc.arg('reservation_id')::bigint,
        sqlc.arg('stock_layer_id')::bigint,
        sqlc.arg('reserved_qty')::numeric)
RETURNING id,
          reservation_id,
          stock_layer_id,
          reserved_qty,
          released_qty,
          consumed_qty,
          created_at,
          updated_at;

-- name: ListInventoryReservationItemsForUpdate :many
SELECT i.id,
       i.reservation_id,
       i.stock_layer_id,
       i.reserved_qty,
       i.released_qty,
       i.consumed_qty,
       l.batch_id
FROM inventory_reservation_items i
JOIN inventory_stock_layers l
  ON l.id = i.stock_layer_id
WHERE i.reservation_id = sqlc.arg('reservation_id')::bigint
  AND i.deleted_at IS NULL
ORDER BY i.id
FOR UPDATE OF i;

-- name: ListInventoryReservationItems :many
SELECT i.id,
       i.reservation_id,
       i.stock_layer_id,
       l.batch_id,
       l.received_at,
       i.reserved_qty,
       i.released_qty,
       i.consumed_qty,
       i.created_at,
       i.updated_at
FROM inventory_reservation_items i
LEFT JOIN inventory_stock_layers l
  ON l.id = i.stock_layer_id
WHERE i.reservation_id = sqlc.arg('reservation_id')::bigint
  AND i.deleted_at IS NULL
ORDER BY i.id;

-- name: AddInventoryReservationItemReleased :execrows
UPDATE inventory_reservation_items
SET released_qty = released_qty + sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND released_qty + consumed_qty + sqlc.arg('qty')::numeric <= reserved_qty;

-- name: AddInventoryReservationItemConsumed :execrows
UPDATE inventory_reservation_items
SET consumed_qty = consumed_qty + sqlc.arg('qty')::numeric,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL
  AND released_qty + consumed_qty + sqlc.arg('qty')::numeric <= reserved_qty;

-- name: InsertInventoryMovement :one
INSERT INTO inventory_movements (operation_type,
                                 warehouse_id,
                                 sku_id,
                                 batch_id,
                                 stock_layer_id,
                                 reservation_id,
                                 reservation_item_id,
                                 qty,
                                 idempotency_key,
                                 source_type,
                                 source_id,
                                 source_line_id,
                                 actor_type,
                                 actor_id,
                                 actor_label)
VALUES (sqlc.arg('operation_type')::text,
        sqlc.arg('warehouse_id')::bigint,
        sqlc.arg('sku_id')::bigint,
        sqlc.narg('batch_id')::bigint,
        sqlc.narg('stock_layer_id')::bigint,
        sqlc.narg('reservation_id')::bigint,
        sqlc.narg('reservation_item_id')::bigint,
        sqlc.arg('qty')::numeric,
        sqlc.arg('idempotency_key')::text,
        sqlc.narg('source_type')::text,
        sqlc.narg('source_id')::text,
        sqlc.narg('source_line_id')::text,
        sqlc.narg('actor_type')::text,
        sqlc.narg('actor_id')::bigint,
        sqlc.narg('actor_label')::text)
RETURNING id,
          operation_type,
          warehouse_id,
          sku_id,
          batch_id,
          stock_layer_id,
          reservation_id,
          reservation_item_id,
          qty,
          idempotency_key,
          source_type,
          source_id,
          source_line_id,
          actor_type,
          actor_id,
          actor_label,
          created_at;

-- name: ClaimInventoryIdempotencyKey :one
INSERT INTO inventory_idempotency_keys (operation_type, idempotency_key, request_hash, request_payload)
VALUES (sqlc.arg('operation_type')::text,
        sqlc.arg('idempotency_key')::text,
        sqlc.arg('request_hash')::text,
        sqlc.arg('request_payload')::jsonb)
ON CONFLICT DO NOTHING
RETURNING id,
          operation_type,
          idempotency_key,
          request_hash,
          request_payload,
          status,
          response_payload;

-- name: GetInventoryIdempotencyKey :one
SELECT id,
       operation_type,
       idempotency_key,
       request_hash,
       request_payload,
       status,
       response_payload
FROM inventory_idempotency_keys
WHERE operation_type = sqlc.arg('operation_type')::text
  AND idempotency_key = sqlc.arg('idempotency_key')::text
  AND deleted_at IS NULL;

-- name: CompleteInventoryIdempotencyKey :execrows
UPDATE inventory_idempotency_keys
SET status = 'succeeded',
    response_ref_type = sqlc.narg('response_ref_type')::text,
    response_ref_id = sqlc.narg('response_ref_id')::bigint,
    response_payload = sqlc.arg('response_payload')::jsonb,
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint;

-- name: GetInventoryBatchForSKU :one
SELECT id,
       sku_id,
       batch_no,
       dispatch_priority,
       status
FROM inventory_batches
WHERE id = sqlc.arg('id')::bigint
  AND sku_id = sqlc.arg('sku_id')::bigint
  AND deleted_at IS NULL;

-- name: ListInventoryMovements :many
SELECT id,
       operation_type,
       warehouse_id,
       sku_id,
       batch_id,
       stock_layer_id,
       reservation_id,
       reservation_item_id,
       qty,
       idempotency_key,
       source_type,
       source_id,
       source_line_id,
       actor_type,
       actor_id,
       actor_label,
       created_at
FROM inventory_movements
WHERE deleted_at IS NULL
  AND (sqlc.narg('warehouse_id')::bigint IS NULL OR warehouse_id = sqlc.narg('warehouse_id')::bigint)
  AND (sqlc.narg('sku_id')::bigint IS NULL OR sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('operation_type')::text IS NULL OR operation_type = sqlc.narg('operation_type')::text)
  AND (sqlc.narg('created_from')::timestamptz IS NULL OR created_at >= sqlc.narg('created_from')::timestamptz)
  AND (sqlc.narg('created_to')::timestamptz IS NULL OR created_at < sqlc.narg('created_to')::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountInventoryMovements :one
-- 必须与 ListInventoryMovements 的过滤条件保持一致。
SELECT COUNT(*)
FROM inventory_movements
WHERE deleted_at IS NULL
  AND (sqlc.narg('warehouse_id')::bigint IS NULL OR warehouse_id = sqlc.narg('warehouse_id')::bigint)
  AND (sqlc.narg('sku_id')::bigint IS NULL OR sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('operation_type')::text IS NULL OR operation_type = sqlc.narg('operation_type')::text)
  AND (sqlc.narg('created_from')::timestamptz IS NULL OR created_at >= sqlc.narg('created_from')::timestamptz)
  AND (sqlc.narg('created_to')::timestamptz IS NULL OR created_at < sqlc.narg('created_to')::timestamptz);
