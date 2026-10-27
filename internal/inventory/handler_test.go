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
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerGetStock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeService{
		getStockFunc: func(_ context.Context, query inventory.GetStockQuery) (*inventory.StockBalance, error) {
			if query.WarehouseID != 1 || query.SKUID != 2 {
				t.Fatalf("expected warehouse/sku 1/2, got %d/%d", query.WarehouseID, query.SKUID)
			}
			if !query.IncludeLayers {
				t.Fatal("expected include_layers true")
			}
			return &inventory.StockBalance{
				WarehouseID:  1,
				SKUID:        2,
				OnHandQty:    "10",
				ReservedQty:  "3",
				AvailableQty: "7",
				UpdatedAt:    &now,
			}, nil
		},
	}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stocks/1/2?include_layers=true", nil)
	req.Header.Set("X-Trace-ID", "req_stock")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			WarehouseID  int64  `json:"warehouse_id"`
			SKUID        int64  `json:"sku_id"`
			AvailableQty string `json:"available_qty"`
		} `json:"data"`
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeSuccess {
		t.Fatalf("expected response code %d, got %d", response.CodeSuccess, body.Code)
	}
	if body.Data.WarehouseID != 1 || body.Data.SKUID != 2 || body.Data.AvailableQty != "7" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.TraceID != "req_stock" {
		t.Fatalf("expected trace id req_stock, got %q", body.TraceID)
	}
}

func TestHandlerMapsSKUNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		getStockFunc: func(context.Context, inventory.GetStockQuery) (*inventory.StockBalance, error) {
			return nil, inventory.ErrSKUNotFound
		},
	}
	router := newInventoryRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stocks/1/2", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeBadRequest {
		t.Fatalf("expected response code %d, got %d", response.CodeBadRequest, body.Code)
	}
	if body.Message != inventory.ErrSKUNotFound.Error() {
		t.Fatalf("expected sku not found message, got %q", body.Message)
	}
}

func newInventoryRouter(service inventory.Service) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	inventory.NewHandler(service).RegisterRoutes(api)
	return router
}

type fakeService struct {
	listStocksFunc       func(context.Context, inventory.ListStocksFilter) (inventory.StockListResult, error)
	getStockFunc         func(context.Context, inventory.GetStockQuery) (*inventory.StockBalance, error)
	listLayersFunc       func(context.Context, inventory.ListLayersFilter) (inventory.LayerListResult, error)
	listReservationsFunc func(context.Context, inventory.ListReservationsFilter) (inventory.ReservationListResult, error)
	getReservationFunc   func(context.Context, int64) (*inventory.Reservation, error)
	increaseStockFunc    func(context.Context, inventory.IncreaseStockRequest) (*inventory.StockOperationResult, error)
	reserveStockFunc     func(context.Context, inventory.ReserveStockRequest) (*inventory.StockOperationResult, error)
	releaseStockFunc     func(context.Context, inventory.ReleaseStockRequest) (*inventory.StockOperationResult, error)
	decreaseStockFunc    func(context.Context, inventory.DecreaseStockRequest) (*inventory.StockOperationResult, error)
	listMovementsFunc    func(context.Context, inventory.ListMovementsFilter) (inventory.MovementListResult, error)
}

func (s *fakeService) ListReservations(ctx context.Context, filter inventory.ListReservationsFilter) (inventory.ReservationListResult, error) {
	if s.listReservationsFunc != nil {
		return s.listReservationsFunc(ctx, filter)
	}
	return inventory.ReservationListResult{}, nil
}

func (s *fakeService) GetReservation(ctx context.Context, reservationID int64) (*inventory.Reservation, error) {
	if s.getReservationFunc != nil {
		return s.getReservationFunc(ctx, reservationID)
	}
	return nil, inventory.ErrReservationNotFound
}

func (s *fakeService) ListStocks(ctx context.Context, filter inventory.ListStocksFilter) (inventory.StockListResult, error) {
	if s.listStocksFunc != nil {
		return s.listStocksFunc(ctx, filter)
	}
	return inventory.StockListResult{}, nil
}

func (s *fakeService) GetStock(ctx context.Context, query inventory.GetStockQuery) (*inventory.StockBalance, error) {
	if s.getStockFunc != nil {
		return s.getStockFunc(ctx, query)
	}
	return nil, inventory.ErrNotFound
}

func (s *fakeService) ListLayers(ctx context.Context, filter inventory.ListLayersFilter) (inventory.LayerListResult, error) {
	if s.listLayersFunc != nil {
		return s.listLayersFunc(ctx, filter)
	}
	return inventory.LayerListResult{}, nil
}

func (s *fakeService) IncreaseStock(ctx context.Context, req inventory.IncreaseStockRequest) (*inventory.StockOperationResult, error) {
	if s.increaseStockFunc != nil {
		return s.increaseStockFunc(ctx, req)
	}
	return nil, errFakeRepositoryNotImplemented
}

func (s *fakeService) ReserveStock(ctx context.Context, req inventory.ReserveStockRequest) (*inventory.StockOperationResult, error) {
	if s.reserveStockFunc != nil {
		return s.reserveStockFunc(ctx, req)
	}
	return nil, errFakeRepositoryNotImplemented
}

func (s *fakeService) ReleaseStock(ctx context.Context, req inventory.ReleaseStockRequest) (*inventory.StockOperationResult, error) {
	if s.releaseStockFunc != nil {
		return s.releaseStockFunc(ctx, req)
	}
	return nil, errFakeRepositoryNotImplemented
}

func (s *fakeService) DecreaseStock(ctx context.Context, req inventory.DecreaseStockRequest) (*inventory.StockOperationResult, error) {
	if s.decreaseStockFunc != nil {
		return s.decreaseStockFunc(ctx, req)
	}
	return nil, errFakeRepositoryNotImplemented
}

func (s *fakeService) ListMovements(ctx context.Context, filter inventory.ListMovementsFilter) (inventory.MovementListResult, error) {
	if s.listMovementsFunc != nil {
		return s.listMovementsFunc(ctx, filter)
	}
	return inventory.MovementListResult{}, errFakeRepositoryNotImplemented
}
