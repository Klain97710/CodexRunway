package sqliteruntime

import (
	"context"
	"errors"
	"reflect"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Verify exercises the same loader as usage scans, using only an in-memory DB.
func Verify() error {
	if err := Load(); err != nil {
		return err
	}
	db, err := sqlite.Open(context.Background(), ":memory:", sqlite.OpenOptions{})
	if err != nil {
		return err
	}
	rows, queryErr := db.Query(context.Background(), "SELECT ?, ?", int64(42), "Quosparr")
	if err := errors.Join(queryErr, db.Close()); err != nil {
		return err
	}
	if !reflect.DeepEqual(rows.Values, [][]any{{int64(42), "Quosparr"}}) {
		return errors.New("sqlite runtime: parameterized query returned incorrect values")
	}
	return nil
}
