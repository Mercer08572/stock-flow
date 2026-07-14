package inventory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	inventorydb "github.com/Mercer08572/stock-flow/internal/inventory/db"
)

type postgresRepository struct {
	queries inventorydb.Querier
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{queries: inventorydb.New(db)}
}

func (r *postgresRepository) ListStocks(ctx context.Context, filter ListStocksFilter) ([]StockBalance, error) {
	rows, err := r.queries.ListInventoryStocks(ctx, inventorydb.ListInventoryStocksParams{
		WarehouseID: filter.WarehouseID,
		SkuID:       filter.SKUID,
		Offset:      filter.Offset,
		Limit:       filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	stocks := make([]StockBalance, 0, len(rows))
	for _, row := range rows {
		stock, err := stockFromListRow(row)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, stock)
	}

	return stocks, nil
}

func (r *postgresRepository) GetStock(ctx context.Context, warehouseID int64, skuID int64) (*StockBalance, error) {
	row, err := r.queries.GetInventoryStock(ctx, inventorydb.GetInventoryStockParams{
		WarehouseID: warehouseID,
		SkuID:       skuID,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	stock, err := stockFromGetRow(row)
	if err != nil {
		return nil, err
	}

	return &stock, nil
}

func (r *postgresRepository) ListLayers(ctx context.Context, filter ListLayersFilter) ([]StockLayer, error) {
	rows, err := r.queries.ListInventoryStockLayers(ctx, inventorydb.ListInventoryStockLayersParams{
		WarehouseID: filter.WarehouseID,
		SkuID:       filter.SKUID,
		BatchID:     filter.BatchID,
		Offset:      filter.Offset,
		Limit:       filter.Limit,
	})
	if err != nil {
		return nil, mapPostgresError(err)
	}

	layers := make([]StockLayer, 0, len(rows))
	for _, row := range rows {
		layer, err := layerFromListRow(row)
		if err != nil {
			return nil, err
		}
		layers = append(layers, layer)
	}

	return layers, nil
}

func (r *postgresRepository) BatchExistsForSKU(ctx context.Context, batchID int64, skuID int64) (bool, error) {
	exists, err := r.queries.InventoryBatchExistsForSKU(ctx, inventorydb.InventoryBatchExistsForSKUParams{
		BatchID: batchID,
		SkuID:   skuID,
	})
	if err != nil {
		return false, mapPostgresError(err)
	}

	return exists, nil
}

func (r *postgresRepository) HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error) {
	referenced, err := r.queries.HasWarehouseInventoryReferences(ctx, warehouseID)
	if err != nil {
		return false, mapPostgresError(err)
	}

	return referenced, nil
}

func (r *postgresRepository) HasSKUReferences(ctx context.Context, skuID int64) (bool, error) {
	referenced, err := r.queries.HasSKUInventoryReferences(ctx, skuID)
	if err != nil {
		return false, mapPostgresError(err)
	}

	return referenced, nil
}

func mapPostgresError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	return err
}

func numericString(value pgtype.Numeric) (string, error) {
	driverValue, err := value.Value()
	if err != nil {
		return "", err
	}
	if driverValue == nil {
		return "0", nil
	}

	numeric, ok := driverValue.(string)
	if !ok {
		return "", errors.New("inventory quantity is not numeric text")
	}

	return numeric, nil
}

func stockFromListRow(row inventorydb.ListInventoryStocksRow) (StockBalance, error) {
	onHand, err := numericString(row.OnHandQty)
	if err != nil {
		return StockBalance{}, err
	}
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return StockBalance{}, err
	}
	available, err := numericString(row.AvailableQty)
	if err != nil {
		return StockBalance{}, err
	}
	updatedAt := row.UpdatedAt.Time

	return StockBalance{
		WarehouseID:  row.WarehouseID,
		SKUID:        row.SkuID,
		OnHandQty:    onHand,
		ReservedQty:  reserved,
		AvailableQty: available,
		UpdatedAt:    &updatedAt,
	}, nil
}

func stockFromGetRow(row inventorydb.GetInventoryStockRow) (StockBalance, error) {
	onHand, err := numericString(row.OnHandQty)
	if err != nil {
		return StockBalance{}, err
	}
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return StockBalance{}, err
	}
	available, err := numericString(row.AvailableQty)
	if err != nil {
		return StockBalance{}, err
	}
	updatedAt := row.UpdatedAt.Time

	return StockBalance{
		WarehouseID:  row.WarehouseID,
		SKUID:        row.SkuID,
		OnHandQty:    onHand,
		ReservedQty:  reserved,
		AvailableQty: available,
		UpdatedAt:    &updatedAt,
	}, nil
}

func layerFromListRow(row inventorydb.ListInventoryStockLayersRow) (StockLayer, error) {
	onHand, err := numericString(row.OnHandQty)
	if err != nil {
		return StockLayer{}, err
	}
	reserved, err := numericString(row.ReservedQty)
	if err != nil {
		return StockLayer{}, err
	}
	available, err := numericString(row.AvailableQty)
	if err != nil {
		return StockLayer{}, err
	}

	return StockLayer{
		ID:           row.ID,
		WarehouseID:  row.WarehouseID,
		SKUID:        row.SkuID,
		BatchID:      row.BatchID,
		ReceivedAt:   row.ReceivedAt.Time,
		OnHandQty:    onHand,
		ReservedQty:  reserved,
		AvailableQty: available,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}, nil
}
