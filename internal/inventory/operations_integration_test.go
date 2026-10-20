//go:build integration

package inventory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/shared/testdb"
	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
)

type inventoryFixture struct {
	db          *testdb.DB
	service     inventory.Service
	repo        inventory.Repository
	uow         inventory.UnitOfWork
	warehouseID int64
	skuID       int64
	unitID      int64
	categoryID  int64
	materialID  int64
}

func newInventoryFixture(t *testing.T) *inventoryFixture {
	t.Helper()

	db := testdb.Require(t)
	db.Truncate(t)

	ctx := context.Background()
	fixture := &inventoryFixture{db: db}

	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO units (code, name, symbol, unit_type, precision)
		 VALUES ('PCS', 'Piece', 'pcs', 'count', 0)
		 RETURNING id`).Scan(&fixture.unitID); err != nil {
		t.Fatalf("insert unit: %v", err)
	}
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO material_categories (code, name)
		 VALUES ('CAT-1', 'Category 1')
		 RETURNING id`).Scan(&fixture.categoryID); err != nil {
		t.Fatalf("insert material category: %v", err)
	}
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO materials (code, name, category_id, base_unit_id)
		 VALUES ('MAT-1', 'Material 1', $1, $2)
		 RETURNING id`, fixture.categoryID, fixture.unitID).Scan(&fixture.materialID); err != nil {
		t.Fatalf("insert material: %v", err)
	}
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO skus (material_id, code, name, unit_id)
		 VALUES ($1, 'SKU-1', 'SKU 1', $2)
		 RETURNING id`, fixture.materialID, fixture.unitID).Scan(&fixture.skuID); err != nil {
		t.Fatalf("insert sku: %v", err)
	}
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO warehouses (code, name)
		 VALUES ('WH-1', 'Warehouse 1')
		 RETURNING id`).Scan(&fixture.warehouseID); err != nil {
		t.Fatalf("insert warehouse: %v", err)
	}

	fixture.repo = inventory.NewPostgresRepository(db.Pool)
	fixture.uow = inventory.NewPostgresUnitOfWork(db.Pool)
	fixture.service = inventory.NewService(
		fixture.repo,
		warehouse.NewService(warehouse.NewPostgresRepository(db.Pool)),
		sku.NewService(sku.NewPostgresRepository(db.Pool), nil),
		fixture.uow,
	)

	return fixture
}

func (f *inventoryFixture) operationContext(key string) inventory.OperationContext {
	sourceType := "integration_test"
	sourceID := "SOURCE-1"
	sourceLineID := "1"
	return inventory.OperationContext{
		Actor:          &inventory.Actor{Type: "admin", ID: 1, Label: "integration-tester"},
		IdempotencyKey: key,
		SourceType:     &sourceType,
		SourceID:       &sourceID,
		SourceLineID:   &sourceLineID,
	}
}

func (f *inventoryFixture) createBatch(t *testing.T, batchNo string, priority int32) int64 {
	t.Helper()

	var batchID int64
	if err := f.db.Pool.QueryRow(context.Background(),
		`INSERT INTO inventory_batches (sku_id, batch_no, status, dispatch_priority)
		 VALUES ($1, $2, 'active', $3)
		 RETURNING id`, f.skuID, batchNo, priority).Scan(&batchID); err != nil {
		t.Fatalf("insert batch %s: %v", batchNo, err)
	}
	return batchID
}

func (f *inventoryFixture) stock(t *testing.T) (onHand string, reserved string) {
	t.Helper()

	if err := f.db.Pool.QueryRow(context.Background(),
		`SELECT on_hand_qty::text, reserved_qty::text
		 FROM inventory_stocks
		 WHERE warehouse_id = $1 AND sku_id = $2 AND deleted_at IS NULL`,
		f.warehouseID, f.skuID).Scan(&onHand, &reserved); err != nil {
		t.Fatalf("read stock: %v", err)
	}
	return onHand, reserved
}

func (f *inventoryFixture) count(t *testing.T, table string) int {
	t.Helper()

	var value int
	if err := f.db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&value); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return value
}

func TestIncreaseStockWritesStockLayerMovementAndIsIdempotent(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	request := inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("increase-1"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}

	result, err := fixture.service.IncreaseStock(ctx, request)
	if err != nil {
		t.Fatalf("increase stock: %v", err)
	}
	if result.StockAfter.OnHandQty != "10" || result.StockAfter.ReservedQty != "0" || result.StockAfter.AvailableQty != "10" {
		t.Fatalf("unexpected stock after increase: %#v", result.StockAfter)
	}
	if len(result.Movements) != 1 || result.Movements[0].StockLayerID == nil {
		t.Fatalf("expected one movement with stock layer, got %#v", result.Movements)
	}

	onHand, reserved := fixture.stock(t)
	if onHand != "10.000000" || reserved != "0.000000" {
		t.Fatalf("expected 10/0 stock, got %s/%s", onHand, reserved)
	}
	if fixture.count(t, "inventory_stock_layers") != 1 {
		t.Fatal("expected one stock layer")
	}
	if fixture.count(t, "inventory_movements") != 1 {
		t.Fatal("expected one movement")
	}

	var actorType, actorLabel string
	if err := fixture.db.Pool.QueryRow(ctx,
		`SELECT actor_type, actor_label FROM inventory_movements LIMIT 1`).Scan(&actorType, &actorLabel); err != nil {
		t.Fatalf("read movement actor: %v", err)
	}
	if actorType != "admin" || actorLabel != "integration-tester" {
		t.Fatalf("unexpected movement actor: %s/%s", actorType, actorLabel)
	}

	replay, err := fixture.service.IncreaseStock(ctx, request)
	if err != nil {
		t.Fatalf("replay increase stock: %v", err)
	}
	if replay.StockAfter != result.StockAfter || replay.Movements[0].ID != result.Movements[0].ID {
		t.Fatalf("expected identical replay, got %#v vs %#v", replay, result)
	}

	onHand, _ = fixture.stock(t)
	if onHand != "10.000000" {
		t.Fatalf("expected replay not to apply the increase again, got %s", onHand)
	}
	if fixture.count(t, "inventory_stock_layers") != 1 || fixture.count(t, "inventory_movements") != 1 {
		t.Fatal("expected replay not to create layers or movements")
	}

	request.Qty = "11"
	if _, err := fixture.service.IncreaseStock(ctx, request); !errors.Is(err, inventory.ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestReserveReleaseAndDecreaseFlow(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("increase-flow"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "100",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	reserve, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("reserve-flow"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "30",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}
	if reserve.Reservation == nil || reserve.Reservation.Status != "active" || reserve.StockAfter.ReservedQty != "30" {
		t.Fatalf("unexpected reserve result: %#v", reserve)
	}

	release, err := fixture.service.ReleaseStock(ctx, inventory.ReleaseStockRequest{
		OperationContext: fixture.operationContext("release-flow"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    reserve.Reservation.ID,
		Qty:              stringPointer("10"),
	})
	if err != nil {
		t.Fatalf("release stock: %v", err)
	}
	if release.StockAfter.ReservedQty != "20" || release.Reservation == nil || release.Reservation.ReleasedQty != "10" {
		t.Fatalf("unexpected release result: %#v", release)
	}

	decrease, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("decrease-reserved-flow"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    &reserve.Reservation.ID,
		Qty:              "20",
	})
	if err != nil {
		t.Fatalf("decrease reserved stock: %v", err)
	}
	if decrease.StockAfter.OnHandQty != "80" || decrease.StockAfter.ReservedQty != "0" || decrease.StockAfter.AvailableQty != "80" {
		t.Fatalf("unexpected stock after consuming reservation: %#v", decrease.StockAfter)
	}
	if decrease.Reservation == nil || decrease.Reservation.Status != "closed" || decrease.Reservation.CloseReason == nil || *decrease.Reservation.CloseReason != "mixed" {
		t.Fatalf("expected mixed closed reservation, got %#v", decrease.Reservation)
	}

	decreased, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("decrease-available-flow"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "50",
	})
	if err != nil {
		t.Fatalf("decrease available stock: %v", err)
	}
	if decreased.StockAfter.OnHandQty != "30" || decreased.StockAfter.ReservedQty != "0" {
		t.Fatalf("unexpected stock after available decrease: %#v", decreased.StockAfter)
	}
}

func TestConcurrentReserveDoesNotOversell(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("concurrent-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "100",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"concurrent-reserve-a", "concurrent-reserve-b"} {
		key := key
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
				OperationContext: fixture.operationContext(key),
				WarehouseID:      fixture.warehouseID,
				SKUID:            fixture.skuID,
				Qty:              "60",
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var success, insufficient int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, inventory.ErrInsufficientAvailable):
			insufficient++
		default:
			t.Fatalf("unexpected concurrent reserve error: %v", err)
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("expected one success and one insufficient error, got success=%d insufficient=%d", success, insufficient)
	}

	_, reserved := fixture.stock(t)
	if reserved != "60.000000" {
		t.Fatalf("expected reserved 60, got %s", reserved)
	}
}

func TestAllocationOrderUsesDispatchPriorityAndTargetBatchFirst(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	delayed := fixture.createBatch(t, "BATCH-DELAYED", 11)
	boosted := fixture.createBatch(t, "BATCH-BOOSTED", 0)
	defaultPriority := fixture.createBatch(t, "BATCH-DEFAULT", 10)

	increases := []struct {
		key     string
		batchID int64
	}{
		{"increase-delayed", delayed},
		{"increase-boosted", boosted},
		{"increase-default", defaultPriority},
	}
	for _, item := range increases {
		batchID := item.batchID
		if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
			OperationContext: fixture.operationContext(item.key),
			WarehouseID:      fixture.warehouseID,
			SKUID:            fixture.skuID,
			BatchID:          &batchID,
			Qty:              "10",
		}); err != nil {
			t.Fatalf("increase batch %d: %v", batchID, err)
		}
	}

	reserved, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("reserve-priority"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "15",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}
	if len(reserved.Reservation.Allocations) != 2 {
		t.Fatalf("expected two allocations, got %#v", reserved.Reservation.Allocations)
	}
	if reserved.Reservation.Allocations[0].BatchID == nil || *reserved.Reservation.Allocations[0].BatchID != boosted {
		t.Fatalf("expected boosted batch first, got %#v", reserved.Reservation.Allocations)
	}
	if reserved.Reservation.Allocations[0].Qty != "10" || reserved.Reservation.Allocations[1].Qty != "5" {
		t.Fatalf("unexpected allocation quantities: %#v", reserved.Reservation.Allocations)
	}

	targeted, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("reserve-target-batch"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		BatchID:          &defaultPriority,
		Qty:              "15",
	})
	if err != nil {
		t.Fatalf("reserve target batch: %v", err)
	}
	if len(targeted.Reservation.Allocations) != 2 {
		t.Fatalf("expected target batch plus spill, got %#v", targeted.Reservation.Allocations)
	}
	if targeted.Reservation.Allocations[0].BatchID == nil || *targeted.Reservation.Allocations[0].BatchID != defaultPriority {
		t.Fatalf("expected target batch first, got %#v", targeted.Reservation.Allocations)
	}
}

func TestDecreaseAvailableDoesNotConsumeReserved(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("decrease-guard-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}
	if _, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("decrease-guard-reserve"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "8",
	}); err != nil {
		t.Fatalf("reserve stock: %v", err)
	}

	if _, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("decrease-guard-over"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "3",
	}); !errors.Is(err, inventory.ErrInsufficientAvailable) {
		t.Fatalf("expected insufficient available, got %v", err)
	}

	if _, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("decrease-guard-ok"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "2",
	}); err != nil {
		t.Fatalf("decrease available stock: %v", err)
	}

	onHand, reserved := fixture.stock(t)
	if onHand != "8.000000" || reserved != "8.000000" {
		t.Fatalf("expected 8/8 stock, got %s/%s", onHand, reserved)
	}
}

func TestTransactionRollsBackOnFailure(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	err := fixture.uow.WithinTx(ctx, func(ctx context.Context, repo inventory.Repository) error {
		if err := repo.EnsureStockRow(ctx, fixture.warehouseID, fixture.skuID); err != nil {
			return err
		}
		if err := repo.AddStockOnHand(ctx, fixture.warehouseID, fixture.skuID, "10"); err != nil {
			return err
		}
		layerID := int64(1)
		_, err := repo.InsertMovement(ctx, inventory.MovementInput{
			OperationType:  "not_a_valid_operation_type",
			WarehouseID:    fixture.warehouseID,
			SKUID:          fixture.skuID,
			StockLayerID:   &layerID,
			Qty:            "10",
			IdempotencyKey: "rollback-test",
		})
		return err
	})
	if err == nil {
		t.Fatal("expected the invalid movement to fail")
	}

	var onHand, reserved string
	if err := fixture.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(on_hand_qty), 0)::text, COALESCE(MAX(reserved_qty), 0)::text
		 FROM inventory_stocks
		 WHERE warehouse_id = $1 AND sku_id = $2`,
		fixture.warehouseID, fixture.skuID).Scan(&onHand, &reserved); err != nil {
		t.Fatalf("read stock after rollback: %v", err)
	}
	if onHand != "0" || reserved != "0" {
		t.Fatalf("expected rollback to restore 0/0 stock, got %s/%s", onHand, reserved)
	}
}

