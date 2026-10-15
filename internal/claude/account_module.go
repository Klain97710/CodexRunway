package claude

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CommandKind selects Module.Apply.
type CommandKind int

const (
	// CommandLogin refuses in-app OAuth. See ErrLoginUseCLI.
	CommandLogin CommandKind = iota
	// CommandImportOfficial copies the official login into the library.
	CommandImportOfficial
	// CommandSwitch installs an account as the official login.
	CommandSwitch
	// CommandRemove deletes a non-current account.
	CommandRemove
	// CommandSetAlias sets or clears the alias.
	CommandSetAlias
	// CommandReorder reorders accounts.
	CommandReorder
	// CommandPaste imports a pasted claudeAiOauth document.
	CommandPaste
)

// Command is one account action.
type Command struct {
	Kind              CommandKind
	ID                string
	Alias             string
	IDs               []string
	Text              string
	Identity          CredentialIdentity
	AllowWhileRunning bool
}

// AccountState is the library after an operation.
type AccountState struct {
	CurrentAccountID string
	Accounts         []ManagedAccount
}

// Module orchestrates the account library. The current account's token is
// never refreshed. Operations are serialized.
type Module struct {
	Store             *Store
	Usage             *UsageClient
	Tokens            RefreshClient
	Throttle          *UsageThrottle
	RunningProcessIDs func(ctx context.Context) ([]int, error)
	Now               func() time.Time
	Replacer          OAuthAccountReplacer

	mu              sync.Mutex
	profileKey      string
	profileIdentity CredentialIdentity
}

// NewModule returns a module over store.
func NewModule(store *Store) *Module { return &Module{Store: store} }

func (m *Module) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Module) throttle() *UsageThrottle {
	if m.Throttle == nil {
		m.Throttle = &UsageThrottle{}
	}
	return m.Throttle
}

func (m *Module) configPath() string {
	if m.Replacer.Path != "" {
		return m.Replacer.Path
	}
	return GlobalConfigPath(m.Store.ConfigDir(), m.Store.HomeDir())
}

func (m *Module) replacer(path string) OAuthAccountReplacer {
	r := m.Replacer
	r.Path = path
	return r
}

// Load returns the account index.
func (m *Module) Load(ctx context.Context) (AccountState, error) {
	if err := ctx.Err(); err != nil {
		return AccountState{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state()
}

func (m *Module) state() (AccountState, error) {
	idx, err := m.Store.LoadIndex()
	if err != nil {
		return AccountState{}, err
	}
	return AccountState{CurrentAccountID: idx.CurrentAccountID, Accounts: idx.OrderedAccounts()}, nil
}

// Apply runs one command.
func (m *Module) Apply(ctx context.Context, cmd Command) (AccountState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return AccountState{}, err
	}
	var err error
	switch cmd.Kind {
	case CommandLogin:
		err = Login(ctx)
	case CommandImportOfficial:
		err = m.importOfficial(ctx)
	case CommandSwitch:
		err = m.switchLocked(ctx, cmd.ID, cmd.AllowWhileRunning)
	case CommandRemove:
		err = m.Store.Remove(cmd.ID)
	case CommandSetAlias:
		err = m.setAlias(cmd.ID, cmd.Alias)
	case CommandReorder:
		err = m.Store.Reorder(cmd.IDs)
	case CommandPaste:
		_, err = m.importPasted(cmd.Text, cmd.Identity)
	default:
		err = errors.New("claude: unknown account command")
	}
	if err != nil {
		return AccountState{}, err
	}
	return m.state()
}

func (m *Module) importOfficial(ctx context.Context) error {
	login, err := m.syncOfficial(ctx)
	switch {
	case err != nil:
		return err
	case login.Data == nil:
		return ErrNotLoggedIn
	case !login.Known:
		return ErrIdentityRequired
	}
	return nil
}

func (m *Module) importPasted(text string, identity CredentialIdentity) (ManagedAccount, error) {
	data, err := ParsePasted(text)
	if err != nil {
		return ManagedAccount{}, err
	}
	return m.Store.Upsert(data, identity, UpsertOptions{Now: m.now()})
}

func (m *Module) setAlias(id, alias string) error {
	idx, err := m.Store.LoadIndex()
	if err != nil {
		return err
	}
	acc, ok := idx.Account(id)
	if !ok {
		return &AccountNotFoundError{ID: id}
	}
	acc.Alias = alias
	return m.Store.UpdateMetadata(acc)
}

// Refresh reads usage. The current account is never token-refreshed. A
// non-current account with an expired access token is refreshed first, and
// both rotated tokens are stored. invalid_grant marks that account.
func (m *Module) Refresh(ctx context.Context, id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshAccount(ctx, id, force)
}

func (m *Module) refreshUsage(ctx context.Context, id string, force bool) {
	_ = m.refreshAccount(ctx, id, force)
}
