package chat

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/compact"
	"github.com/ryokusnadi/mooring/internal/paths"
)

// Limits on what a prompt carries along.
const (
	maxAttachments  = 8
	maxAttachBytes  = 64 << 10
	maxContextBytes = 16 << 10
)

// Attachment is what the browser sends: a selection of a file, or a saved command output.
type Attachment struct {
	Kind     string `json:"kind"` // "selection" or "output"
	Path     string `json:"path,omitempty"`
	From     int    `json:"from,omitempty"`
	To       int    `json:"to,omitempty"`
	Text     string `json:"text,omitempty"`
	OutputID string `json:"outputId,omitempty"`
}

// AttachRef is the chip the user message keeps: what was sent, without the text.
type AttachRef struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	From     int    `json:"from,omitempty"`
	To       int    `json:"to,omitempty"`
	OutputID string `json:"outputId,omitempty"`
	Lines    int    `json:"lines,omitempty"`
}

// sent is an attachment ready for the agent.
type sent struct {
	uri   string
	label string // "src/a.go, lines 3 to 9" or "command output"
	text  string
}

// resolveAttachments checks the attachments and reads what they point to.
func (c *Chat) resolveAttachments(list []Attachment) ([]sent, []AttachRef, error) {
	if len(list) > maxAttachments {
		return nil, nil, apiErr("BAD_REQUEST", "A message takes at most %d attachments.", maxAttachments)
	}
	var out []sent
	var refs []AttachRef
	total := 0
	for _, a := range list {
		var s sent
		var ref AttachRef
		var err error
		switch a.Kind {
		case "selection":
			s, ref, err = c.selection(a)
		case "output":
			s, ref, err = c.outputAttachment(a)
		default:
			err = apiErr("BAD_REQUEST", "An attachment is a selection or an output.")
		}
		if err != nil {
			return nil, nil, err
		}
		if total += len(s.text); total > maxAttachBytes {
			return nil, nil, apiErr("BAD_REQUEST", "The attachments are over %d KB together.", maxAttachBytes>>10)
		}
		out, refs = append(out, s), append(refs, ref)
	}
	return out, refs, nil
}

func (c *Chat) selection(a Attachment) (sent, AttachRef, error) {
	if a.Path == "" || a.From < 1 || a.To < a.From || a.Text == "" {
		return sent{}, AttachRef{}, apiErr("BAD_REQUEST", "A selection needs a path, a line range and its text.")
	}
	real, err := paths.Resolve(c.projectDir, a.Path)
	if errors.Is(err, paths.ErrOutsideRoot) {
		return sent{}, AttachRef{}, apiErr("OUTSIDE_ROOT", "The file must be inside the project.")
	}
	if err != nil {
		return sent{}, AttachRef{}, apiErr("BAD_REQUEST", "That path cannot be used.")
	}
	shown := real
	if root, err := paths.Root(c.projectDir); err == nil {
		if rel, err := filepath.Rel(root, real); err == nil && !strings.HasPrefix(rel, "..") {
			shown = rel
		}
	}
	label := fmt.Sprintf("%s, lines %d to %d", shown, a.From, a.To)
	return sent{uri: fmt.Sprintf("file://%s#L%d-%d", real, a.From, a.To), label: label, text: a.Text},
		AttachRef{Kind: "selection", Path: shown, From: a.From, To: a.To}, nil
}

// outputAttachment sends a saved output; a long one goes through the compact filter first.
func (c *Chat) outputAttachment(a Attachment) (sent, AttachRef, error) {
	if !idPattern.MatchString(a.OutputID) {
		return sent{}, AttachRef{}, apiErr("BAD_REQUEST", "That output id is not valid.")
	}
	raw, err := c.m.loadOutput(c.id, a.OutputID)
	if errors.Is(err, errNoOutput) {
		return sent{}, AttachRef{}, apiErr("NOT_FOUND", "That output is no longer kept.")
	}
	if err != nil {
		return sent{}, AttachRef{}, apiErr("INTERNAL", "The output could not be read.")
	}
	text, _ := compact.Apply(raw)
	return sent{uri: "mooring://output/" + a.OutputID, label: "command output", text: text},
		AttachRef{Kind: "output", OutputID: a.OutputID, Lines: compact.CountLines(raw)}, nil
}

// fence wraps text in a code fence longer than any run of backticks inside it.
func fence(text string) string {
	n, run := 3, 0
	for _, r := range text {
		if r == '`' {
			run++
			n = max(n, run+1)
		} else {
			run = 0
		}
	}
	f := strings.Repeat("`", n)
	return f + "\n" + strings.TrimSuffix(text, "\n") + "\n" + f
}

func quote(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return strings.Join(lines, "\n")
}

func resource(uri, text string) map[string]any {
	return map[string]any{"type": "resource", "resource": map[string]string{"uri": uri, "mimeType": "text/plain", "text": text}}
}

// promptBlocks builds the content blocks of a prompt. With embedded context the
// context and the attachments are resource blocks; without it they are text in
// the one text block: the context quoted before the user's words, fenced attachments after.
func promptBlocks(text string, embedded bool, context, fromChat string, atts []sent) []any {
	if embedded {
		var blocks []any
		if context != "" {
			blocks = append(blocks, resource("mooring://summary/"+fromChat, context))
		}
		blocks = append(blocks, map[string]string{"type": "text", "text": text})
		for _, a := range atts {
			blocks = append(blocks, resource(a.uri, a.text))
		}
		return blocks
	}
	var b strings.Builder
	if context != "" {
		b.WriteString("Summary of an earlier chat:\n" + quote(context) + "\n\n")
	}
	b.WriteString(text)
	for _, a := range atts {
		b.WriteString("\n\n" + a.label + ":\n" + fence(a.text))
	}
	return []any{map[string]string{"type": "text", "text": b.String()}}
}
