package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	sharedmiddleware "github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
)

type fakeAuditService struct {
	filter      ListFilter
	result      ListResult
	err         error
	stats       Stats
	statsError  error
	statsFilter StatsFilter
}

func (s *fakeAuditService) List(_ context.Context, filter ListFilter) (ListResult, error) {
	s.filter = filter
	return s.result, s.err
}

func (s *fakeAuditService) Stats(_ context.Context, filter StatsFilter) (Stats, error) {
	s.statsFilter = filter
	return s.stats, s.statsError
}

func TestHandlerListParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeAuditService{result: ListResult{Items: []Log{{
		ID: 1, OccurredAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		TraceID: "req_1", Method: "GET", Path: "/api/v1/inventory/stocks",
		StatusCode: 200, AuthMethod: AuthMethodAPIApp,
	}}}}
	router := gin.New()
	router.Use(sharedmiddleware.TraceID())
	api := router.Group("/api/v1")
	NewHandler(service).RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/logs?from=2026-11-10T00:00:00Z&to=2026-11-10T12:00:00Z"+
			"&auth_method=api_app&actor_type=api_app&actor_id=9&actor_label=app_test"+
			"&client_ip=127.0.0.1&status_code=200&path=/api/v1/inventory/stocks"+
			"&trace_id=req_trace&limit=20&cursor=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	filter := service.filter
	if filter.From.IsZero() || filter.To.IsZero() {
		t.Fatalf("expected parsed time range, got %+v", filter)
	}
	if filter.AuthMethod == nil || *filter.AuthMethod != AuthMethodAPIApp {
		t.Fatalf("unexpected auth method: %+v", filter.AuthMethod)
	}
	if filter.ActorType == nil || *filter.ActorType != ActorTypeAPIApp {
		t.Fatalf("unexpected actor type: %+v", filter.ActorType)
	}
	if filter.ActorID == nil || *filter.ActorID != 9 {
		t.Fatalf("unexpected actor id: %+v", filter.ActorID)
	}
	if filter.ActorLabel != "app_test" || filter.ClientIP != "127.0.0.1" {
		t.Fatalf("unexpected text filters: %+v", filter)
	}
	if filter.StatusCode == nil || *filter.StatusCode != 200 || filter.Limit != 20 {
		t.Fatalf("unexpected status/limit: %+v", filter)
	}
	if filter.Path != "/api/v1/inventory/stocks" || filter.Cursor != "abc" {
		t.Fatalf("unexpected path/cursor: %+v", filter)
	}
	if filter.TraceID != "req_trace" {
		t.Fatalf("unexpected trace id: %q", filter.TraceID)
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Items []Log `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != 200 || len(body.Data.Items) != 1 || body.Data.Items[0].TraceID != "req_1" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestHandlerListRejectsMalformedQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeAuditService{}
	router := gin.New()
	api := router.Group("/api/v1")
	NewHandler(service).RegisterRoutes(api)

	tests := []string{
		"/api/v1/audit/logs?from=not-a-time",
		"/api/v1/audit/logs?to=not-a-time",
		"/api/v1/audit/logs?actor_id=abc",
		"/api/v1/audit/logs?status_code=abc",
		"/api/v1/audit/logs?limit=abc",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}
