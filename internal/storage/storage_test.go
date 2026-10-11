package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitDetectsChangeAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "data")
	_ = os.WriteFile(target, []byte("old"), 0600)
	before, _ := Read(target)
	backup, err := Commit(filepath.Join(dir, "backups"), []Change{{Before: before, Data: []byte("new")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(target)
	old, _ := os.ReadFile(filepath.Join(backup, "000.bin"))
	if string(b) != "new" || string(old) != "old" {
		t.Fatal("backup/replace failed")
	}
	if _, err = Commit(filepath.Join(dir, "backups"), []Change{{Before: before, Data: []byte("overwrite")}}, nil); err == nil {
		t.Fatal("ignored external change")
	}
}
func TestRollbackPartialWrite(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	_ = os.WriteFile(first, []byte("old"), 0600)
	firstBefore, _ := Read(first)
	second := Snapshot{Path: filepath.Join(dir, "second")}
	writer := func(path string, data []byte) error {
		if path == second.Path {
			return errors.New("simulated disk failure")
		}
		return Replace(path, data)
	}
	_, err := commit(filepath.Join(dir, "backups"), []Change{{Before: firstBefore, Data: []byte("new")}, {Before: second, Data: []byte("second")}}, nil, writer)
	if err == nil {
		t.Fatal("expected failure")
	}
	b, _ := os.ReadFile(first)
	if string(b) != "old" {
		t.Fatal("changed original on failure")
	}
}
func TestPreflightStopsWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	before, _ := Read(path)
	_, err := Commit(filepath.Join(dir, "backups"), []Change{{Before: before, Data: []byte("x")}}, func() error { return errors.New("Steam running") })
	if err == nil {
		t.Fatal("expected rejection")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unexpected write")
	}
}
func TestCorruptStateRejected(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "state.json"), []byte("invalid"), 0600)
	if _, err := Load(dir); err == nil {
		t.Fatal("accepted corrupt state")
	}
}
