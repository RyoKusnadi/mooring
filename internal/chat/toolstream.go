package chat

import (
	"encoding/json"
	"time"
)

// streamEvery is the least time between two publishes of one streamed tool line.
const streamEvery = 150 * time.Millisecond

// toolStream is the output a command has shown so far. claude-agent-acp 0.108
// sends it in `_meta.terminal_output_delta` (append) and `_meta.terminal_output`
// (replace) of tool_call_update, to a client that declared
// `_meta.terminal_output_delta` at initialize.
type toolStream struct {
	text  string
	last  time.Time
	timer *time.Timer
}

type terminalChunk struct {
	Data string `json:"data"`
}

// terminalText reads the streamed output of an update: the text, whether it
// replaces what was shown, and whether the update carried any.
func terminalText(meta map[string]json.RawMessage) (text string, replace, ok bool) {
	for _, key := range []string{"terminal_output_delta", "terminal_output"} {
		raw, has := meta[key]
		if !has {
			continue
		}
		var t terminalChunk
		if json.Unmarshal(raw, &t) == nil {
			return t.Data, key == "terminal_output", true
		}
	}
	return "", false, false
}

// onlyTerminal reports whether every content block just points at a terminal.
// Such a block carries no output: the text comes from the _meta fields.
func onlyTerminal(raw json.RawMessage) bool {
	var blocks []struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &blocks) != nil || len(blocks) == 0 {
		return false
	}
	for _, b := range blocks {
		if b.Type != "terminal" {
			return false
		}
	}
	return true
}

// streamLocked applies streamed output to the tool message at i and returns true
// when the update carried some. It keeps the tail when the text grows past maxBlob.
func (c *Chat) streamLocked(i int, u update) bool {
	text, replace, ok := terminalText(u.Meta)
	if !ok {
		return false
	}
	if c.streams == nil {
		c.streams = map[string]*toolStream{}
	}
	s := c.streams[u.ToolID]
	if s == nil {
		s = &toolStream{}
		c.streams[u.ToolID] = s
	}
	if replace {
		s.text = text
	} else {
		s.text += text
	}
	if len(s.text) > maxBlob/2 {
		s.text = "(earlier output left out)\n" + s.text[len(s.text)-maxBlob/2:]
	}
	c.msgs[i].Streamed = true
	if s.text == "" {
		c.msgs[i].Content = nil
		return true
	}
	block, _ := json.Marshal([]map[string]any{{"type": "content", "content": map[string]string{"type": "text", "text": s.text}}})
	c.msgs[i].Content = block
	return true
}

// publishToolLocked sends the tool message at i. A partial update (streamed output
// with no status change) goes out at most once per streamEvery; the rest waits for
// a timer, so ten quick chunks make a few publishes, not ten.
func (c *Chat) publishToolLocked(i int, id string, partial bool) {
	s := c.streams[id]
	if s == nil || !partial {
		if s != nil && s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		c.updateLocked(i)
		return
	}
	wait := streamEvery - time.Since(s.last)
	if wait <= 0 {
		s.last = time.Now()
		c.updateLocked(i)
		return
	}
	if s.timer != nil {
		return
	}
	s.timer = time.AfterFunc(wait, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if s.timer == nil {
			return
		}
		s.timer, s.last = nil, time.Now()
		if j, ok := c.tools[id]; ok && j < len(c.msgs) {
			c.updateLocked(j)
		}
	})
}

// endStreamLocked frees the stream of a tool call that finished.
func (c *Chat) endStreamLocked(id string) {
	if s := c.streams[id]; s != nil && s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	delete(c.streams, id)
}

// dropStreamsLocked stops every pending publish and frees every stream: the agent stopped or the chat went away.
func (c *Chat) dropStreamsLocked() {
	for id := range c.streams {
		c.endStreamLocked(id)
	}
}
