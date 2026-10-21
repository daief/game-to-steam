package ui

import (
	"testing"

	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func TestEpicFiltersAndVisibleSelection(t *testing.T) {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{AppID: 1}
	a.snapshot.Games[1].InstallDir = `D:\Games\Beta`
	tt := nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click("已导入 (1)"); err != nil {
		t.Fatal(err)
	}
	if games := a.visibleEpicGames(); len(games) != 1 || games[0].ID != "one" {
		t.Fatal(games)
	}
	if err := tt.Click("全选可见"); err != nil {
		t.Fatal(err)
	}
	if !a.selectedGames["one"] || a.selectedGames["two"] {
		t.Fatal(a.selectedGames)
	}
	if err := tt.Click("未导入 (1)"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("搜索游戏"); err != nil {
		t.Fatal(err)
	}
	tt.Type(`D:\Games`)
	if games := a.visibleEpicGames(); len(games) != 1 || games[0].ID != "two" {
		t.Fatal(games)
	}
	if err := tt.Click("选择当前列表"); err != nil {
		t.Fatal(err)
	}
	if a.gameSelectionCount() != 2 {
		t.Fatal(a.selectedGames)
	}
	if err := tt.Click("选择当前列表"); err != nil {
		t.Fatal(err)
	}
	if !a.selectedGames["one"] || a.selectedGames["two"] {
		t.Fatal("header changed hidden selection", a.selectedGames)
	}
}

func TestEpicRemovedShortcutIsNotImported(t *testing.T) {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{AppID: 99}
	if a.importedGame("one") {
		t.Fatal("stale association treated as imported")
	}
	a.epicFilter = "已导入"
	if len(a.visibleEpicGames()) != 0 {
		t.Fatal("stale association included in imported filter")
	}
	a.epicFilter = "未导入"
	if len(a.visibleEpicGames()) != 2 {
		t.Fatal("missing reimportable game")
	}
}

func TestEpicSearchKeyboardShortcut(t *testing.T) {
	a := testApp(t)
	tt := nativeui.NewTester(a.view, 1100, 800)
	tt.Key(nativeui.Ctrl, nativeui.KeyF)
	tt.Type("Beta")
	if a.query != "Beta" {
		t.Fatalf("Ctrl+F did not focus search: %q", a.query)
	}
	tt.Key(0, nativeui.KeyEscape)
	if a.query != "" {
		t.Fatal("Escape did not clear search")
	}
}

func TestEpicSingleImportDoesNotIncludeOtherSelections(t *testing.T) {
	a := testApp(t)
	a.snapshot.Shortcuts = []model.Shortcut{{AppID: 8, Name: "Beta", Exe: "other.exe"}}
	a.selectedGames["one"] = true
	a.importOne(a.snapshot.Games[1])
	if !a.adoptOpen || len(a.imports) != 1 || a.imports[0].Game.ID != "two" {
		t.Fatal("single import consumed batch selection", a.imports)
	}
}
