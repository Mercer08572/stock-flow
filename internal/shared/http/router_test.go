package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
