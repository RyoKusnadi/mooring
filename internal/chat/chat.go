package chat

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ryokusnadi/mooring/internal/events"
)

// Chat states.
const (
	StateStarting   = "starting"
	StateIdle       = "idle"
	StateRunning    = "running"
	StateNeedsLogin = "needs_login"
	StateStopped    = "stopped"
)

// Choice is one pick in a list the agent offers (a mode or a model).
type Choice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Command is a slash command the agent announced.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Usage is the context meter and, when the agent reports it, cost and limits.
type Usage struct {
	Used      int64           `json:"used"`
	Size      int64           `json:"size"`
	Cost      float64         `json:"cost,omitempty"`
	Currency  string          `json:"currency,omitempty"`
	RateLimit json.RawMessage `json:"rateLimit,omitempty"` // agent's own object, passed through
}

// ErrInfo says why a chat is not working.
type ErrInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Info is a chat without its messages: what the sidebar and header need.
type Info struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Agent     string    `json:"agent"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Error     *ErrInfo  `json:"error"`
	Attention bool      `json:"attention"` // a permission request is waiting, or the account hit a limit
	LimitOnly bool      `json:"limitOnly"` // the attention is for the limit alone: no permission is waiting
	Mode      string    `json:"mode"`
	Modes     []Choice  `json:"modes"`
	Model     string    `json:"model"`
	Models    []Choice  `json:"models"`
	Usage     *Usage    `json:"usage"`
	Caps      caps      `json:"caps"`
	Commands  []Command `json:"commands"`
	Spec      Spec      `json:"spec"`
	SwitchTo  *string   `json:"switchTo"` // after a limit: the account to move to, if any
	Messages  int       `json:"messages"`
	LastLine  string    `json:"lastLine"`           // the end of the newest message of the user or the agent, scrubbed
	FromChat  string    `json:"fromChat,omitempty"` // the chat whose summary this one started from
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
}

// Chat is one conversation: its messages, its stream and, while it runs, its agent.
type Chat struct {
	m          *Manager
	id         string
	projectID  string
	projectDir string
	dir        string
	spec       Spec
	created    int64
	fromChat   string

	// pubMu keeps "build the info, publish it" one step, so two updates
	// cannot reach the browser in the wrong order. It is taken before mu.
	pubMu sync.Mutex
	hub   *events.Hub // per-chat message stream

	mu        sync.Mutex // lock order: pubMu, mu, hub
	title     string
	state     string
	errInfo   *ErrInfo
	updated   int64
	sessionID string
	caps      caps
	mode      string
	modes     []Choice
	model     string
	modelCfg  string // config option id that holds the model; "" when the agent uses set_model
	models    []Choice
	usage     *Usage
	commands  []Command
	removed   bool

	// Token saving: raw usage per turn, whether the agent named the chat, how many
	// terminal/* calls it made, and outputs not yet shown on a tool message.
	usages      []TurnUsage
	agentTitled bool
	autoTitle   bool // the title is still the first line Mooring made; a title job may replace it
	titleTried  bool
	termCalls   int
	pendingOut  map[string]OutputRef // by terminal id

	loaded     bool
	savedCount int    // message count before they are loaded
	savedLast  string // last line before they are loaded
	msgs       []Message
	openIdx    int // message that streamed text is appended to; -1 when none
	planIdx    int
	tools      map[string]int
	streams    map[string]*toolStream // streamed command output by tool id
	perms      map[string]*pendingPerm
	dirty      map[int]struct{}
	flushT     *time.Timer
	flushMu    sync.Mutex
	replaying  bool

	// limited: the last turn stopped for the account's limit. turnRejected and
	// turnResetsAt collect what the agent said about it during the turn.
	limited      bool
	moving       bool // MoveAccount is changing the account: no prompt or resume until it is done
	bootGen      int  // counts stops; a boot that started before the last stop gives up
	turnRejected bool
	turnResetsAt int64

	cur        *run
	turn       int
	turnCancel context.CancelFunc
	// promptSent closes once the turn's session/prompt is written, so a cancel
	// never reaches the agent before the prompt it cancels.
	promptSent chan struct{}
	cancelling bool
}

func newChat(m *Manager, id, projectID, projectDir, dir string, spec Spec) (*Chat, error) {
	hub, err := events.NewHub(m.log)
	if err != nil {
		return nil, err
	}
	now := m.now().UnixMilli()
	return &Chat{
		m: m, id: id, projectID: projectID, projectDir: projectDir, dir: dir, spec: spec,
		created: now, updated: now, hub: hub, state: StateStopped, loaded: true,
		openIdx: -1, planIdx: -1, pendingOut: map[string]OutputRef{}, tools: map[string]int{}, perms: map[string]*pendingPerm{}, dirty: map[int]struct{}{},
		modes: []Choice{}, models: []Choice{}, commands: []Command{},
	}, nil
}

func (c *Chat) infoLocked() Info {
	return Info{
		ID: c.id, ProjectID: c.projectID, Agent: c.spec.Agent.ID, Title: c.title, State: c.state,
		Error: c.errInfo, Attention: len(c.perms) > 0 || c.limited, LimitOnly: c.limited && len(c.perms) == 0, Mode: c.mode, Modes: c.modes, Model: c.model,
		Models: c.models, Usage: c.usage, Caps: c.caps, Commands: c.commands, Spec: c.spec.redacted(), SwitchTo: c.switchToLocked(),
		Messages: c.count(), LastLine: c.lastLineLocked(), FromChat: c.fromChat, CreatedAt: c.created, UpdatedAt: c.updated,
	}
}

// Info returns the chat's current state.
func (c *Chat) Info() Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.infoLocked()
}

// publishInfo saves the chat row and tells every device. Call it without mu.
func (c *Chat) publishInfo(kind string) {
	c.pubMu.Lock()
	defer c.pubMu.Unlock()
	c.mu.Lock()
	info := c.infoLocked()
	removed := c.removed
	c.mu.Unlock()
	if removed {
		return
	}
	c.m.saveRow(info, c.sessionIDSafe())
	c.m.pub.Publish(kind, info)
}

func (c *Chat) sessionIDSafe() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

func (c *Chat) touchLocked() { c.updated = c.m.now().UnixMilli() }

// addLocked appends a message to the log and the stream.
func (c *Chat) addLocked(msg Message) int {
	msg.ID = len(c.msgs)
	msg.At = c.m.now().UnixMilli()
	c.msgs = append(c.msgs, msg)
	c.touchLocked()
	c.hub.Publish("set", change{Msg: &msg, ID: msg.ID})
	c.markDirtyLocked(msg.ID)
	return msg.ID
}

// updateLocked announces the new whole state of message i.
func (c *Chat) updateLocked(i int) {
	msg := c.msgs[i]
	c.touchLocked()
	c.hub.Publish("set", change{Msg: &msg, ID: i})
	c.markDirtyLocked(i)
}

// chunkLocked adds streamed text to the open message of the same kind, or opens one.
func (c *Chat) chunkLocked(kind, text string) {
	if text == "" {
		return
	}
	if i := c.openIdx; i >= 0 && c.msgs[i].Kind == kind {
		c.msgs[i].Text += text
		c.touchLocked()
		c.hub.Publish("append", change{ID: i, Text: text})
		c.markDirtyLocked(i)
		return
	}
	c.openIdx = c.addLocked(Message{Kind: kind, Text: text})
}

func (c *Chat) noticeLocked(text string) {
	c.openIdx = -1
	c.addLocked(Message{Kind: KindNotice, Text: text})
}

// snapshotBody is what a client that must reset starts from.
type snapshotBody struct {
	Messages []Message `json:"messages"`
	Info     Info      `json:"info"`
}

// attach decides resume or reset for a socket.
func (c *Chat) attach(epoch string, pos uint64) (*events.Attachment, error) {
	if err := c.load(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hub.Attach(epoch, pos, func() (any, error) {
		return snapshotBody{Messages: append([]Message{}, c.msgs...), Info: c.infoLocked()}, nil
	})
}

// lastLineLocked is the list line of the chat; the saved one is used until the messages are loaded.
func (c *Chat) lastLineLocked() string {
	if !c.loaded {
		return c.savedLast
	}
	return lastLineOf(c.msgs)
}

func (c *Chat) count() int {
	if !c.loaded {
		return c.savedCount
	}
	return len(c.msgs)
}
