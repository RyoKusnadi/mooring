package chat

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ryokusnadi/mooring/internal/files"
)

// An agent's fs/write_text_file goes through Config.WriteFile, the editor's save lock.
func TestAgentWriteUsesTheSaveLock(t *testing.T) {
	r := newRig(t)
	var mu sync.Mutex
	var seen []string
	r.m.WriteFile = func(real string, data []byte) error {
		mu.Lock()
		seen = append(seen, real)
		mu.Unlock()
		return files.WriteAtomic(real, data)
	}
	c := r.start("c1")
	target := filepath.Join(r.dir, "b.txt")
	r.prompt(c, "writefile "+target+" hi")
	r.waitState(c, StateIdle)
	if b, err := os.ReadFile(target); err != nil || string(b) != "hi" {
		t.Fatalf("file: %q %v", b, err)
	}
	r.prompt(c, "writefile /etc/mooring-should-not-exist nope")
	r.waitState(c, StateIdle)
	mu.Lock()
	defer mu.Unlock()
	want, _ := filepath.EvalSymlinks(r.dir)
	if len(seen) != 1 || filepath.Dir(seen[0]) != want {
		t.Fatalf("writes through the lock: %v (want one in %s)", seen, want)
	}
}
