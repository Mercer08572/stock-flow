package inventory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
)

func TestServiceGetStockCalculatesAvailableQty(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	repo := newFakeRepository()
	repo.stocks[[2]int64{1, 2}] = inventory.StockBalance{
		WarehouseID: 1,
		SKUID:       2,
		OnHandQty:   "10.500000",
		ReservedQty: "3.125000",
		UpdatedAt:   &now,
	}
	service := inventory.NewService(repo, newFakeWarehouseReader(1), newFakeSKUReader(2))

	got, err := service.GetStock(ctx, inventory.GetStockQuery{WarehouseID: 1, SKUID: 2})
	if err != nil {
		t.Fatalf("get stock: %v", err)
	}

	if got.AvailableQty != "7.375" {
		t.Fatalf("expected available qty 7.375, got %q", got.AvailableQty)
	}
	if got.OnHandQty != "10.5" || got.ReservedQty != "3.125" {
		t.Fatalf("expected normalized quantities, got on_hand=%q reserved=%q", got.OnHandQty, got.ReservedQty)
	}
	if got.Warehouse == nil || got.Warehouse.ID != 1 || got.SKU == nil || got.SKU.ID != 2 {
		t.Fatalf("expected warehouse and sku references, got %#v", got)
	}
}

func TestServiceGetStockAllowsDeletedMasterDataReferences(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	repo.stocks[[2]int64{1, 2}] = inventory.StockBalance{
		WarehouseID: 1,
		SKUID:       2,
		OnHandQty:   "0",
		ReservedQty: "0",
	}
	warehouseReader := newFakeWarehouseReader(1)
	warehouseReader.references[1] = warehouse.Reference{ID: 1, Code: "WH-001", Name: "Main", Deleted: true}
	skuReader := newFakeSKUReader(2)
	skuReader.references[2] = sku.Reference{ID: 2, Code: "SKU-001", Name: "Item", Deleted: true}

	service := inventory.NewService(repo, warehouseReader, skuReader)
	got, err := service.GetStock(ctx, inventory.GetStockQuery{WarehouseID: 1, SKUID: 2})
	if err != nil {
		t.Fatalf("get historical stock: %v", err)
	}
	if got.Warehouse == nil || !got.Warehouse.Deleted || got.Warehouse.Code != "WH-001" {
		t.Fatalf("expected deleted warehouse reference, got %#v", got.Warehouse)
	}
	if got.SKU == nil || !got.SKU.Deleted || got.SKU.Code != "SKU-001" {
		t.Fatalf("expected deleted sku reference, got %#v", got.SKU)
	}
}

func TestServiceGetStockReturnsZeroBalanceForMissingStock(t *testing.T) {
	ctx := context.Background()
	service := inventory.NewService(newFakeRepository(), newFakeWarehouseReader(1), newFakeSKUReader(2))

	got, err := service.GetStock(ctx, inventory.GetStockQuery{WarehouseID: 1, SKUID: 2})
	if err != nil {
		t.Fatalf("get stock: %v", err)
	}

	if got.OnHandQty != "0" || got.ReservedQty != "0" || got.AvailableQty != "0" {
		t.Fatalf("expected zero quantities, got %#v", got)
	}
	if got.UpdatedAt != nil {
		t.Fatalf("expected nil updated_at for zero balance, got %#v", got.UpdatedAt)
	}
}

func TestServiceGetStockValidatesReferences(t *testing.T) {
	ctx := context.Background()
	service := inventory.NewService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader(2))

	_, err := service.GetStock(ctx, inventory.GetStockQuery{WarehouseID: 1, SKUID: 2})

	if !errors.Is(err, inventory.ErrWarehouseNotFound) {
		t.Fatalf("expected warehouse not found, got %v", err)
	}
}

func TestServiceListStocksNormalizesFilter(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	repo := newFakeRepository()
	repo.stocks[[2]int64{1, 2}] = inventory.StockBalance{
		WarehouseID: 1,
		SKUID:       2,
		OnHandQty:   "5.000000",
		ReservedQty: "1.000000",
		UpdatedAt:   &now,
	}
	repo.stocks[[2]int64{1, 3}] = inventory.StockBalance{
		WarehouseID: 1,
		SKUID:       3,
		OnHandQty:   "8.000000",
		ReservedQty: "0",
		UpdatedAt:   &now,
	}
	warehouseID := int64(1)
	skuID := int64(2)
	service := inventory.NewService(repo, newFakeWarehouseReader(1), newFakeSKUReader(2, 3))

	result, err := service.ListStocks(ctx, inventory.ListStocksFilter{
		WarehouseID: &warehouseID,
		SKUID:       &skuID,
		Limit:       999,
		Offset:      -1,
	})
	if err != nil {
		t.Fatalf("list stocks: %v", err)
	}

	if result.Limit != inventory.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", inventory.MaxListLimit, result.Limit)
	}
	if result.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", result.Offset)
	}
	if len(result.Items) != 1 || result.Items[0].AvailableQty != "4" {
		t.Fatalf("expected filtered stock with available 4, got %#v", result.Items)
	}
}

