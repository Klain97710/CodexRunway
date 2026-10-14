// verify-sqlite exercises only the explicitly named packaged runtime, in memory.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/egoist/mygo/plugins/sqlite"
)

func main() {
	if err := verify(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Packaged SQLite runtime: load and parameterized query passed")
}

func verify() error {
	path := os.Getenv("MYGO_SQLITE_LIBRARY")
	if !filepath.IsAbs(path) {
		return errors.New("MYGO_SQLITE_LIBRARY must name an absolute packaged runtime path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("SQLite runtime must be a regular file")
	}
	db, err := sqlite.Open(context.Background(), ":memory:", sqlite.OpenOptions{})
	if err != nil {
		return err
	}
	rows, queryErr := db.Query(context.Background(), "SELECT ?, ?", int64(42), "Quosparr")
	closeErr := db.Close()
	if queryErr != nil || closeErr != nil {
		return errors.Join(queryErr, closeErr)
	}
	if !reflect.DeepEqual(rows.Values, [][]any{{int64(42), "Quosparr"}}) {
		return errors.New("packaged SQLite query returned incorrect values")
	}
	return nil
}
