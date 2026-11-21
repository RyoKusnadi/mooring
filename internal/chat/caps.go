package chat

import (
	"encoding/json"
	"strings"
)

// caps are the parts of the protocol the agent supports; the chat screen hides the rest.
// They are read once from the `initialize` reply.
type caps struct {
	Load            bool `json:"load"`
	Resume          bool `json:"resume"`
	Image           bool `json:"image"`
	EmbeddedContext bool `json:"embeddedContext"`
	MCPHTTP         bool `json:"mcpHttp"`
	MCPSSE          bool `json:"mcpSse"`
	// Audio is promptCapabilities.audio.
	Audio bool `json:"audio"`
	// MCPACP is mcpCapabilities.acp: the agent can reach tool servers the client serves over this connection.
	MCPACP bool `json:"mcpAcp"`
	// The session methods the agent lists under sessionCapabilities.
	List                  bool `json:"list"`
	Close                 bool `json:"close"`
	Delete                bool `json:"delete"`
	Fork                  bool `json:"fork"`
	SystemPrompt          bool `json:"systemPrompt"`
	AdditionalDirectories bool `json:"additionalDirectories"`
	// AuthMethods are the ids of the sign-in methods the agent offers.
	AuthMethods []string `json:"authMethods"`
}

type initResult struct {
	AgentCapabilities struct {
		LoadSession        bool `json:"loadSession"`
		PromptCapabilities struct {
			Image           bool `json:"image"`
			Audio           bool `json:"audio"`
			EmbeddedContext bool `json:"embeddedContext"`
		} `json:"promptCapabilities"`
		MCPCapabilities struct {
			HTTP bool `json:"http"`
			SSE  bool `json:"sse"`
			ACP  bool `json:"acp"`
		} `json:"mcpCapabilities"`
		SessionCapabilities map[string]json.RawMessage `json:"sessionCapabilities"`
	} `json:"agentCapabilities"`
	AuthMethods []struct {
		ID string `json:"id"`
	} `json:"authMethods"`
}

// caps reads what the agent advertises. A key the agent leaves out, or sends as
// null or false, is false; keys Mooring does not know are ignored.
func (r initResult) caps() caps {
	ac := r.AgentCapabilities
	has := func(key string) bool {
		return supported(ac.SessionCapabilities[key])
	}
	out := caps{
		Load: ac.LoadSession, Resume: has("resume"), Image: ac.PromptCapabilities.Image,
		Audio: ac.PromptCapabilities.Audio, EmbeddedContext: ac.PromptCapabilities.EmbeddedContext,
		MCPHTTP: ac.MCPCapabilities.HTTP, MCPSSE: ac.MCPCapabilities.SSE, MCPACP: ac.MCPCapabilities.ACP,
		List: has("list"), Close: has("close"), Delete: has("delete"), Fork: has("fork"),
		SystemPrompt: has("systemPrompt"), AdditionalDirectories: has("additionalDirectories"),
		AuthMethods: []string{},
	}
	for _, m := range r.AuthMethods {
		if m.ID != "" && len(out.AuthMethods) < 20 {
			out.AuthMethods = append(out.AuthMethods, m.ID)
		}
	}
	return out
}

// supported reads one capability value: an object (ACP marks support with {}) or
// true means supported. false, null, numbers, strings, arrays and anything else do not.
func supported(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "true" || strings.HasPrefix(s, "{")
}
