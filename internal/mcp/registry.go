package mcp

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

//go:embed registry.json.gz
var embeddedRegistry []byte

// rawRegistry is the registry's own shape, reduced to the fields the catalog
// uses. The shipped copy and the downloaded one have it too.
type rawRegistry struct {
	Date    string `json:"date"`
	Servers []struct {
		Server rawServer `json:"server"`
	} `json:"servers"`
}

type rawServer struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	WebsiteURL  string `json:"websiteUrl"`
	Repository  struct {
		URL string `json:"url"`
	} `json:"repository"`
	Packages []struct {
		RegistryType string `json:"registryType"`
		Identifier   string `json:"identifier"`
		Version      string `json:"version"`
		Transport    struct {
			Type string `json:"type"`
		} `json:"transport"`
		Env []rawVar `json:"environmentVariables"`
	} `json:"packages"`
	Remotes []struct {
		Type    string   `json:"type"`
		URL     string   `json:"url"`
		Headers []rawVar `json:"headers"`
	} `json:"remotes"`
}

type rawVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsRequired  bool   `json:"isRequired"`
	IsSecret    bool   `json:"isSecret"`
}

// catalog is the parsed registry, ready to search.
type catalog struct {
	date    string
	entries []Entry // sorted by name
	words   []string
}

func parseCatalog(raw []byte) (*catalog, error) {
	var rr rawRegistry
	if err := json.Unmarshal(raw, &rr); err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	c := &catalog{date: rr.Date}
	seen := map[string]bool{}
	for _, s := range rr.Servers {
		if seen[s.Server.Name] {
			continue
		}
		seen[s.Server.Name] = true
		if e, ok := buildEntry(s.Server); ok {
			c.entries = append(c.entries, e)
		}
	}
	sort.Slice(c.entries, func(i, j int) bool { return c.entries[i].Name < c.entries[j].Name })
	c.words = make([]string, len(c.entries))
	for i, e := range c.entries {
		c.words[i] = strings.ToLower(e.Name + "\n" + e.Title + "\n" + e.Description)
	}
	return c, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func loadEmbedded() (*catalog, error) {
	zr, err := gzip.NewReader(bytes.NewReader(embeddedRegistry))
	if err != nil {
		return nil, fmt.Errorf("open the shipped registry: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(zr, maxRegistryBytes))
	if err != nil {
		return nil, fmt.Errorf("unpack the shipped registry: %w", err)
	}
	return parseCatalog(raw)
}

// registryHolder loads the catalog on first use: the downloaded copy when there
// is a good one, else the shipped one.
type registryHolder struct {
	file string // <data>/mcp-registry.json, "" for none
	mu   sync.Mutex
	cat  *catalog
}

func (h *registryHolder) get() (*catalog, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cat != nil {
		return h.cat, nil
	}
	if h.file != "" {
		if raw, err := os.ReadFile(h.file); err == nil {
			if c, err := parseCatalog(raw); err == nil && len(c.entries) > 0 {
				h.cat = c
				return c, nil
			}
		}
	}
	c, err := loadEmbedded()
	if err != nil {
		return nil, err
	}
	h.cat = c
	return c, nil
}

func (h *registryHolder) set(c *catalog) {
	h.mu.Lock()
	h.cat = c
	h.mu.Unlock()
}

// search returns the matches number offset to offset+limit-1 for the words of q
// (any order, any case). next is the offset of the following page, or -1.
func (c *catalog) search(q string, offset, limit int) (page []Entry, next int) {
	words := strings.Fields(strings.ToLower(q))
	next = -1
	if offset < 0 {
		offset = 0
	}
	n := 0
	for i := range c.entries {
		ok := true
		for _, w := range words {
			if !strings.Contains(c.words[i], w) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if n >= offset+limit {
			return page, offset + limit
		}
		if n >= offset {
			page = append(page, c.entries[i])
		}
		n++
	}
	return page, next
}
