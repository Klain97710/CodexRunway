package claude

import "sort"

func sortWindows(out []UsageWindow) {
	sort.Slice(out, func(i, j int) bool {
		if windowRank(out[i]) != windowRank(out[j]) {
			return windowRank(out[i]) < windowRank(out[j])
		}
		return out[i].Key < out[j].Key
	})
}
