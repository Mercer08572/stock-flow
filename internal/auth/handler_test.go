package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	sharedmiddleware "github.com/Mercer08572/stock-flow/internal/shared/http/middleware"
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
}

func newHandlerTestRouter(t *testing.T) (Service, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hasher := testPasswordHasher()
	encoded, err := hasher.Hash("admin-test-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &fakeRepository{admin: &AdminUser{ID: 1, Username: "admin", PasswordHash: encoded, PasswordInitialized: true, Status: StatusActive}}
	service := NewService(repo, NewInMemorySessionStore(), hasher, ServiceOptions{
		Random: &sequenceRandom{values: []string{"handler-session"}},
	})
	middleware := NewMiddleware(service, MiddlewareOptions{})
	router := gin.New()
	router.Use(sharedmiddleware.TraceID())
	api := router.Group("/api/v1")
	NewHandler(service, CookieOptions{}).RegisterRoutes(api, middleware.AdminSession(), middleware.AdminSessionAllowPasswordChange())
	return service, router
}
