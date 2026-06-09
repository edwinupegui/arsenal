package sqliteutil

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx runs fn inside a database transaction, committing on success and
// rolling back (best-effort) on any error. The caller is responsible for
// constructing whatever query/store object it needs from the *sql.Tx — this
// keeps WithTx free of any specific sqlc-generated package and makes it
// usable by every domain (resources, todos, finance, calendar).
//
// The standard pattern is:
//
//	err := sqliteutil.WithTx(ctx, db, func(tx *sql.Tx) error {
//	    q := store.New(db).WithTx(tx)   // or any package's WithTx(tx)
//	    // ... do work
//	    return nil
//	})
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return fn(tx)
}
