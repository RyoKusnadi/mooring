package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	summaryPrompt = "Summarise this chat so a new chat can continue it, in at most 250 words: goal, decisions, files changed, current state, next step. No preamble."
	maxSummary    = 8 << 10
	summaryWait   = 2 * time.Minute
)

// capText cuts s to at most n bytes on a character border.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// Summarize writes the text a fresh chat can start from. An idle chat is asked
// through its own agent; otherwise it is built from the stored messages.
func (c *Chat) Summarize() (text, source string, err error) {
	if err := c.load(); err != nil {
		return "", "", apiErr("INTERNAL", "The saved chat could not be read.")
	}
	c.mu.Lock()
	state, limited, moving := c.state, c.limited, c.moving
	c.mu.Unlock()
	if state == StateStarting || state == StateRunning || moving {
		return "", "", apiErr("BUSY", "The chat is busy. Wait for it, or cancel the reply.")
	}
	if state == StateIdle && !limited {
		if text := c.askSummary(); text != "" {
			return text, "agent", nil
		}
	}
	return c.summaryFromMessages(), "messages", nil
}

// askSummary sends the summary prompt as a job turn and returns the reply, or ""
// when the turn did not give one.
func (c *Chat) askSummary() string {
	done, idx, err := c.beginTurn(turnReq{text: summaryPrompt, job: "summary"})
	if err != nil {
		return ""
	}
	select {
	case <-done:
	case <-time.After(summaryWait):
		c.Cancel()
		<-done
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != StateIdle || c.limited {
		return ""
	}
	var b strings.Builder
	for _, m := range c.msgs[idx+1:] {
		if m.Kind == KindAgent {
			b.WriteString(m.Text)
		}
	}
	return capText(strings.TrimSpace(b.String()), maxSummary)
}

// summaryFromMessages builds a summary with no model: the first user message, the
// last three, the last agent reply, the files the tools touched and the last plan.
func (c *Chat) summaryFromMessages() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var users []string
	lastAgent := ""
	var files []string
	seen := map[string]bool{}
	var plan json.RawMessage
	for _, m := range c.msgs {
		switch m.Kind {
		case KindUser:
			if m.Job == "" {
				users = append(users, m.Text)
			}
		case KindAgent:
			lastAgent = m.Text
		case KindPlan:
			plan = m.Entries
		case KindTool:
			for _, p := range locationPaths(m.Locations) {
				if !seen[p] {
					seen[p] = true
					files = append(files, p)
				}
			}
		}
	}
	var b strings.Builder
	if len(users) > 0 {
		b.WriteString("First request:\n" + capText(users[0], 1500) + "\n\n")
	}
	if recent := users[min(1, len(users)):]; len(recent) > 0 {
		recent = recent[max(len(recent)-3, 0):]
		b.WriteString("Latest requests:\n")
		for _, u := range recent {
			b.WriteString("- " + capText(strings.ReplaceAll(u, "\n", " "), 500) + "\n")
		}
		b.WriteString("\n")
	}
	if lastAgent != "" {
		b.WriteString("Last reply of the agent:\n" + tail(lastAgent, 2<<10) + "\n\n")
	}
	if len(files) > 0 {
		b.WriteString("Files the agent worked on:\n")
		for _, f := range files[:min(len(files), 40)] {
			b.WriteString("- " + f + "\n")
		}
		b.WriteString("\n")
	}
	if steps := planSteps(plan); steps != "" {
		b.WriteString("Last plan:\n" + steps)
	}
	return capText(strings.TrimSpace(b.String()), maxSummary)
}

// tail keeps the last n bytes of s, on a character border.
func tail(s string, n int) string {
	t, _ := lastBytes(s, n)
	return t
}

func locationPaths(raw json.RawMessage) []string {
	var locs []struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(raw, &locs) != nil {
		return nil
	}
	var out []string
	for _, l := range locs {
		if l.Path != "" {
			out = append(out, l.Path)
		}
	}
	return out
}

func planSteps(raw json.RawMessage) string {
	var entries []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	if json.Unmarshal(raw, &entries) != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- [%s] %s\n", e.Status, capText(e.Content, 200))
	}
	return b.String()
}

// addContext puts the summary a fresh chat starts from at the top of its messages.
func (c *Chat) addContext(text, fromChat string) {
	c.mu.Lock()
	c.addLocked(Message{Kind: KindContext, Text: text, FromChat: fromChat})
	c.mu.Unlock()
	c.flush()
}
