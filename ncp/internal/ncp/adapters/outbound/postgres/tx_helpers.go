package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func exec(ctx context.Context, db *DB, q string, args ...any) error {
	if tx, ok := getTx(ctx); ok {
		_, err := tx.Exec(ctx, q, args...)
		return err
	}
	_, err := db.Pool.Exec(ctx, q, args...)
	return err
}

func queryRow(ctx context.Context, db *DB, q string, args ...any) pgx.Row {
	if tx, ok := getTx(ctx); ok {
		return tx.QueryRow(ctx, q, args...)
	}
	return db.Pool.QueryRow(ctx, q, args...)
}

func execResult(ctx context.Context, db *DB, q string, args ...any) (int64, error) {
	if tx, ok := getTx(ctx); ok {
		tag, err := tx.Exec(ctx, q, args...)
		if err != nil {
			return 0, err
		}
		return tag.RowsAffected(), nil
	}
	tag, err := db.Pool.Exec(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
