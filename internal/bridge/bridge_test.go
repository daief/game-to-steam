package bridge

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"game-to-steam/internal/model"
)

type fakeBackend struct {
	snapshots       [][]Process
	index, launches int
	launcher, uri   string
	launchErr       error
	snapshotErrAt   int
}

func (f *fakeBackend) Snapshot([]string) ([]Process, error) {
	f.index++
	if f.snapshotErrAt == f.index {
		return nil, errors.New("access denied")
	}
	if len(f.snapshots) == 0 {
		return nil, nil
	}
	index := f.index - 1
	if index >= len(f.snapshots) {
		index = len(f.snapshots) - 1
	}
	return f.snapshots[index], nil
}

func (f *fakeBackend) Launch(launcher, uri string) (uint32, error) {
	f.launches++
	f.launcher, f.uri = launcher, uri
	return 123, f.launchErr
}

func testConfig(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	return Config{
		Game:           model.Game{ID: "ns:item:app", Name: "游戏", InstallDir: dir, Executable: filepath.Join(dir, "game.exe")},
		Launcher:       filepath.Join(dir, "EpicGamesLauncher.exe"),
		StartupTimeout: 100 * time.Millisecond, ExitGrace: 4 * time.Millisecond, PollInterval: time.Millisecond,
	}
}

func TestMatchesPathsAndWatchOverride(t *testing.T) {
	c := testConfig(t)
	for _, tc := range []struct {
		path string
		want bool
	}{
		{c.Game.Executable, true},
		{filepath.Join(c.Game.InstallDir, "Phoenix", "Binaries", "Game.EXE"), true},
		{filepath.Join(c.Game.InstallDir, "CrashReportClient.exe"), false},
		{filepath.Join(c.Game.InstallDir+"-other", "game.exe"), false},
	} {
		if got := c.matches(Process{Path: tc.path}); got != tc.want {
			t.Errorf("%s: got %t", tc.path, got)
		}
	}
	c.WatchExe = filepath.Join(c.Game.InstallDir, "bin", "real.exe")
	if !c.matches(Process{Path: c.WatchExe}) || c.matches(Process{Path: c.Game.Executable}) {
		t.Fatal("override must match exact path")
	}
	c.WatchExe = filepath.Join(c.Game.InstallDir, "..", "other.exe")
	if c.validate() == nil {
		t.Fatal("accepted watch executable outside install")
	}
}

func TestMonitorBootstrapHandoffAndEpicRemaining(t *testing.T) {
	c := testConfig(t)
	entry := Process{PID: 1, Created: 1, Path: c.Game.Executable}
	real := Process{PID: 2, Created: 2, Path: filepath.Join(c.Game.InstallDir, "Phoenix", "Binaries", "game.exe")}
	epic := Process{PID: 3, Path: c.Launcher}
	f := &fakeBackend{snapshots: [][]Process{{epic}, {entry, epic}, {epic}, {real, epic}, {real, epic}, {epic}}}
	var output bytes.Buffer
	if err := Run(context.Background(), c, f, log.New(&output, "", 0)); err != nil {
		t.Fatal(err)
	}
	if f.launches != 1 || f.uri != "com.epicgames.launcher://apps/ns%3Aitem%3Aapp?action=launch&silent=true" {
		t.Fatal(f)
	}
	for _, expected := range []string{"game-start pid=1", "game-start pid=2", "game-exit pid=2", "remaining pid=3", "session finished"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q: %s", expected, output.String())
		}
	}
}

func TestExistingGameRefusesLaunch(t *testing.T) {
	c := testConfig(t)
	f := &fakeBackend{snapshots: [][]Process{{{PID: 1, Path: c.Game.Executable}}}}
	if err := Run(context.Background(), c, f, log.Default()); err == nil || f.launches != 0 {
		t.Fatalf("err=%v launches=%d", err, f.launches)
	}
}

func TestStartupTimeoutCancellationAndErrors(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "preflight", "monitor", "launch", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			c := testConfig(t)
			c.StartupTimeout = 5 * time.Millisecond
			f := &fakeBackend{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "cancel":
				cancel()
			case "preflight":
				f.snapshotErrAt = 1
			case "monitor":
				f.snapshotErrAt = 2
			case "launch":
				f.launchErr = errors.New("no desktop")
			case "invalid":
				c.ExitGrace = -time.Second
			}
			if err := Run(ctx, c, f, log.Default()); err == nil {
				t.Fatal("expected error")
			}
			if (mode == "cancel" || mode == "preflight" || mode == "invalid") && f.launches != 0 {
				t.Fatal("unexpected launch")
			}
			if f.launches > 1 {
				t.Fatal("unexpected retry/fallback")
			}
		})
	}
}

func TestCancellationDuringRunningGame(t *testing.T) {
	c := testConfig(t)
	f := &fakeBackend{snapshots: [][]Process{nil, {{PID: 1, Path: c.Game.Executable}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := Run(ctx, c, f, log.Default()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
