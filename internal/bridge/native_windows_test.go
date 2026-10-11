//go:build windows

package bridge

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanSteamEnvironment(t *testing.T) {
	env := cleanEnvironment([]string{"SteamAppId=123", "STEAM_GAME_ID=456", "SteamOverlayGameId=789", "PATH=C:\\Windows", "USERPROFILE=C:\\User"})
	if strings.Join(env, "|") != "PATH=C:\\Windows|USERPROFILE=C:\\User" {
		t.Fatal(env)
	}
}

func TestDesktopHelperProcess(t *testing.T) {
	if os.Getenv("EPIC_BRIDGE_TEST_HELPER") != "1" {
		return
	}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(strings.ToLower(entry), "steam") {
			os.Exit(2)
		}
	}
	if os.WriteFile(os.Getenv("EPIC_BRIDGE_TEST_MARKER"), []byte("ok"), 0600) != nil {
		os.Exit(3)
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

// Opt-in: starts only our own short-lived test helper, never Epic/a game.
func TestDesktopParentAndNativeSnapshot(t *testing.T) {
	if os.Getenv("GAME_TO_STEAM_BRIDGE_NATIVE_TEST") != "1" {
		t.Skip("opt-in desktop-parent check")
	}
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("EPIC_BRIDGE_TEST_HELPER", "1")
	t.Setenv("EPIC_BRIDGE_TEST_MARKER", marker)
	t.Setenv("SteamAppId", "123")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pid, parent, err := startDesktop(exe, []string{"-test.run=^TestDesktopHelperProcess$"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		p, err := os.FindProcess(int(pid))
		if err == nil {
			_ = p.Kill()
			_ = p.Release()
		} // only the test helper we just created
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "ok" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test helper did not start or environment was not cleaned")
		}
		time.Sleep(20 * time.Millisecond)
	}
	processes, err := (Native{Log: log.Default()}).Snapshot([]string{filepath.Base(exe)})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range processes {
		if p.PID == pid {
			if p.ParentPID != parent || parent == uint32(os.Getpid()) || p.Created == 0 || !strings.EqualFold(p.Path, exe) {
				t.Fatalf("unexpected process: %+v desktop=%d", p, parent)
			}
			t.Logf("helper pid=%d desktop parent=%d image=%s", p.PID, p.ParentPID, p.Path)
			return
		}
	}
	t.Fatal("native snapshot did not find helper")
}
