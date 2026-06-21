package material_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/material/material"
	"github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestHandlerCreateMaterial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	service := &fakeService{
		createFunc: func(_ context.Context, input material.CreateInput) (*material.Material, error) {
			if input.Code != "M-001" {
				t.Fatalf("expected code M-001, got %q", input.Code)
			}

			return &material.Material{
				ID:         1,
				Code:       input.Code,
				Name:       input.Name,
				CategoryID: input.CategoryID,
				BaseUnitID: input.BaseUnitID,
				Status:     material.StatusActive,
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		},
	}
	router := newMaterialRouter(service)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials", bytes.NewBufferString(`{
		"code":"M-001",
		"name":"Steel plate",
		"category_id":10,
		"base_unit_id":20
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
	if body.Data.ID != 1 || body.Data.Code != "M-001" {
		t.Fatalf("unexpected response data: %#v", body.Data)
	}
	if body.TraceID != "req_create" {
		t.Fatalf("expected trace id req_create, got %q", body.TraceID)
	}
}

func TestHandlerMapsMaterialNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeService{
		getFunc: func(context.Context, int64) (*material.Material, error) {
			return nil, material.ErrNotFound
		},
	}
	router := newMaterialRouter(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/materials/404", nil)
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
	if body.Message != material.ErrNotFound.Error() {
		t.Fatalf("expected not found message, got %q", body.Message)
	}
}

func newMaterialRouter(service material.Service) *gin.Engine {
	router := gin.New()
	router.Use(middleware.TraceID())
	api := router.Group("/api/v1")
	material.NewHandler(service).RegisterRoutes(api)

	return router
}

type fakeService struct {
	listFunc   func(context.Context, material.ListFilter) (material.ListResult, error)
	getFunc    func(context.Context, int64) (*material.Material, error)
	createFunc func(context.Context, material.CreateInput) (*material.Material, error)
	updateFunc func(context.Context, material.UpdateInput) (*material.Material, error)
	deleteFunc func(context.Context, int64) error
}

func (s *fakeService) List(ctx context.Context, filter material.ListFilter) (material.ListResult, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, filter)
	}
	return material.ListResult{}, nil
}

func (s *fakeService) Get(ctx context.Context, id int64) (*material.Material, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return nil, material.ErrNotFound
}

func (s *fakeService) Create(ctx context.Context, input material.CreateInput) (*material.Material, error) {
	if s.createFunc != nil {
		return s.createFunc(ctx, input)
	}
	return nil, nil
}

func (s *fakeService) Update(ctx context.Context, input material.UpdateInput) (*material.Material, error) {
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
