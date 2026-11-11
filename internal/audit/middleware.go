package audit

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/internal/auth"
	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
	"github.com/Mercer08572/stock-flow/pkg/response"
)

const (
	maxRouteLength     = 256
	maxUserAgentLength = 512
)

type MiddlewareOptions struct {
	SessionCookieName string
	AppIDHeader       string
	SecretHeader      string
	Now               func() time.Time
}

// NewMiddleware captures the request after the handler chain returns. It must
// sit outside the auth middleware so rejected (401/400) and aborted requests are
// recorded too; the caller identity is read from the context the auth middleware
// populated on success.
func NewMiddleware(sink EntrySink, options MiddlewareOptions) gin.HandlerFunc {
	if options.SessionCookieName == "" {
		options.SessionCookieName = auth.DefaultSessionCookieName
	}
	if options.AppIDHeader == "" {
		options.AppIDHeader = auth.AppIDHeader
	}
	if options.SecretHeader == "" {
		options.SecretHeader = auth.SecretHeader
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if sink == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		startedAt := time.Now()
		occurredAt := options.Now()

		c.Next()

		statusCode := c.Writer.Status()
		if statusCode == 0 {
			statusCode = 200
		}
		method := c.Request.Method
		path := truncate(c.Request.URL.Path, maxPathLength)
		if path == "" {
			path = "/"
		}

		entry := Entry{
			OccurredAt:    occurredAt,
			TraceID:       c.GetString(response.TraceIDKey),
			Method:        method,
			Path:          path,
			Route:         truncate(c.FullPath(), maxRouteLength),
			StatusCode:    statusCode,
			LatencyMS:     time.Since(startedAt).Milliseconds(),
			AuthMethod:    detectAuthMethod(c, options),
			ClientIP:      c.ClientIP(),
			UserAgent:     truncate(c.Request.UserAgent(), maxUserAgentLength),
			ErrorCode:     c.GetString(httperr.ErrorCodeKey),
			RequestBytes:  maxInt64(c.Request.ContentLength, 0),
			ResponseBytes: maxInt64(int64(c.Writer.Size()), 0),
		}
		if entry.TraceID == "" {
			// TraceID middleware always sets one, but never write an empty NOT NULL column.
			entry.TraceID = "req_unknown"
		}

		if caller, ok := auth.CallerFromGinContext(c); ok {
			switch {
			case caller.Admin != nil:
				actorType := ActorTypeAdmin
				actorID := caller.Admin.AdminUserID
				entry.ActorType = &actorType
				entry.ActorID = &actorID
				entry.ActorLabel = caller.Admin.Username
			case caller.APIApp != nil:
				actorType := ActorTypeAPIApp
				actorID := caller.APIApp.APIAppID
				entry.ActorType = &actorType
				entry.ActorID = &actorID
				entry.ActorLabel = caller.APIApp.AppID
				entry.CredentialID = caller.APIApp.SecretID
			}
		}

		sink.Enqueue(entry)
	}
}

func detectAuthMethod(c *gin.Context, options MiddlewareOptions) AuthMethod {
	hasSession := false
	if value, err := c.Cookie(options.SessionCookieName); err == nil && strings.TrimSpace(value) != "" {
		hasSession = true
	}
	hasAPIHeaders := strings.TrimSpace(c.GetHeader(options.AppIDHeader)) != "" ||
		strings.TrimSpace(c.GetHeader(options.SecretHeader)) != ""

	switch {
	case hasSession && hasAPIHeaders:
		return AuthMethodAmbiguous
	case hasSession:
		return AuthMethodAdminSession
	case hasAPIHeaders:
		return AuthMethodAPIApp
	default:
		return AuthMethodNone
	}
}

func maxInt64(value int64, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
