package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"game-to-steam/internal/model"
	"game-to-steam/internal/steam"
	"game-to-steam/internal/storage"
)

func TestDeleteReimportRetainsPlaytimeAndIdentity(t *testing.T) {
	s, g := fixture(t)
	requests := []ImportRequest{{Game: g}}
	if _, err := s.Import(context.Background(), requests, ""); err != nil {
		t.Fatal(err)
	}
	id := list(t, s)[0].AppID
	key := strconv.FormatInt(int64(int32(id)), 10)
	path := filepath.Join(s.Profile.ConfigDir, "localconfig.vdf")
	data, err := steam.RestorePlaytime(nil, map[string]uint64{key: 153, "42": 900})
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.Replace(path, data); err != nil {
		t.Fatal(err)
	}
	if err = s.Delete([]uint32{id}, true); err != nil {
		t.Fatal(err)
	}
	if len(list(t, s)) != 0 {
		t.Fatal("shortcut not deleted")
	}
	// Simulate Steam dropping the removed shortcut's local stats.
	if err = storage.Replace(path, []byte(`"UserLocalConfigStore" { "keep" "yes" }`)); err != nil {
		t.Fatal(err)
	}
	// Persisted history survives application restart and a changed launch path.
	s.State, err = storage.Load(s.Dir)
	if err != nil {
		t.Fatal(err)
	}
	g.Name = "renamed game"
	results, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if err != nil || results[0].Failed {
		t.Fatalf("%v %v", results, err)
	}
	if list(t, s)[0].AppID != id {
		t.Fatal("identity changed")
	}
	got, _ := os.ReadFile(path)
	minutes, err := steam.ReadPlaytime(got, id)
	if err != nil || minutes[key] != 153 {
		t.Fatalf("lost minutes: %s %v", got, err)
	}
}

func TestPlaytimeHistoryIsAccountScoped(t *testing.T) {
	s, g := fixture(t)
	s.State.PlaytimeHistory = map[string]map[string]model.SteamPlaytime{
		"another-account": {g.ID: {AppID: 123, Minutes: map[string]uint64{"123": 999}}},
	}
	results, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if err != nil || results[0].Failed {
		t.Fatalf("%v %v", results, err)
	}
	if list(t, s)[0].AppID == 123 {
		t.Fatal("used another account's identity")
	}
	if _, err := os.Stat(filepath.Join(s.Profile.ConfigDir, "localconfig.vdf")); !os.IsNotExist(err) {
		t.Fatal("restored another account's minutes")
	}
}

func TestFailedPlaytimeRestoreDoesNotImportShortcut(t *testing.T) {
	s, g := fixture(t)
	_, _ = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	id := list(t, s)[0].AppID
	path := filepath.Join(s.Profile.ConfigDir, "localconfig.vdf")
	data, err := steam.RestorePlaytime(nil, map[string]uint64{strconv.FormatUint(uint64(id), 10): 73})
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Replace(path, data); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete([]uint32{id}, false); err != nil {
		t.Fatal(err)
	}
	if err := storage.Replace(path, []byte(`"broken" {`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err == nil {
		t.Fatal("accepted unreadable config")
	}
	if len(list(t, s)) != 0 || len(s.State.Accounts[s.Profile.AccountID]) != 0 {
		t.Fatal("partially imported shortcut")
	}
}

func TestDeleteRefusesUnreadablePlaytimeWithoutRemovingShortcut(t *testing.T) {
	s, g := fixture(t)
	_, _ = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	id := list(t, s)[0].AppID
	if err := storage.Replace(filepath.Join(s.Profile.ConfigDir, "localconfig.vdf"), []byte(`"broken" {`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete([]uint32{id}, false); err == nil {
		t.Fatal("deleted despite unreadable stats")
	}
	if len(list(t, s)) != 1 {
		t.Fatal("removed shortcut")
	}
}

func TestReimportRefusesAppIDCollision(t *testing.T) {
	s, g := fixture(t)
	_, _ = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	id := list(t, s)[0].AppID
	if err := s.Delete([]uint32{id}, false); err != nil {
		t.Fatal(err)
	}
	root := steam.NewRoot()
	_ = steam.Upsert(root, model.Shortcut{AppID: id, Name: "unrelated", Exe: "other"})
	if err := storage.Replace(filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf"), steam.Encode(root)); err != nil {
		t.Fatal(err)
	}
	results, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if err != nil || !results[0].Failed {
		t.Fatal(fmt.Sprint(results, err))
	}
	if len(list(t, s)) != 1 || list(t, s)[0].Name != "unrelated" {
		t.Fatal("overwrote unrelated shortcut")
	}
}
