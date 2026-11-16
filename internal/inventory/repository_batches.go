package inventory

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	inventorydb "github.com/Mercer08572/stock-flow/internal/inventory/db"
	"github.com/Mercer08572/stock-flow/internal/shared/reference"
)

// CreateBatchRecord 是仓储层的新增入参：所有可选值都已由服务层归一化。
type CreateBatchRecord struct {
	SKUID            int64
	BatchNo          string
	ProductionDate   *string
	ExpirationDate   *string
	Status           BatchStatus
	DispatchPriority int32
	Remark           *string
}

// UpdateBatchRecord 是仓储层的编辑入参；批次标识（id / sku_id / batch_no）不可改。
type UpdateBatchRecord struct {
	ID               int64
	ProductionDate   *string
	ExpirationDate   *string
	Status           BatchStatus
	DispatchPriority int32
	Remark           *string
}

// NewPostgresBatchRepository 返回只暴露批次读写的仓储。
//
// 批次读写与库存操作共用同一个 postgresRepository 实现，但对外只暴露
// BatchRepository，避免把批次方法塞进库存操作使用的 Repository 接口。
func NewPostgresBatchRepository(db *pgxpool.Pool) BatchRepository {
	return newPostgresBatchRepository(inventorydb.New(db))
}

func newPostgresBatchRepository(queries inventorydb.Querier) BatchRepository {
	return &postgresRepository{queries: queries}
}

