package password

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const goodPassword = "correct horse battery"

func TestMain(m *testing.M) {
	UseLowCost()
	os.Exit(m.Run())
}

func TestHashAndVerify(t *testing.T) {
	r, err := NewRecord("  ada ", goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	if r.Username != "ada" {
		t.Fatalf("username not trimmed: %q", r.Username)
	}
	if !r.Verify("ada", goodPassword) || !r.Verify(" ada", goodPassword) {
		t.Fatal("right username and password must verify")
	}
	if r.Verify("ada", goodPassword+"x") {
		t.Fatal("wrong password verified")
	}
	if r.Verify("bob", goodPassword) {
		t.Fatal("wrong username verified")
	}
	if r.Verify("", "") {
		t.Fatal("empty input verified")
	}
}

func TestLimits(t *testing.T) {
	if _, err := NewRecord("ada", "short"); err == nil {
		t.Fatal("a 5 character password must be refused")
	}
	if _, err := NewRecord("   ", goodPassword); err == nil {
		t.Fatal("a blank username must be refused")
	}
	if _, err := NewRecord(strings.Repeat("a", 65), goodPassword); err == nil {
		t.Fatal("a 65 character username must be refused")
	}
	if _, err := NewRecord("ada", strings.Repeat("a", MaxPasswordBytes+1)); err == nil {
		t.Fatal("a very long password must be refused")
	}
}

func TestFileIsPrivateAndHoldsNoPassword(t *testing.T) {
	dir := t.TempDir()
	r, _ := NewRecord("ada", goodPassword)
	if err := Save(dir, r); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(filepath.Join(dir, FileName))
	if strings.Contains(string(data), goodPassword) {
		t.Fatal("the file holds the password")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
	back, err := Load(dir)
	if err != nil || back == nil || !back.Verify("ada", goodPassword) {
		t.Fatalf("load: %v %v", back, err)
	}
}

func TestLoadNothingAndRemove(t *testing.T) {
	dir := t.TempDir()
	if r, err := Load(dir); r != nil || err != nil {
		t.Fatalf("empty folder: %v %v", r, err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("removing nothing: %v", err)
	}
	r, _ := NewRecord("ada", goodPassword)
	_ = Save(dir, r)
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if r, _ := Load(dir); r != nil {
		t.Fatal("still set after remove")
	}
}

func TestUnknownVersionAndDamageRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	for _, c := range []struct {
		body    string
		version bool
	}{
		{"v2:YWRh:3:65536:1:c2FsdHNhbHRzYWx0c2FsdA:a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5\n", true},
		{"plain-password\n", true},
		{"v1:YWRh:3:65536:1\n", false},
		{"v1:YWRh:3:99999999:1:c2FsdHNhbHRzYWx0c2FsdA:a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5\n", false},
	} {
		if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		r, err := Load(dir)
		if r != nil || err == nil {
			t.Fatalf("%q accepted", c.body)
		}
		if errors.Is(err, ErrVersion) != c.version {
			t.Fatalf("%q: ErrVersion=%v, want %v", c.body, errors.Is(err, ErrVersion), c.version)
		}
	}
}

func TestSaveSweepsOldTempFilesAndKeepsNewOnes(t *testing.T) {
	dir := t.TempDir()
	old, fresh := filepath.Join(dir, "password-new-old"), filepath.Join(dir, "password-new-fresh")
	_ = os.WriteFile(old, []byte("x"), 0o600)
	_ = os.WriteFile(fresh, []byte("x"), 0o600)
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(old, past, past)
	r, _ := NewRecord("ada", goodPassword)
	if err := Save(dir, r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("old temp file kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("a recent temp file may belong to another process")
	}
}
