package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"game-to-steam/internal/model"
	"game-to-steam/internal/steam"
	"game-to-steam/internal/storage"
)

func fixture(t *testing.T) (*Service, model.Game) {
	t.Helper()
	dir := t.TempDir()
	steamDir := filepath.Join(dir, "steam")
	config := filepath.Join(steamDir, "config")
	_ = os.MkdirAll(config, 0700)
	_ = os.WriteFile(filepath.Join(config, "loginusers.vdf"), []byte(`"users" { "76561197960265729" { "PersonaName" "Test" "MostRecent" "1" } }`), 0600)
	profile, e := steam.RecentProfile(steamDir)
	if e != nil {
		t.Fatal(e)
	}
	state := model.State{Settings: model.Settings{SteamPath: steamDir}, Accounts: map[string]map[string]model.ManagedGame{}}
	dir = filepath.Join(dir, "app")
	before, _ := storage.Read(filepath.Join(dir, "state.json"))
	s := &Service{Dir: dir, State: state, Profile: profile, stateFile: before, Preflight: func() error { return nil }}
	exe := filepath.Join(t.TempDir(), "中文 game.exe")
	_ = os.WriteFile(exe, nil, 0600)
	g := model.Game{ID: "ns:item:app", Namespace: "ns", CatalogID: "item", AppName: "app", Name: "游戏", Executable: exe, InstallDir: filepath.Dir(exe)}
	s.State.Settings.EpicManifestPath = filepath.Join(dir, "manifests")
	manifest, _ := json.Marshal(map[string]any{"DisplayName": g.Name, "InstallLocation": g.InstallDir, "LaunchExecutable": filepath.Base(exe), "CatalogNamespace": "ns", "CatalogItemId": "item", "AppName": "app"})
	if e = storage.Replace(filepath.Join(s.State.Settings.EpicManifestPath, "game.item"), manifest); e != nil {
		t.Fatal(e)
	}
	return s, g
}
func list(t *testing.T, s *Service) []model.Shortcut {
	t.Helper()
	root, _, e := s.readShortcuts()
	if e != nil {
		t.Fatal(e)
	}
	return steam.List(root)
}
func TestImportIdempotencyAndAdoptionPreservation(t *testing.T) {
	s, g := fixture(t)
	r, e := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if e != nil || r[0].Failed || len(list(t, s)) != 1 {
		t.Fatalf("%v %v", r, e)
	}
	r, e = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if e != nil || len(list(t, s)) != 1 {
		t.Fatal("duplicate import", r, e)
	}
	// Existing shortcut from another tool is recognized but not silently rewritten.
	s, g = fixture(t)
	root := steam.NewRoot()
	sc := model.Shortcut{AppID: 123, Name: g.Name, Exe: `"other.exe"`, Options: "custom", Icon: "custom.png", Tags: []string{"user tag"}}
	_ = steam.Upsert(root, sc)
	data := steam.Encode(root)
	if e = storage.Replace(filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf"), data); e != nil {
		t.Fatal(e)
	}
	r, e = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if e != nil || !r[0].Failed || len(s.State.Accounts[s.Profile.AccountID]) != 0 {
		t.Fatal("silent adoption", r, e)
	}
	r, e = s.Import(context.Background(), []ImportRequest{{Game: g, AdoptID: 123}}, "")
	if e != nil || r[0].Failed {
		t.Fatal(r, e)
	}
	newData, _ := os.ReadFile(filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf"))
	if !bytes.Equal(data, newData) {
		t.Fatal("adoption altered shortcut bytes")
	}
	if !s.State.Accounts[s.Profile.AccountID][g.ID].Adopted {
		t.Fatal("adoption not recorded")
	}
	if e = s.UpdateLaunch(g.ID, false); e != nil {
		t.Fatal(e)
	}
	changed := list(t, s)[0]
	if changed.AppID != 123 || changed.Name != sc.Name || changed.Icon != sc.Icon || changed.Tags[0] != "user tag" || changed.Exe == sc.Exe {
		t.Fatal("launch change lost custom fields", changed)
	}
}
func TestDeleteAllShortcutsAndOptionalCleanup(t *testing.T) {
	for _, cleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep images", true: "remove images"}[cleanup], func(t *testing.T) {
			s, _ := fixture(t)
			root := steam.NewRoot()
			_ = steam.Upsert(root, model.Shortcut{AppID: 123, Name: "external"})
			_ = steam.Upsert(root, model.Shortcut{AppID: 1234, Name: "keep"})
			_ = storage.Replace(filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf"), steam.Encode(root))
			own := filepath.Join(s.Profile.GridDir, "123p.png")
			other := filepath.Join(s.Profile.GridDir, "1234p.png")
			_ = storage.Replace(own, []byte("image"))
			_ = storage.Replace(other, []byte("other"))
			if e := s.Delete([]uint32{123}, cleanup); e != nil {
				t.Fatal(e)
			}
			shortcuts := list(t, s)
			if len(shortcuts) != 1 || shortcuts[0].AppID != 1234 {
				t.Fatal(shortcuts)
			}
			_, e := os.Stat(own)
			if cleanup != os.IsNotExist(e) {
				t.Fatal("wrong image cleanup", e)
			}
			if _, e = os.Stat(other); e != nil {
				t.Fatal("deleted other game's image")
			}
		})
	}
}
func TestPreflightAndCorruptVDF(t *testing.T) {
	s, g := fixture(t)
	s.Preflight = func() error { return errors.New("Steam running") }
	if _, e := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); e == nil {
		t.Fatal("running Steam allowed")
	}
	s.Preflight = func() error { return nil }
	path := filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf")
	_ = storage.Replace(path, []byte("corrupt"))
	if _, e := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); e == nil {
		t.Fatal("corrupt file overwritten")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "corrupt" {
		t.Fatal("corrupt bytes modified")
	}
}
func TestArtworkOwnershipPreservationAndReplace(t *testing.T) {
	s, g := fixture(t)
	_, e := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if e != nil {
		t.Fatal(e)
	}
	id := s.State.Accounts[s.Profile.AccountID][g.ID].AppID
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if e = s.ApplyArtwork("external", model.Portrait, b.Bytes(), false); e == nil {
		t.Fatal("external artwork editable")
	}
	old := filepath.Join(s.Profile.GridDir, steam.ArtBase(id, model.Portrait)+".jpg")
	_ = storage.Replace(old, []byte("custom existing"))
	if e = s.ApplyArtwork(g.ID, model.Portrait, b.Bytes(), true); e != nil {
		t.Fatal(e)
	}
	oldData, _ := os.ReadFile(old)
	if string(oldData) != "custom existing" {
		t.Fatal("overwrote existing artwork")
	}
	if e = s.ApplyArtwork(g.ID, model.Portrait, b.Bytes(), false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(old); !os.IsNotExist(e) {
		t.Fatal("old conflicting image not removed")
	}
	if _, e = os.Stat(filepath.Join(s.Profile.GridDir, steam.ArtBase(id, model.Portrait)+".png")); e != nil {
		t.Fatal(e)
	}
}
func TestCandidateMatching(t *testing.T) {
	g := model.Game{ID: "ns:item:app", Name: "Game", Executable: `C:\Games\game.exe`}
	items := []model.Shortcut{{AppID: 1, Name: "other", Exe: `"c:/Games/game.exe"`}, {AppID: 2, Name: "launcher", Options: "com.epicgames.launcher://apps/ns%3Aitem%3Aapp?action=launch"}, {AppID: 3, Name: "GAME"}, {AppID: 4, Name: "unrelated"}}
	if got := Candidates(g, items); len(got) != 3 {
		t.Fatal(got)
	}
}

func TestUpdateLaunchAfterInstallationMove(t *testing.T) {
	s, g := fixture(t)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	originalID := s.State.Accounts[s.Profile.AccountID][g.ID].AppID
	newDir := t.TempDir()
	newExe := filepath.Join(newDir, "moved.exe")
	if err := os.WriteFile(newExe, nil, 0600); err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(map[string]any{"DisplayName": g.Name, "InstallLocation": newDir, "LaunchExecutable": "moved.exe", "CatalogNamespace": "ns", "CatalogItemId": "item", "AppName": "app", "LaunchCommand": "-updated"})
	if err := storage.Replace(filepath.Join(s.State.Settings.EpicManifestPath, "game.item"), manifest); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateLaunch(g.ID, false); err != nil {
		t.Fatal(err)
	}
	sc := list(t, s)[0]
	if sc.Exe != `"`+newExe+`"` || sc.Options != "-updated" || sc.AppID != originalID {
		t.Fatal(sc)
	}
}

func TestRefreshReloadsExternalStateAndRejectsAccountSwitch(t *testing.T) {
	s, g := fixture(t)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	state := cloneState(s.State)
	state.Settings.EpicLauncherPath = "external change"
	data, _ := storage.StateBytes(state)
	if err := storage.Replace(s.stateFile.Path, data); err != nil {
		t.Fatal(err)
	}
	snap := s.Refresh()
	if snap.Settings.EpicLauncherPath != "external change" {
		t.Fatal("refresh did not reload state")
	}
	login := filepath.Join(s.Profile.Root, "config", "loginusers.vdf")
	if err := storage.Replace(login, []byte(`"users" { "76561197960265730" { "MostRecent" "1" } }`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete([]uint32{list(t, s)[0].AppID}, false); err == nil {
		t.Fatal("operation followed a newly changed account")
	}
}

func TestSaveCustomLaunchAndRejectStaleEditor(t *testing.T) {
	s, g := fixture(t)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	original := list(t, s)[0]
	draft := model.LaunchSettings{ForceEpic: true, Exe: `"C:\Custom Launcher\launcher.exe"`, StartDir: `"C:\Custom Launcher"`, Options: `--flag "中文 value"`}
	if err := s.SaveLaunch(g.ID, draft, original); err != nil {
		t.Fatal(err)
	}
	current := list(t, s)[0]
	if current.AppID != original.AppID || current.Name != original.Name || current.Icon != original.Icon || current.Tags[0] != original.Tags[0] || current.Exe != draft.Exe || current.Options != draft.Options || current.StartDir != draft.StartDir {
		t.Fatal(current)
	}
	if !s.State.Accounts[s.Profile.AccountID][g.ID].Game.ForceEpic {
		t.Fatal("force option not persisted")
	}
	if err := s.SaveLaunch(g.ID, draft, original); err == nil {
		t.Fatal("stale editor accepted")
	}
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	if list(t, s)[0].Options != draft.Options {
		t.Fatal("repeat import overwrote flags")
	}
	draft.Exe = ""
	if err := s.SaveLaunch(g.ID, draft, current); err == nil {
		t.Fatal("empty target accepted")
	}
	draft.Exe = current.Exe
	draft.Options = "bad\x00flag"
	if err := s.SaveLaunch(g.ID, draft, current); err == nil {
		t.Fatal("NUL accepted")
	}
}

func TestPreviewLaunchDoesNotWriteAndRespectsForce(t *testing.T) {
	s, g := fixture(t)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	original := list(t, s)[0]
	launcher := filepath.Join(t.TempDir(), "Epic.exe")
	if err := os.WriteFile(launcher, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.State.Settings.EpicLauncherPath = launcher
	s.Preflight = func() error { return errors.New("Steam running") }
	draft, err := s.PreviewLaunch(g.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !draft.ForceEpic || draft.Exe != `"`+launcher+`"` || draft.Options != "com.epicgames.launcher://apps/ns%3Aitem%3Aapp?action=launch&silent=true" {
		t.Fatal(draft)
	}
	if got := list(t, s)[0]; got.Exe != original.Exe || got.Options != original.Options {
		t.Fatal("preview wrote command")
	}
	if err = s.SaveLaunch(g.ID, draft, original); err == nil {
		t.Fatal("saved while Steam running")
	}
	draft, err = s.PreviewLaunch(g.ID, false)
	if err != nil || draft.Exe != original.Exe {
		t.Fatal(draft, err)
	}
}

func TestNetworkSettingsPersistAndValidate(t *testing.T) {
	s, _ := fixture(t)
	settings := s.State.Settings
	settings.Network = model.NetworkSettings{Mode: "custom", ProxyURL: "http://127.0.0.1:7890"}
	if e := s.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	state, e := storage.Load(s.Dir)
	if e != nil || state.Settings.Network != settings.Network {
		t.Fatal(state.Settings.Network, e)
	}
	settings.Network.ProxyURL = "invalid"
	if e = s.SaveSettings(settings); e == nil {
		t.Fatal("invalid proxy saved")
	}
}
