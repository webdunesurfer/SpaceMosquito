package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitWorkspace_createsLayout(t *testing.T) {
	dir := t.TempDir()
	result, err := InitWorkspace(InitOptions{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.DataDir != dir {
		t.Fatalf("DataDir = %q, want %q", result.DataDir, dir)
	}
	if !result.ConfigCreated {
		t.Fatal("expected config to be created")
	}
	for _, sub := range []string{"saved", "browser"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Fatalf("missing %s: %v", sub, err)
		}
	}
	info, err := os.Stat(result.SessionPath)
	if err != nil {
		t.Fatalf("session file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session perm = %o, want 0600", info.Mode().Perm())
	}
}