func TestServiceListLayersFiltersByBatch(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	batchID := int64(9)
	repo := newFakeRepository()
	repo.batches[[2]int64{batchID, 2}] = true
	repo.layers = append(repo.layers,
		inventory.StockLayer{
			ID:          1,
			WarehouseID: 1,
			SKUID:       2,
			BatchID:     &batchID,
			ReceivedAt:  now,
			OnHandQty:   "6.000000",
			ReservedQty: "2.000000",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		inventory.StockLayer{
			ID:          2,
			WarehouseID: 1,
			SKUID:       2,
			ReceivedAt:  now.Add(time.Second),
			OnHandQty:   "3.000000",
			ReservedQty: "0",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	)
	service := inventory.NewService(repo, newFakeWarehouseReader(1), newFakeSKUReader(2))

	result, err := service.ListLayers(ctx, inventory.ListLayersFilter{
		WarehouseID: 1,
		SKUID:       2,
		BatchID:     &batchID,
	})
	if err != nil {
		t.Fatalf("list layers: %v", err)
	}

	if len(result.Items) != 1 || result.Items[0].AvailableQty != "4" {
		t.Fatalf("expected filtered layer with available 4, got %#v", result.Items)
	}
}

func TestServiceListLayersRejectsInvalidBatchForSKU(t *testing.T) {
	ctx := context.Background()
	batchID := int64(9)
	service := inventory.NewService(newFakeRepository(), newFakeWarehouseReader(1), newFakeSKUReader(2))

	_, err := service.ListLayers(ctx, inventory.ListLayersFilter{
		WarehouseID: 1,
		SKUID:       2,
		BatchID:     &batchID,
	})

	if !errors.Is(err, inventory.ErrBatchNotFound) {
		t.Fatalf("expected batch not found, got %v", err)
	}
}

type fakeRepository struct {
	stocks  map[[2]int64]inventory.StockBalance
	layers  []inventory.StockLayer
	batches map[[2]int64]bool
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		stocks:  make(map[[2]int64]inventory.StockBalance),
		batches: make(map[[2]int64]bool),
	}
}

func (r *fakeRepository) ListStocks(_ context.Context, filter inventory.ListStocksFilter) ([]inventory.StockBalance, error) {
	items := make([]inventory.StockBalance, 0)
	for _, item := range r.stocks {
		if filter.WarehouseID != nil && item.WarehouseID != *filter.WarehouseID {
			continue
		}
		if filter.SKUID != nil && item.SKUID != *filter.SKUID {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *fakeRepository) GetStock(_ context.Context, warehouseID int64, skuID int64) (*inventory.StockBalance, error) {
	item, ok := r.stocks[[2]int64{warehouseID, skuID}]
	if !ok {
		return nil, inventory.ErrNotFound
	}
	return &item, nil
}

func (r *fakeRepository) ListLayers(_ context.Context, filter inventory.ListLayersFilter) ([]inventory.StockLayer, error) {
	items := make([]inventory.StockLayer, 0)
	for _, item := range r.layers {
		if item.WarehouseID != filter.WarehouseID || item.SKUID != filter.SKUID {
			continue
		}
		if filter.BatchID != nil {
			if item.BatchID == nil || *item.BatchID != *filter.BatchID {
				continue
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *fakeRepository) BatchExistsForSKU(_ context.Context, batchID int64, skuID int64) (bool, error) {
	return r.batches[[2]int64{batchID, skuID}], nil
}

func (r *fakeRepository) HasWarehouseReferences(_ context.Context, warehouseID int64) (bool, error) {
	for key := range r.stocks {
		if key[0] == warehouseID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepository) HasSKUReferences(_ context.Context, skuID int64) (bool, error) {
	for key := range r.stocks {
		if key[1] == skuID {
			return true, nil
		}
	}
	return false, nil
}

type fakeWarehouseReader struct {
	references map[int64]warehouse.Reference
}

func newFakeWarehouseReader(ids ...int64) *fakeWarehouseReader {
	reader := &fakeWarehouseReader{references: make(map[int64]warehouse.Reference)}
	for _, id := range ids {
		reader.references[id] = warehouse.Reference{ID: id}
	}
	return reader
}

func (r *fakeWarehouseReader) GetReference(_ context.Context, id int64) (*warehouse.Reference, error) {
	reference, exists := r.references[id]
	if !exists {
		return nil, warehouse.ErrNotFound
	}
	return &reference, nil
}

type fakeSKUReader struct {
	references map[int64]sku.Reference
}

func newFakeSKUReader(ids ...int64) *fakeSKUReader {
	reader := &fakeSKUReader{references: make(map[int64]sku.Reference)}
	for _, id := range ids {
		reader.references[id] = sku.Reference{ID: id}
	}
	return reader
}

func (r *fakeSKUReader) GetReference(_ context.Context, id int64) (*sku.Reference, error) {
	reference, exists := r.references[id]
	if !exists {
		return nil, sku.ErrNotFound
	}
	return &reference, nil
}
