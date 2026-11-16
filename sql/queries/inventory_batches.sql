-- 批次主数据的读写查询。
-- 批次是 SKU 级主数据：没有数量字段，也没有 warehouse_id；
-- 批次库存只能通过在 inventory_stock_layers 上按 batch_id 过滤得到。

-- name: ListInventoryBatches :many
-- SKU 引用快照随行内联（JOIN 不过滤 status / deleted_at，见 ListSKUs 说明）。
SELECT b.id,
       b.sku_id,
       b.batch_no,
       b.first_received_at,
       b.production_date,
       b.expiration_date,
       b.status,
       b.dispatch_priority,
       b.remark,
       b.created_at,
       b.updated_at,
       s.code       AS sku_code,
       s.name       AS sku_name,
       s.status     AS sku_status,
       s.deleted_at AS sku_deleted_at
FROM inventory_batches b
JOIN skus s ON s.id = b.sku_id
WHERE b.deleted_at IS NULL
  AND (sqlc.narg('sku_id')::bigint IS NULL OR b.sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')::text)
  AND (sqlc.narg('keyword')::text IS NULL
       OR strpos(lower(b.batch_no), lower(sqlc.narg('keyword')::text)) > 0)
ORDER BY b.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountInventoryBatches :one
-- 必须与 ListInventoryBatches 的过滤条件保持一致。
SELECT COUNT(*)
FROM inventory_batches b
JOIN skus s ON s.id = b.sku_id
WHERE b.deleted_at IS NULL
  AND (sqlc.narg('sku_id')::bigint IS NULL OR b.sku_id = sqlc.narg('sku_id')::bigint)
  AND (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status')::text)
  AND (sqlc.narg('keyword')::text IS NULL
       OR strpos(lower(b.batch_no), lower(sqlc.narg('keyword')::text)) > 0);

-- name: GetInventoryBatch :one
SELECT b.id,
       b.sku_id,
       b.batch_no,
       b.first_received_at,
       b.production_date,
       b.expiration_date,
       b.status,
       b.dispatch_priority,
       b.remark,
       b.created_at,
       b.updated_at,
       s.code       AS sku_code,
       s.name       AS sku_name,
       s.status     AS sku_status,
       s.deleted_at AS sku_deleted_at
FROM inventory_batches b
JOIN skus s ON s.id = b.sku_id
WHERE b.id = sqlc.arg('id')::bigint
  AND b.deleted_at IS NULL;

-- name: CreateInventoryBatch :one
INSERT INTO inventory_batches (sku_id,
                               batch_no,
                               production_date,
                               expiration_date,
                               status,
                               dispatch_priority,
                               remark)
VALUES (sqlc.arg('sku_id')::bigint,
        sqlc.arg('batch_no')::text,
        sqlc.narg('production_date')::date,
        sqlc.narg('expiration_date')::date,
        sqlc.arg('status')::text,
        sqlc.arg('dispatch_priority')::integer,
        sqlc.narg('remark')::text)
RETURNING id;

-- name: UpdateInventoryBatch :execrows
-- 批号与 sku_id 不在可更新列中：它们是批次的业务标识，创建后不可修改。
UPDATE inventory_batches
SET production_date   = sqlc.narg('production_date')::date,
    expiration_date   = sqlc.narg('expiration_date')::date,
    status            = sqlc.arg('status')::text,
    dispatch_priority = sqlc.arg('dispatch_priority')::integer,
    remark            = sqlc.narg('remark')::text,
    updated_at        = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL;

-- name: DisableInventoryBatch :execrows
UPDATE inventory_batches
SET status     = 'inactive',
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL;

-- name: SoftDeleteInventoryBatch :execrows
UPDATE inventory_batches
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND deleted_at IS NULL;

-- name: BatchNoExistsForSKU :one
-- 与唯一部分索引 ux_inventory_batches_sku_batch_no 的条件保持一致。
SELECT EXISTS (
    SELECT 1
    FROM inventory_batches
    WHERE sku_id = sqlc.arg('sku_id')::bigint
      AND batch_no = sqlc.arg('batch_no')::text
      AND deleted_at IS NULL
      AND (sqlc.narg('exclude_id')::bigint IS NULL OR id <> sqlc.narg('exclude_id')::bigint)
) AS exists;

-- name: CountInventoryBatchReferences :one
-- 删除守卫：库存层与库存流水都通过外键引用批次，任一存在即不可删除。
-- 这里不过滤 deleted_at：账本行即使用户不可见也仍然引用该批次。
SELECT (SELECT COUNT(*) FROM inventory_stock_layers WHERE batch_id = sqlc.arg('batch_id')::bigint)
     + (SELECT COUNT(*) FROM inventory_movements WHERE batch_id = sqlc.arg('batch_id')::bigint) AS reference_count;

-- name: ListInventoryBatchWarehouseStocks :many
-- 某批次在各仓库的库存分布；批次库存通过在库存层上按 batch_id 过滤得到。
SELECT w.id         AS warehouse_id,
       w.code       AS warehouse_code,
       w.name       AS warehouse_name,
       w.status     AS warehouse_status,
       w.deleted_at AS warehouse_deleted_at,
       COALESCE(SUM(l.on_hand_qty), 0)::numeric               AS on_hand_qty,
       COALESCE(SUM(l.reserved_qty), 0)::numeric              AS reserved_qty,
       COALESCE(SUM(l.on_hand_qty - l.reserved_qty), 0)::numeric AS available_qty,
       COUNT(l.id)::bigint                                     AS layer_count
FROM inventory_stock_layers l
JOIN warehouses w ON w.id = l.warehouse_id
WHERE l.batch_id = sqlc.arg('batch_id')::bigint
  AND l.deleted_at IS NULL
GROUP BY w.id, w.code, w.name, w.status, w.deleted_at
ORDER BY w.code;

-- name: TouchInventoryBatchFirstReceived :execrows
-- 首次入库时记录批次首次收货时间；已有值时不覆盖。
UPDATE inventory_batches
SET first_received_at = NOW(),
    updated_at        = NOW()
WHERE id = sqlc.arg('id')::bigint
  AND first_received_at IS NULL
  AND deleted_at IS NULL;
