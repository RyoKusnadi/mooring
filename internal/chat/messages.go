package chat

import "encoding/json"

// Message kinds in a chat's stream.
const (
	KindUser       = "user"
	KindAgent      = "agent"
	KindThought    = "thought"
	KindTool       = "tool"
	KindPermission = "permission"
	KindPlan       = "plan"
	KindNotice     = "notice"
	KindError      = "error"
	KindContext    = "context" // the summary a fresh chat starts from
)

// Permission states.
const (
	PermPending   = "pending"
	PermAnswered  = "answered"
	PermCancelled = "cancelled"
)

// maxBlob caps stored raw input and tool output per message.
const maxBlob = 32 << 10

// Option is one answer the agent offers to a permission request.
type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // allow_once, allow_always, reject_once, reject_always
}

// Message is one item of a chat. Kind decides which fields are used.
type Message struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	At   int64  `json:"at"` // Unix milliseconds

	Text string `json:"text,omitempty"` // user, agent, thought, notice, error

	// tool, permission
	ToolID    string          `json:"toolId,omitempty"`
	Title     string          `json:"title,omitempty"`
	ToolKind  string          `json:"toolKind,omitempty"` // read, edit, execute, ...
	Status    string          `json:"status,omitempty"`   // tool: pending, in_progress, completed, failed
	RawInput  json.RawMessage `json:"rawInput,omitempty"`
	Locations json.RawMessage `json:"locations,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // agent's content blocks; diffs live here
	Truncated bool            `json:"truncated,omitempty"`
	Streamed  bool            `json:"-"` // a running command sent its output as it ran; the terminal block carries none

	// tool: the raw output of a command the agent ran through Mooring
	Outputs []OutputRef `json:"outputs,omitempty"`

	// user: a job prompt (such as "summary") and what the user attached
	Job         string      `json:"job,omitempty"`
	Attachments []AttachRef `json:"attachments,omitempty"`

	// context
	FromChat string `json:"fromChat,omitempty"`

	// permission
	RequestID string   `json:"requestId,omitempty"`
	Options   []Option `json:"options,omitempty"`
	State     string   `json:"state,omitempty"`
	Chosen    string   `json:"chosen,omitempty"`

	// plan
	Entries json.RawMessage `json:"entries,omitempty"`

	// error
	Code     string `json:"code,omitempty"`
	ResetsAt int64  `json:"resetsAt,omitempty"` // LIMIT_REACHED: when the limit ends (ms), if known
}

// change is the data of a stream event. Event kind "set" carries Msg (a new
// message or the whole new state of one), "append" carries ID and Text.
type change struct {
	Msg  *Message `json:"msg,omitempty"`
	ID   int      `json:"id"`
	Text string   `json:"text,omitempty"`
}

// capBlob shortens raw JSON that is too big to keep, and says so.
func capBlob(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) <= maxBlob {
		return raw, false
	}
	return json.RawMessage(`"(too long to keep)"`), true
}
