package inventory

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	inventorydb "github.com/Mercer08572/stock-flow/internal/inventory/db"
)

// UnitOfWork 在应用服务层开启事务，并把事务绑定的 Repository 传给用例。
//
// 仓储只负责使用传入的 Querier，不启动、提交或回滚事务。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error
}

type pgxUnitOfWork struct {
	pool *pgxpool.Pool
}

func NewPostgresUnitOfWork(pool *pgxpool.Pool) UnitOfWork {
	return &pgxUnitOfWork{pool: pool}
}

func (u *pgxUnitOfWork) WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Commit 成功后再 Rollback 会返回 ErrTxClosed，可以安全忽略。
		_ = tx.Rollback(context.Background())
	}()

	repo := newPostgresRepository(inventorydb.New(tx))
	if err := fn(ctx, repo); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
