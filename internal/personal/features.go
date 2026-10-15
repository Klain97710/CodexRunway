package personal

// Features is injectable only at construction. The zero value is production:
// Codex only. Retained provider regression tests must opt in explicitly.
type Features struct{ Grok, Claude bool }

func (f Features) Enabled(provider string) bool {
	switch provider {
	case "codex":
		return true
	case "grok":
		return f.Grok
	case "claude":
		return f.Claude
	default:
		return false
	}
}
