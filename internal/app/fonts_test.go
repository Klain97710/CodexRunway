package app

import (
	"slices"
	"testing"
)

func TestOfferedFontFamiliesDropsHiddenVerticalAndLists(t *testing.T) {
	got := offeredFontFamilies([]string{"Arial", ".SF NS", "@MS Gothic", "", "Bad, List", "PingFang SC"})
	if want := []string{"Arial", "PingFang SC"}; !slices.Equal(got, want) {
		t.Errorf("offeredFontFamilies = %q, want %q", got, want)
	}
}
