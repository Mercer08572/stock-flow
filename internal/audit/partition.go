package audit

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Arbitrary but stable advisory-lock key. It serializes partition
	// maintenance across API instances sharing the same database.
	partitionAdvisoryLock int64 = 0x41554449544C4F47
	partitionMaintenance        = 6 * time.Hour
)

type PartitionManager struct {
	pool          *pgxpool.Pool
	retentionDays int
	logger        *log.Logger
	now           func() time.Time
}

func NewPartitionManager(pool *pgxpool.Pool, retentionDays int) *PartitionManager {
	if retentionDays <= 0 {
		retentionDays = 180
	}
	return &PartitionManager{
		pool:          pool,
		retentionDays: retentionDays,
		logger:        log.Default(),
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// Ensure creates the current and next two monthly partitions and drops whole
// partitions that are older than the retention window. A missing partition
// would make CopyFrom fail, so startup calls Ensure before the HTTP server
// starts accepting audit writes.
func (m *PartitionManager) Ensure(ctx context.Context, at time.Time) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", partitionAdvisoryLock); err != nil {
		return fmt.Errorf("lock audit partition maintenance: %w", err)
	}

	start := monthStart(at)
	for offset := 0; offset <= 2; offset++ {
		if err := createPartition(ctx, tx, start.AddDate(0, offset, 0)); err != nil {
			return err
		}
	}

	cutoff := at.UTC().AddDate(0, 0, -m.retentionDays)
	if err := m.dropExpiredPartitions(ctx, tx, cutoff); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (m *PartitionManager) Run(ctx context.Context) {
	ticker := time.NewTicker(partitionMaintenance)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Ensure(ctx, m.now()); err != nil {
				m.logger.Printf("audit: partition maintenance failed: %v", err)
			}
		}
	}
}

func (m *PartitionManager) dropExpiredPartitions(ctx context.Context, tx pgx.Tx, cutoff time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT child.relname
		FROM pg_inherits AS inherit
		JOIN pg_class AS child ON child.oid = inherit.inhrelid
		JOIN pg_class AS parent ON parent.oid = inherit.inhparent
		JOIN pg_namespace AS namespace ON namespace.oid = parent.relnamespace
		WHERE parent.relname = 'audit_logs'
		  AND namespace.nspname = 'public'`)
	if err != nil {
		return err
	}

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	// Close before running DDL on the same transaction: an open result set can
	// keep the connection busy.
	rows.Close()

	for _, name := range names {
		start, ok := parsePartitionName(name)
		if !ok {
			continue
		}
		// Drop only when the whole month is older than the cutoff, so the
		// retention window is never shortened by a partial month.
		if start.AddDate(0, 1, 0).After(cutoff) {
			continue
		}
		query := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{name}.Sanitize())
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("drop audit partition %s: %w", name, err)
		}
		m.logger.Printf("audit: dropped expired partition %s", name)
	}
	return nil
}

func createPartition(ctx context.Context, tx pgx.Tx, start time.Time) error {
	start = monthStart(start)
	next := start.AddDate(0, 1, 0)
	name := partitionName(start)
	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs FOR VALUES FROM ('%s') TO ('%s')",
		pgx.Identifier{name}.Sanitize(),
		start.Format("2006-01-02"),
		next.Format("2006-01-02"),
	)
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("create audit partition %s: %w", name, err)
	}
	return nil
}

func monthStart(at time.Time) time.Time {
	at = at.UTC()
	return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func partitionName(start time.Time) string {
	start = monthStart(start)
	return fmt.Sprintf("audit_logs_%04d%02d", start.Year(), int(start.Month()))
}

func parsePartitionName(name string) (time.Time, bool) {
	const prefix = "audit_logs_"
	if !strings.HasPrefix(name, prefix) {
		return time.Time{}, false
	}
	suffix := strings.TrimPrefix(name, prefix)
	if len(suffix) != 6 {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(suffix[:4])
	if err != nil {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(suffix[4:])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), true
}
