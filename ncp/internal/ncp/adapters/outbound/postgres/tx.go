package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type TxRunner struct{ DB *DB }

type txKey struct{}

func (t TxRunner) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.DB.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ctx2 := context.WithValue(ctx, txKey{}, tx)
	if err := fn(ctx2); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func getTx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
