package main

import (
	"testing"

	"github.com/licoy/quosparr/internal/sqliteruntime/assets/amd64"
	"github.com/licoy/quosparr/internal/sqliteruntime/assets/arm64"
)

func TestWindowsExecutableMustContainItsPinnedSQLite(t *testing.T) {
	for _, tc := range []struct {
		arch           string
		library, other []byte
	}{{"amd64", amd64.Library, arm64.Library}, {"arm64", arm64.Library, amd64.Library}} {
		t.Run(tc.arch, func(t *testing.T) {
			executable := append([]byte("PE executable fixture"), tc.library...)
			if err := verifyEmbeddedSQLite(executable, tc.arch); err != nil {
				t.Fatal(err)
			}
			for _, invalid := range [][]byte{nil, tc.other, executable[:len(executable)-1]} {
				if err := verifyEmbeddedSQLite(invalid, tc.arch); err == nil {
					t.Fatal("missing, wrong-architecture or truncated SQLite accepted")
				}
			}
		})
	}
}

func TestWindowsArchiveRejectsExternalSQLite(t *testing.T) {
	file := t.TempDir() + "/archive.tar.gz"
	fixtureArchive(t, file, []string{"mygo-sqlite3.dll"})
	if err := verifyArchive(file, "windows-amd64"); err == nil {
		t.Fatal("external SQLite DLL accepted in Windows archive")
	}
}
