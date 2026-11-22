package chat

import "encoding/json"

type configOption struct {
	ID           string          `json:"id"`
	Category     string          `json:"category"`
	CurrentValue json.RawMessage `json:"currentValue"`
	Options      []struct {
		Value   string `json:"value"`
		Name    string `json:"name"`
		Options []struct {
			Value string `json:"value"`
			Name  string `json:"name"`
		} `json:"options"` // a group holds its choices here
	} `json:"options"`
}

// applyConfigLocked reads the model from the config option with category
// "model" (the adapter has no `models` key).
func (c *Chat) applyConfigLocked(raw []json.RawMessage) {
	if id, cur, choices, ok := modelOption(raw); ok {
		c.model, c.modelCfg, c.models = cur, id, choices
	}
}

// modelOption finds the config option with category "model": its id, the current
// value and the choices (groups flattened).
func modelOption(raw []json.RawMessage) (id, cur string, choices []Choice, ok bool) {
	for _, r := range raw {
		var o configOption
		if json.Unmarshal(r, &o) != nil || o.Category != "model" {
			continue
		}
		_ = json.Unmarshal(o.CurrentValue, &cur)
		choices = []Choice{}
		for _, opt := range o.Options {
			if opt.Value != "" {
				choices = append(choices, Choice{ID: opt.Value, Name: opt.Name})
			}
			for _, sub := range opt.Options {
				choices = append(choices, Choice{ID: sub.Value, Name: sub.Name})
			}
		}
		return o.ID, cur, choices, true
	}
	return "", "", nil, false
}
