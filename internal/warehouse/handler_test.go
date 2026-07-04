package warehouse_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/internal/warehouse"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerCreateWarehouse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeService{
		createFunc: func(_ context.Context, input warehouse.CreateInput) (*warehouse.Warehouse, error) {
			if input.Code != "WH-001" {
				t.Fatalf("expected code WH-001, got %q", input.Code)
			}

			return &warehouse.Warehouse{
				ID:        1,
				Code:      input.Code,
				Name:      input.Name,
				Type:      warehouse.TypeNormal,
				Status:    warehouse.StatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
	router := newWarehouseRouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouses", bytes.NewBufferString(`{
		"code":"WH-001",
		"name":"Main",
		"type":"normal",
		"status":"active"
	}`))
	req.Header.Set("X-Trace-ID", "req_create")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ID   int64  `json:"id"`
			Code string `json:"code"`
		} `json:"data"`
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Code != response.CodeSuccess {
		t.Fatalf("expected response code %d, got %d", response.CodeSuccess, body.Code)
	}
	if body.Message != "success" {
		t.Fatalf("expected success message, got %q", body.Message)
	}
	if body.Data.ID != 1 || body.Data.Code != "WH-001" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.TraceID != "req_create" {
		t.Fatalf("expected trace id req_create, got %q", body.TraceID)
	}
}

func TestHandlerDisableWarehouse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		disableFunc: func(_ context.Context, id int64) (*warehouse.Warehouse, error) {
			if id != 1 {
				t.Fatalf("expected id 1, got %d", id)
			}

			return &warehouse.Warehouse{
				ID:     1,
				Code:   "WH-001",
				Name:   "Main",
				Type:   warehouse.TypeNormal,
				Status: warehouse.StatusInactive,
			}, nil
		},
	}
	router := newWarehouseRouter(service)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/warehouses/1/disable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Data struct {
			Status warehouse.Status `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Data.Status != warehouse.StatusInactive {
		t.Fatalf("expected inactive status, got %q", body.Data.Status)
	}
}

func TestHandlerMapsWarehouseNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		getFunc: func(context.Context, int64) (*warehouse.Warehouse, error) {
			return nil, warehouse.ErrNotFound
		},
	}
	router := newWarehouseRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/warehouses/404", nil)
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
	if body.Code != response.CodeNotFound {
		t.Fatalf("expected response code %d, got %d", response.CodeNotFound, body.Code)
	}
	if body.Message != warehouse.ErrNotFound.Error() {
		t.Fatalf("expected not found message, got %q", body.Message)
	}
}

func newWarehouseRouter(service warehouse.Service) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	warehouse.NewHandler(service).RegisterRoutes(api)

	return router
}

type fakeService struct {
	listFunc    func(context.Context, warehouse.ListFilter) (warehouse.ListResult, error)
	getFunc     func(context.Context, int64) (*warehouse.Warehouse, error)
	createFunc  func(context.Context, warehouse.CreateInput) (*warehouse.Warehouse, error)
	updateFunc  func(context.Context, warehouse.UpdateInput) (*warehouse.Warehouse, error)
	deleteFunc  func(context.Context, int64) error
	disableFunc func(context.Context, int64) (*warehouse.Warehouse, error)
}

func (s *fakeService) List(ctx context.Context, filter warehouse.ListFilter) (warehouse.ListResult, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, filter)
	}
	return warehouse.ListResult{}, nil
}

func (s *fakeService) Get(ctx context.Context, id int64) (*warehouse.Warehouse, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return nil, warehouse.ErrNotFound
}

func (s *fakeService) Create(ctx context.Context, input warehouse.CreateInput) (*warehouse.Warehouse, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Update(ctx context.Context, input warehouse.UpdateInput) (*warehouse.Warehouse, error) {
	if s.updateFunc != nil {
		return s.updateFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Delete(ctx context.Context, id int64) error {
	if s.deleteFunc != nil {
		return s.deleteFunc(ctx, id)
	}
	return nil
}

func (s *fakeService) Disable(ctx context.Context, id int64) (*warehouse.Warehouse, error) {
	if s.disableFunc != nil {
		return s.disableFunc(ctx, id)
	}
	return nil, nil
}
