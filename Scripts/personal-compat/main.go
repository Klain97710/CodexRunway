// Round-trip only the synthetic files created by the Swift compatibility probe.
package main

import (
	"fmt"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/personal"
	"github.com/licoy/quosparr/internal/store"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		panic("synthetic fixture directory required")
	}
	dir := os.Args[1]
	read := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			panic(e)
		}
		return b
	}
	write := func(name string, b []byte) {
		if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			panic(e)
		}
	}
	p, e := store.DecodePreferences(read("swift-preferences.json"))
	if e != nil {
		panic(e)
	}
	p.ApplyFeatures(personal.Features{})
	b, e := store.EncodePreferences(p)
	if e != nil {
		panic(e)
	}
	write("go-preferences.json", b)
	idx, e := codex.ParseIndex(read("swift-index.json"))
	if e != nil {
		panic(e)
	}
	b, e = codex.MarshalIndex(idx)
	if e != nil {
		panic(e)
	}
	write("go-index.json", b)
	fmt.Println("Synthetic preferences and account index written by Go")
}
