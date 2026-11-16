//go:build integration

package inventory_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/sku"
)

func newBatchServiceForFixture(fixture *inventoryFixture) inventory.BatchService {
	return inventory.NewBatchService(
		inventory.NewPostgresBatchRepository(fixture.db.Pool),
		sku.NewService(sku.NewPostgresRepository(fixture.db.Pool), nil),
	)
}

func mustCreateBatch(t *testing.T, service inventory.BatchService, skuID int64, batchNo string, priority int32) *inventory.Batch {
	t.Helper()

	batch, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{
		SKUID:            skuID,
		BatchNo:          batchNo,
		DispatchPriority: &priority,
	})
	if err != nil {
		t.Fatalf("create batch %s: %v", batchNo, err)
	}
	return batch
}

func TestBatchLifecycleAffectsAllocation(t *testing.T) {
	fixture := newInventoryFixture(t)
	batchService := newBatchServiceForFixture(fixture)

	batch := mustCreateBatch(t, batchService, fixture.skuID, "B-ALLOC-1", 10)
	if batch.FirstReceivedAt != nil {
		t.Fatal("expected first_received_at to be NULL before any receipt")
	}

	if _, err := fixture.service.IncreaseStock(context.Background(), inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("batch-increase-1"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &batch.ID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	afterFirstReceipt, err := batchService.GetBatch(context.Background(), batch.ID)
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if afterFirstReceipt.FirstReceivedAt == nil {
		t.Fatal("expected first_received_at to be set after the first receipt")
	}
	if len(afterFirstReceipt.Stocks) != 1 {
		t.Fatalf("expected one warehouse stock row, got %d", len(afterFirstReceipt.Stocks))
	}
	onHand, err := strconv.ParseFloat(afterFirstReceipt.Stocks[0].OnHandQty, 64)
	if err != nil || onHand != 10 {
		t.Fatalf("expected on hand 10, got %q (%v)", afterFirstReceipt.Stocks[0].OnHandQty, err)
	}

	firstReceivedAt := *afterFirstReceipt.FirstReceivedAt
	if _, err := fixture.service.IncreaseStock(context.Background(), inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("batch-increase-2"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &batch.ID,
		Qty:              "5",
	}); err != nil {
		t.Fatalf("second increase: %v", err)
	}
	afterSecondReceipt, err := batchService.GetBatch(context.Background(), batch.ID)
	if err != nil {
		t.Fatalf("get batch after second receipt: %v", err)
	}
	if afterSecondReceipt.FirstReceivedAt == nil || !afterSecondReceipt.FirstReceivedAt.Equal(firstReceivedAt) {
		t.Fatal("expected first_received_at to stay unchanged on later receipts")
	}

	if _, err := batchService.DisableBatch(context.Background(), batch.ID); err != nil {
		t.Fatalf("disable batch: %v", err)
	}

	// 停用后不再参与自动分配：即使账面有 15，未指定批次的扣减也应失败。
	_, err = fixture.service.DecreaseStock(context.Background(), inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("batch-decrease-auto"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "1",
	})
	if !errors.Is(err, inventory.ErrInsufficientAvailable) {
		t.Fatalf("expected insufficient available for auto allocation, got %v", err)
	}

	// 显式指定停用批次仍可扣减存量。
	decreased, err := fixture.service.DecreaseStock(context.Background(), inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("batch-decrease-explicit"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &batch.ID,
		Qty:              "1",
	})
	if err != nil {
		t.Fatalf("explicit decrease of disabled batch: %v", err)
	}
	if decreased.StockAfter.OnHandQty != "14" {
		t.Fatalf("expected on hand 14 after explicit decrease, got %s", decreased.StockAfter.OnHandQty)
	}

	// 停用批次不允许继续收货。
	_, err = fixture.service.IncreaseStock(context.Background(), inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("batch-increase-disabled"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &batch.ID,
		Qty:              "1",
	})
	if !errors.Is(err, inventory.ErrBatchInactive) {
		t.Fatalf("expected batch inactive on increase, got %v", err)
	}
}

func TestBatchDeleteGuard(t *testing.T) {
	fixture := newInventoryFixture(t)
	batchService := newBatchServiceForFixture(fixture)

	usedBatch := mustCreateBatch(t, batchService, fixture.skuID, "B-USED", 10)
	if _, err := fixture.service.IncreaseStock(context.Background(), inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("batch-used-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &usedBatch.ID,
		Qty:              "3",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	if err := batchService.DeleteBatch(context.Background(), usedBatch.ID); !errors.Is(err, inventory.ErrBatchReferenced) {
		t.Fatalf("expected referenced error, got %v", err)
	}

	unusedBatch := mustCreateBatch(t, batchService, fixture.skuID, "B-UNUSED", 10)
	if err := batchService.DeleteBatch(context.Background(), unusedBatch.ID); err != nil {
		t.Fatalf("delete unused batch: %v", err)
	}

	if _, err := batchService.GetBatch(context.Background(), unusedBatch.ID); !errors.Is(err, inventory.ErrBatchNotFound) {
		t.Fatalf("expected soft-deleted batch to be hidden, got %v", err)
	}

	list, err := batchService.ListBatches(context.Background(), inventory.ListBatchesFilter{SKUID: &fixture.skuID})
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	for _, item := range list.Items {
		if item.ID == unusedBatch.ID {
			t.Fatal("soft-deleted batch must not appear in the list")
		}
	}
}

func TestBatchDuplicateNoIsRejected(t *testing.T) {
	fixture := newInventoryFixture(t)
	batchService := newBatchServiceForFixture(fixture)

	mustCreateBatch(t, batchService, fixture.skuID, "B-DUP", 10)

	_, err := batchService.CreateBatch(context.Background(), inventory.CreateBatchInput{
		SKUID:   fixture.skuID,
		BatchNo: "B-DUP",
	})
	if !errors.Is(err, inventory.ErrBatchNoDuplicate) {
		t.Fatalf("expected duplicate batch no error, got %v", err)
	}
}
