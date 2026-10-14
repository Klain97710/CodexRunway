package grok

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// The official Grok auth.json may only be written on an explicit switch (or
// first-login bootstrap) and when refreshing the current account. This test
// pins every call site of the functions that can touch it, so a new caller
// has to be a conscious decision.
func TestOfficialAuthWriteCallSites(t *testing.T) {
	want := map[string][]string{
		"SaveOfficialAuthDataAtomically": {"Module.persistRefreshed", "Module.switchOfficial"},
		"restoreOfficialAuthData":        {"Module.rollbackSwitch"},
		"writeOfficial":                  {"Store.SaveOfficialAuthDataAtomically", "Store.restoreOfficialAuthData"},
	}
	got := map[string][]string{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			owner := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
					if id, ok := star.X.(*ast.Ident); ok {
						owner = id.Name + "." + owner
					}
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if _, tracked := want[sel.Sel.Name]; tracked {
						got[sel.Sel.Name] = append(got[sel.Sel.Name], owner)
					}
				}
				return true
			})
		}
	}
	for name, owners := range want {
		sort.Strings(got[name])
		if strings.Join(got[name], ",") != strings.Join(owners, ",") {
			t.Errorf("%s is called from %v, want only %v", name, got[name], owners)
		}
	}
}
