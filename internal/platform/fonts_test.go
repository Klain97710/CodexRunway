package platform

import (
	"slices"
	"testing"
)

func TestTidyFamiliesSortsAndDropsHiddenAndDuplicates(t *testing.T) {
	got := tidyFamilies([]string{"menlo", ".SF NS", "Arial", " Menlo ", "@MS Gothic", "", "Bad, List", "PingFang SC", "arial"})
	want := []string{"Arial", "menlo", "PingFang SC"}
	if !slices.Equal(got, want) {
		t.Errorf("tidyFamilies = %q, want %q", got, want)
	}
}

func TestFontFamiliesAreTidy(t *testing.T) {
	got := FontFamilies()
	if !slices.Equal(got, tidyFamilies(got)) {
		t.Errorf("FontFamilies is not sorted and unique: %q", got)
	}
}
