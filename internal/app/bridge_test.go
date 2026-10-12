package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"game-to-steam/internal/bridge"
	"game-to-steam/internal/model"
	"game-to-steam/internal/storage"
)

func bridgeLauncher(t *testing.T, s *Service) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "EpicGamesLauncher.exe")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.State.Settings.EpicLauncherPath = path
	return path
}

func TestEpicLaunchPolicyUsesBridge(t *testing.T) {
	for _, mode := range []string{"managed", "dlc", "auth", "ownership", "forced"} {
		t.Run(mode, func(t *testing.T) {
			s, g := fixture(t)
			bridgeLauncher(t, s)
			g.Managed = mode == "managed"
			g.HasDLC = mode == "dlc"
			g.RequiresAuth = mode == "auth"
			g.RequiresOwnershipToken = mode == "ownership"
			g.ForceEpic = mode == "forced"
			results, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
			if err != nil || len(results) == 0 || results[0].Failed {
				t.Fatal(results, err)
			}
			sc := list(t, s)[0]
			if sc.Exe != `"`+s.BridgePath+`"` || bridge.ShortcutGameID(sc) != g.ID {
				t.Fatal(sc)
			}
		})
	}
}

func TestMissingBridgeFailsEpicButNotDirect(t *testing.T) {
	s, g := fixture(t)
	bridgeLauncher(t, s)
	s.BridgePath = filepath.Join(t.TempDir(), "missing.exe")
	g.RequiresAuth = true
	results, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if err != nil || !results[0].Failed || !strings.Contains(results[0].Message, "找不到桥接程序") || len(list(t, s)) != 0 {
		t.Fatal(results, err)
	}
	g.RequiresAuth = false
	results, err = s.Import(context.Background(), []ImportRequest{{Game: g}}, "")
	if err != nil || results[0].Failed || list(t, s)[0].Exe != `"`+g.Executable+`"` {
		t.Fatal(results, err)
	}
}

func TestBridgeGenerationPreservesExistingCommandsUntilSaved(t *testing.T) {
	s, g := fixture(t)
	bridgeLauncher(t, s)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	original := list(t, s)[0]
	// Simulate a freshly scanned manifest now requiring authentication.
	data, err := os.ReadFile(filepath.Join(s.State.Settings.EpicManifestPath, "game.item"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["bRequiresAuth"] = true
	data, _ = json.Marshal(manifest)
	if err := storage.Replace(filepath.Join(s.State.Settings.EpicManifestPath, "game.item"), data); err != nil {
		t.Fatal(err)
	}
	g.RequiresAuth = true
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list(t, s)[0], original) {
		t.Fatal("repeat import rewrote original")
	}
	draft, err := s.PreviewLaunch(g.ID, false)
	if err != nil || draft.Exe != `"`+s.BridgePath+`"` {
		t.Fatal(draft, err)
	}
	if !reflect.DeepEqual(list(t, s)[0], original) {
		t.Fatal("preview rewrote original")
	}
	if err := s.SaveLaunch(g.ID, draft, original); err != nil {
		t.Fatal(err)
	}
	saved := list(t, s)[0]
	if saved.AppID != original.AppID || saved.Name != original.Name || saved.Icon != original.Icon || !reflect.DeepEqual(saved.Tags, original.Tags) {
		t.Fatal(saved)
	}
	if saved.Exe != draft.Exe || saved.Options != draft.Options {
		t.Fatal(saved)
	}
	// A renamed bridge shortcut remains a duplicate candidate by game identifier.
	saved.Name = "renamed"
	if len(Candidates(g, []model.Shortcut{saved})) != 1 {
		t.Fatal("bridge duplicate not identified")
	}
}

func TestForceBridgeAndRevertToDirect(t *testing.T) {
	s, g := fixture(t)
	bridgeLauncher(t, s)
	if _, err := s.Import(context.Background(), []ImportRequest{{Game: g}}, ""); err != nil {
		t.Fatal(err)
	}
	original := list(t, s)[0]
	if err := s.UpdateLaunch(g.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := list(t, s)[0]; got.AppID != original.AppID || got.Exe != `"`+s.BridgePath+`"` {
		t.Fatal(got)
	}
	if err := s.UpdateLaunch(g.ID, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list(t, s)[0], original) {
		t.Fatal("direct command not restored")
	}
}
