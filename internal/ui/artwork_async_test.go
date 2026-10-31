package ui

import (
	"context"
	"errors"
	"testing"

	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func TestArtworkRejectsObsoleteCompletion(t *testing.T) {
	a := testApp(t)
	a.detailID = "one"
	a.artRequest = 2
	a.artLoading = true
	applied := false
	update := func() { applied = true }
	a.finishArtwork(1, "one", update, errors.New("old request"))
	a.finishArtwork(2, "two", update, nil)
	if applied || !a.artLoading || a.artError != "" {
		t.Fatal("obsolete result changed current request")
	}
	a.finishArtwork(2, "one", update, nil)
	if !applied || a.artLoading {
		t.Fatal("current result not applied")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.artCancel = cancel
	a.artLoading = true
	a.cancelArtwork()
	if ctx.Err() == nil || a.artLoading || a.artRequest != 3 {
		t.Fatal("request not cancelled")
	}
}

func TestArtworkLoadingDoesNotDisableNavigation(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	a.selectShortcut(a.snapshot.Shortcuts[0])
	a.shortcutTab = 1
	a.openShortcutArtwork(a.snapshot.Shortcuts[0])
	a.artLoading = true
	tt := nativeui.NewTester(a.view, 980, 600)
	if !tt.HasText("封面搜索中") {
		t.Fatal("missing local loading indicator")
	}
	button, _ := tt.Find("搜索")
	buttonSpinner, _ := tt.Find("封面搜索中")
	if buttonSpinner.X < button.X || buttonSpinner.X+buttonSpinner.W > button.X+button.W {
		t.Fatal("search spinner outside button")
	}
	gallery, _ := tt.Find("候选图片列表")
	spinner, _ := tt.Find("正在加载封面")
	if dx, dy := (spinner.X+spinner.W/2)-(gallery.X+gallery.W/2), (spinner.Y+spinner.H/2)-(gallery.Y+gallery.H/2); dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		t.Fatal("gallery spinner not centered", spinner, gallery)
	}
	request := a.artRequest
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if !a.artLoading || a.artRequest != request {
		t.Fatal("loading search accepted another request")
	}
	if a.busy {
		t.Fatal("network read disabled global UI")
	}
	if err := tt.Click("查看 Death Stranding · 死亡搁浅"); err != nil {
		t.Fatal(err)
	}
	if a.detailID != "two" || a.artLoading {
		t.Fatal("game switch did not cancel previous request")
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if a.page != "设置" {
		t.Fatal("network read blocked navigation")
	}
}

func TestFixedLayoutAndCompactArtworkToolbar(t *testing.T) {
	for _, size := range [][2]int{{980, 600}, {1100, 800}, {1600, 900}} {
		a := artworkTestApp(t)
		a.key = "mock"
		tt := nativeui.NewTester(a.view, size[0], size[1])
		first := assertVisible(t, tt, "竖版封面", size)
		for _, label := range []string{"封面搜索词", "搜索", "本地图片"} {
			r := assertVisible(t, tt, label, size)
			if d := (r.Y + r.H/2) - (first.Y + first.H/2); d < -3 || d > 3 {
				t.Fatalf("%s wrapped at %v: %v vs %v", label, size, r, first)
			}
		}
		assertVisible(t, tt, "Epic 游戏", size)
		assertVisible(t, tt, "快捷方式列表", size)
		if tt.HasText("epic-bridge.exe") {
			t.Fatal("bridge executable exposed in game list")
		}
	}
}

func TestShortcutIconPrefersSteamAndRealGame(t *testing.T) {
	sc := model.Shortcut{AppID: 1, Icon: `C:\icons\game.ico`, Exe: `"C:\apps\epic-bridge.exe"`}
	paths := shortcutIconPaths(sc, map[string]model.ManagedGame{"one": {AppID: 1, Game: model.Game{Executable: `D:\game\game.exe`}}})
	if len(paths) != 2 || paths[0] != sc.Icon || paths[1] != `D:\game\game.exe` {
		t.Fatal(paths)
	}
}

func TestSettingsCardsHaveEqualHeight(t *testing.T) {
	a := testApp(t)
	a.page = "设置"
	tt := nativeui.NewTester(a.view, 1600, 900)
	epic, ok := tt.Find("Epic 认证分组")
	if !ok {
		t.Fatal("missing Epic settings")
	}
	art, ok := tt.Find("SteamGridDB 封面服务分组")
	if !ok || epic.Y != art.Y || epic.H != art.H {
		t.Fatal("settings cards not aligned", epic, art)
	}
}
