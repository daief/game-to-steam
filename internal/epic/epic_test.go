package epic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"game-to-steam/internal/model"
)

func TestScanValidDuplicateBrokenAndStale(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "中文 game")
	if err := os.Mkdir(install, 0700); err != nil {
		t.Fatal(err)
	}
	m := manifest{DisplayName: "游戏", InstallLocation: install, LaunchExecutable: "bin/game.exe", CatalogNamespace: "ns", CatalogItemId: "item", AppName: "app"}
	put := func(name string, m manifest) {
		t.Helper()
		b, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("one.item", m)
	put("duplicate.item", m)
	m.AppName = "stale"
	m.InstallLocation = filepath.Join(dir, "missing")
	put("stale.item", m)
	_ = os.WriteFile(filepath.Join(dir, "broken.item"), []byte("{"), 0600)
	games, warnings, err := Scan(dir)
	if err != nil || len(games) != 1 || len(warnings) != 2 {
		t.Fatalf("%+v %v %v", games, warnings, err)
	}
	if games[0].ID != "ns:item:app" || games[0].Executable != filepath.Join(install, "bin", "game.exe") {
		t.Fatal(games[0])
	}
}
func TestLaunchModesAndUnicodePaths(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "游戏 exe.exe")
	launcher := filepath.Join(dir, "Epic launcher.exe")
	_ = os.WriteFile(exe, nil, 0600)
	_ = os.WriteFile(launcher, nil, 0600)
	base := model.Game{ID: "ns:item:app", Name: "游戏", InstallDir: dir, Executable: exe, LaunchCommand: "-flag"}
	for _, mode := range []string{"direct", "managed", "dlc", "auth", "ownership", "forced"} {
		g := base
		g.Managed = mode == "managed"
		g.HasDLC = mode == "dlc"
		g.ForceEpic = mode == "forced"
		g.RequiresAuth = mode == "auth"
		g.RequiresOwnershipToken = mode == "ownership"
		sc, err := MakeShortcut(g, launcher)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "direct" {
			if sc.Exe != `"`+exe+`"` || sc.Options != "-flag" {
				t.Fatal(sc)
			}
		} else if sc.Exe != `"`+launcher+`"` || sc.Options != "com.epicgames.launcher://apps/ns%3Aitem%3Aapp?action=launch&silent=true" {
			t.Fatal(sc)
		}
	}
	base.ForceEpic = true
	if _, err := MakeShortcut(base, filepath.Join(dir, "missing.exe")); err == nil {
		t.Fatal("accepted missing launcher")
	}
}

func TestScanAuthenticationRequirements(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		viaEpic      bool
	}{
		{"missing", "", false},
		{"false", `,"bRequiresAuth":false,"OwnershipToken":"false"`, false},
		{"auth", `,"bRequiresAuth":true,"bCanRunOffline":true`, true},
		{"token-string", `,"OwnershipToken":"true"`, true},
		{"token-bool", `,"OwnershipToken":true`, true},
		{"token-false", `,"OwnershipToken":false`, false},
		{"token-null", `,"OwnershipToken":null`, false},
		{"hogwarts", `,"bIsManaged":false,"bRequiresAuth":true,"bCanRunOffline":true,"OwnershipToken":"true","LaunchCommand":"-SaveToUserDir -UserDir=HogwartsLegacy"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			install, _ := json.Marshal(dir)
			data := `{"DisplayName":"游戏","InstallLocation":` + string(install) + `,"LaunchExecutable":"game.exe","CatalogNamespace":"ns","CatalogItemId":"item","AppName":"app"` + tc.fields + `}`
			if err := os.WriteFile(filepath.Join(dir, "game.item"), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			games, warnings, err := Scan(dir)
			if err != nil || len(warnings) != 0 || len(games) != 1 {
				t.Fatalf("games=%+v warnings=%v err=%v", games, warnings, err)
			}
			if games[0].ViaEpic() != tc.viaEpic {
				t.Fatalf("unexpected launch strategy: %+v", games[0])
			}
		})
	}
}
