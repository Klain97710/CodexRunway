package app

var previousLoginIDs = []string{"com.github.quosparr", "com.github.codex-runway", "dev.licoy.codex-runway"}

func previousLoginID(id string) bool {
	for _, previous := range previousLoginIDs {
		if previous == id {
			return true
		}
	}
	return false
}
