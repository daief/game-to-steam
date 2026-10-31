package system

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSteamExitUsesNormalShutdown(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Steam Client")
	cmd, err := steamExitCommand(root)
	if err != nil || !reflect.DeepEqual(cmd.Args, []string{filepath.Join(root, "steam.exe"), "-shutdown"}) {
		t.Fatal(cmd, err)
	}
	if _, err := steamExitCommand("relative"); err == nil {
		t.Fatal("relative executable accepted")
	}
}
