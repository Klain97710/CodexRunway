package main

import "testing"

func TestProviderArchiveRejectsChangedData(t *testing.T) {
	if _, err := readProviders([]byte("changed archive")); err == nil {
		t.Fatal("provider archive accepted without matching pinned integrity")
	}
}
