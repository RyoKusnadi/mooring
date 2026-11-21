package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/ryokusnadi/mooring/internal/chat"
)

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// Usable says whether a chat may be started with this server (it must exist and be on).
func (s *Service) Usable(id string) error {
	r, err := s.get(id)
	if err != nil {
		return err
	}
	if !r.Enabled {
		return apiErr("NOT_ALLOWED", "The tool server %q is off. Turn it on first.", r.Name)
	}
	return nil
}

// Resolve fills in a server for a chat: command, env and headers from the saved
// record and the secret store, and the sign-in token as a Bearer header. The
// result goes to the agent and is never saved. An error is the reason, in plain
// words, that the server is left out.
func (s *Service) Resolve(ctx context.Context, id string) (chat.ToolServer, error) {
	r, err := s.get(id)
	if err != nil {
		return chat.ToolServer{}, errors.New("it was removed")
	}
	out := chat.ToolServer{ID: id, Name: unsafeName.ReplaceAllString(r.Name, "-")}
	if out.Name == "" {
		out.Name = "tool-server"
	}
	skip := func(why string) (chat.ToolServer, error) { return chat.ToolServer{Name: r.Name}, errors.New(why) }
	if !r.Enabled || !r.Allowed {
		return skip("it is turned off")
	}
	if r.remote() {
		out.Type, out.URL = r.Kind, r.URL
		hv, missing, err := s.values(id, r.Headers, headerKey)
		if err != nil {
			return skip("its saved values could not be read")
		}
		if len(missing) > 0 {
			return skip(fmt.Sprintf("it needs values for %v", missing))
		}
		out.Headers = hv
		if r.SignIn == "needed" || r.SignIn == "failed" {
			return skip("its sign-in has expired")
		}
		if r.SignIn == "signedIn" {
			tok, err := s.accessToken(ctx, r, false)
			if err != nil {
				return skip("its sign-in has expired")
			}
			if out.Headers == nil {
				out.Headers = map[string]string{}
			}
			out.Headers["Authorization"] = "Bearer " + tok
		}
		return out, nil
	}
	ev, missing, err := s.values(id, r.Env, envKey)
	if err != nil {
		return skip("its saved values could not be read")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return skip(fmt.Sprintf("it needs values for %v", missing))
	}
	out.Type, out.Command, out.Args, out.Env = "stdio", r.Command, r.Args, ev
	return out, nil
}
