package lsp

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/ryokusnadi/mooring/internal/paths"
)

// withURI returns m with params.textDocument.uri set to the spelling the
// server was given when the document was opened.
func withURI(m rpcMap, uri string) rpcMap {
	var p map[string]json.RawMessage
	if json.Unmarshal(m["params"], &p) != nil {
		return m
	}
	var td map[string]json.RawMessage
	if json.Unmarshal(p["textDocument"], &td) != nil {
		return m
	}
	td["uri"] = mustJSON(uri)
	p["textDocument"] = mustJSON(td)
	out := rpcMap{}
	for k, v := range m {
		out[k] = v
	}
	out["params"] = mustJSON(p)
	return out
}

// docOf finds the open document a URI names, whatever its spelling.
func (i *inst) docOf(uri string) *doc {
	real, err := i.checkURI(uri)
	if err != nil {
		return nil
	}
	return i.docs[real]
}

// checkParams requires every URI in the params to be a file inside the place:
// fields named uri or ending in Uri, and lists named uris or ending in Uris.
// A textDocument without a string uri is refused.
func (i *inst) checkParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return i.walkURIs(v, 0)
}

func (i *inst) walkURIs(v any, depth int) error {
	if depth > 32 {
		return paths.ErrOutsideRoot
	}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if err := i.walkURIs(e, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		if td, ok := x["textDocument"].(map[string]any); ok {
			if _, isStr := td["uri"].(string); !isStr {
				return paths.ErrOutsideRoot
			}
		}
		for k, e := range x {
			lk := strings.ToLower(k)
			switch {
			case strings.HasSuffix(lk, "uri"):
				s, ok := e.(string)
				if !ok {
					return paths.ErrOutsideRoot
				}
				if _, err := i.checkURI(s); err != nil {
					return err
				}
			case strings.HasSuffix(lk, "uris"):
				list, ok := e.([]any)
				if !ok {
					return paths.ErrOutsideRoot
				}
				for _, it := range list {
					s, ok := it.(string)
					if !ok {
						return paths.ErrOutsideRoot
					}
					if _, err := i.checkURI(s); err != nil {
						return err
					}
				}
			default:
				if err := i.walkURIs(e, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkURI requires a file: URI of a path inside the place and returns the real path.
func (i *inst) checkURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") ||
		!filepath.IsAbs(u.Path) || strings.ContainsRune(u.Path, 0) {
		return "", paths.ErrOutsideRoot
	}
	return paths.Resolve(i.pl.real, u.Path)
}
