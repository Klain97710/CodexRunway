package claude

import (
	"time"

	"github.com/licoy/quosparr/internal/codex"
)

// Exporter builds a quosparr-accounts pack of Claude credentials.
type Exporter struct {
	Store      *Store
	AppVersion string
}

// Export reads the managed credentials of ids. Missing accounts are skipped
// and named in Failures. The pack uses provider "claude".
func (e Exporter) Export(ids []string, now time.Time) (codex.ExportResult, error) {
	if len(ids) == 0 {
		return codex.ExportResult{}, codex.ErrEmptySelection
	}
	idx, err := e.Store.LoadIndex()
	if err != nil {
		return codex.ExportResult{}, err
	}
	var entries []codex.TransferEntry
	var failures []string
	for _, id := range ids {
		meta, ok := idx.Account(id)
		if !ok {
			failures = append(failures, id+": account not found")
			continue
		}
		credential, err := e.Store.LoadCredentialData(id)
		if err != nil {
			failures = append(failures, meta.ResolvedDisplayName()+": credential missing")
			continue
		}
		entries = append(entries, codex.TransferEntry{
			ID: meta.ID, Alias: meta.Alias, DisplayName: meta.ResolvedDisplayName(),
			Email: meta.Identity.Email, Credential: credential,
		})
	}
	if len(entries) == 0 {
		return codex.ExportResult{}, codex.ErrNoAccountsExported
	}
	pack := codex.NewTransferPack(codex.ProviderClaude, now, e.AppVersion, entries)
	return codex.ExportResult{Pack: pack, ExportedCount: len(entries), Failures: failures}, nil
}

// Write exports ids and writes the pack to path.
func (e Exporter) Write(ids []string, path string, now time.Time) (codex.ExportResult, error) {
	result, err := e.Export(ids, now)
	if err != nil {
		return codex.ExportResult{}, err
	}
	if err := codex.WriteTransferPack(path, result.Pack); err != nil {
		return codex.ExportResult{}, err
	}
	return result, nil
}

// CredentialsFromPack returns the Claude entries of a quosparr-accounts pack.
func CredentialsFromPack(data []byte) (codex.TransferPack, error) {
	pack, err := codex.DecodeTransferPack(data)
	if err != nil {
		return codex.TransferPack{}, err
	}
	if pack.Provider != codex.ProviderClaude {
		return codex.TransferPack{}, &codex.ProviderMismatchError{Expected: codex.ProviderClaude, Actual: pack.Provider}
	}
	return pack, nil
}
