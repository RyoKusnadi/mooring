package mcp

func (s *Service) deleteSecrets(r rec) {
	for _, v := range r.Env {
		s.secretDelete(envKey(r.ID, v.Name))
	}
	for _, v := range r.Headers {
		s.secretDelete(headerKey(r.ID, v.Name))
	}
	s.dropSignIn(r.ID)
}

func (s *Service) dropSignIn(id string) {
	s.pending.purge(id)
	s.secretDelete(tokenKey(id))
	s.secretDelete(clientKey(id))
}

// Remove deletes a server with its secrets and tokens.
func (s *Service) Remove(id string) error {
	s.mu.Lock()
	r, err := s.get(id)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM tool_servers WHERE id = ?`, id); err != nil {
		s.mu.Unlock()
		return apiErr("INTERNAL", "Could not remove the tool server.")
	}
	s.gen[id]++
	s.mu.Unlock()
	s.cache.drop(id)
	s.notes.Delete(id)
	s.deleteSecrets(r)
	s.Pub.Publish("toolServer.removed", map[string]string{"id": id})
	return nil
}

// Enable turns a server on or off. Turning on needs consent and a passed check.
func (s *Service) Enable(id string, on bool) (ToolServer, error) {
	var failure error
	r, err := s.update(id, func(r *rec) {
		if on && !r.Allowed {
			failure = apiErr("NOT_ALLOWED", "Allow this command first.")
			return
		}
		if on && !r.checkOK {
			failure = apiErr("CHECK_FAILED", "Run a check first: this server has not passed one since it was last changed.")
			return
		}
		r.Enabled = on
	})
	if err != nil {
		return ToolServer{}, err
	}
	return r.ToolServer, failure
}
