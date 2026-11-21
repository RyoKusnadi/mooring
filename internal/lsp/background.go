package lsp

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Running is one entry of lsp.list's running.
type Running struct {
	ProjectID string `json:"projectId"`
	Folder    string `json:"folder"`
	ID        string `json:"id"`
	PID       int    `json:"pid"`
	MB        int    `json:"mb"`
	Clients   int    `json:"clients"`
	StartedAt int64  `json:"startedAt"`
}

func (s *Service) snapshot() []*inst {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*inst, 0, len(s.insts))
	for _, i := range s.insts {
		out = append(out, i)
	}
	return out
}

func (s *Service) running() []Running {
	out := []Running{}
	for _, i := range s.snapshot() {
		i.mu.Lock()
		out = append(out, Running{ProjectID: i.key.project, Folder: i.key.folder, ID: i.key.id,
			PID: i.proc.pid(), MB: i.mb, Clients: len(i.socks), StartedAt: i.startedAt})
		i.mu.Unlock()
	}
	return out
}

// stopKey stops one server, as lsp.stop does.
func (s *Service) stopKey(key instKey, reason string) {
	s.mu.Lock()
	i := s.insts[key]
	s.mu.Unlock()
	if i != nil {
		i.stop(reason, nil)
	}
}

// stopServer stops every instance of one server id.
func (s *Service) stopServer(id, reason string) {
	s.stopWhere(func(k instKey) bool { return k.id == id }, reason)
}

// CloseProject stops the servers of a removed project.
func (s *Service) CloseProject(projectID string) {
	s.stopWhere(func(k instKey) bool { return k.project == projectID }, "closed")
}

func (s *Service) stopWhere(match func(instKey) bool, reason string) {
	var wg sync.WaitGroup
	for _, i := range s.snapshot() {
		if match(i.key) {
			wg.Add(1)
			go func() { defer wg.Done(); i.stop(reason, nil) }()
		}
	}
	wg.Wait()
}

// Shutdown stops every server and waits for them. No socket is accepted after it.
func (s *Service) Shutdown() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.quit)
	s.mu.Unlock()
	if s.unsub != nil {
		s.unsub()
	}
	s.stopWhere(func(instKey) bool { return true }, "closed")
	s.wg.Wait()
}

// background samples memory and renews the folder watches.
func (s *Service) background() {
	defer s.wg.Done()
	mem := time.NewTicker(s.cfg.MemoryEvery)
	defer mem.Stop()
	renew := time.NewTicker(s.cfg.WatchRenew)
	defer renew.Stop()
	warned := false
	for {
		select {
		case <-mem.C:
			if err := s.checkMemory(); err != nil && !warned {
				warned = true
				s.log.Warn("lsp: read memory use with ps", "err", err)
			}
		case <-renew.C:
			for _, i := range s.snapshot() {
				i.syncWatch()
			}
		case <-s.quit:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) checkMemory() error {
	list := s.snapshot()
	if len(list) == 0 {
		return nil
	}
	use, err := groupMemory()
	if err != nil {
		return err
	}
	for _, i := range list {
		mb := use[i.proc.pid()] / 1024
		i.mu.Lock()
		i.mb = mb
		i.mu.Unlock()
		if mb > s.cfg.MemoryMB {
			go i.restartForMemory(mb)
		}
	}
	return nil
}

// groupMemory sums the resident memory (KiB) of every process group, by group id.
func groupMemory() (map[int]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pgid=,rss=").Output()
	if err != nil {
		return nil, err
	}
	use := map[int]int{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		pg, err1 := strconv.Atoi(f[0])
		kb, err2 := strconv.Atoi(f[1])
		if err1 == nil && err2 == nil {
			use[pg] += kb
		}
	}
	return use, nil
}
