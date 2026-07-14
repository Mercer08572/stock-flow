package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProtectedMiddlewareAuthenticationModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authenticator := &fakeAuthenticator{}
	middleware := NewMiddleware(authenticator, MiddlewareOptions{})

	tests := []struct {
		name       string
		configure  func(*http.Request)
		wantStatus int
		wantType   CallerType
	}{
		{
			name: "admin session",
			configure: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: DefaultSessionCookieName, Value: "valid-session"})
			},
			wantStatus: http.StatusOK,
			wantType:   CallerTypeAdmin,
		},
		{
			name: "api app secret",
			configure: func(req *http.Request) {
				req.Header.Set(AppIDHeader, "app_external")
				req.Header.Set(SecretHeader, "secret")
			},
			wantStatus: http.StatusOK,
			wantType:   CallerTypeAPIApp,
		},
		{
			name:       "missing credentials",
			configure:  func(*http.Request) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "ambiguous credentials",
			configure: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: DefaultSessionCookieName, Value: "valid-session"})
				req.Header.Set(AppIDHeader, "app_external")
				req.Header.Set(SecretHeader, "secret")
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.Protected())
			router.GET("/protected", func(c *gin.Context) {
				caller, ok := CallerFromContext(c.Request.Context())
				if !ok {
					c.Status(http.StatusInternalServerError)
					return
				}
				c.JSON(http.StatusOK, gin.H{"type": caller.Type})
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			tt.configure(req)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK && !containsJSONType(rec.Body.String(), tt.wantType) {
				t.Fatalf("expected caller type %q, got %s", tt.wantType, rec.Body.String())
			}
		})
	}
}

func TestProtectedMiddlewareRejectsAdminRequiringPasswordChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware := NewMiddleware(forcedPasswordChangeAuthenticator{}, MiddlewareOptions{})
	router := gin.New()
	router.Use(middleware.Protected())
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: DefaultSessionCookieName, Value: "valid-session"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

type forcedPasswordChangeAuthenticator struct{}

func (forcedPasswordChangeAuthenticator) AuthenticateAdminSession(context.Context, string) (Caller, error) {
	return Caller{Type: CallerTypeAdmin, Admin: &AdminCaller{AdminUserID: 1, Username: "admin", MustChangePassword: true}}, nil
}
func (forcedPasswordChangeAuthenticator) AuthenticateAPIApp(context.Context, string, string) (Caller, error) {
	return Caller{}, ErrUnauthorized
}

type fakeAuthenticator struct{}

func (a *fakeAuthenticator) AuthenticateAdminSession(context.Context, string) (Caller, error) {
	return Caller{Type: CallerTypeAdmin, Admin: &AdminCaller{AdminUserID: 1, Username: "admin"}}, nil
}

func (a *fakeAuthenticator) AuthenticateAPIApp(context.Context, string, string) (Caller, error) {
	return Caller{Type: CallerTypeAPIApp, APIApp: &APIAppCaller{APIAppID: 2, AppID: "app_external", SecretRecordID: 3, SecretID: "key_primary"}}, nil
}

func containsJSONType(body string, callerType CallerType) bool {
	return body == `{"type":"`+string(callerType)+`"}`
}
