package audit

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const auditLogSelectColumns = `id, occurred_at, trace_id, method, path, route, status_code, latency_ms,
	auth_method, actor_type, actor_id, actor_label, credential_id, client_ip, user_agent,
	error_code, request_bytes, response_bytes`

var auditLogInsertColumns = []string{
	"occurred_at",
	"trace_id",
	"method",
	"path",
	"route",
	"status_code",
	"latency_ms",
	"auth_method",
	"actor_type",
	"actor_id",
	"actor_label",
	"credential_id",
	"client_ip",
	"user_agent",
	"error_code",
	"request_bytes",
	"response_bytes",
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// InsertBatch uses CopyFrom because the audit path is append-only and batch
// oriented. A single multi-row COPY avoids per-row round trips and lets
// PostgreSQL amortize WAL/fsync cost across the whole batch.
func (r *postgresRepository) InsertBatch(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	rows := make([][]any, len(entries))
	for index, entry := range entries {
		rows[index] = []any{
			entry.OccurredAt,
			entry.TraceID,
			entry.Method,
			entry.Path,
			nullableString(entry.Route),
			entry.StatusCode,
			entry.LatencyMS,
			string(entry.AuthMethod),
			nullableActorType(entry.ActorType),
			nullableInt64(entry.ActorID),
			nullableString(entry.ActorLabel),
			nullableString(entry.CredentialID),
			auditClientIP(entry.ClientIP),
			nullableString(entry.UserAgent),
			nullableString(entry.ErrorCode),
			entry.RequestBytes,
			entry.ResponseBytes,
		}
	}

	_, err := r.pool.CopyFrom(ctx, pgx.Identifier{"audit_logs"}, auditLogInsertColumns, pgx.CopyFromRows(rows))
	return err
}

func (r *postgresRepository) List(ctx context.Context, filter QueryFilter) ([]Log, error) {
	query, args := buildListQuery(filter)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]Log, 0, filter.Limit)
	for rows.Next() {
		var (
			entry        Log
			authMethod   string
			route        pgtype.Text
			actorType    pgtype.Text
			actorID      pgtype.Int8
			actorLabel   pgtype.Text
			credentialID pgtype.Text
			clientIP     netip.Addr
			userAgent    pgtype.Text
			errorCode    pgtype.Text
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.OccurredAt,
			&entry.TraceID,
			&entry.Method,
			&entry.Path,
			&route,
			&entry.StatusCode,
			&entry.LatencyMS,
			&authMethod,
			&actorType,
			&actorID,
			&actorLabel,
			&credentialID,
			&clientIP,
			&userAgent,
			&errorCode,
			&entry.RequestBytes,
			&entry.ResponseBytes,
		); err != nil {
			return nil, err
		}

		entry.AuthMethod = AuthMethod(authMethod)
		entry.Route = textPointer(route)
		if actorType.Valid {
			value := ActorType(actorType.String)
			entry.ActorType = &value
		}
		if actorID.Valid {
			value := actorID.Int64
			entry.ActorID = &value
		}
		entry.ActorLabel = textPointer(actorLabel)
		entry.CredentialID = textPointer(credentialID)
		if clientIP.IsValid() {
			value := clientIP.String()
			entry.ClientIP = &value
		}
		entry.UserAgent = textPointer(userAgent)
		entry.ErrorCode = textPointer(errorCode)
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

// buildListQuery adds only the conditions the caller actually supplied. The
// alternative (one query with `($n IS NULL OR column = $n)`) is simpler but
// often prevents the planner from using the per-filter indexes.
func buildListQuery(filter QueryFilter) (string, []any) {
	args := []any{filter.From, filter.To}
	conditions := []string{"occurred_at >= $1", "occurred_at < $2"}

	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.AuthMethod != nil {
		add("auth_method = $%d", string(*filter.AuthMethod))
	}
	if filter.ActorType != nil {
		add("actor_type = $%d", string(*filter.ActorType))
	}
	if filter.ActorID != nil {
		add("actor_id = $%d", *filter.ActorID)
	}
	if filter.ActorLabel != nil {
		add("actor_label = $%d", *filter.ActorLabel)
	}
	if filter.ClientIP != nil {
		add("client_ip = $%d", *filter.ClientIP)
	}
	if filter.StatusCode != nil {
		add("status_code = $%d", *filter.StatusCode)
	}
	if filter.Path != nil {
		add("path = $%d", *filter.Path)
	}
	if filter.TraceID != nil {
		add("trace_id = $%d", *filter.TraceID)
	}
	if filter.Cursor != nil {
		args = append(args, filter.Cursor.OccurredAt, filter.Cursor.ID)
		conditions = append(conditions, fmt.Sprintf("(occurred_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, filter.Limit)
	query := fmt.Sprintf(
		"SELECT %s FROM audit_logs WHERE %s ORDER BY occurred_at DESC, id DESC LIMIT $%d",
		strings.Join(strings.Fields(auditLogSelectColumns), " "),
		strings.Join(conditions, " AND "),
		len(args),
	)
	return query, args
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableActorType(value *ActorType) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func auditClientIP(value string) any {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return addr
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}
