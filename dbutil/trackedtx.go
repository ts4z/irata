package dbutil

import (
	"context"
	"database/sql"
)

type Tx struct {
	tx *sql.Tx
}

func (tt *Tx) Tx() *sql.Tx {
	return tt.tx
}

func NewTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (tt *Tx) MaybeRollback() {
	if tt.tx != nil {
		tt.tx.Rollback()
		tt.tx = nil
	}
}

func (tt *Tx) Commit() error {
	err := tt.tx.Commit()
	if err == nil {
		tt.tx = nil
	}
	return err
}

func (tt *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return tt.tx.QueryRowContext(ctx, query, args...)
}

func (tt *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tt.tx.ExecContext(ctx, query, args...)
}
