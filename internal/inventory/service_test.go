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

type fakeWarehouseReader struct {
	warehouses map[int64]bool
}

func newFakeWarehouseReader(ids ...int64) *fakeWarehouseReader {
	reader := &fakeWarehouseReader{warehouses: make(map[int64]bool)}
	for _, id := range ids {
		reader.warehouses[id] = true
	}
	return reader
}

func (r *fakeWarehouseReader) Get(_ context.Context, id int64) (*warehouse.Warehouse, error) {
	if !r.warehouses[id] {
		return nil, warehouse.ErrNotFound
	}
	return &warehouse.Warehouse{ID: id}, nil
}

type fakeSKUReader struct {
	skus map[int64]bool
}

func newFakeSKUReader(ids ...int64) *fakeSKUReader {
	reader := &fakeSKUReader{skus: make(map[int64]bool)}
	for _, id := range ids {
		reader.skus[id] = true
	}
	return reader
}

func (r *fakeSKUReader) Get(_ context.Context, id int64) (*sku.SKU, error) {
	if !r.skus[id] {
		return nil, sku.ErrNotFound
	}
	return &sku.SKU{ID: id}, nil
}
