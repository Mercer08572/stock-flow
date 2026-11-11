package audit

import (
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestBuildListQueryOnlyAddsSuppliedFilters(t *testing.T) {
	from := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	authMethod := AuthMethodAPIApp
	actorType := ActorTypeAPIApp
	actorID := int64(9)
	actorLabel := "app_test"
	clientIP := netip.MustParseAddr("127.0.0.1")
	status := 401
	path := "/api/v1/inventory/stocks"
	cursor := &Cursor{OccurredAt: to.Add(-time.Hour), ID: 77}

	query, args := buildListQuery(QueryFilter{
		From:       from,
		To:         to,
		AuthMethod: &authMethod,
		ActorType:  &actorType,
		ActorID:    &actorID,
		ActorLabel: &actorLabel,
		ClientIP:   &clientIP,
		StatusCode: &status,
		Path:       &path,
		Limit:      51,
		Cursor:     cursor,
	})

	for _, fragment := range []string{
		"occurred_at >= $1",
		"occurred_at < $2",
		"auth_method = $3",
		"actor_type = $4",
		"actor_id = $5",
		"actor_label = $6",
		"client_ip = $7",
		"status_code = $8",
		"path = $9",
		"(occurred_at, id) < ($10, $11)",
		"LIMIT $12",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query is missing %q:\n%s", fragment, query)
		}
	}
	if len(args) != 12 {
		t.Fatalf("expected 12 args, got %d: %+v", len(args), args)
	}
	if args[0] != from || args[1] != to {
		t.Fatalf("unexpected time args: %+v", args[:2])
	}
	if args[9] != cursor.OccurredAt || args[10] != cursor.ID || args[11] != 51 {
		t.Fatalf("unexpected cursor/limit args: %+v", args[9:])
	}
}

func TestBuildListQueryOmitsUnsetFilters(t *testing.T) {
	from := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

	query, args := buildListQuery(QueryFilter{From: from, To: to, Limit: 50})
	if strings.Contains(query, "auth_method =") || strings.Contains(query, "client_ip =") {
		t.Fatalf("query unexpectedly contains optional filter conditions:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("expected from/to/limit args, got %d: %+v", len(args), args)
	}
	if !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("expected LIMIT $3, got:\n%s", query)
	}
}
