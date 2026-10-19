package tomlstmt

import (
	"reflect"
	"testing"
)

func TestQuoteAndApplyEdits(t *testing.T) {
	if got, want := Quote(`C:\Users\me\SKILL.md`), `"C:\\Users\\me\\SKILL.md"`; got != want {
		t.Fatalf("Quote = %q, want %q", got, want)
	}
	edits := []Edit{{Start: 2, End: 3, Text: "X"}, {Start: 0, End: 1, Text: "Y"}}
	if got := string(ApplyEdits([]byte("abcd"), edits)); got != "YbXd" {
		t.Fatalf("ApplyEdits = %q", got)
	}
	if !reflect.DeepEqual(edits, []Edit{{Start: 2, End: 3, Text: "X"}, {Start: 0, End: 1, Text: "Y"}}) {
		t.Fatalf("ApplyEdits mutated input: %#v", edits)
	}
}
