package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolOptions tunes a pool that shares one database with another workload.
//
// The audit writer uses a small dedicated pool so a slow batch flush cannot take
// every connection away from business queries. ApplicationName also shows up in
// pg_stat_activity, which makes the two workloads distinguishable during incidents.
type PoolOptions struct {
	ApplicationName string
	MaxConns        int32
	MinConns        int32
}

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPostgresPoolWithOptions(ctx, databaseURL, PoolOptions{})
}

func NewPostgresPoolWithOptions(ctx context.Context, databaseURL string, options PoolOptions) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database url is required")
	}
	if options.ApplicationName == "" {
		options.ApplicationName = "stock-flow"
	}
	if options.MaxConns > 0 && options.MinConns > options.MaxConns {
		return nil, errors.New("postgres pool min conns must not exceed max conns")
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = options.ApplicationName
	if options.MaxConns > 0 {
		poolConfig.MaxConns = options.MaxConns
	}
	if options.MinConns > 0 {
		poolConfig.MinConns = options.MinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
