package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// CatalogUpdate downloads the current registry (pages of 100, at most 20 MiB, 60 s)
// and uses it instead of the shipped copy.
func (s *Service) CatalogUpdate(ctx context.Context) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := s.RegistryClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	var all rawRegistry
	all.Date = s.Now().UTC().Format("2006-01-02")
	var total int64
	cursor := ""
	for page := 0; page < 1000; page++ {
		q := url.Values{"limit": {"100"}, "version": {"latest"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.RegistryURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, apiErr("CATALOG_FAILED", "The registry address is not valid.")
		}
		req.Header.Set("User-Agent", "mooring")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, apiErr("CATALOG_FAILED", "Mooring could not reach the registry.")
		}
		body, rerr := readCapped(resp.Body, maxRegistryBytes-total)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || rerr != nil {
			return nil, apiErr("CATALOG_FAILED", "The registry did not give a usable answer.")
		}
		total += int64(len(body))
		var pg struct {
			rawRegistry
			Metadata struct {
				NextCursor string `json:"nextCursor"`
			} `json:"metadata"`
		}
		if json.Unmarshal(body, &pg) != nil {
			return nil, apiErr("CATALOG_FAILED", "The registry answer could not be read.")
		}
		all.Servers = append(all.Servers, pg.Servers...)
		if cursor = pg.Metadata.NextCursor; cursor == "" {
			break
		}
	}
	raw, _ := json.Marshal(all)
	cat, err := parseCatalog(raw)
	if err != nil || len(cat.entries) == 0 {
		return nil, apiErr("CATALOG_FAILED", "The registry list had no usable servers.")
	}
	if s.reg.file != "" {
		if err := writeAtomic(s.reg.file, raw); err != nil {
			s.log.Warn("tool server: could not keep the downloaded catalog", "err", err)
		}
	}
	s.reg.set(cat)
	return map[string]any{"count": len(cat.entries), "date": cat.date}, nil
}

func writeAtomic(file string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), ".mcp-registry-*")
	if err != nil {
		return fmt.Errorf("make temporary file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write catalog: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write catalog: %w", err)
	}
	return os.Rename(tmp.Name(), file)
}
