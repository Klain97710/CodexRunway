package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/quosparr/internal/export"
	"github.com/licoy/quosparr/internal/personal"
)

// Go emits this protocol; WidgetKit persists the Swift names across upgrades.
func TestPersonalWidgetContractMatchesSwift(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("..", "..", "Sources", "QuosparrWidget", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	contract := read("RunwayWidgetContract.swift")
	for _, kind := range export.Kinds() {
		if !strings.Contains(contract, `"`+string(kind)+`"`) {
			t.Fatalf("Swift kind lost: %s", kind)
		}
	}
	for _, value := range []string{personal.Scheme, personal.DevScheme, "grok", "both"} {
		if !strings.Contains(contract, `"`+value+`"`) && !strings.Contains(contract, "case "+value) {
			t.Fatalf("legacy contract lost: %s", value)
		}
	}
	intent := read("RunwayWidgetIntent.swift")
	for _, field := range []string{"struct RunwayProviderSelectionIntent:", "struct RunwayMetricSelectionIntent:", "var provider: String", "var metric: String"} {
		if !strings.Contains(intent, field) {
			t.Fatalf("saved intent identity changed: %s", field)
		}
	}
	snapshot := read("RunwayWidgetSnapshot.swift")
	if !strings.Contains(snapshot, `"group.`+personal.BundleID+`"`) || !strings.Contains(snapshot, `"`+personal.DataDir+`"`) {
		t.Fatal("snapshot location no longer matches legacy app")
	}
	if strings.Contains(intent, "import CodexRunwayCore") {
		t.Fatal("Widget depends on removed Swift app Core")
	}
}
