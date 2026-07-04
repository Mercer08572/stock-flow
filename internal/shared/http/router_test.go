package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/material/conversion"
	material "github.com/Mercer08572/stock-flow/internal/material/material"
	httpserver "github.com/Mercer08572/stock-flow/internal/shared/http"
)

func TestHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := httpserver.NewRouter(httpserver.Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Trace-ID", "req_test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Code      int             `json:"code"`
		Message   string          `json:"message"`
		Data      json.RawMessage `json:"data"`
		TraceID   string          `json:"trace_id"`
		Timestamp int64           `json:"timestamp"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Code != http.StatusOK {
		t.Fatalf("expected response code %d, got %d", http.StatusOK, body.Code)
	}
	if body.Message != "success" {
		t.Fatalf("expected message success, got %q", body.Message)
	}
	if body.TraceID != "req_test" {
		t.Fatalf("expected trace id req_test, got %q", body.TraceID)
	}
	if body.Timestamp == 0 {
		t.Fatal("expected timestamp to be set")
	}
}

func TestMaterialConversionRouteRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := httpserver.NewRouter(httpserver.Dependencies{
		MaterialService:   &fakeMaterialService{},
		ConversionService: &fakeConversionService{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/materials/10/unit-conversions", bytes.NewBufferString(`{
		"from_unit_id":30,
		"to_unit_id":20,
		"factor":"12"
	}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var body struct {
		Data struct {
			MaterialID int64 `json:"material_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Data.MaterialID != 10 {
		t.Fatalf("expected material id 10, got %d", body.Data.MaterialID)
	}
}

type fakeMaterialService struct{}

func (s *fakeMaterialService) List(context.Context, material.ListFilter) (material.ListResult, error) {
	return material.ListResult{}, nil
}

func (s *fakeMaterialService) Get(context.Context, int64) (*material.Material, error) {
	return nil, material.ErrNotFound
}

func (s *fakeMaterialService) Create(context.Context, material.CreateInput) (*material.Material, error) {
	return nil, nil
}

func (s *fakeMaterialService) Update(context.Context, material.UpdateInput) (*material.Material, error) {
	return nil, nil
}

func (s *fakeMaterialService) Delete(context.Context, int64) error {
	return nil
}

func (s *fakeMaterialService) ValidateSKUUnit(context.Context, int64, int64) error {
	return nil
}

type fakeConversionService struct{}

func (s *fakeConversionService) List(context.Context, conversion.ListFilter) (conversion.ListResult, error) {
	return conversion.ListResult{}, nil
}

func (s *fakeConversionService) Get(context.Context, int64, int64) (*conversion.MaterialUnitConversion, error) {
	return nil, conversion.ErrNotFound
}

func (s *fakeConversionService) Create(_ context.Context, input conversion.CreateInput) (*conversion.MaterialUnitConversion, error) {
	return &conversion.MaterialUnitConversion{
		ID:         1,
		MaterialID: input.MaterialID,
		FromUnitID: input.FromUnitID,
		ToUnitID:   input.ToUnitID,
		Factor:     input.Factor,
	}, nil
}

func (s *fakeConversionService) Update(context.Context, conversion.UpdateInput) (*conversion.MaterialUnitConversion, error) {
	return nil, nil
}

func (s *fakeConversionService) Delete(context.Context, int64, int64) error {
	return nil
}
