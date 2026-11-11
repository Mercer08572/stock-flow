package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestHandlerAPISecretLifecycle(t *testing.T) {
	_, router := newHandlerTestRouterWithOptions(t, ServiceOptions{
		Random: &sequenceRandom{values: []string{
			"handler-session",
			"app-id",
			"secret-id", "plain-secret",
			"reissue-id", "reissue-secret",
			"blocked-id", "blocked-secret",
		}},
	})
	cookie := loginAdminCookie(t, router)

	app := createAPIApp(t, router, cookie, `{"name":"WMS","status":"active"}`)
	secret := issueAPISecret(t, router, cookie, app.ID, `{"name":"primary"}`)
	if secret.Secret == "" || secret.SecretID == "" {
		t.Fatalf("expected issued secret, got %+v", secret)
	}

	// Update keeps the secret usable and changes only the display name.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/apps/1/secrets/"+secret.SecretID, strings.NewReader(`{"name":"renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating secret, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated struct {
		Data APISecret `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal updated secret: %v", err)
	}
	if updated.Data.Name != "renamed" || updated.Data.Status != SecretStatusActive {
		t.Fatalf("unexpected updated secret: %+v", updated.Data)
	}
	if strings.Contains(rec.Body.String(), "secret_hash") {
		t.Fatalf("update response exposed the secret hash: %s", rec.Body.String())
	}

	// Block then unblock through the admin API.
	blockRec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps/1/secrets/"+secret.SecretID+"/block", "")
	if blockRec.Code != http.StatusOK || !strings.Contains(blockRec.Body.String(), `"status":"blocked"`) {
		t.Fatalf("expected blocked secret, got %d: %s", blockRec.Code, blockRec.Body.String())
	}
	unblockRec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps/1/secrets/"+secret.SecretID+"/unblock", "")
	if unblockRec.Code != http.StatusOK || !strings.Contains(unblockRec.Body.String(), `"status":"active"`) {
		t.Fatalf("expected unblocked secret, got %d: %s", unblockRec.Code, unblockRec.Body.String())
	}

	// Reissue creates a new secret and blocks the old one in one request.
	reissueRec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps/1/secrets/"+secret.SecretID+"/reissue", `{"name":"rotated"}`)
	if reissueRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 reissuing secret, got %d: %s", reissueRec.Code, reissueRec.Body.String())
	}
	var reissued struct {
		Data IssuedSecret `json:"data"`
	}
	if err := json.Unmarshal(reissueRec.Body.Bytes(), &reissued); err != nil {
		t.Fatalf("unmarshal reissued secret: %v", err)
	}
	if reissued.Data.Secret == "" || reissued.Data.SecretID == secret.SecretID {
		t.Fatalf("expected a distinct replacement secret, got %+v", reissued.Data)
	}
	if strings.Contains(reissueRec.Body.String(), "secret_hash") {
		t.Fatalf("reissue response exposed the secret hash: %s", reissueRec.Body.String())
	}

	// The old secret is blocked, so reissuing it again must fail with 409.
	againRec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps/1/secrets/"+secret.SecretID+"/reissue", `{"name":"again"}`)
	if againRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 reissuing a blocked secret, got %d: %s", againRec.Code, againRec.Body.String())
	}

	// Delete removes it from the list.
	deleteRec := doAdminRequest(t, router, cookie, http.MethodDelete, "/api/v1/auth/apps/1/secrets/"+secret.SecretID, "")
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting secret, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	listRec := doAdminRequest(t, router, cookie, http.MethodGet, "/api/v1/auth/apps/1/secrets", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing secrets, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), `"secret_id":"`+secret.SecretID+`"`) {
		t.Fatalf("deleted secret still listed: %s", listRec.Body.String())
	}
}

func TestHandlerSecretRoutesRequireAdminSession(t *testing.T) {
	_, router := newHandlerTestRouter(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/apps/1/secrets/key_1", strings.NewReader(`{"name":"renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without an admin session, got %d: %s", rec.Code, rec.Body.String())
	}
}

func loginAdminCookie(t *testing.T, router *gin.Engine) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/admin/login", strings.NewReader(`{"username":"admin","password":"admin-test-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %+v", cookies)
	}
	return cookies[0]
}

func createAPIApp(t *testing.T, router *gin.Engine, cookie *http.Cookie, body string) APIApp {
	t.Helper()
	rec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api app: %d: %s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Data APIApp `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal created api app: %v", err)
	}
	return parsed.Data
}

func issueAPISecret(t *testing.T, router *gin.Engine, cookie *http.Cookie, appID int64, body string) IssuedSecret {
	t.Helper()
	rec := doAdminRequest(t, router, cookie, http.MethodPost, "/api/v1/auth/apps/"+strconv.FormatInt(appID, 10)+"/secrets", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue api secret: %d: %s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Data IssuedSecret `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal issued api secret: %v", err)
	}
	return parsed.Data
}

func doAdminRequest(t *testing.T, router *gin.Engine, cookie *http.Cookie, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
