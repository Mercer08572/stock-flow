package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/auth"
	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func newInventoryRouterWithActor(service inventory.Service, caller auth.Caller) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	router.Use(func(c *gin.Context) {
		c.Set(auth.CallerGinContextKey, caller)
		c.Next()
	})
	api := router.Group("/api/v1")
	inventory.NewHandler(service).RegisterRoutes(api)
	return router
}

func TestHandlerIncreaseStockMapsActorAndRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var captured inventory.IncreaseStockRequest
	service := &fakeService{
		increaseStockFunc: func(_ context.Context, req inventory.IncreaseStockRequest) (*inventory.StockOperationResult, error) {
			captured = req
			return &inventory.StockOperationResult{
				OperationType: "increase",
				WarehouseID:   req.WarehouseID,
				SKUID:         req.SKUID,
				Qty:           "10",
				StockAfter:    inventory.StockQuantities{OnHandQty: "10", ReservedQty: "0", AvailableQty: "10"},
				Movements:     []inventory.MovementRef{{ID: 1, Qty: "10"}},
			}, nil
		},
	}
	caller := auth.Caller{
		Type:  auth.CallerTypeAdmin,
		Admin: &auth.AdminCaller{AdminUserID: 7, Username: "operator"},
	}
	router := newInventoryRouterWithActor(service, caller)

	body := `{"warehouse_id":1,"sku_id":2,"qty":"10","idempotency_key":"key-1","source_type":"sales_order","source_id":"SO-1","source_line_id":"1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/increase", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if captured.Actor == nil || captured.Actor.Type != "admin" || captured.Actor.ID != 7 || captured.Actor.Label != "operator" {
		t.Fatalf("unexpected actor: %#v", captured.Actor)
	}
	if captured.IdempotencyKey != "key-1" || captured.SourceID == nil || *captured.SourceID != "SO-1" {
		t.Fatalf("unexpected operation context: %#v", captured)
	}

	var responseBody struct {
		Data inventory.StockOperationResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseBody.Data.StockAfter.OnHandQty != "10" {
		t.Fatalf("unexpected response data: %#v", responseBody.Data)
	}
}

func TestHandlerIncreaseStockMapsBusinessError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		increaseStockFunc: func(context.Context, inventory.IncreaseStockRequest) (*inventory.StockOperationResult, error) {
			return nil, inventory.ErrInsufficientAvailable
		},
	}
	router := newInventoryRouterWithActor(service, auth.Caller{})

	body := `{"warehouse_id":1,"sku_id":2,"qty":"10","idempotency_key":"key-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/increase", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	var responseBody struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseBody.ErrorCode != string(apperr.CodeInventoryInsufficientAvailable) {
		t.Fatalf("unexpected error code: %q", responseBody.ErrorCode)
	}
}

func TestHandlerIncreaseStockRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newInventoryRouterWithActor(&fakeService{}, auth.Caller{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/increase", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerListMovementsParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var captured inventory.ListMovementsFilter
	service := &fakeService{
		listMovementsFunc: func(_ context.Context, filter inventory.ListMovementsFilter) (inventory.MovementListResult, error) {
			captured = filter
			return inventory.MovementListResult{Items: []inventory.Movement{}, Limit: filter.Limit, Offset: filter.Offset}, nil
		},
	}
	router := newInventoryRouterWithActor(service, auth.Caller{})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/inventory/movements?warehouse_id=1&sku_id=2&operation_type=increase&created_from=2026-01-01T00:00:00Z&limit=5&offset=10",
		nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if captured.WarehouseID == nil || *captured.WarehouseID != 1 {
		t.Fatalf("unexpected warehouse filter: %#v", captured.WarehouseID)
	}
	if captured.SKUID == nil || *captured.SKUID != 2 {
		t.Fatalf("unexpected sku filter: %#v", captured.SKUID)
	}
	if captured.OperationType == nil || *captured.OperationType != "increase" {
		t.Fatalf("unexpected operation filter: %#v", captured.OperationType)
	}
	if captured.Limit != 5 || captured.Offset != 10 || captured.CreatedFrom == nil {
		t.Fatalf("unexpected pagination/time filter: %#v", captured)
	}
}
