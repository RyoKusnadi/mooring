package bench

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// prepare copies the fixture to dst and commits it, so a check can compare against it.
func prepare(benchDir, dst string) error {
	src := filepath.Join(benchDir, "fixture")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	})
	if err != nil {
		return fmt.Errorf("copy the fixture: %w", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "fixture"}} {
		if out, err := gitIn(dst, args...); err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(out))
		}
	}
	return nil
}

// gitIn runs git in dir with a fixed identity and no hooks.
func gitIn(dir string, args ...string) (string, error) {
	base := []string{"-C", dir, "-c", "user.name=mooring-bench", "-c", "user.email=bench@localhost",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
	out, err := exec.Command("git", append(base, args...)...).CombinedOutput()
	return string(out), err
}
