package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDir(t *testing.T) {
	base := t.TempDir()
	old, dest := filepath.Join(base, "old"), filepath.Join(base, "new")
	for path, data := range map[string]string{
		filepath.Join(old, "bridge", "auth", "session.dpapi"): "encrypted",
		filepath.Join(old, "state.json"):                      "settings",
		filepath.Join(old, "window-state.json"):               "old window",
		filepath.Join(dest, "window-state.json"):              "current window",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := migrateDir(old, dest); err != nil {
			t.Fatal(err)
		}
	}
	for path, want := range map[string]string{
		"bridge/auth/session.dpapi": "encrypted", "state.json": "settings",
		"window-state.json": "current window", "window-state.legacy.json": "old window",
	} {
		data, err := os.ReadFile(filepath.Join(dest, path))
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q, %v", path, data, err)
		}
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old directory remains: %v", err)
	}
}

func TestMigrateDirConflict(t *testing.T) {
	base := t.TempDir()
	old, dest := filepath.Join(base, "old"), filepath.Join(base, "new")
	for _, dir := range []string{old, dest} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(dir), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateDir(old, dest); err == nil {
		t.Fatal("expected conflict")
	}
	for _, dir := range []string{old, dest} {
		data, err := os.ReadFile(filepath.Join(dir, "state.json"))
		if err != nil || string(data) != dir {
			t.Fatalf("conflict changed data: %q, %v", data, err)
		}
	}
}
