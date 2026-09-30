package sku_test

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
	"github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerCreateSKU(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeService{
		createFunc: func(_ context.Context, input sku.CreateInput) (*sku.SKU, error) {
			if input.Code != "SKU-001" {
				t.Fatalf("expected code SKU-001, got %q", input.Code)
			}
			if input.MaterialID != 10 || input.UnitID != 20 {
				t.Fatalf("expected material/unit 10/20, got %d/%d", input.MaterialID, input.UnitID)
			}

			return &sku.SKU{
				ID:         1,
				MaterialID: input.MaterialID,
				Code:       input.Code,
				Name:       input.Name,
				UnitID:     input.UnitID,
				Status:     sku.StatusActive,
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
	}
	router := newSKURouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/skus", bytes.NewBufferString(`{
		"material_id":10,
		"code":"SKU-001",
		"name":"Steel plate pcs",
		"unit_id":20,
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
	if body.Data.ID != 1 || body.Data.Code != "SKU-001" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.TraceID != "req_create" {
		t.Fatalf("expected trace id req_create, got %q", body.TraceID)
	}
}

func TestHandlerMapsActiveSKUConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		createFunc: func(context.Context, sku.CreateInput) (*sku.SKU, error) {
			return nil, sku.ErrActiveSKUForMaterial
		},
	}
	router := newSKURouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/skus", bytes.NewBufferString(`{
		"material_id":10,
		"code":"SKU-001",
		"name":"Steel plate pcs",
		"unit_id":20,
		"status":"active"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var body struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeConflict {
		t.Fatalf("expected response code %d, got %d", response.CodeConflict, body.Code)
	}
	if body.Message != sku.ErrActiveSKUForMaterial.Error() {
		t.Fatalf("expected conflict message, got %q", body.Message)
	}
	if body.ErrorCode != string(apperr.CodeSKUActiveExistsForMaterial) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeSKUActiveExistsForMaterial, body.ErrorCode)
	}
}

func TestHandlerMapsInventoryReferenceConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeService{
		deleteFunc: func(context.Context, int64) error {
			return sku.ErrReferencedByInventory
		},
	}
	router := newSKURouter(service)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/skus/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	var body struct {
		Code      int    `json:"code"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeConflict {
		t.Fatalf("expected response code %d, got %d", response.CodeConflict, body.Code)
	}
	if body.ErrorCode != string(apperr.CodeSKUReferencedByInventory) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeSKUReferencedByInventory, body.ErrorCode)
	}
}

func newSKURouter(service sku.Service) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	sku.NewHandler(service).RegisterRoutes(api)

	return router
}

type fakeService struct {
	listFunc   func(context.Context, sku.ListFilter) (sku.ListResult, error)
	getFunc    func(context.Context, int64) (*sku.SKU, error)
	createFunc func(context.Context, sku.CreateInput) (*sku.SKU, error)
	updateFunc func(context.Context, sku.UpdateInput) (*sku.SKU, error)
	deleteFunc func(context.Context, int64) error
}

func (s *fakeService) List(ctx context.Context, filter sku.ListFilter) (sku.ListResult, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, filter)
	}
	return sku.ListResult{}, nil
}

func (s *fakeService) Get(ctx context.Context, id int64) (*sku.SKU, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return nil, sku.ErrNotFound
}

func (s *fakeService) GetReference(context.Context, int64) (*sku.Reference, error) {
	return nil, sku.ErrNotFound
}

func (s *fakeService) Create(ctx context.Context, input sku.CreateInput) (*sku.SKU, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Update(ctx context.Context, input sku.UpdateInput) (*sku.SKU, error) {
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
