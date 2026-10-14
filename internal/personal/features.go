package personal

// Features is injectable only at construction. The zero value is production:
// Codex only. Retained Grok regression tests must opt in explicitly.
type Features struct{ Grok bool }
