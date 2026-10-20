//go:build integration

package warehouse_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/shared/testdb"
	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
)

func TestWarehouseInventoryReferenceProtection(t *testing.T) {
	db := testdb.Require(t)
	db.Truncate(t)

	ctx := context.Background()

	var unitID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO units (code, name, symbol, unit_type, precision)
		 VALUES ('PCS', 'Piece', 'pcs', 'count', 0)
		 RETURNING id`).Scan(&unitID); err != nil {
		t.Fatalf("insert unit: %v", err)
	}
	var categoryID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO material_categories (code, name)
		 VALUES ('CAT-1', 'Category 1')
		 RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatalf("insert material category: %v", err)
	}
	var materialID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO materials (code, name, category_id, base_unit_id)
		 VALUES ('MAT-1', 'Material 1', $1, $2)
		 RETURNING id`, categoryID, unitID).Scan(&materialID); err != nil {
		t.Fatalf("insert material: %v", err)
	}
	var skuID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO skus (material_id, code, name, unit_id)
		 VALUES ($1, 'SKU-1', 'SKU 1', $2)
		 RETURNING id`, materialID, unitID).Scan(&skuID); err != nil {
		t.Fatalf("insert sku: %v", err)
	}
	var referencedWarehouseID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO warehouses (code, name) VALUES ('WH-REF', 'Referenced') RETURNING id`).Scan(&referencedWarehouseID); err != nil {
		t.Fatalf("insert referenced warehouse: %v", err)
	}
	var unreferencedWarehouseID int64
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO warehouses (code, name) VALUES ('WH-FREE', 'Unreferenced') RETURNING id`).Scan(&unreferencedWarehouseID); err != nil {
		t.Fatalf("insert unreferenced warehouse: %v", err)
	}

	inventoryRepo := inventory.NewPostgresRepository(db.Pool)
	warehouseService := warehouse.NewService(
		warehouse.NewPostgresRepository(db.Pool),
		inventory.NewReferenceService(inventoryRepo),
	)
	skuService := sku.NewService(sku.NewPostgresRepository(db.Pool), nil)
	inventoryService := inventory.NewService(
		inventoryRepo,
		warehouseService,
		skuService,
		inventory.NewPostgresUnitOfWork(db.Pool),
	)

	sourceType := "integration_test"
	increaseRequest := inventory.IncreaseStockRequest{
		OperationContext: inventory.OperationContext{
			Actor:          &inventory.Actor{Type: "admin", ID: 1, Label: "integration-tester"},
			IdempotencyKey: "warehouse-reference-increase",
			SourceType:     &sourceType,
		},
		WarehouseID: referencedWarehouseID,
		SKUID:       skuID,
		Qty:         "5",
	}
	if _, err := inventoryService.IncreaseStock(ctx, increaseRequest); err != nil {
		t.Fatalf("increase stock: %v", err)
	}

	if err := warehouseService.Delete(ctx, referencedWarehouseID); !errors.Is(err, warehouse.ErrReferencedByInventory) {
		t.Fatalf("expected referenced warehouse delete to fail, got %v", err)
	}

	disabled, err := warehouseService.Disable(ctx, referencedWarehouseID)
	if err != nil {
		t.Fatalf("disable referenced warehouse: %v", err)
	}
	if disabled.Status != warehouse.StatusInactive {
		t.Fatalf("expected inactive warehouse, got %s", disabled.Status)
	}

	stock, err := inventoryService.GetStock(ctx, inventory.GetStockQuery{WarehouseID: referencedWarehouseID, SKUID: skuID})
	if err != nil {
		t.Fatalf("query stock of inactive warehouse: %v", err)
	}
	if stock.OnHandQty != "5" {
		t.Fatalf("expected historical stock 5, got %s", stock.OnHandQty)
	}

	if err := warehouseService.Delete(ctx, unreferencedWarehouseID); err != nil {
		t.Fatalf("delete unreferenced warehouse: %v", err)
	}
	var deletedAt *string
	if err := db.Pool.QueryRow(ctx,
		`SELECT deleted_at::text FROM warehouses WHERE id = $1`, unreferencedWarehouseID).Scan(&deletedAt); err != nil {
		t.Fatalf("read deleted warehouse: %v", err)
	}
	if deletedAt == nil {
		t.Fatal("expected unreferenced warehouse to be soft deleted")
	}
}
