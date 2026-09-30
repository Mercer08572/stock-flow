package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	sharedmiddleware "github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestHandlerAdminLoginSetsSessionCookie(t *testing.T) {
	service, router := newHandlerTestRouter(t)
	_ = service

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/admin/login", bytes.NewBufferString(`{
		"username":"admin",
		"password":"admin-test-password"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != DefaultSessionCookieName || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Value == "" {
		t.Fatalf("unexpected login cookie: %+v", cookies)
	}
	if strings.Contains(rec.Body.String(), "admin-test-password") || strings.Contains(rec.Body.String(), "password_hash") || strings.Contains(rec.Body.String(), "sess_") {
		t.Fatalf("login response exposed sensitive data: %s", rec.Body.String())
	}

	var body struct {
		Data LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if body.Data.Admin.Username != "admin" {
		t.Fatalf("unexpected admin: %+v", body.Data.Admin)
	}
}

func TestHandlerAdminLoginRejectsInvalidCredentials(t *testing.T) {
	_, router := newHandlerTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/admin/login", bytes.NewBufferString(`{
		"username":"admin",
		"password":"wrong"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Message   string `json:"message"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	// 对外文案保持历史上的扁平化写法，具体原因靠业务码区分。
	if body.Message != "authentication failed" {
		t.Fatalf("expected flattened message, got %q", body.Message)
	}
	if body.ErrorCode != string(apperr.CodeAuthInvalidCredentials) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeAuthInvalidCredentials, body.ErrorCode)
	}
}

func TestHandlerAdminLoginRateLimitedSetsRetryAfter(t *testing.T) {
	fixed := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	_, router := newHandlerTestRouterWithOptions(t, ServiceOptions{
		Now:                func() time.Time { return fixed },
		Random:             &sequenceRandom{values: []string{"handler-session"}},
		LoginFailurePolicy: RateLimitPolicy{MaxAttempts: 1, Window: time.Minute, Lockout: 90 * time.Second},
	})

	login := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/admin/login", bytes.NewBufferString(`{
			"username":"admin",
			"password":"wrong"
		}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := login(); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected first attempt to be 401, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := login()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("expected Retry-After %q, got %q", "90", got)
	}

	var body struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.ErrorCode != string(apperr.CodeAuthRateLimited) {
		t.Fatalf("expected error code %q, got %q", apperr.CodeAuthRateLimited, body.ErrorCode)
	}
}

func newHandlerTestRouter(t *testing.T) (Service, *gin.Engine) {
	t.Helper()
	return newHandlerTestRouterWithOptions(t, ServiceOptions{
		Random: &sequenceRandom{values: []string{"handler-session"}},
	})
}

func newHandlerTestRouterWithOptions(t *testing.T, options ServiceOptions) (Service, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hasher := testPasswordHasher()
	encoded, err := hasher.Hash("admin-test-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &fakeRepository{admin: &AdminUser{ID: 1, Username: "admin", PasswordHash: encoded, PasswordInitialized: true, Status: StatusActive}}
	service := NewService(repo, NewInMemorySessionStore(), hasher, options)
	middleware := NewMiddleware(service, MiddlewareOptions{})
	router := gin.New()
	router.Use(sharedmiddleware.TraceID())
	api := router.Group("/api/v1")
	NewHandler(service, CookieOptions{}).RegisterRoutes(api, middleware.AdminSession(), middleware.AdminSessionAllowPasswordChange())
	return service, router
}