func TestListMovementsFiltersAndPagination(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("movement-list-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	warehouseID := fixture.warehouseID
	skuID := fixture.skuID
	operationType := "increase"
	createdFrom := time.Now().Add(-time.Hour)
	createdTo := time.Now().Add(time.Hour)
	result, err := fixture.service.ListMovements(ctx, inventory.ListMovementsFilter{
		WarehouseID:   &warehouseID,
		SKUID:         &skuID,
		OperationType: &operationType,
		CreatedFrom:   &createdFrom,
		CreatedTo:     &createdTo,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list movements: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].OperationType != "increase" {
		t.Fatalf("unexpected movements: %#v", result.Items)
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestReleaseRejectsOverRemainingAndClosesReservation(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("release-close-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}
	reserved, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("release-close-reserve"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "5",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}

	if _, err := fixture.service.ReleaseStock(ctx, inventory.ReleaseStockRequest{
		OperationContext: fixture.operationContext("release-close-over"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    reserved.Reservation.ID,
		Qty:              stringPointer("6"),
	}); !errors.Is(err, inventory.ErrInsufficientReserved) {
		t.Fatalf("expected insufficient reserved, got %v", err)
	}

	closed, err := fixture.service.ReleaseStock(ctx, inventory.ReleaseStockRequest{
		OperationContext: fixture.operationContext("release-close-all"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    reserved.Reservation.ID,
	})
	if err != nil {
		t.Fatalf("release remaining reservation: %v", err)
	}
	if closed.Reservation == nil || closed.Reservation.Status != "closed" || closed.Reservation.CloseReason == nil || *closed.Reservation.CloseReason != "released" {
		t.Fatalf("expected released closed reservation, got %#v", closed.Reservation)
	}
	if closed.StockAfter.ReservedQty != "0" || closed.StockAfter.OnHandQty != "10" {
		t.Fatalf("unexpected stock after full release: %#v", closed.StockAfter)
	}

	if _, err := fixture.service.ReleaseStock(ctx, inventory.ReleaseStockRequest{
		OperationContext: fixture.operationContext("release-close-again"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    reserved.Reservation.ID,
	}); !errors.Is(err, inventory.ErrReservationNotActive) {
		t.Fatalf("expected reservation not active, got %v", err)
	}
}

func TestMutationRejectsInactiveWarehouse(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.db.Pool.Exec(ctx,
		`UPDATE warehouses SET status = 'inactive' WHERE id = $1`, fixture.warehouseID); err != nil {
		t.Fatalf("disable warehouse: %v", err)
	}

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("inactive-warehouse"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "1",
	}); !errors.Is(err, inventory.ErrWarehouseInactive) {
		t.Fatalf("expected inactive warehouse error, got %v", err)
	}
}

func TestReservationRecordsActor(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("reservation-actor-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}
	reserved, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("reservation-actor-reserve"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "3",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}

	var actorType, actorLabel string
	if err := fixture.db.Pool.QueryRow(ctx,
		`SELECT actor_type, actor_label FROM inventory_reservations WHERE id = $1`,
		reserved.Reservation.ID).Scan(&actorType, &actorLabel); err != nil {
		t.Fatalf("read reservation actor: %v", err)
	}
	if actorType != "admin" || actorLabel != "integration-tester" {
		t.Fatalf("unexpected reservation actor: %s/%s", actorType, actorLabel)
	}
}

func TestConsumingReservationWithoutReleaseClosesAsConsumed(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("consume-close-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "10",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}
	reserved, err := fixture.service.ReserveStock(ctx, inventory.ReserveStockRequest{
		OperationContext: fixture.operationContext("consume-close-reserve"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "4",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}
	consumed, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
		OperationContext: fixture.operationContext("consume-close-decrease"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		ReservationID:    &reserved.Reservation.ID,
		Qty:              "4",
	})
	if err != nil {
		t.Fatalf("consume reservation: %v", err)
	}
	if consumed.Reservation == nil || consumed.Reservation.Status != "closed" || consumed.Reservation.CloseReason == nil || *consumed.Reservation.CloseReason != "consumed" {
		t.Fatalf("expected consumed closed reservation, got %#v", consumed.Reservation)
	}
	if consumed.StockAfter.OnHandQty != "6" || consumed.StockAfter.ReservedQty != "0" {
		t.Fatalf("unexpected stock after consuming reservation: %#v", consumed.StockAfter)
	}
}

func TestConcurrentDecreaseDoesNotOverdraw(t *testing.T) {
	fixture := newInventoryFixture(t)
	ctx := context.Background()

	if _, err := fixture.service.IncreaseStock(ctx, inventory.IncreaseStockRequest{
		OperationContext: fixture.operationContext("concurrent-decrease-increase"),
		WarehouseID:      fixture.warehouseID,
		SKUID:            fixture.skuID,
		Qty:              "100",
	}); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"concurrent-decrease-a", "concurrent-decrease-b"} {
		key := key
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := fixture.service.DecreaseStock(ctx, inventory.DecreaseStockRequest{
				OperationContext: fixture.operationContext(key),
				WarehouseID:      fixture.warehouseID,
				SKUID:            fixture.skuID,
				Qty:              "60",
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var success, insufficient int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, inventory.ErrInsufficientAvailable):
			insufficient++
		default:
			t.Fatalf("unexpected concurrent decrease error: %v", err)
		}
	}
	if success != 1 || insufficient != 1 {
		t.Fatalf("expected one success and one insufficient error, got success=%d insufficient=%d", success, insufficient)
	}

	onHand, reserved := fixture.stock(t)
	if onHand != "40.000000" || reserved != "0.000000" {
		t.Fatalf("expected 40/0 stock, got %s/%s", onHand, reserved)
	}
}
