//go:build windows

package bridge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"game-to-steam/internal/model"
	"golang.org/x/sys/windows"
)

func TestShortcutCommandQuotingAndDirectBypass(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "中文 space")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(dir, "EpicGamesLauncher.exe")
	executable := filepath.Join(dir, "epic-bridge.exe")
	gameExe := filepath.Join(dir, "game.exe")
	for _, path := range []string{launcher, executable, gameExe} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	g := model.Game{ID: "ns:item:app", Name: "游戏", InstallDir: dir, Executable: gameExe, LaunchCommand: "-custom", RequiresAuth: true}
	sc, err := MakeShortcut(g, launcher, executable, dir)
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine("bridge " + sc.Options)
	want := []string{"bridge", "--game", g.ID, "--manifest-dir", dir, "--launcher", launcher}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%q err=%v", args, err)
	}
	if sc.Exe != `"`+executable+`"` || sc.StartDir != `"`+dir+`"` || sc.Icon != gameExe || ShortcutGameID(sc) != g.ID {
		t.Fatal(sc)
	}
	// Direct launch does not depend on bridge, launcher, or manifest availability.
	g.RequiresAuth = false
	sc, err = MakeShortcut(g, "", "", "")
	if err != nil || sc.Exe != `"`+gameExe+`"` || sc.Options != "-custom" {
		t.Fatal(sc, err)
	}
	g.RequiresAuth = true
	if _, err := MakeShortcut(g, launcher, filepath.Join(dir, "missing.exe"), dir); err == nil {
		t.Fatal("missing bridge silently accepted")
	}
	g.Executable = ""
	if _, err := MakeShortcut(g, launcher, executable, dir); err == nil {
		t.Fatal("accepted no executable to monitor")
	}
}

func TestShortcutGameIDOnlyRecognizesBridge(t *testing.T) {
	for _, tc := range []struct{ exe, options, want string }{
		{`"C:\apps\epic-bridge.exe"`, `--game "ns:item:app" --exit-grace 10s`, "ns:item:app"},
		{`"C:\apps\EPIC-BRIDGE.EXE"`, `--game=ns:item:app`, "ns:item:app"},
		{`"C:\apps\other.exe"`, `--game ns:item:app`, ""},
		{`"C:\apps\epic-bridge.exe"`, `--game`, ""},
	} {
		if got := ShortcutGameID(model.Shortcut{Exe: tc.exe, Options: tc.options}); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}

func TestAuthenticatedShortcutDoesNotNeedLauncherOrLogin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "中文 space")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "game.exe")
	bridge := filepath.Join(dir, "epic-bridge.exe")
	for _, p := range []string{exe, bridge} {
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	g := model.Game{ID: "ns:item:app", Name: "Game", InstallDir: dir, Executable: exe, RequiresAuth: true}
	sc, err := MakeAuthenticatedShortcut(g, bridge, dir)
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine("bridge " + sc.Options)
	want := []string{"bridge", "--game", g.ID, "--manifest-dir", dir, "--launch-mode", "authenticated"}
	if err != nil || !reflect.DeepEqual(args, want) || sc.Icon != exe {
		t.Fatal(sc, args, err)
	}
	if _, err = MakeAuthenticatedShortcut(g, "missing", dir); err == nil {
		t.Fatal("accepted missing bridge")
	}
	g.Executable = filepath.Join(dir, "missing.exe")
	if _, err = MakeAuthenticatedShortcut(g, bridge, dir); err == nil {
		t.Fatal("accepted missing game")
	}
}
