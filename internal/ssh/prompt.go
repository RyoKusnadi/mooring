package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// prompt is a question waiting for the user: a host key, a password, a key
// passphrase or a keyboard-interactive question. Its fields are the event.
type prompt struct {
	ID          string `json:"promptId"`
	ConnID      string `json:"connId"`
	Host        string `json:"host"` // user@address:port
	Kind        string `json:"kind"`
	Text        string `json:"text,omitempty"`
	Echo        bool   `json:"echo"`
	KeyType     string `json:"keyType,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Retry       bool   `json:"retry"` // asks again after the last answer was refused

	ans chan answer
}

type answer struct {
	value    string
	remember bool
	accept   bool
	cancel   bool
}

// ask publishes a prompt and waits for the first answer, a cancel, the timeout
// or the end of the connection. A cancel and a timeout are CANCELLED.
func (m *Manager) ask(ctx context.Context, c *Conn, p *prompt) (answer, error) {
	p.ConnID, p.Host, p.ans = c.id, c.hostLabel(), make(chan answer, 1)
	m.mu.Lock()
	m.nextID++
	p.ID = fmt.Sprintf("p%d-%d", m.nextID, time.Now().UnixNano()%1_000_000)
	m.prompts[p.ID] = p
	m.mu.Unlock()
	m.Pub.Publish("ssh.prompt", p)
	defer func() {
		m.mu.Lock()
		delete(m.prompts, p.ID)
		m.mu.Unlock()
		m.Pub.Publish("ssh.prompt.closed", map[string]string{"promptId": p.ID})
	}()
	c.handshakeClock(false) // the user's time is not the server's
	defer c.handshakeClock(true)
	timer := time.NewTimer(m.PromptTimeout)
	defer timer.Stop()
	select {
	case a := <-p.ans:
		if a.cancel {
			return a, apiErr("CANCELLED", "The question was dismissed.")
		}
		return a, nil
	case <-timer.C:
		return answer{}, apiErr("CANCELLED", "Nobody answered the question in %d minutes.", int(m.PromptTimeout/time.Minute))
	case <-ctx.Done():
		return answer{}, apiErr("CANCELLED", "The connection was stopped.")
	}
}

type answerArgs struct {
	PromptID string `json:"promptId"`
	Value    string `json:"value"`
	Remember bool   `json:"remember"`
	Accept   *bool  `json:"accept"`
	Cancel   bool   `json:"cancel"`
}

// Answer gives the first answer to a prompt. Later answers find nothing to answer.
func (m *Manager) Answer(raw json.RawMessage) error {
	var a answerArgs
	if err := json.Unmarshal(raw, &a); err != nil || a.PromptID == "" {
		return apiErr("BAD_REQUEST", "The command arguments are not valid.")
	}
	m.mu.Lock()
	p := m.prompts[a.PromptID]
	if p != nil && p.Kind == "hostkey" && a.Accept == nil && !a.Cancel {
		m.mu.Unlock()
		return apiErr("BAD_REQUEST", "Say whether to accept the host key.") // the prompt stays open
	}
	if p != nil {
		delete(m.prompts, a.PromptID)
	}
	m.mu.Unlock()
	if p == nil {
		return apiErr("NOT_FOUND", "That question was already answered or has ended.")
	}
	ans := answer{value: a.Value, remember: a.Remember, cancel: a.Cancel}
	if p.Kind == "hostkey" {
		ans.accept = a.Accept != nil && *a.Accept
	}
	p.ans <- ans
	return nil
}