func (r *postgresRepository) ListBatches(ctx context.Context, filter ListBatchesFilter) ([]Batch, error) {
	rows, err := r.queries.ListInventoryBatches(ctx, inventorydb.ListInventoryBatchesParams{
		SkuID:   filter.SKUID,
		Status:  batchStatusParam(filter.Status),
		Keyword: filter.Keyword,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	batches := make([]Batch, 0, len(rows))
	for _, row := range rows {
		batches = append(batches, batchFromRow(inventorydb.GetInventoryBatchRow(row)))
	}

	return batches, nil
}

func (r *postgresRepository) CountBatches(ctx context.Context, filter ListBatchesFilter) (int64, error) {
	count, err := r.queries.CountInventoryBatches(ctx, inventorydb.CountInventoryBatchesParams{
		SkuID:   filter.SKUID,
		Status:  batchStatusParam(filter.Status),
		Keyword: filter.Keyword,
	})
	if err != nil {
		return 0, mapPostgresError(err)
	}
	return count, nil
}

func (r *postgresRepository) GetBatchByID(ctx context.Context, id int64) (*Batch, error) {
	row, err := r.queries.GetInventoryBatch(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	batch := batchFromRow(row)
	return &batch, nil
}

func (r *postgresRepository) CreateBatch(ctx context.Context, input CreateBatchRecord) (int64, error) {
	id, err := r.queries.CreateInventoryBatch(ctx, inventorydb.CreateInventoryBatchParams{
		SkuID:            input.SKUID,
		BatchNo:          input.BatchNo,
		ProductionDate:   pgDateFromString(input.ProductionDate),
		ExpirationDate:   pgDateFromString(input.ExpirationDate),
		Status:           string(input.Status),
		DispatchPriority: input.DispatchPriority,
		Remark:           input.Remark,
	})
	if err != nil {
		return 0, mapPostgresError(err)
	}
	return id, nil
}

func (r *postgresRepository) UpdateBatch(ctx context.Context, input UpdateBatchRecord) error {
	affected, err := r.queries.UpdateInventoryBatch(ctx, inventorydb.UpdateInventoryBatchParams{
		ID:               input.ID,
		ProductionDate:   pgDateFromString(input.ProductionDate),
		ExpirationDate:   pgDateFromString(input.ExpirationDate),
		Status:           string(input.Status),
		DispatchPriority: input.DispatchPriority,
		Remark:           input.Remark,
	})
	if err != nil {
		return mapPostgresError(err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) DisableBatch(ctx context.Context, id int64) (*Batch, error) {
	affected, err := r.queries.DisableInventoryBatch(ctx, id)
	if err != nil {
		return nil, mapPostgresError(err)
	}
	if affected == 0 {
		return nil, ErrNotFound
	}

	return r.GetBatchByID(ctx, id)
}

func (r *postgresRepository) DeleteBatch(ctx context.Context, id int64) error {
	affected, err := r.queries.SoftDeleteInventoryBatch(ctx, id)
	if err != nil {
		return mapPostgresError(err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) BatchNoExistsForSKU(ctx context.Context, skuID int64, batchNo string, excludeID *int64) (bool, error) {
	exists, err := r.queries.BatchNoExistsForSKU(ctx, inventorydb.BatchNoExistsForSKUParams{
		SkuID:     skuID,
		BatchNo:   batchNo,
		ExcludeID: excludeID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}
	return exists, nil
}

func (r *postgresRepository) CountBatchReferences(ctx context.Context, id int64) (int64, error) {
	count, err := r.queries.CountInventoryBatchReferences(ctx, id)
	if err != nil {
		return 0, mapPostgresError(err)
	}
	return int64(count), nil
}

func (r *postgresRepository) ListBatchWarehouseStocks(ctx context.Context, batchID int64) ([]BatchWarehouseStock, error) {
	rows, err := r.queries.ListInventoryBatchWarehouseStocks(ctx, batchID)
	if err != nil {
		return nil, mapPostgresError(err)
	}

	stocks := make([]BatchWarehouseStock, 0, len(rows))
	for _, row := range rows {
		onHand, err := numericString(row.OnHandQty)
		if err != nil {
			return nil, err
		}
		reserved, err := numericString(row.ReservedQty)
		if err != nil {
			return nil, err
		}
		available, err := numericString(row.AvailableQty)
		if err != nil {
			return nil, err
		}

		stocks = append(stocks, BatchWarehouseStock{
			WarehouseID:  row.WarehouseID,
			Warehouse:    reference.FromJoin(row.WarehouseID, &row.WarehouseCode, &row.WarehouseName, &row.WarehouseStatus, row.WarehouseDeletedAt.Valid),
			OnHandQty:    onHand,
			ReservedQty:  reserved,
			AvailableQty: available,
			LayerCount:   row.LayerCount,
		})
	}

	return stocks, nil
}

// TouchBatchFirstReceived 由库存入库流程调用：只在首次收货时写入时间。
func (r *postgresRepository) TouchBatchFirstReceived(ctx context.Context, id int64) error {
	if _, err := r.queries.TouchInventoryBatchFirstReceived(ctx, id); err != nil {
		return mapPostgresError(err)
	}
	return nil
}

func batchStatusParam(status *BatchStatus) *string {
	if status == nil {
		return nil
	}
	value := string(*status)
	return &value
}

// batchFromRow 把列表/详情两种同形行统一成领域模型。
// 两者字段完全一致，因此直接做结构体转换，避免两份映射逻辑漂移。
func batchFromRow(row inventorydb.GetInventoryBatchRow) Batch {
	batch := Batch{
		ID:               row.ID,
		SKUID:            row.SkuID,
		BatchNo:          row.BatchNo,
		ProductionDate:   formatPGDate(row.ProductionDate),
		ExpirationDate:   formatPGDate(row.ExpirationDate),
		Status:           BatchStatus(row.Status),
		DispatchPriority: row.DispatchPriority,
		Remark:           row.Remark,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}

	if row.FirstReceivedAt.Valid {
		firstReceivedAt := row.FirstReceivedAt.Time
		batch.FirstReceivedAt = &firstReceivedAt
	}
	batch.SKU = reference.FromJoin(row.SkuID, &row.SkuCode, &row.SkuName, &row.SkuStatus, row.SkuDeletedAt.Valid)

	return batch
}

// formatPGDate 把日历日转成 `YYYY-MM-DD`；NULL 保持 nil。
func formatPGDate(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(DateLayout)
	return &formatted
}

// pgDateFromString 把 `YYYY-MM-DD` 转成 pgtype.Date；nil 或空值表示 NULL。
// 入参在服务层已校验，这里解析失败按 NULL 处理，数据库约束会兜底。
func pgDateFromString(value *string) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	parsed, err := time.Parse(DateLayout, *value)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}
