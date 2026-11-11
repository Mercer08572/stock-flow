// Package audit records one row per API request and exposes a cursor-based query API.
//
// The write path is deliberately asynchronous: request handlers only enqueue a
// small immutable Entry, while a background writer batches entries and inserts
// them with pgx.CopyFrom. The read path uses the normal business pool and keyset
// pagination so a long time range never needs COUNT(*) or a huge OFFSET.
package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"strings"
	"time"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// AuthMethod is the authentication style observed on the request, not the
// eventual result. Ambiguous means both a session cookie and API headers were
// supplied, which the protected middleware rejects before authentication.
type AuthMethod string

const (
	AuthMethodAdminSession AuthMethod = "admin_session"
	AuthMethodAPIApp       AuthMethod = "api_app"
	AuthMethodNone         AuthMethod = "none"
	AuthMethodAmbiguous    AuthMethod = "ambiguous"
)

func (m AuthMethod) IsValid() bool {
	switch m {
	case AuthMethodAdminSession, AuthMethodAPIApp, AuthMethodNone, AuthMethodAmbiguous:
		return true
	default:
		return false
	}
}

// ActorType identifies the successful caller. Failed requests have no actor.
type ActorType string

const (
	ActorTypeAdmin  ActorType = "admin"
	ActorTypeAPIApp ActorType = "api_app"
)

func (t ActorType) IsValid() bool {
	return t == ActorTypeAdmin || t == ActorTypeAPIApp
}

// Entry is the immutable value captured by the middleware and handed to the
// background writer. It contains no request body, credential, or response body.
type Entry struct {
	OccurredAt    time.Time
	TraceID       string
	Method        string
	Path          string
	Route         string
	StatusCode    int
	LatencyMS     int64
	AuthMethod    AuthMethod
	ActorType     *ActorType
	ActorID       *int64
	ActorLabel    string
	CredentialID  string
	ClientIP      string
	UserAgent     string
	ErrorCode     string
	RequestBytes  int64
	ResponseBytes int64
}

// Log is the query representation. Nullable columns use pointers so the JSON
// shape stays explicit for the admin UI.
type Log struct {
	ID            int64      `json:"id"`
	OccurredAt    time.Time  `json:"occurred_at"`
	TraceID       string     `json:"trace_id"`
	Method        string     `json:"method"`
	Path          string     `json:"path"`
	Route         *string    `json:"route,omitempty"`
	StatusCode    int        `json:"status_code"`
	LatencyMS     int64      `json:"latency_ms"`
	AuthMethod    AuthMethod `json:"auth_method"`
	ActorType     *ActorType `json:"actor_type,omitempty"`
	ActorID       *int64     `json:"actor_id,omitempty"`
	ActorLabel    *string    `json:"actor_label,omitempty"`
	CredentialID  *string    `json:"credential_id,omitempty"`
	ClientIP      *string    `json:"client_ip,omitempty"`
	UserAgent     *string    `json:"user_agent,omitempty"`
	ErrorCode     *string    `json:"error_code,omitempty"`
	RequestBytes  int64      `json:"request_bytes"`
	ResponseBytes int64      `json:"response_bytes"`
}

// ListFilter is the raw query input parsed by the HTTP handler.
type ListFilter struct {
	From       time.Time
	To         time.Time
	AuthMethod *AuthMethod
	ActorType  *ActorType
	ActorID    *int64
	ActorLabel string
	ClientIP   string
	StatusCode *int
	Path       string
	Limit      int
	Cursor     string
}

// QueryFilter is the validated filter consumed by the repository.
type QueryFilter struct {
	From       time.Time
	To         time.Time
	AuthMethod *AuthMethod
	ActorType  *ActorType
	ActorID    *int64
	ActorLabel *string
	ClientIP   *netip.Addr
	StatusCode *int
	Path       *string
	Limit      int
	Cursor     *Cursor
}

// Cursor is the keyset position of the last row of a page.
type Cursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         int64     `json:"id"`
}

type ListResult struct {
	Items      []Log  `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type EntrySink interface {
	Enqueue(entry Entry) bool
}

type BatchInserter interface {
	InsertBatch(ctx context.Context, entries []Entry) error
}

const (
	defaultListLimit = 50
	maxListLimit     = 200
	maxRange         = 366 * 24 * time.Hour
	maxPathLength    = 1024
)

func EncodeCursor(log Log) string {
	encoded, err := json.Marshal(Cursor{OccurredAt: log.OccurredAt, ID: log.ID})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func DecodeCursor(value string) (*Cursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, apperr.NewValidationError("cursor is invalid")
	}
	var cursor Cursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.OccurredAt.IsZero() || cursor.ID <= 0 {
		return nil, apperr.NewValidationError("cursor is invalid")
	}
	return &cursor, nil
}

func truncate(value string, maxRunes int) string {
	if value == "" || maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
