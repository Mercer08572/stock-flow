package audit

import (
	"context"
	"testing"
	"time"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

func TestServiceListAppliesDefaultsAndBuildsCursor(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	items := make([]Log, 51)
	for index := range items {
		items[index] = Log{
			ID:         int64(100 - index),
			OccurredAt: now.Add(-time.Duration(index) * time.Minute),
			AuthMethod: AuthMethodAPIApp,
		}
	}
	repo := &fakeRepository{items: items}
	service := NewService(repo, ServiceOptions{Now: func() time.Time { return now }})

	result, err := service.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(result.Items) != defaultListLimit {
		t.Fatalf("expected %d items, got %d", defaultListLimit, len(result.Items))
	}
	if result.NextCursor == "" {
		t.Fatal("expected a next cursor")
	}
	cursor, err := DecodeCursor(result.NextCursor)
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	if cursor.ID != result.Items[len(result.Items)-1].ID {
		t.Fatalf("cursor does not point at the last returned row: %+v", cursor)
	}

	if len(repo.filters) != 1 {
		t.Fatalf("expected one repository call, got %d", len(repo.filters))
	}
	filter := repo.filters[0]
	if !filter.To.Equal(now) || !filter.From.Equal(now.Add(-defaultRange)) {
		t.Fatalf("unexpected default range: %+v", filter)
	}
	if filter.Limit != defaultListLimit+1 {
		t.Fatalf("expected repository limit %d, got %d", defaultListLimit+1, filter.Limit)
	}
}

func TestServiceListNormalizesFilters(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	service := NewService(repo, ServiceOptions{Now: func() time.Time { return now }})

	from := now.Add(-time.Hour)
	to := now.Add(-time.Minute)
	authMethod := AuthMethodAPIApp
	actorType := ActorTypeAPIApp
	actorID := int64(9)
	status := 401
	result, err := service.List(context.Background(), ListFilter{
		From:       from,
		To:         to,
		AuthMethod: &authMethod,
		ActorType:  &actorType,
		ActorID:    &actorID,
		ActorLabel: " app_test ",
		ClientIP:   "127.0.0.1",
		StatusCode: &status,
		Path:       " /api/v1/inventory/stocks ",
		Limit:      500,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("expected an empty non-nil slice, got %#v", result.Items)
	}

	filter := repo.filters[0]
	if filter.Limit != maxListLimit+1 {
		t.Fatalf("expected limit to be clamped to %d, got %d", maxListLimit+1, filter.Limit)
	}
	if filter.AuthMethod == nil || *filter.AuthMethod != AuthMethodAPIApp {
		t.Fatalf("unexpected auth method: %+v", filter.AuthMethod)
	}
	if filter.ActorType == nil || *filter.ActorType != ActorTypeAPIApp {
		t.Fatalf("unexpected actor type: %+v", filter.ActorType)
	}
	if filter.ActorID == nil || *filter.ActorID != 9 {
		t.Fatalf("unexpected actor id: %+v", filter.ActorID)
	}
	if filter.ActorLabel == nil || *filter.ActorLabel != "app_test" {
		t.Fatalf("unexpected actor label: %+v", filter.ActorLabel)
	}
	if filter.ClientIP == nil || filter.ClientIP.String() != "127.0.0.1" {
		t.Fatalf("unexpected client ip: %+v", filter.ClientIP)
	}
	if filter.StatusCode == nil || *filter.StatusCode != 401 {
		t.Fatalf("unexpected status code: %+v", filter.StatusCode)
	}
	if filter.Path == nil || *filter.Path != "/api/v1/inventory/stocks" {
		t.Fatalf("unexpected path: %+v", filter.Path)
	}
}

func TestServiceListRejectsInvalidFilters(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{}, ServiceOptions{Now: func() time.Time { return now }})

	invalidMethod := AuthMethod("bearer")
	invalidActorType := ActorType("system")
	badID := int64(0)
	badStatus := 99
	longPath := string(make([]rune, maxPathLength+1))
	longLabel := string(make([]rune, 257))
	from := now.Add(time.Hour)

	tests := []struct {
		name   string
		filter ListFilter
	}{
		{name: "from after to", filter: ListFilter{From: from, To: now}},
		{name: "range too large", filter: ListFilter{From: now.AddDate(-2, 0, 0), To: now}},
		{name: "invalid auth method", filter: ListFilter{AuthMethod: &invalidMethod}},
		{name: "invalid actor type", filter: ListFilter{ActorType: &invalidActorType}},
		{name: "invalid actor id", filter: ListFilter{ActorID: &badID}},
		{name: "invalid client ip", filter: ListFilter{ClientIP: "not-an-ip"}},
		{name: "invalid status", filter: ListFilter{StatusCode: &badStatus}},
		{name: "invalid cursor", filter: ListFilter{Cursor: "not-a-cursor"}},
		{name: "path too long", filter: ListFilter{Path: longPath}},
		{name: "actor label too long", filter: ListFilter{ActorLabel: longLabel}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.List(context.Background(), test.filter); !apperr.IsValidationError(err) {
				t.Fatalf("expected a validation error, got %v", err)
			}
		})
	}
}
