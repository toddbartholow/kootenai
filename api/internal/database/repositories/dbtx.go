package repositories

import (
	"context"
	"database/sql"
	"errors"
)

// DBTX is an interface satisfied by both *sql.DB and *sql.Tx,
// enabling repositories to work within transactions.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DBTXBeginner is an optional extension of DBTX that supports beginning
// transactions. *sql.DB satisfies this interface but *sql.Tx does not.
type DBTXBeginner interface {
	DBTX
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// ErrNotTransactable is returned when BeginTx is called on a DBTX that
// does not support starting transactions (e.g., an *sql.Tx).
var ErrNotTransactable = errors.New("underlying DBTX does not support BeginTx")

// beginTx attempts to start a transaction on the given DBTX. If the
// underlying value supports BeginTx (e.g., *sql.DB), it starts one.
// Otherwise it returns ErrNotTransactable.
func beginTx(ctx context.Context, db DBTX, opts *sql.TxOptions) (*sql.Tx, error) {
	if beginner, ok := db.(DBTXBeginner); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return nil, ErrNotTransactable
}
