package chat

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/accounts"
	"github.com/ryokusnadi/mooring/internal/ledger"
	"github.com/ryokusnadi/mooring/internal/settings"
)

// Limits on chats with a running agent.
const (
	MaxRunning    = 20
	MaxPerProject = 8
	maxPromptText = 256 << 10
	startTimeout  = 90 * time.Second
	cancelWait    = 15 * time.Second
	shutdownWait  = 5 * time.Second
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Publisher receives the events chats produce on the events socket.
type Publisher interface {
	Publish(kind string, data any)
}

// Worktrees creates git worktrees (internal/git).
type Worktrees interface {
	WorktreeFolder(projectID, branch string) (folder, madeBranch string, err error)
	// DropWorktree removes a worktree made for a chat that then failed to start.
	DropWorktree(projectID, folder, branch string)
}

// ToolServers looks up the saved tool servers (internal/mcp).
type ToolServers interface {
	// Usable fails when a chat may not be started with the server: missing or off.
	Usable(id string) error
	// Resolve returns the server with command, env, headers and sign-in token filled
	// in. On error the reason is plain words for a notice; the returned Name may name it.
	Resolve(ctx context.Context, id string) (ToolServer, error)
}

// Config is what a Manager needs.
type Config struct {
	DB  *sql.DB
	Pub Publisher
	// Dir maps a project id to its folder. Errors should be *ledger.Error.
	Dir func(projectID string) (string, error)
	// Catalog lists the agents; nil uses DefaultCatalog.
	Catalog []AgentDef
	// DefaultAgent is used when a chat names none; "" means "claude".
	DefaultAgent string
	// Worktrees makes the git worktree of a chat started with folder.worktree.
	// It returns the folder relative to the project. Errors should be *ledger.Error.
	Worktrees Worktrees
	// Tools resolves the tool servers of a chat; nil means a chat can use none.
	Tools ToolServers
	// Accounts knows the logins of the agents; nil means every chat uses the agent's own login.
	Accounts *accounts.Service
	Log      *slog.Logger
	Context  context.Context
	// DataDir is Mooring's data folder; small jobs run in <DataDir>/jobs.
	DataDir string
	// Settings holds the job switches and the small model; nil means jobs are off.
	Settings *settings.Service
	// Staged gives the staged diff of a folder for commit messages (git.Service.Staged).
	Staged func(ctx context.Context, projectID, folder string) (stat, diff string, err error)
	// Uncommitted gives a folder's changes against HEAD for a second opinion (git.Service.Uncommitted).
	Uncommitted func(ctx context.Context, projectID, folder string) (stat, diff string, err error)
	// JobHook sees every finished job's usage (the bench reads it).
	JobHook func(JobRecord)
	// JobTimeout is how long a small job may take; 0 means 60 seconds.
	JobTimeout time.Duration
	Now        func() time.Time
	// Tests shrink these.
	KillGrace  time.Duration
	CancelWait time.Duration
	Idle       time.Duration
}

// Manager owns every chat.
type Manager struct {
	Config
	log     *slog.Logger
	pub     Publisher
	db      *sql.DB
	now     func() time.Time
	catalog map[string]AgentDef
	order   []string

	mu     sync.Mutex
	chats  map[string]*Chat
	closed bool

	jobMu   sync.Mutex
	jobBusy map[string]bool // projects with a small job running
	jobsWG  sync.WaitGroup  // turns and title jobs in the background
}

// New loads the saved chats. They are all stopped until the browser resumes them.
func New(cfg Config) (*Manager, error) {
	m := &Manager{Config: cfg, log: cfg.Log, pub: cfg.Pub, db: cfg.DB, now: cfg.Now, chats: map[string]*Chat{}, catalog: map[string]AgentDef{}}
	if m.log == nil {
		m.log = slog.New(slog.DiscardHandler)
	}
	if m.now == nil {
		m.now = time.Now
	}
	if m.KillGrace == 0 {
		m.KillGrace = killGrace
	}
	if m.CancelWait == 0 {
		m.CancelWait = cancelWait
	}
	if m.DefaultAgent == "" {
		m.DefaultAgent = "claude"
	}
	defs := cfg.Catalog
	if defs == nil {
		defs = DefaultCatalog()
	}
	for _, d := range defs {
		m.catalog[d.ID] = d
		m.order = append(m.order, d.ID)
	}
	if err := m.loadChats(); err != nil {
		return nil, err
	}
	return m, nil
}

func apiErr(code, format string, a ...any) *ledger.Error {
	return &ledger.Error{Code: code, Message: fmt.Sprintf(format, a...)}
}

// Agents reports the catalog and which agents are installed.
func (m *Manager) Agents() []AgentInfo {
	out := make([]AgentInfo, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.catalog[id].Info())
	}
	return out
}

