package conversion_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/material/conversion"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerCreateConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	service := &fakeService{
		createFunc: func(_ context.Context, input conversion.CreateInput) (*conversion.MaterialUnitConversion, error) {
			if input.MaterialID != 10 {
				t.Fatalf("expected material id 10, got %d", input.MaterialID)
			}
			if input.FromUnitID != 30 || input.ToUnitID != 20 {
				t.Fatalf("expected unit pair 30/20, got %d/%d", input.FromUnitID, input.ToUnitID)
			}
			if input.Factor != "12.5" {
				t.Fatalf("expected factor 12.5, got %q", input.Factor)
			}

			return &conversion.MaterialUnitConversion{
				ID:         1,
				MaterialID: input.MaterialID,
				FromUnitID: input.FromUnitID,
				ToUnitID:   input.ToUnitID,
				Factor:     input.Factor,
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
	}
	router := newConversionRouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/10/unit-conversions", bytes.NewBufferString(`{
		"from_unit_id":30,
		"to_unit_id":20,
		"factor":12.5
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
			ID     int64  `json:"id"`
			Factor string `json:"factor"`
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
	if body.Data.ID != 1 || body.Data.Factor != "12.5" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.TraceID != "req_create" {
		t.Fatalf("expected trace id req_create, got %q", body.TraceID)
	}
}

func TestHandlerMapsReversePairConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		createFunc: func(context.Context, conversion.CreateInput) (*conversion.MaterialUnitConversion, error) {
			return nil, conversion.ErrReversePair
		},
	}
	router := newConversionRouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/10/unit-conversions", bytes.NewBufferString(`{
		"from_unit_id":30,
		"to_unit_id":20,
		"factor":"12"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != response.CodeConflict {
		t.Fatalf("expected response code %d, got %d", response.CodeConflict, body.Code)
	}
	if body.Message != conversion.ErrReversePair.Error() {
		t.Fatalf("expected reverse pair message, got %q", body.Message)
	}
}

func newConversionRouter(service conversion.Service) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	conversion.NewHandler(service).RegisterRoutes(api)

	return router
}

type fakeService struct {
	listFunc   func(context.Context, conversion.ListFilter) (conversion.ListResult, error)
	getFunc    func(context.Context, int64, int64) (*conversion.MaterialUnitConversion, error)
	createFunc func(context.Context, conversion.CreateInput) (*conversion.MaterialUnitConversion, error)
	updateFunc func(context.Context, conversion.UpdateInput) (*conversion.MaterialUnitConversion, error)
	deleteFunc func(context.Context, int64, int64) error
}

func (s *fakeService) List(ctx context.Context, filter conversion.ListFilter) (conversion.ListResult, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, filter)
	}
	return conversion.ListResult{}, nil
}

func (s *fakeService) Get(ctx context.Context, materialID int64, id int64) (*conversion.MaterialUnitConversion, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, materialID, id)
	}
	return nil, conversion.ErrNotFound
}

func (s *fakeService) Create(ctx context.Context, input conversion.CreateInput) (*conversion.MaterialUnitConversion, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Update(ctx context.Context, input conversion.UpdateInput) (*conversion.MaterialUnitConversion, error) {
	if s.updateFunc != nil {
		return s.updateFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Delete(ctx context.Context, materialID int64, id int64) error {
	if s.deleteFunc != nil {
		return s.deleteFunc(ctx, materialID, id)
	}
	return nil
}
