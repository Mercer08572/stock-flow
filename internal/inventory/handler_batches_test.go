package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
)

type fakeBatchService struct {
	listFunc    func(context.Context, inventory.ListBatchesFilter) (inventory.BatchListResult, error)
	getFunc     func(context.Context, int64) (*inventory.BatchDetail, error)
	createFunc  func(context.Context, inventory.CreateBatchInput) (*inventory.Batch, error)
	updateFunc  func(context.Context, inventory.UpdateBatchInput) (*inventory.Batch, error)
	disableFunc func(context.Context, int64) (*inventory.Batch, error)
	deleteFunc  func(context.Context, int64) error
}

func (s *fakeBatchService) ListBatches(ctx context.Context, filter inventory.ListBatchesFilter) (inventory.BatchListResult, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, filter)
	}
	return inventory.BatchListResult{}, nil
}

func (s *fakeBatchService) GetBatch(ctx context.Context, id int64) (*inventory.BatchDetail, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return nil, nil
}

func (s *fakeBatchService) CreateBatch(ctx context.Context, input inventory.CreateBatchInput) (*inventory.Batch, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeBatchService) UpdateBatch(ctx context.Context, input inventory.UpdateBatchInput) (*inventory.Batch, error) {
	if s.updateFunc != nil {
		return s.updateFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeBatchService) DisableBatch(ctx context.Context, id int64) (*inventory.Batch, error) {
	if s.disableFunc != nil {
		return s.disableFunc(ctx, id)
	}
	return nil, nil
}

func (s *fakeBatchService) DeleteBatch(ctx context.Context, id int64) error {
	if s.deleteFunc != nil {
		return s.deleteFunc(ctx, id)
	}
	return nil
}

func newBatchRouter(service inventory.BatchService) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	inventory.NewBatchHandler(service).RegisterRoutes(api)
	return router
}

func TestBatchHandlerListPassesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{
		listFunc: func(_ context.Context, filter inventory.ListBatchesFilter) (inventory.BatchListResult, error) {
			if filter.SKUID == nil || *filter.SKUID != 7 {
				t.Fatalf("expected sku_id 7, got %v", filter.SKUID)
			}
			if filter.Status == nil || *filter.Status != inventory.BatchStatusInactive {
				t.Fatalf("expected status inactive, got %v", filter.Status)
			}
			if filter.Keyword == nil || *filter.Keyword != "B-2" {
				t.Fatalf("expected keyword B-2, got %v", filter.Keyword)
			}
			if filter.Limit != 10 || filter.Offset != 20 {
				t.Fatalf("expected limit/offset 10/20, got %d/%d", filter.Limit, filter.Offset)
			}
			return inventory.BatchListResult{
				Items:  []inventory.Batch{{ID: 1, BatchNo: "B-2", Status: inventory.BatchStatusInactive}},
				Total:  1,
				Limit:  10,
				Offset: 20,
			}, nil
		},
	}
	router := newBatchRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/batches?sku_id=7&status=inactive&q=B-2&limit=10&offset=20", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data inventory.BatchListResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Total != 1 || len(body.Data.Items) != 1 {
		t.Fatalf("unexpected list result: %+v", body.Data)
	}
}

func TestBatchHandlerCreateReturns201(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{
		createFunc: func(_ context.Context, input inventory.CreateBatchInput) (*inventory.Batch, error) {
			if input.SKUID != 7 || input.BatchNo != "B-1" {
				t.Fatalf("unexpected create input: %+v", input)
			}
			if input.DispatchPriority == nil || *input.DispatchPriority != 3 {
				t.Fatalf("expected priority 3, got %v", input.DispatchPriority)
			}
			return &inventory.Batch{ID: 5, SKUID: 7, BatchNo: "B-1", Status: inventory.BatchStatusActive}, nil
		},
	}
	router := newBatchRouter(service)
	body := `{"sku_id":7,"batch_no":"B-1","dispatch_priority":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBatchHandlerGetMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{
		getFunc: func(context.Context, int64) (*inventory.BatchDetail, error) {
			return nil, inventory.ErrBatchNotFound
		},
	}
	router := newBatchRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/batches/42", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ErrorCode != "INVENTORY_BATCH_NOT_FOUND" {
		t.Fatalf("expected batch not found code, got %q", body.ErrorCode)
	}
}

func TestBatchHandlerDeleteMapsReferencedConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{
		deleteFunc: func(context.Context, int64) error {
			return inventory.ErrBatchReferenced
		},
	}
	router := newBatchRouter(service)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/inventory/batches/9", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ErrorCode != "INVENTORY_BATCH_REFERENCED" {
		t.Fatalf("expected referenced code, got %q", body.ErrorCode)
	}
}

func TestBatchHandlerRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{}
	router := newBatchRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/batches/0", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBatchHandlerDeleteReturnsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeBatchService{
		deleteFunc: func(context.Context, int64) error { return nil },
	}
	router := newBatchRouter(service)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/inventory/batches/9", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
