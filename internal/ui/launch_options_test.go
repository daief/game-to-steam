package ui

import (
	"reflect"
	"testing"

	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
	"golang.org/x/sys/windows"
)

func TestGameOptionsPreserveBridgeCommand(t *testing.T) {
	prefix := []string{"--game", "ns:item:game", "--manifest-dir", `C:\Program Data\Epic`, "--launch-mode", "authenticated"}
	original := windows.ComposeCommandLine(append(append([]string{}, prefix...), "--", "-windowed", "--title", "a b"))
	e := &shortcutEditor{draft: model.LaunchSettings{Exe: `"C:\Game2Steam\epic-bridge.exe"`, Options: original}}
	e.syncOptions()
	if e.draft.Options != original || e.gameOptions != `-windowed --title "a b"` {
		t.Fatal("opening editor changed command or exposed bridge options", e)
	}
	for _, gameOptions := range []string{`--title "new value" -windowed`, "", `--name "--"`} {
		e.gameOptions = gameOptions
		e.updateGameOptions()
		if e.optionsError != nil {
			t.Fatal(e.optionsError)
		}
		actual, err := windows.DecomposeCommandLine("bridge " + e.draft.Options)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual[1:1+len(prefix)], prefix) {
			t.Fatal("bridge options changed", actual)
		}
		if gameOptions == "" && len(actual) != len(prefix)+1 {
			t.Fatal("empty args retained separator", actual)
		}
	}
	e.draft.Options = original
	e.syncOptions()
	if e.gameOptions != `-windowed --title "a b"` {
		t.Fatal("advanced editing did not refresh game options")
	}
	e.gameOptions = "invalid\x00argument"
	e.updateGameOptions()
	if e.optionsError == nil || e.draft.Options != original {
		t.Fatal("invalid game options damaged bridge command")
	}
}

func TestDirectGameOptionsRemainRaw(t *testing.T) {
	e := &shortcutEditor{draft: model.LaunchSettings{Exe: `"D:\Game\game.exe"`, Options: `--game arbitrary  -windowed`}}
	e.syncOptions()
	if e.bridgeOptions != nil || e.gameOptions != e.draft.Options {
		t.Fatal("ordinary executable treated as bridge")
	}
	e.gameOptions = `--name "some value"  -windowed`
	e.updateGameOptions()
	if e.draft.Options != e.gameOptions {
		t.Fatal("direct args were rewritten")
	}
}

func TestLaunchAdvancedFieldsAreOptional(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	a.selectShortcut(a.snapshot.Shortcuts[0])
	tt := nativeui.NewTester(a.view, 1100, 800)
	if tt.HasText("启动程序") || tt.HasText("完整启动参数编辑") {
		t.Fatal("advanced command fields exposed by default")
	}
	if err := tt.Click("高级设置"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("启动程序") || !tt.HasText("完整启动参数编辑") || !tt.HasText("启动程序说明") {
		t.Fatal("advanced command fields unavailable")
	}
	tt.SetSize(980, 600)
	if !a.shortcutEditors[101].advanced {
		t.Fatal("resize closed advanced fields")
	}
	if err := tt.Click("收起高级设置"); err != nil {
		t.Fatal(err)
	}
	assertVisible(t, tt, "保存启动设置", [2]int{980, 600})
}

func TestBridgeGameOptionsEditThroughUI(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	sc := &a.snapshot.Shortcuts[0]
	sc.Exe = `"C:\Game2Steam\epic-bridge.exe"`
	sc.Options = `--game opaque-id --manifest-dir "C:\Epic Manifests" --launch-mode authenticated`
	a.selectShortcut(*sc)
	tt := nativeui.NewTester(a.view, 1100, 800)
	e := a.shortcutEditors[101]
	if e.gameOptions != "" || tt.HasText("opaque-id") {
		t.Fatal("internal bridge args leaked into game editor")
	}
	if err := tt.Click("启动参数"); err != nil {
		t.Fatal(err)
	}
	tt.Type(`-windowed`)
	args, err := windows.DecomposeCommandLine("bridge " + e.draft.Options)
	if err != nil || len(args) != 9 || args[7] != "--" || args[8] != "-windowed" {
		t.Fatal("game args not attached after bridge separator", args, err)
	}
	if sc.Options != `--game opaque-id --manifest-dir "C:\Epic Manifests" --launch-mode authenticated` {
		t.Fatal("draft changed saved shortcut")
	}
}

func TestLaunchPathsDisplayWithoutQuotes(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	a.selectShortcut(a.snapshot.Shortcuts[0])
	e := a.shortcutEditors[101]
	e.advanced = true
	original := e.draft
	tt := nativeui.NewTester(a.view, 1100, 800)
	if e.draft != original {
		t.Fatal("rendering changed stored command")
	}
	if err := tt.Click("启动程序路径"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, nativeui.KeyHome)
	tt.Key(0, nativeui.KeyDelete)
	if e.draft.Exe != `":\Games\Hades\Hades.exe"` {
		t.Fatal("input still displayed a leading quote", e.draft.Exe)
	}
	tt.Key(nativeui.Ctrl, nativeui.KeyA)
	tt.Type(`C:\Program Files\Game\game.exe`)
	if e.draft.Exe != `"C:\Program Files\Game\game.exe"` {
		t.Fatal("edited command path lost required quoting", e.draft.Exe)
	}
}

func TestLaunchEditorShowsUnsavedChanges(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	a.selectShortcut(a.snapshot.Shortcuts[0])
	tt := nativeui.NewTester(a.view, 980, 600)
	if tt.HasText("未保存") {
		t.Fatal("unchanged command marked unsaved")
	}
	if err := tt.Click("启动参数"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" --changed")
	if !tt.HasText("未保存") {
		t.Fatal("edited command missing unsaved marker")
	}
	tt.Key(nativeui.Ctrl, nativeui.KeyA)
	tt.Type(a.snapshot.Shortcuts[0].Options)
	if tt.HasText("未保存") {
		t.Fatal("restored command still marked unsaved")
	}
	e := a.shortcutEditors[101]
	for _, field := range []*string{&e.draft.Exe, &e.draft.StartDir} {
		original := *field
		*field += "changed"
		tt.Frame()
		if !tt.HasText("未保存") {
			t.Fatal("path edit missing unsaved marker")
		}
		*field = original
	}
	tt.Frame()
	if tt.HasText("未保存") {
		t.Fatal("restored paths still marked unsaved")
	}
}
