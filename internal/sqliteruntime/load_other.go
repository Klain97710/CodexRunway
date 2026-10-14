//go:build !windows || (!amd64 && !arm64)

// Package sqliteruntime owns SQLite runtime preparation and loading.
package sqliteruntime

import "github.com/egoist/mygo/plugins/sqlite"

// Load preserves the platform's existing packaged SQLite loader.
func Load() error { return sqlite.Load() }
