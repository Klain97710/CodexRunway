// Package personal contains the small, shared policy of this edition.
package personal

const (
	Name               = "CodexRunway"
	Edition            = "Klain97710 定制版"
	Repository         = "Klain97710/CodexRunway"
	Homepage           = "https://github.com/" + Repository
	BundleID           = "com.github.codex-runway"
	Scheme             = "codex-runway"
	DataDir            = ".codex-runway"
	LockFile           = "codex-runway.lock"
	CredentialService  = BundleID + ".network-proxy"
	DevName            = "CodexRunway Migration"
	DevBundleID        = BundleID + ".migration"
	DevScheme          = Scheme + "-migration"
	SandboxEnv         = "RUNWAY_MIGRATION_SANDBOX"
	SandboxMarker      = ".codex-runway-migration-sandbox"
	SandboxMarkerValue = "synthetic-fixtures-only-v1\n"
)
