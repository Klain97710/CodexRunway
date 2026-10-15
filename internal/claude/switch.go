package claude

import (
	"context"
	"errors"
	"os"

	"github.com/licoy/quosparr/internal/platform"
)

// Switch installs id as the Claude Code login.
//
// The current account is mirrored from the official credential first, because
// Claude Code may already have rotated that token. If the official identity is
// a different account, it is upserted and no other account's copy is replaced.
// An expired target is refreshed while it is still not current. Official
// credentials then receive only a new claudeAiOauth. .claude.json receives
// only a new oauthAccount. Failure restores both.
//
// oauthAccount is updated because public switch tools show /status staying on
// the previous account otherwise. This was not checked on a live subscription
// login.
// Switch installs id as the Claude Code login. See switchLocked.
func (m *Module) Switch(ctx context.Context, id string, allowWhileRunning bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.switchLocked(ctx, id, allowWhileRunning)
}

func (m *Module) switchLocked(ctx context.Context, id string, allowWhileRunning bool) error {
	if err := m.runningBlocked(ctx, allowWhileRunning); err != nil {
		return err
	}
	if _, err := m.syncOfficial(ctx); err != nil {
		return err
	}
	target, data, err := m.targetCredential(id)
	if err != nil {
		return err
	}
	data, err = m.refreshTargetIfNeeded(ctx, target, data)
	if err != nil {
		return err
	}
	return m.install(ctx, target, data)
}

func (m *Module) runningBlocked(ctx context.Context, allow bool) error {
	if allow || m.RunningProcessIDs == nil {
		return nil
	}
	pids, err := m.RunningProcessIDs(ctx)
	if err != nil {
		return err
	}
	if len(pids) > 0 {
		return &ProcessesRunningError{PIDs: pids}
	}
	return nil
}

func (m *Module) targetCredential(id string) (ManagedAccount, []byte, error) {
	idx, err := m.Store.LoadIndex()
	if err != nil {
		return ManagedAccount{}, nil, err
	}
	target, ok := idx.Account(id)
	if !ok {
		return ManagedAccount{}, nil, &AccountNotFoundError{ID: id}
	}
	data, err := m.Store.LoadCredentialData(id)
	if err != nil {
		return ManagedAccount{}, nil, err
	}
	return target, data, nil
}

func (m *Module) refreshTargetIfNeeded(ctx context.Context, target ManagedAccount, data []byte) ([]byte, error) {
	idx, err := m.Store.LoadIndex()
	if err != nil {
		return nil, err
	}
	if idx.CurrentAccountID == target.ID {
		return data, nil
	}
	doc, err := ParseAuthDocument(data)
	if err != nil {
		return nil, err
	}
	if !doc.AccessExpired(m.now()) {
		return data, nil
	}
	if err := m.Store.RefreshManaged(ctx, target.ID, m.Tokens, m.now()); err != nil {
		return nil, err
	}
	return m.Store.LoadCredentialData(target.ID)
}

func (m *Module) install(ctx context.Context, target ManagedAccount, data []byte) error {
	official, hadOfficial, err := m.readOfficial(ctx)
	if err != nil {
		return err
	}
	updated, err := ReplaceOAuth(official, data)
	if err != nil {
		return err
	}
	configPath := m.configPath()
	configBytes, configPerm, hadConfig, err := readOptional(configPath)
	if err != nil {
		return err
	}
	if err := m.Store.SaveOfficial(ctx, updated); err != nil {
		return err
	}
	if err := m.replacer(configPath).Replace(target.Identity.OAuthAccountValue()); err != nil {
		m.rollbackSwitch(ctx, official, hadOfficial, configPath, configBytes, configPerm, hadConfig)
		return switchFailed(err)
	}
	if err := m.Store.SetCurrentAccountID(target.ID, m.now()); err != nil {
		m.rollbackSwitch(ctx, official, hadOfficial, configPath, configBytes, configPerm, hadConfig)
		return switchFailed(err)
	}
	m.refreshUsage(ctx, target.ID, true)
	return nil
}

func (m *Module) readOfficial(ctx context.Context) ([]byte, bool, error) {
	data, err := m.Store.LoadOfficial(ctx)
	if errors.Is(err, ErrNotLoggedIn) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (m *Module) rollbackSwitch(ctx context.Context, official []byte, hadOfficial bool, configPath string, config []byte, configPerm os.FileMode, hadConfig bool) {
	if hadOfficial {
		_ = m.Store.SaveOfficial(ctx, official)
	} else if file, ok := m.Store.official().(interface{ Remove(context.Context) error }); ok {
		_ = file.Remove(ctx)
	}
	if hadConfig {
		_ = platform.WriteFileAtomic(configPath, config, configPerm)
		return
	}
	_ = os.Remove(configPath)
}

func switchFailed(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(err, errors.New("claude: if the login is unusable, run claude auth login"))
}

func readOptional(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.Mode().Perm(), true, nil
}
