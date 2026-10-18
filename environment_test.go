package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/model"
	"game-to-steam/internal/steam"
	"github.com/egoist/mygo/ui"
)

// Opt-in diagnostics inspect real launcher files but never write to them.
func TestInstalledEnvironmentReadOnly(t *testing.T) {
	if os.Getenv("GAME_TO_STEAM_LIVE_READONLY") != "1" {
		t.Skip("opt-in read-only environment check")
	}
	s, err := service.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Refresh()
	for _, warning := range snap.Warnings {
		t.Log(warning)
	}
	if snap.Profile.AccountID == "" {
		t.Fatal("Steam profile discovery failed")
	}
	path := filepath.Join(snap.Profile.ConfigDir, "shortcuts.vdf")
	data, err := os.ReadFile(path)
	if err == nil {
		root, e := steam.Parse(data)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(data, steam.Encode(root)) {
			t.Fatal("real VDF roundtrip changes data")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Logf("discovered %d installed Epic games and %d non-Steam shortcuts", len(snap.Games), len(snap.Shortcuts))
	t.Logf("Epic launcher discovered: %t", s.State.Settings.EpicLauncherPath != "")
}

// Export an optional preview using MyGo's actual native software renderer.
func TestPreviewArtifact(t *testing.T) {
	dir := os.Getenv("GAME_TO_STEAM_PREVIEW_DIR")
	if dir == "" {
		t.Skip("optional UI preview")
	}
	a := testApp(t)
	a.snapshot.Games = []model.Game{
		{ID: "alpha", Name: "Hades · 黑帝斯", InstallDir: `D:\Games\Hades`, Executable: `D:\Games\Hades\Hades.exe`},
		{ID: "beta", Name: "Death Stranding · 死亡搁浅", InstallDir: `D:\Games\DeathStranding`, Managed: true},
		{ID: "gamma", Name: "Celeste · 蔚蓝", InstallDir: `D:\Games\Celeste`},
	}
	a.status = "扫描完成 · 3 个 Epic 游戏"
	a.selectedGames["alpha"] = true
	tt := ui.NewTester(a.view, 1100, 780)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "ui-preview.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	a.snapshot.Managed["alpha"] = model.ManagedGame{Game: a.snapshot.Games[0], AppID: 123}
	a.snapshot.Shortcuts = []model.Shortcut{{AppID: 123, Name: "Hades · 黑帝斯", Exe: `"D:\Games\Hades\Hades.exe"`, StartDir: `"D:\Games\Hades"`, Options: "-windowed --example"}}
	a.openLaunch("alpha")
	editor := ui.NewTester(a.view, 1100, 780)
	launchFile, e := os.Create(filepath.Join(dir, "launch-preview.png"))
	if e != nil {
		t.Fatal(e)
	}
	defer launchFile.Close()
	if e = png.Encode(launchFile, editor.Image()); e != nil {
		t.Fatal(e)
	}
	a.launchOpen = false
	a.page = "设置"
	a.settings.Network.Mode = "custom"
	a.settings.Network.ProxyURL = "http://127.0.0.1:7890"
	settingsView := ui.NewTester(a.view, 1100, 780)
	settingsFile, e := os.Create(filepath.Join(dir, "settings-preview.png"))
	if e != nil {
		t.Fatal(e)
	}
	defer settingsFile.Close()
	if e = png.Encode(settingsFile, settingsView.Image()); e != nil {
		t.Fatal(e)
	}
	a.epicAuthOpen = true
	a.epicAuthCode = "preview-not-a-real-code"
	authView := ui.NewTester(a.view, 1100, 780)
	authFile, e := os.Create(filepath.Join(dir, "auth-preview.png"))
	if e != nil {
		t.Fatal(e)
	}
	defer authFile.Close()
	if e = png.Encode(authFile, authView.Image()); e != nil {
		t.Fatal(e)
	}
}
