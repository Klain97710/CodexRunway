package app

// The production identity is unchanged; never remove its current login item.
var previousLoginIDs = []string{"dev.licoy.codex-runway"}

func previousLoginID(id string) bool {
	for _, previous := range previousLoginIDs {
		if previous == id {
			return true
		}
	}
	return false
}
