package claude

import "testing"

func TestTierFrom(t *testing.T) {
	tests := []struct {
		sub, rate string
		want      TierKind
	}{
		{"max", "default_claude_max_20x", TierMax20x},
		{"max", "default_claude_max_5x", TierMax5x},
		{"max", "", TierMax},
		{"claude_max", "", TierMax},
		{"", "default_claude_max_20x", TierMax20x},
		{"pro", "default_claude_ai", TierPro},
		{"team", "", TierTeam},
		{"enterprise", "", TierEnterprise},
		{"", "default_claude_ai", TierUnknown},
		{"free", "", TierUnknown},
	}
	for _, tc := range tests {
		if got := TierFrom(tc.sub, tc.rate); got != tc.want {
			t.Errorf("TierFrom(%q,%q) = %d, want %d", tc.sub, tc.rate, got, tc.want)
		}
	}
}

func TestTierResolveRoundTrips(t *testing.T) {
	for _, k := range tierKinds {
		if got := TierResolve(k.DisplayName()); got != k {
			t.Errorf("TierResolve(%q) = %d, want %d", k.DisplayName(), got, k)
		}
	}
	if TierResolve("") != TierUnknown || TierResolve("Ultra") != TierUnknown || TierUnknown.DisplayName() != "" {
		t.Fatal("unknown label must stay unknown and unlabeled")
	}
}
