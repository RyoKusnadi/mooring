package mcp

import (
	"net/url"
	"strings"
)

// Var names an environment variable or header the user fills in. Set says a
// value is saved in the secret store; the value itself never leaves the server.
type Var struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Set         bool   `json:"set"`
}

// Kind is one way to run a catalog entry.
type Kind struct {
	Kind      string   `json:"kind"`
	Package   string   `json:"package,omitempty"`
	Version   string   `json:"version,omitempty"`
	URL       string   `json:"url,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Env       []Var    `json:"env"`
	Headers   []Var    `json:"headers"`
	Available bool     `json:"available"`
	Missing   string   `json:"missing,omitempty"`
}

// Entry is one server of the catalog (its latest version).
type Entry struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Website     string `json:"website"`
	Repository  string `json:"repository"`
	Kinds       []Kind `json:"kinds"`
}

// runTemplate is the only way a registry package becomes a command. It returns
// ok=false when the package name or version is not one the registry type allows.
// Nothing else from the registry (arguments, runtime hints) is used.
func runTemplate(kind, pkg, version string, env []Var) (command string, args []string, ok bool) {
	if len(pkg) == 0 || len(pkg) > 214 {
		return "", nil, false
	}
	switch kind {
	case "npm", "pypi":
		if !validPackage(kind, pkg) || !pkgVersion.MatchString(version) {
			return "", nil, false
		}
		if kind == "npm" {
			return "npx", []string{"-y", pkg + "@" + version}, true
		}
		return "uvx", []string{pkg + "==" + version}, true
	case "oci":
		name, tag := pkg, version
		if i := strings.LastIndex(pkg, ":"); i > strings.LastIndex(pkg, "/") {
			name, tag = pkg[:i], pkg[i+1:]
		}
		if !ociName.MatchString(name) || !ociTag.MatchString(tag) {
			return "", nil, false
		}
		args = []string{"run", "-i", "--rm"}
		for _, v := range env {
			args = append(args, "-e", v.Name)
		}
		return "docker", append(args, name+":"+tag), true
	}
	return "", nil, false
}

func convertVars(in []rawVar, rename func(string) bool) ([]Var, bool) {
	out := make([]Var, 0, len(in))
	for _, v := range in {
		if !rename(v.Name) {
			return nil, false
		}
		out = append(out, Var{Name: v.Name, Description: clip(v.Description, 200), Required: v.IsRequired, Secret: v.IsSecret})
	}
	return out, true
}

func buildEntry(s rawServer) (Entry, bool) {
	if s.Name == "" || len(s.Name) > 200 {
		return Entry{}, false
	}
	e := Entry{
		Name: s.Name, Title: clip(s.Title, 100), Description: clip(s.Description, 400), Version: clip(s.Version, 64),
		Website: webURL(s.WebsiteURL), Repository: webURL(s.Repository.URL), Kinds: []Kind{},
	}
	for _, p := range s.Packages {
		if p.RegistryType != "npm" && p.RegistryType != "pypi" && p.RegistryType != "oci" {
			continue
		}
		if p.Transport.Type != "" && p.Transport.Type != "stdio" {
			continue
		}
		env, ok := convertVars(p.Env, envName.MatchString)
		if !ok {
			continue
		}
		cmd, args, ok := runTemplate(p.RegistryType, p.Identifier, p.Version, env)
		if !ok {
			continue
		}
		e.Kinds = append(e.Kinds, Kind{Kind: p.RegistryType, Package: p.Identifier, Version: p.Version,
			Command: cmd, Args: args, Env: env, Headers: []Var{}})
	}
	for _, r := range s.Remotes {
		kind := map[string]string{"streamable-http": "http", "sse": "sse"}[r.Type]
		if kind == "" || !remoteURL(r.URL) {
			continue
		}
		headers, ok := convertVars(r.Headers, headerName.MatchString)
		if !ok {
			continue
		}
		e.Kinds = append(e.Kinds, Kind{Kind: kind, URL: r.URL, Env: []Var{}, Headers: headers})
	}
	return e, len(e.Kinds) > 0
}

// remoteURL accepts an https address with a host and no template variables.
func remoteURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(raw, "{}\x00 ")
}

func webURL(raw string) string {
	if u, err := url.Parse(raw); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		return raw
	}
	return ""
}
