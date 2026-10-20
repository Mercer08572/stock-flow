//go:build integration

// Package testdb 提供数据库集成测试的公共辅助能力。
//
// 每次 Require 都会在 TEST_DATABASE_URL 指向的测试库中创建一个独立的 schema，
// 并在其中执行 migrations/ 全量迁移；测试结束后删除该 schema。
// 这样测试不会与测试库中已有的 public schema 争夺对象所有权，也不会互相污染。
package testdb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const (
	// TestDatabaseURLEnv 是测试库连接串的环境变量名。
	TestDatabaseURLEnv = "TEST_DATABASE_URL"
	// AllowDestructiveEnv 显式确认允许对测试库执行 TRUNCATE / DROP。
	AllowDestructiveEnv = "TEST_DATABASE_ALLOW_DESTRUCTIVE"
	// MigrateBinEnv 可选地指定 migrate CLI 路径。
	MigrateBinEnv = "MIGRATE_BIN"

	// advisoryLockKey 是测试库迁移/清理的全局锁，避免多个测试进程同时改 schema。
	advisoryLockKey int64 = 0x53544F434B464C57
)

// DefaultTables 是库存相关测试默认清空的表。
//
// CASCADE 会处理外键依赖，因此这里的顺序不重要；schema_migrations 刻意保留。
var DefaultTables = []string{
	"inventory_movements",
	"inventory_idempotency_keys",
	"inventory_reservation_items",
	"inventory_reservations",
	"inventory_stock_layers",
	"inventory_stocks",
	"inventory_batches",
	"skus",
	"warehouses",
	"material_unit_conversions",
	"materials",
	"material_categories",
	"units",
}

// DB 是一次集成测试使用的独立 schema 句柄。
type DB struct {
	URL           string
	Schema        string
	Pool          *pgxpool.Pool
	MigrationsDir string

	migrateURL string
	migrateBin string
}

// Require 加载测试配置、创建独立 schema、连接并执行全量迁移。
//
// 未配置 TEST_DATABASE_URL 或缺少 migrate CLI 时调用 t.Skip。
func Require(t *testing.T) *DB {
	t.Helper()

	root := repoRoot(t)
	loadDotEnv(t, root)
	pinEnvFile(t)

	baseURL := strings.TrimSpace(os.Getenv(TestDatabaseURLEnv))
	if baseURL == "" {
		t.Skipf("set %s (for example in stock-flow/.env.test) to run database integration tests", TestDatabaseURLEnv)
	}

	migrateBin, err := resolveMigrateBinary()
	if err != nil {
		t.Skipf("%v", err)
	}

	requireDestructiveAllowed(t, baseURL)

	schema := fmt.Sprintf("stock_flow_it_%d_%d", os.Getpid(), time.Now().UnixNano())
	migrateURL, err := withSearchPath(baseURL, schema)
	if err != nil {
		t.Fatalf("build migration URL: %v", err)
	}

	pool := newPool(t, baseURL, schema)
	db := &DB{
		URL:           baseURL,
		Schema:        schema,
		Pool:          pool,
		MigrationsDir: filepath.Join(root, "migrations"),
		migrateURL:    migrateURL,
		migrateBin:    migrateBin,
	}
	t.Cleanup(func() {
		dropSchema(db)
		pool.Close()
	})

	if _, err := pool.Exec(context.Background(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create test schema %s: %v", schema, err)
	}

	db.MigrateUp(t)

	return db
}

// MigrateUp 执行全量迁移。
func (db *DB) MigrateUp(t *testing.T) {
	t.Helper()

	if err := db.withLock(context.Background(), func() error {
		return db.runMigrate("up")
	}); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}

// ResetSchema 删除并重建当前测试 schema 中的全部对象。
func (db *DB) ResetSchema(t *testing.T) {
	t.Helper()

	err := db.withLock(context.Background(), func() error {
		if err := db.runMigrate("drop", "-f"); err != nil {
			return err
		}
		return db.runMigrate("up")
	})
	if err != nil {
		t.Fatalf("reset test database schema: %v", err)
	}
}

// Truncate 清空指定表；不传参数时清空 DefaultTables。
func (db *DB) Truncate(t *testing.T, tables ...string) {
	t.Helper()

	if len(tables) == 0 {
		tables = DefaultTables
	}
	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, pgx.Identifier{table}.Sanitize())
	}
	statement := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE"

	err := db.withLock(context.Background(), func() error {
		_, execErr := db.Pool.Exec(context.Background(), statement)
		return execErr
	})
	if err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}
}

func (db *DB) withLock(ctx context.Context, fn func() error) error {
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	conn, err := db.Pool.Acquire(lockCtx)
	if err != nil {
		return fmt.Errorf("acquire test database connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(lockCtx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("acquire test database advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	return fn()
}

func (db *DB) runMigrate(args ...string) error {
	cmdArgs := append([]string{"-path", db.MigrationsDir, "-database", db.migrateURL}, args...)
	cmd := exec.Command(db.migrateBin, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("migrate %s: %w: %s", strings.Join(args, " "), err, redact(output, db.migrateURL))
	}
	return nil
}

func newPool(t *testing.T, baseURL string, schema string) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatalf("parse %s: %v", TestDatabaseURLEnv, err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test database: %v", err)
	}

	return pool
}

func dropSchema(db *DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _ = db.Pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{db.Schema}.Sanitize()+" CASCADE")
}

func withSearchPath(baseURL string, schema string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func requireDestructiveAllowed(t *testing.T, url string) {
	t.Helper()

	if strings.TrimSpace(os.Getenv(AllowDestructiveEnv)) == "1" {
		return
	}

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse %s: %v", TestDatabaseURLEnv, err)
	}
	database := strings.ToLower(config.ConnConfig.Database)
	if strings.Contains(database, "test") {
		return
	}

	t.Fatalf(
		"refusing to run destructive integration tests against database %q: set %s=1 or use a database whose name contains \"test\"",
		config.ConnConfig.Database,
		AllowDestructiveEnv,
	)
}

func loadDotEnv(t *testing.T, root string) {
	t.Helper()

	if strings.TrimSpace(os.Getenv(TestDatabaseURLEnv)) != "" {
		return
	}

	path := filepath.Join(root, ".env.test")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("stat %s: %v", path, err)
	}
	if err := godotenv.Load(path); err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
}

// pinEnvFile 让 config.Load 只读取一个空文件，避免开发用 .env 干扰测试库。
func pinEnvFile(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "empty.env")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write isolated env file: %v", err)
	}
	t.Setenv("ENV_FILE", path)
}

func resolveMigrateBinary() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(MigrateBinEnv)); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("%s points at %q, which is not an executable file", MigrateBinEnv, configured)
	}

	path, err := exec.LookPath("migrate")
	if err != nil {
		return "", fmt.Errorf("the golang-migrate CLI is required to run database integration tests: %w", err)
	}
	return path, nil
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the test working directory")
		}
		dir = parent
	}
}

func redact(output []byte, url string) string {
	text := string(output)
	if url != "" {
		text = strings.ReplaceAll(text, url, "<test-database-url>")
	}
	return strings.TrimSpace(text)
}
