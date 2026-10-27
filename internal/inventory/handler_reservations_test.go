package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerListReservations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	var captured inventory.ListReservationsFilter
	service := &fakeService{
		listReservationsFunc: func(_ context.Context, filter inventory.ListReservationsFilter) (inventory.ReservationListResult, error) {
			captured = filter
			return inventory.ReservationListResult{
				Items: []inventory.Reservation{{
					ID:             2,
					WarehouseID:    1,
					SKUID:          9,
					TotalQty:       "5",
					ReleasedQty:    "0",
					ConsumedQty:    "0",
					RemainingQty:   "5",
					Status:         "active",
					IdempotencyKey: "sales_order:SO-20261027-002:5:reserve",
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
				Limit:  50,
				Offset: 100,
			}, nil
		},
	}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/reservations?warehouse_id=1&sku_id=9&status=active&limit=50&offset=100", nil)
	req.Header.Set("X-Trace-ID", "req_reservations")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if captured.WarehouseID == nil || *captured.WarehouseID != 1 {
		t.Fatalf("expected warehouse filter 1, got %#v", captured.WarehouseID)
	}
	if captured.SKUID == nil || *captured.SKUID != 9 {
		t.Fatalf("expected sku filter 9, got %#v", captured.SKUID)
	}
	if captured.Status == nil || *captured.Status != "active" {
		t.Fatalf("expected status filter active, got %#v", captured.Status)
	}
	if captured.Limit != 50 || captured.Offset != 100 {
		t.Fatalf("expected limit/offset 50/100, got %d/%d", captured.Limit, captured.Offset)
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Items  []inventory.Reservation `json:"items"`
			Limit  int32                   `json:"limit"`
			Offset int32                   `json:"offset"`
		} `json:"data"`
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeSuccess {
		t.Fatalf("expected response code %d, got %d", response.CodeSuccess, body.Code)
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0].ID != 2 || body.Data.Items[0].RemainingQty != "5" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.Data.Limit != 50 || body.Data.Offset != 100 {
		t.Fatalf("unexpected pagination: %d/%d", body.Data.Limit, body.Data.Offset)
	}
	if body.TraceID != "req_reservations" {
		t.Fatalf("expected trace id req_reservations, got %q", body.TraceID)
	}
}

func TestHandlerListReservationsRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/reservations?limit=abc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandlerGetReservation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 10, 27, 3, 0, 0, 0, time.UTC)
	service := &fakeService{
		getReservationFunc: func(_ context.Context, reservationID int64) (*inventory.Reservation, error) {
			if reservationID != 7 {
				t.Fatalf("expected reservation id 7, got %d", reservationID)
			}
			return &inventory.Reservation{
				ID:           7,
				WarehouseID:  1,
				SKUID:        9,
				TotalQty:     "5",
				RemainingQty: "5",
				Status:       "active",
				CreatedAt:    now,
				UpdatedAt:    now,
				Items: []inventory.ReservationItem{{
					ID:            11,
					ReservationID: 7,
					StockLayerID:  3,
					ReservedQty:   "5",
					RemainingQty:  "5",
				}},
			}, nil
		},
	}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/reservations/7", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Code int                   `json:"code"`
		Data inventory.Reservation `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeSuccess {
		t.Fatalf("expected response code %d, got %d", response.CodeSuccess, body.Code)
	}
	if body.Data.ID != 7 || len(body.Data.Items) != 1 || body.Data.Items[0].ID != 11 {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
}

func TestHandlerGetReservationNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		getReservationFunc: func(context.Context, int64) (*inventory.Reservation, error) {
			return nil, inventory.ErrReservationNotFound
		},
	}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/reservations/404", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Message != inventory.ErrReservationNotFound.Error() {
		t.Fatalf("expected reservation not found message, got %q", body.Message)
	}
}

func TestHandlerGetReservationRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/reservations/abc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
