package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

const (
	DefaultSessionCookieName = "stock_flow_admin_session"
	AppIDHeader              = "X-Stock-Flow-App-ID"
	SecretHeader             = "X-Stock-Flow-Secret"
	CallerGinContextKey      = "auth_caller"
	CodeUnauthorized         = 1002
)

type Middleware interface {
	AdminSession() gin.HandlerFunc
	APIAppSecret() gin.HandlerFunc
	Protected() gin.HandlerFunc
}

type MiddlewareOptions struct {
	SessionCookieName string
}

type authMiddleware struct {
	authenticator Authenticator
	cookieName    string
}

func NewMiddleware(authenticator Authenticator, options MiddlewareOptions) Middleware {
	if options.SessionCookieName == "" {
		options.SessionCookieName = DefaultSessionCookieName
	}
	return &authMiddleware{authenticator: authenticator, cookieName: options.SessionCookieName}
}

func (m *authMiddleware) AdminSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.sessionToken(c)
		hasAPIHeaders := hasAPIAuthHeaders(c)
		if token != "" && hasAPIHeaders {
			abortAuth(c, ErrAmbiguousAuth)
			return
		}
		if token == "" || hasAPIHeaders || m.authenticator == nil {
			abortAuth(c, ErrUnauthorized)
			return
		}

		caller, err := m.authenticator.AuthenticateAdminSession(c.Request.Context(), token)
		if err != nil {
			abortAuth(c, err)
			return
		}
		setCaller(c, caller)
		c.Next()
	}
}

func (m *authMiddleware) APIAppSecret() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.sessionToken(c)
		hasAPIHeaders := hasAPIAuthHeaders(c)
		if token != "" && hasAPIHeaders {
			abortAuth(c, ErrAmbiguousAuth)
			return
		}
		if token != "" || !hasAPIHeaders || m.authenticator == nil {
			abortAuth(c, ErrUnauthorized)
			return
		}

		caller, err := m.authenticateAPIApp(c)
		if err != nil {
			abortAuth(c, err)
			return
		}
		setCaller(c, caller)
		c.Next()
	}
}

func (m *authMiddleware) Protected() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.sessionToken(c)
		hasAPIHeaders := hasAPIAuthHeaders(c)
		if token != "" && hasAPIHeaders {
			abortAuth(c, ErrAmbiguousAuth)
			return
		}
		if m.authenticator == nil {
			abortAuth(c, ErrUnauthorized)
			return
		}

		var (
			caller Caller
			err    error
		)
		switch {
		case token != "":
			caller, err = m.authenticator.AuthenticateAdminSession(c.Request.Context(), token)
		case hasAPIHeaders:
			caller, err = m.authenticateAPIApp(c)
		default:
			err = ErrUnauthorized
		}
		if err != nil {
			abortAuth(c, err)
			return
		}

		setCaller(c, caller)
		c.Next()
	}
}

func (m *authMiddleware) authenticateAPIApp(c *gin.Context) (Caller, error) {
	appID := strings.TrimSpace(c.GetHeader(AppIDHeader))
	secret := c.GetHeader(SecretHeader)
	if appID == "" || secret == "" {
		return Caller{}, ErrUnauthorized
	}
	return m.authenticator.AuthenticateAPIApp(c.Request.Context(), appID, secret)
}

func (m *authMiddleware) sessionToken(c *gin.Context) string {
	token, err := c.Cookie(m.cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(token)
}

func hasAPIAuthHeaders(c *gin.Context) bool {
	return c.GetHeader(AppIDHeader) != "" || c.GetHeader(SecretHeader) != ""
}

func setCaller(c *gin.Context, caller Caller) {
	c.Set(CallerGinContextKey, caller)
	c.Request = c.Request.WithContext(WithCaller(c.Request.Context(), caller))
}

func CallerFromGinContext(c *gin.Context) (Caller, bool) {
	if c == nil {
		return Caller{}, false
	}
	if caller, ok := c.Get(CallerGinContextKey); ok {
		if identity, valid := caller.(Caller); valid {
			return identity, true
		}
	}
	return CallerFromContext(c.Request.Context())
}

func abortAuth(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrAmbiguousAuth):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrSessionExpired), errors.Is(err, ErrAPISecretNotFound):
		response.Error(c, http.StatusUnauthorized, CodeUnauthorized, "authentication failed")
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
	c.Abort()
}
