package inventory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceListReservationsNormalizesQuantitiesAndReferences(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.reservations[2] = inventory.Reservation{
		ID:             2,
		WarehouseID:    1,
		SKUID:          9,
		TotalQty:       "5.000000",
		ReleasedQty:    "1.000000",
		ConsumedQty:    "0.500000",
		Status:         "active",
		IdempotencyKey: "sales_order:SO-20261027-002:5:reserve",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	service := newTestService(repo, newFakeWarehouseReader(1), newFakeSKUReader(9))

	warehouseID := int64(1)
	skuID := int64(9)
	status := "active"
	result, err := service.ListReservations(ctx, inventory.ListReservationsFilter{
		WarehouseID: &warehouseID,
		SKUID:       &skuID,
		Status:      &status,
	})
	if err != nil {
		t.Fatalf("list reservations: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.TotalQty != "5" || item.ReleasedQty != "1" || item.ConsumedQty != "0.5" {
		t.Fatalf("expected normalized quantities, got total=%q released=%q consumed=%q", item.TotalQty, item.ReleasedQty, item.ConsumedQty)
	}
	if item.RemainingQty != "3.5" {
		t.Fatalf("expected remaining qty 3.5, got %q", item.RemainingQty)
	}
	if item.Warehouse == nil || item.Warehouse.ID != 1 || item.SKU == nil || item.SKU.ID != 9 {
		t.Fatalf("expected warehouse and sku references, got %#v", item)
	}
	if result.Limit != inventory.DefaultListLimit || result.Offset != 0 {
		t.Fatalf("unexpected pagination: %d/%d", result.Limit, result.Offset)
	}
}

func TestServiceListReservationsCapsLimit(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader())

	result, err := service.ListReservations(ctx, inventory.ListReservationsFilter{Limit: 500})
	if err != nil {
		t.Fatalf("list reservations: %v", err)
	}
	if result.Limit != inventory.MaxListLimit {
		t.Fatalf("expected capped limit %d, got %d", inventory.MaxListLimit, result.Limit)
	}
}

func TestServiceListReservationsRejectsInvalidStatus(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader())

	status := "cancelled"
	_, err := service.ListReservations(ctx, inventory.ListReservationsFilter{Status: &status})
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceListReservationsValidatesReferences(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader())

	warehouseID := int64(99)
	_, err := service.ListReservations(ctx, inventory.ListReservationsFilter{WarehouseID: &warehouseID})
	if !errors.Is(err, inventory.ErrWarehouseNotFound) {
		t.Fatalf("expected warehouse not found, got %v", err)
	}
}

func TestServiceGetReservationIncludesItems(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	receivedAt := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.reservations[7] = inventory.Reservation{
		ID:             7,
		WarehouseID:    1,
		SKUID:          9,
		TotalQty:       "5.000000",
		ReleasedQty:    "0",
		ConsumedQty:    "0",
		Status:         "active",
		IdempotencyKey: "sales_order:SO-20261027-007:5:reserve",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	repo.reservationItems[7] = []inventory.ReservationItem{{
		ID:            11,
		ReservationID: 7,
		StockLayerID:  3,
		BatchID:       nil,
		ReceivedAt:    &receivedAt,
		ReservedQty:   "5.000000",
		ReleasedQty:   "1.000000",
		ConsumedQty:   "1.000000",
		CreatedAt:     now,
		UpdatedAt:     now,
	}}
	service := newTestService(repo, newFakeWarehouseReader(1), newFakeSKUReader(9))

	reservation, err := service.GetReservation(ctx, 7)
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}

	if reservation.RemainingQty != "5" {
		t.Fatalf("expected remaining qty 5, got %q", reservation.RemainingQty)
	}
	if len(reservation.Items) != 1 {
		t.Fatalf("expected 1 reservation item, got %d", len(reservation.Items))
	}
	item := reservation.Items[0]
	if item.RemainingQty != "3" {
		t.Fatalf("expected item remaining qty 3, got %q", item.RemainingQty)
	}
	if item.ReceivedAt == nil || !item.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("expected received_at %v, got %#v", receivedAt, item.ReceivedAt)
	}
	if reservation.Warehouse == nil || reservation.SKU == nil {
		t.Fatalf("expected warehouse and sku references, got %#v", reservation)
	}
}

func TestServiceGetReservationNotFound(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader())

	_, err := service.GetReservation(ctx, 404)
	if !errors.Is(err, inventory.ErrReservationNotFound) {
		t.Fatalf("expected reservation not found, got %v", err)
	}
}

func TestServiceGetReservationRejectsNonPositiveID(t *testing.T) {
	ctx := context.Background()
	service := newTestService(newFakeRepository(), newFakeWarehouseReader(), newFakeSKUReader())

	_, err := service.GetReservation(ctx, 0)
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
