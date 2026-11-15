package audit

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

const defaultRange = 24 * time.Hour

type Repository interface {
	InsertBatch(ctx context.Context, entries []Entry) error
	List(ctx context.Context, filter QueryFilter) ([]Log, error)
	Aggregate(ctx context.Context, filter StatsFilter) ([]StatsBucket, error)
}

type Service interface {
	List(ctx context.Context, filter ListFilter) (ListResult, error)
	Stats(ctx context.Context, filter StatsFilter) (Stats, error)
}

type ServiceOptions struct {
	Now func() time.Time
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, options ServiceOptions) Service {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return &service{repo: repo, now: options.Now}
}

func (s *service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	normalized, limit, err := s.normalizeFilter(filter)
	if err != nil {
		return ListResult{}, err
	}

	// Ask for one extra row so the page can advertise a next cursor without a
	// COUNT(*) over the whole filtered range.
	normalized.Limit = limit + 1
	items, err := s.repo.List(ctx, normalized)
	if err != nil {
		return ListResult{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	result := ListResult{Items: items}
	if result.Items == nil {
		result.Items = []Log{}
	}
	if hasMore && len(items) > 0 {
		result.NextCursor = EncodeCursor(items[len(items)-1])
	}
	return result, nil
}

func (s *service) normalizeFilter(filter ListFilter) (QueryFilter, int, error) {
	now := s.now().UTC()

	to := filter.To.UTC()
	if to.IsZero() {
		to = now
	}
	traceID := strings.TrimSpace(filter.TraceID)
	from := filter.From.UTC()
	if from.IsZero() {
		if filter.From.IsZero() && filter.To.IsZero() && traceID != "" {
			// A trace lookup should search the whole retention window, not just
			// the last 24 hours; the trace_id index keeps this cheap.
			from = to.Add(-maxRange)
		} else {
			from = to.Add(-defaultRange)
		}
	}
	if !from.Before(to) {
		return QueryFilter{}, 0, apperr.NewValidationError("from must be earlier than to")
	}
	if to.Sub(from) > maxRange {
		return QueryFilter{}, 0, apperr.NewValidationError("time range must not exceed 366 days")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	normalized := QueryFilter{From: from, To: to, Limit: limit + 1}

	if filter.AuthMethod != nil {
		method := AuthMethod(strings.TrimSpace(string(*filter.AuthMethod)))
		if !method.IsValid() {
			return QueryFilter{}, 0, apperr.NewValidationError("auth_method must be admin_session, api_app, none, or ambiguous")
		}
		normalized.AuthMethod = &method
	}
	if filter.ActorType != nil {
		actorType := ActorType(strings.TrimSpace(string(*filter.ActorType)))
		if !actorType.IsValid() {
			return QueryFilter{}, 0, apperr.NewValidationError("actor_type must be admin or api_app")
		}
		normalized.ActorType = &actorType
	}
	if filter.ActorID != nil {
		if *filter.ActorID <= 0 {
			return QueryFilter{}, 0, apperr.NewValidationError("actor_id must be greater than zero")
		}
		normalized.ActorID = filter.ActorID
	}
	if actorLabel := strings.TrimSpace(filter.ActorLabel); actorLabel != "" {
		if len([]rune(actorLabel)) > 256 {
			return QueryFilter{}, 0, apperr.NewValidationError("actor_label is too long")
		}
		normalized.ActorLabel = &actorLabel
	}
	if clientIP := strings.TrimSpace(filter.ClientIP); clientIP != "" {
		addr, err := netip.ParseAddr(clientIP)
		if err != nil {
			return QueryFilter{}, 0, apperr.NewValidationError("client_ip must be a valid IP address")
		}
		normalized.ClientIP = &addr
	}
	if filter.StatusCode != nil {
		if *filter.StatusCode < 100 || *filter.StatusCode > 599 {
			return QueryFilter{}, 0, apperr.NewValidationError("status_code must be between 100 and 599")
		}
		normalized.StatusCode = filter.StatusCode
	}
	if path := strings.TrimSpace(filter.Path); path != "" {
		if len([]rune(path)) > maxPathLength {
			return QueryFilter{}, 0, apperr.NewValidationError("path is too long")
		}
		normalized.Path = &path
	}
	if traceID != "" {
		if len([]rune(traceID)) > 128 {
			return QueryFilter{}, 0, apperr.NewValidationError("trace_id is too long")
		}
		normalized.TraceID = &traceID
	}

	cursor, err := DecodeCursor(filter.Cursor)
	if err != nil {
		return QueryFilter{}, 0, err
	}
	normalized.Cursor = cursor

	return normalized, limit, nil
}
