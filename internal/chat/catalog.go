// Package chat runs agent chats: it starts an ACP agent in the project folder,
// keeps each chat's message stream, and saves chats so they survive a restart.
package chat

import (
	"os/exec"

	"github.com/ryokusnadi/mooring/internal/accounts"
)

// ClaudeAdapter is the pinned Claude adapter.
const ClaudeAdapter = "@agentclientprotocol/claude-agent-acp@0.108.1"

// AgentDef describes how to start one agent. Nothing here installs anything.
type AgentDef struct {
	ID      string
	Name    string
	Command string
	Args    []string
	Env     []string // extra KEY=VALUE for every start
	// Needs lists the programs that must be on PATH for the agent to start.
	Needs []string
	// Account says how to point the agent at another login folder, and how to read
	// its usage. Nil when the agent has only its own login: only an agent whose folder setting passed a proof has one.
	Account *accounts.Adapter
	// Tested is true when Mooring ran this agent end to end.
	Tested bool
	// Note is a plain sentence about the agent, shown in the catalog.
	Note string
}

// AgentInfo is one catalog entry as the browser sees it.
type AgentInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Tested    bool   `json:"tested"`
	Missing   string `json:"missing,omitempty"` // the first program not found
	Note      string `json:"note,omitempty"`
}

// DefaultCatalog lists the agents Mooring knows. Only Claude is tested; the
// others follow what their documentation says.
func DefaultCatalog() []AgentDef {
	return []AgentDef{
		{
			ID: "claude", Name: "Claude", Command: "npx", Args: []string{"-y", ClaudeAdapter},
			Needs: []string{"npx"}, Tested: true,
			Account: &accounts.Adapter{Env: "CLAUDE_CONFIG_DIR", Login: "claude, then /login", Shared: "projects", Usage: "claude", Home: ".claude"},
			Note:    "Anthropic's coding agent. Sign in with `claude auth login`.",
		},
		{
			ID: "gemini", Name: "Gemini", Command: "gemini", Args: []string{"--acp"},
			Needs: []string{"gemini"}, Note: "Google's coding agent.",
		},
		{
			ID: "codex", Name: "Codex", Command: "npx", Args: []string{"-y", "@agentclientprotocol/codex-acp"},
			Needs: []string{"npx", "codex"},
			Note:  "OpenAI's coding agent.",
		},
		{
			ID: "grok", Name: "Grok", Command: "grok", Args: []string{"agent", "stdio"},
			Needs: []string{"grok"}, Note: "xAI's coding agent. Sign in with `grok login` or set XAI_API_KEY.",
		},
		{
			ID: "cursor", Name: "Cursor", Command: "agent", Args: []string{"acp"},
			Needs: []string{"agent"}, Note: "Cursor's coding agent.",
		},
		{
			ID: "antigravity", Name: "Antigravity", Command: "agy", Args: []string{"acp"},
			Needs: []string{"agy"}, Note: "Google Antigravity's agent (agy 1.5 or later). Sign in once by running `agy`.",
		},
	}
}

// Info reports whether the programs the agent needs are on PATH.
func (d AgentDef) Info() AgentInfo {
	in := AgentInfo{ID: d.ID, Name: d.Name, Tested: d.Tested, Note: d.Note, Installed: true}
	for _, prog := range d.Needs {
		if _, err := exec.LookPath(prog); err != nil {
			in.Installed, in.Missing = false, prog
			break
		}
	}
	return in
}

// Adapters collects the account adapters of the agents that have one, by agent id.
func Adapters(defs []AgentDef) (adapters map[string]accounts.Adapter, names map[string]string) {
	adapters, names = map[string]accounts.Adapter{}, map[string]string{}
	for _, d := range defs {
		names[d.ID] = d.Name
		if d.Account != nil {
			adapters[d.ID] = *d.Account
		}
	}
	return adapters, names
}
