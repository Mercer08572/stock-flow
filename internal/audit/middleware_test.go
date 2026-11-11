package audit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/auth"
	sharedmiddleware "github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

func TestMiddlewareCapturesAdminSessionRequest(t *testing.T) {
	router, sink, fixed := newMiddlewareTestRouter(t, func(c *gin.Context) {
		caller := auth.Caller{
			Type:  auth.CallerTypeAdmin,
			Admin: &auth.AdminCaller{AdminUserID: 7, Username: "alice"},
		}
		c.Request = c.Request.WithContext(auth.WithCaller(c.Request.Context(), caller))
		response.Success(c, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/widgets?ignored=1", strings.NewReader("body"))
	req.Header.Set("X-Trace-ID", "req_middleware_admin")
	req.Header.Set("User-Agent", "test-agent")
	req.AddCookie(&http.Cookie{Name: auth.DefaultSessionCookieName, Value: "session-token"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entry := sink.last(t)
	if entry.TraceID != "req_middleware_admin" {
		t.Fatalf("unexpected trace id %q", entry.TraceID)
	}
	if entry.Method != http.MethodPost || entry.Path != "/api/v1/widgets" || entry.Route != "/api/v1/widgets" {
		t.Fatalf("unexpected request identity: %+v", entry)
	}
	if entry.StatusCode != http.StatusOK || entry.AuthMethod != AuthMethodAdminSession {
		t.Fatalf("unexpected status/auth method: %+v", entry)
	}
	if entry.ActorType == nil || *entry.ActorType != ActorTypeAdmin || entry.ActorID == nil || *entry.ActorID != 7 {
		t.Fatalf("unexpected actor: %+v", entry)
	}
	if entry.ActorLabel != "alice" || entry.CredentialID != "" {
		t.Fatalf("unexpected actor label/credential: %+v", entry)
	}
	if entry.ClientIP == "" || entry.UserAgent != "test-agent" {
		t.Fatalf("unexpected client metadata: %+v", entry)
	}
	if entry.RequestBytes != 4 || entry.ResponseBytes == 0 {
		t.Fatalf("unexpected byte counts: %+v", entry)
	}
	if !entry.OccurredAt.Equal(fixed) {
		t.Fatalf("expected fixed occurred_at, got %s", entry.OccurredAt)
	}
}

func TestMiddlewareCapturesAPIAppCaller(t *testing.T) {
	router, sink, _ := newMiddlewareTestRouter(t, func(c *gin.Context) {
		caller := auth.Caller{
			Type: auth.CallerTypeAPIApp,
			APIApp: &auth.APIAppCaller{
				APIAppID: 9, AppID: "app_test", SecretRecordID: 11, SecretID: "key_test",
			},
		}
		c.Request = c.Request.WithContext(auth.WithCaller(c.Request.Context(), caller))
		response.Success(c, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stocks", nil)
	req.Header.Set(auth.AppIDHeader, "app_test")
	req.Header.Set(auth.SecretHeader, "sfsec_secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entry := sink.last(t)
	if entry.AuthMethod != AuthMethodAPIApp {
		t.Fatalf("expected api_app auth method, got %q", entry.AuthMethod)
	}
	if entry.ActorType == nil || *entry.ActorType != ActorTypeAPIApp || entry.ActorID == nil || *entry.ActorID != 9 {
		t.Fatalf("unexpected actor: %+v", entry)
	}
	if entry.ActorLabel != "app_test" || entry.CredentialID != "key_test" {
		t.Fatalf("unexpected app identity: %+v", entry)
	}
}

func TestMiddlewareRecordsAmbiguousAndUnauthenticatedRequests(t *testing.T) {
	t.Run("ambiguous", func(t *testing.T) {
		router, sink, _ := newMiddlewareTestRouter(t, func(c *gin.Context) {
			response.Success(c, gin.H{"ok": true})
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stocks", nil)
		req.AddCookie(&http.Cookie{Name: auth.DefaultSessionCookieName, Value: "session-token"})
		req.Header.Set(auth.AppIDHeader, "app_test")
		req.Header.Set(auth.SecretHeader, "sfsec_secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		entry := sink.last(t)
		if entry.AuthMethod != AuthMethodAmbiguous || entry.ActorType != nil {
			t.Fatalf("unexpected ambiguous entry: %+v", entry)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		router, sink, _ := newMiddlewareTestRouter(t, func(c *gin.Context) {
			httperr.Write(c, auth.ErrUnauthorized)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/stocks", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
		entry := sink.last(t)
		if entry.StatusCode != http.StatusUnauthorized || entry.AuthMethod != AuthMethodNone {
			t.Fatalf("unexpected unauthenticated entry: %+v", entry)
		}
		if entry.ErrorCode != "AUTH_REQUIRED" {
			t.Fatalf("expected AUTH_REQUIRED error code, got %q", entry.ErrorCode)
		}
	})
}

func TestMiddlewareRecordsPanicsAsServerErrors(t *testing.T) {
	router, sink, _ := newMiddlewareTestRouter(t, func(*gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	entry := sink.last(t)
	if entry.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 audit status, got %d", entry.StatusCode)
	}
}

func newMiddlewareTestRouter(t *testing.T, handler gin.HandlerFunc) (*gin.Engine, *captureSink, time.Time) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	fixed := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	sink := &captureSink{}
	router := gin.New()
	router.Use(sharedmiddleware.TraceID())
	router.Use(NewMiddleware(sink, MiddlewareOptions{Now: func() time.Time { return fixed }}))
	router.Use(gin.Recovery())
	router.POST("/api/v1/widgets", handler)
	router.GET("/api/v1/inventory/stocks", handler)
	router.GET("/api/v1/panic", handler)
	return router, sink, fixed
}
