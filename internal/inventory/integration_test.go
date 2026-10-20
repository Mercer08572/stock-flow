//go:build integration

package inventory_test

import (
	"context"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/shared/testdb"
)

// TestInventoryDatabaseIsMigrated 是 DB-002 的示范测试：
// 它验证测试库可连接、迁移可执行、清理可重复运行。
func TestInventoryDatabaseIsMigrated(t *testing.T) {
	db := testdb.Require(t)
	db.Truncate(t)

	ctx := context.Background()

	var stocks, layers int
	if err := db.Pool.QueryRow(ctx, "SELECT count(*) FROM inventory_stocks").Scan(&stocks); err != nil {
		t.Fatalf("count inventory_stocks: %v", err)
	}
	if err := db.Pool.QueryRow(ctx, "SELECT count(*) FROM inventory_stock_layers").Scan(&layers); err != nil {
		t.Fatalf("count inventory_stock_layers: %v", err)
	}
	if stocks != 0 || layers != 0 {
		t.Fatalf("expected empty inventory tables after truncate, got stocks=%d layers=%d", stocks, layers)
	}

	// 第二次清理必须可重复执行，验证辅助包不会依赖一次性状态。
	db.Truncate(t)
}