func (m *Manager) get(id string) *Chat {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chats[id]
}

func (m *Manager) must(id string) (*Chat, error) {
	if c := m.get(id); c != nil {
		return c, nil
	}
	return nil, apiErr("NOT_FOUND", "That chat does not exist.")
}

// Snapshot lists chats by project, newest first, and the agent catalog.
func (m *Manager) Snapshot() (chats map[string][]Info, agents []AgentInfo) {
	m.mu.Lock()
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		all = append(all, c)
	}
	m.mu.Unlock()
	chats = map[string][]Info{}
	for _, c := range all {
		info := c.Info()
		chats[info.ProjectID] = append(chats[info.ProjectID], info)
	}
	for _, list := range chats {
		sort.Slice(list, func(i, j int) bool {
			if list[i].UpdatedAt != list[j].UpdatedAt {
				return list[i].UpdatedAt > list[j].UpdatedAt
			}
			return list[i].ID < list[j].ID
		})
	}
	return chats, m.Agents()
}

// liveLimit refuses a new running agent past the limits. Caller holds m.mu.
func (m *Manager) liveLimit(projectID string, except *Chat) error {
	total, project := 0, 0
	for _, c := range m.chats {
		if c == except {
			continue
		}
		c.mu.Lock()
		live := c.state == StateStarting || c.state == StateIdle || c.state == StateRunning
		c.mu.Unlock()
		if live {
			total++
			if c.projectID == projectID {
				project++
			}
		}
	}
	if total >= MaxRunning {
		return apiErr("LIMIT_REACHED", "%d chats are running. Close one to start another.", MaxRunning)
	}
	if project >= MaxPerProject {
		return apiErr("LIMIT_REACHED", "This project has %d chats running. Close one to start another.", MaxPerProject)
	}
	return nil
}

// FolderUse is one chat that works in a folder.
type FolderUse struct {
	Dir  string // real working folder
	ID   string
	Live bool // the agent is running
}

// FolderUse lists the working folder of every chat of a project.
func (m *Manager) FolderUse(projectID string) []FolderUse {
	m.mu.Lock()
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		if c.projectID == projectID {
			all = append(all, c)
		}
	}
	m.mu.Unlock()
	out := make([]FolderUse, 0, len(all))
	for _, c := range all {
		c.mu.Lock()
		live := c.state == StateStarting || c.state == StateIdle || c.state == StateRunning
		out = append(out, FolderUse{Dir: c.dir, ID: c.id, Live: live})
		c.mu.Unlock()
	}
	return out
}

// CloseProject stops and forgets every chat of a project. The saved rows go
// with the project itself (foreign key).
func (m *Manager) CloseProject(projectID string) {
	m.mu.Lock()
	var gone []*Chat
	for id, c := range m.chats {
		if c.projectID == projectID {
			gone = append(gone, c)
			delete(m.chats, id)
		}
	}
	m.mu.Unlock()
	for _, c := range gone {
		c.discard()
	}
}

// Shutdown saves every chat and stops every agent with its whole process group.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	all := make([]*Chat, 0, len(m.chats))
	for _, c := range m.chats {
		all = append(all, c)
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, c := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.stopAgent(nil, StateStopped, nil)
			c.flush()
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownWait):
		m.log.Warn("chat: agents still stopping at shutdown")
	}
}
