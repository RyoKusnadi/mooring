package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// Spec is the session spec: the five slots every chat is started from. The
// defaults are plain: the agent's own login, the project folder, no tool
// servers, no output filters. Later phases fill the slots without changing the
// chat code.
type Spec struct {
	Agent       AgentSlot    `json:"agent"`
	Folder      FolderSlot   `json:"folder"`
	ToolServers []ToolServer `json:"toolServers"`
	Account     AccountSlot  `json:"account"`
	Filters     []string     `json:"filters"`
	// Imported marks a chat made on a session from the agent's own program (chat.import).
	Imported bool `json:"imported,omitempty"`
	// AgentMade: the session was made in the agent's own program, so removing the chat
	// leaves the agent's copy alone. Never cleared.
	AgentMade bool `json:"agentMade,omitempty"`
}

// AgentSlot picks a catalog entry.
type AgentSlot struct {
	ID string `json:"id"`
}

// FolderSlot is the working folder: empty means the project folder. A path
// must be inside the project. Worktree asks chat.start for a new git worktree
// (on Branch, or a generated one); the saved spec then holds its folder in Path.
type FolderSlot struct {
	Path     string `json:"path"`
	Worktree bool   `json:"worktree,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

// AccountSlot names an account of the agent (internal/accounts). The browser sends
// the id only; "" means the project's account, which start fills in. Mooring
// finds the folder from the id and never reads what is inside it.
type AccountSlot struct {
	ID string `json:"id"`
}

// ToolServer is an MCP server of a chat. The browser sends {"id"} only, naming a
// saved tool server; Resolve fills in the rest at start and resume, and the chat
// saves just the id. Type is "stdio", "http" or "sse". Env and Headers values are
// secrets, never saved with the chat. Entries without an id are the phase 4 inline
// form: still read from saved chats, then left out with a notice.
type ToolServer struct {
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name"`
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

const maxToolServers, maxFilters = 16, 16

var serverName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// normalize fills defaults and checks the spec. projectDir is the project's
// resolved folder.
func (s *Spec) normalize(catalog map[string]AgentDef, defaultAgent, projectDir string) (dir string, err error) {
	if s.Agent.ID == "" {
		s.Agent.ID = defaultAgent
	}
	_, ok := catalog[s.Agent.ID]
	if !ok {
		return "", apiErr("NOT_FOUND", "There is no agent called %q.", s.Agent.ID)
	}
	dir = projectDir
	if s.Folder.Path != "" {
		dir, err = paths.Resolve(projectDir, s.Folder.Path)
		if err != nil {
			return "", apiErr("OUTSIDE_ROOT", "The working folder must be inside the project.")
		}
		s.Folder.Path = dir
	}
	if len(s.ToolServers) > maxToolServers || len(s.Filters) > maxFilters {
		return "", apiErr("BAD_REQUEST", "A chat takes at most %d tool servers and %d filters.", maxToolServers, maxFilters)
	}
	seen := map[string]bool{}
	for i := range s.ToolServers {
		t := &s.ToolServers[i]
		if t.ID != "" {
			if !idPattern.MatchString(t.ID) || seen[t.ID] {
				return "", apiErr("BAD_REQUEST", "Tool server %q is not valid.", t.ID)
			}
			seen[t.ID] = true
			continue
		}
		if t.Type == "" {
			t.Type = "stdio"
		}
		bad := !serverName.MatchString(t.Name) || seen[t.Name] ||
			(t.Type == "stdio" && t.Command == "") || (t.Type != "stdio" && t.URL == "") ||
			(t.Type != "stdio" && t.Type != "http" && t.Type != "sse")
		if bad {
			return "", apiErr("BAD_REQUEST", "Tool server %q is not valid.", t.Name)
		}
		seen[t.Name] = true
	}
	if s.ToolServers == nil {
		s.ToolServers = []ToolServer{}
	}
	if s.Filters, err = checkFilters(s.Filters); err != nil {
		return "", err
	}
	return dir, nil
}

// redacted is the spec as saved: secret values blanked, names kept.
func (s Spec) redacted() Spec {
	out := s
	out.ToolServers = make([]ToolServer, len(s.ToolServers))
	for i, t := range s.ToolServers {
		if t.ID != "" {
			out.ToolServers[i] = ToolServer{ID: t.ID}
			continue
		}
		t.Env, t.Headers = blank(t.Env), blank(t.Headers)
		out.ToolServers[i] = t
	}
	return out
}

func blank(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k := range m {
		out[k] = ""
	}
	return out
}

func pairs(m map[string]string, key string) []map[string]string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]map[string]string, 0, len(names))
	for _, k := range names {
		out = append(out, map[string]string{"name": k, key: m[k]})
	}
	return out
}

// mcpServers builds the ACP list from the chat's tool servers. A server that cannot
// be used is left out and named in notes with the reason: the agent did not announce
// its kind, the saved server is gone or off, or it is the inline form of old chats.
func (c *Chat) mcpServers(ctx context.Context, cp caps) (list []any, notes []string) {
	list = []any{}
	for _, ref := range c.spec.ToolServers {
		if ref.ID == "" {
			notes = append(notes, fmt.Sprintf("The tool server %q was set up before tool servers were saved in Mooring, so this chat starts without it. Add it under Tool servers and start a new chat to use it.", ref.Name))
			continue
		}
		var t ToolServer
		var err error
		if c.m.Tools != nil {
			t, err = c.m.Tools.Resolve(ctx, ref.ID)
		} else {
			err = errors.New("tool servers are not available")
		}
		if err != nil {
			name := t.Name
			if name == "" {
				name = ref.ID
			}
			notes = append(notes, fmt.Sprintf("The tool server %q was left out: %s.", name, err))
			continue
		}
		switch t.Type {
		case "stdio":
			args := t.Args
			if args == nil {
				args = []string{}
			}
			list = append(list, map[string]any{"name": t.Name, "command": t.Command, "args": args, "env": pairs(t.Env, "value")})
		case "http", "sse":
			if (t.Type == "http" && !cp.MCPHTTP) || (t.Type == "sse" && !cp.MCPSSE) {
				notes = append(notes, fmt.Sprintf("The tool server %q was left out: %s cannot use %s servers.", t.Name, c.m.catalog[c.spec.Agent.ID].Name, t.Type))
				continue
			}
			list = append(list, map[string]any{"type": t.Type, "name": t.Name, "url": t.URL, "headers": pairs(t.Headers, "value")})
		}
	}
	return list, notes
}

// launchFor turns a spec into the process to start (the one launch path).
// accountEnv is the account's folder variable, from accounts.Service.Env.
func (s Spec) launchFor(def AgentDef, dir string, accountEnv []string) launch {
	env := append(append([]string{}, def.Env...), accountEnv...)
	return launch{command: def.Command, args: def.Args, dir: dir, env: env}
}

func (s Spec) marshal() string {
	raw, _ := json.Marshal(s.redacted())
	return string(raw)
}
