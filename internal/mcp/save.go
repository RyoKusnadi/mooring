package mcp

import (
	"context"
	"slices"
	"strings"
	"unicode"
)

// SecretValues are the values sent with a save: name to value, "" deletes.
type SecretValues struct {
	Env     map[string]string `json:"env"`
	Headers map[string]string `json:"headers"`
}

// SaveArgs is the toolServer.save command.
type SaveArgs struct {
	Server  ToolServer    `json:"server"`
	Secrets *SecretValues `json:"secrets"`
	Allow   bool          `json:"allow"`
}

func printable(s string, max int) bool {
	if s == "" || len([]rune(s)) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func cleanVars(vars []Var, ok func(string) bool, what string) ([]Var, error) {
	out := make([]Var, 0, len(vars))
	for _, v := range vars {
		if !ok(v.Name) {
			return nil, apiErr("BAD_REQUEST", "%q is not a valid %s name.", v.Name, what)
		}
		if slices.ContainsFunc(out, func(o Var) bool { return o.Name == v.Name }) {
			return nil, apiErr("BAD_REQUEST", "The %s name %q is listed twice.", what, v.Name)
		}
		v.Description = clip(v.Description, 200)
		out = append(out, v)
	}
	return out, nil
}

// validate checks a server from the browser and returns it as a row (without ids and state).
func (s *Service) validate(ctx context.Context, in ToolServer) (rec, error) {
	r := rec{ToolServer: in}
	r.Name = strings.TrimSpace(in.Name)
	// State belongs to the server, not to the browser's request.
	r.ID, r.Enabled, r.Allowed, r.SignIn, r.Expires, r.CheckedAt, r.Tools = "", false, false, "", 0, 0, []Tool{}
	if !printable(r.Name, 64) {
		return r, apiErr("BAD_REQUEST", "A tool server name needs 1 to 64 characters.")
	}
	var err error
	if r.Env, err = cleanVars(in.Env, envName.MatchString, "environment variable"); err != nil {
		return r, err
	}
	if r.Headers, err = cleanVars(in.Headers, headerName.MatchString, "header"); err != nil {
		return r, err
	}
	for i := range r.Env {
		r.Env[i].Set = false
	}
	for i := range r.Headers {
		r.Headers[i].Set = false
	}
	if r.Args == nil {
		r.Args = []string{}
	}
	switch r.Kind {
	case "stdio":
		if !printable(r.Command, 1024) || len(r.Args) > 64 || len(r.Headers) > 0 || r.URL != "" {
			return r, apiErr("BAD_REQUEST", "A local server needs a command, and takes no address or headers.")
		}
	case "npm", "pypi", "oci":
		if !templateMatches(r) || len(r.Headers) > 0 || r.URL != "" {
			return r, apiErr("BAD_REQUEST", "A catalog server runs only with the command Mooring builds from its package name and version.")
		}
	case "http", "sse":
		if len(r.URL) > 2048 || r.Command != "" || len(r.Args) > 0 || len(r.Env) > 0 {
			return r, apiErr("BAD_REQUEST", "A remote server needs an address, and takes no command or environment.")
		}
		if err := checkAddress(ctx, r.URL, r.AllowPrivate); err != nil {
			return r, err
		}
	default:
		return r, apiErr("BAD_REQUEST", "The kind must be stdio, http or sse (or npm, pypi or oci for a catalog server).")
	}
	for _, a := range r.Args {
		if len(a) > 4096 || strings.ContainsRune(a, 0) {
			return r, apiErr("BAD_REQUEST", "An argument is not valid.")
		}
	}
	if strings.ContainsRune(r.Command, 0) {
		return r, apiErr("BAD_REQUEST", "The command is not valid.")
	}
	r.Source = Source{Registry: clip(in.Source.Registry, 200), Version: clip(in.Source.Version, 64)}
	return r, nil
}

// templateMatches says the command line is exactly what runTemplate builds for
// the package in its arguments, so registry text never reaches a command another way.
func templateMatches(r rec) bool {
	var pkg, version string
	switch r.Kind {
	case "npm":
		if len(r.Args) != 2 || r.Args[0] != "-y" {
			return false
		}
		i := strings.LastIndex(r.Args[1], "@")
		if i <= 0 {
			return false
		}
		pkg, version = r.Args[1][:i], r.Args[1][i+1:]
	case "pypi":
		if len(r.Args) != 1 {
			return false
		}
		var ok bool
		if pkg, version, ok = strings.Cut(r.Args[0], "=="); !ok {
			return false
		}
	case "oci":
		if len(r.Args) < 4 {
			return false
		}
		pkg = r.Args[len(r.Args)-1]
	}
	cmd, args, ok := runTemplate(r.Kind, pkg, version, r.Env)
	return ok && cmd == r.Command && slices.Equal(args, r.Args)
}

func checkValues(m map[string]string, vars []Var, header bool) error {
	for name, val := range m {
		if !slices.ContainsFunc(vars, func(v Var) bool { return v.Name == name }) {
			return apiErr("BAD_REQUEST", "%q is not in the list of names.", name)
		}
		if len(val) > maxSecretBytes || strings.ContainsRune(val, 0) || (header && strings.ContainsAny(val, "\r\n")) {
			return apiErr("BAD_REQUEST", "The value for %q is not valid (up to 16 KiB, no line breaks in headers).", name)
		}
	}
	return nil
}

// Save adds a server (no ID) or edits one.
func (s *Service) Save(ctx context.Context, a SaveArgs) (ToolServer, error) {
	r, err := s.validate(ctx, a.Server)
	if err != nil {
		return ToolServer{}, err
	}
	var sv SecretValues
	if a.Secrets != nil {
		sv = *a.Secrets
	}
	if err := checkValues(sv.Env, r.Env, false); err != nil {
		return ToolServer{}, err
	}
	if err := checkValues(sv.Headers, r.Headers, true); err != nil {
		return ToolServer{}, err
	}
	given := len(sv.Env)+len(sv.Headers) > 0
	if given && s.Secrets == nil {
		return ToolServer{}, apiErr("SECRETS_UNAVAILABLE", "There is no secret store on this machine, so values cannot be saved.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.list()
	if err != nil {
		return ToolServer{}, err
	}
	var old *rec
	for i := range all {
		if all[i].ID == a.Server.ID && a.Server.ID != "" {
			old = &all[i]
		} else if all[i].Name == r.Name {
			return ToolServer{}, apiErr("ALREADY_EXISTS", "A tool server called %q is already saved.", r.Name)
		}
	}
	if old == nil && a.Server.ID != "" {
		return ToolServer{}, apiErr("NOT_FOUND", "That tool server is not saved.")
	}
	if old == nil && len(all) >= MaxServers {
		return ToolServer{}, apiErr("LIMIT_REACHED", "At most %d tool servers can be saved.", MaxServers)
	}
	position := len(all)
	if old == nil {
		r.ID = newID()
	} else {
		r.ID, position = old.ID, slices.IndexFunc(all, func(o rec) bool { return o.ID == old.ID })
		r.Enabled, r.SignIn, r.Expires, r.CheckedAt, r.Tools = old.Enabled, old.SignIn, old.Expires, old.CheckedAt, old.Tools
		r.allowedSig, r.checkOK = old.allowedSig, old.checkOK
		s.keepSetFlags(&r, *old)
		s.gen[old.ID]++
		if old.remote() && (old.URL != r.URL || old.Kind != r.Kind) {
			s.dropSignIn(old.ID)
			r.SignIn, r.Expires = "", 0
		}
		if r.material(given) != old.material(false) {
			r.Enabled, r.checkOK = false, false
		}
	}
	if a.Allow {
		r.allowedSig = r.sig()
	} else if r.allowedSig != r.sig() {
		r.allowedSig = ""
		r.Enabled = false
	}
	for name, val := range sv.Env {
		s.applySecret(&r.Env, envKey(r.ID, name), name, val)
	}
	for name, val := range sv.Headers {
		s.applySecret(&r.Headers, headerKey(r.ID, name), name, val)
	}
	if old != nil {
		s.dropRemovedVars(*old, r)
	}
	if err := s.put(r, position); err != nil {
		if old == nil {
			s.deleteSecrets(r)
		}
		return ToolServer{}, err
	}
	r.Allowed = r.allowedSig != "" && r.allowedSig == r.sig()
	s.changed(r)
	return r.ToolServer, nil
}

// keepSetFlags carries the "value saved" flag over for names that stay.
func (s *Service) keepSetFlags(r *rec, old rec) {
	carry := func(now []Var, before []Var) {
		for i := range now {
			if j := slices.IndexFunc(before, func(o Var) bool { return o.Name == now[i].Name }); j >= 0 {
				now[i].Set = before[j].Set
			} else {
				now[i].Set = false
			}
		}
	}
	carry(r.Env, old.Env)
	carry(r.Headers, old.Headers)
}

func (s *Service) applySecret(vars *[]Var, key, name, val string) {
	i := slices.IndexFunc(*vars, func(v Var) bool { return v.Name == name })
	if val == "" {
		s.secretDelete(key)
		(*vars)[i].Set = false
		return
	}
	if err := s.Secrets.Set(key, val); err != nil {
		s.log.Error("tool server: could not save a secret", "err", err)
		(*vars)[i].Set = false
		return
	}
	(*vars)[i].Set = true
}

func (s *Service) dropRemovedVars(old, now rec) {
	for _, v := range old.Env {
		if !slices.ContainsFunc(now.Env, func(o Var) bool { return o.Name == v.Name }) {
			s.secretDelete(envKey(old.ID, v.Name))
		}
	}
	for _, v := range old.Headers {
		if !slices.ContainsFunc(now.Headers, func(o Var) bool { return o.Name == v.Name }) {
			s.secretDelete(headerKey(old.ID, v.Name))
		}
	}
}
