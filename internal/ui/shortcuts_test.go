package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func shortcutWorkspaceApp(t *testing.T) *app {
	a := testApp(t)
	a.page = "Steam 快捷方式"
	a.snapshot.Shortcuts = []model.Shortcut{
		{AppID: 101, Name: "Hades · 黑帝斯", Exe: `"D:\Games\Hades\Hades.exe"`, StartDir: `"D:\Games\Hades"`, Options: "-windowed"},
		{AppID: 102, Name: "Death Stranding · 死亡搁浅", Exe: `"D:\Games\DeathStranding\ds.exe"`, Options: "--example"},
		{AppID: 103, Name: "External shortcut", Exe: `"C:\Tools\external.exe"`},
	}
	for i, id := range []string{"one", "two"} {
		a.snapshot.Managed[id] = model.ManagedGame{Game: model.Game{ID: id, Name: a.snapshot.Shortcuts[i].Name}, AppID: a.snapshot.Shortcuts[i].AppID}
	}
	return a
}

func TestShortcutWorkspaceDraftsSurviveGameAndTabSwitch(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	tt := nativeui.NewTester(a.view, 1100, 800)
	if !tt.HasText("选择一个游戏") {
		t.Fatal("missing empty detail state")
	}
	if err := tt.Click("查看 Hades · 黑帝斯"); err != nil {
		t.Fatal(err)
	}
	if a.launchOpen {
		t.Fatal("inline editor opened modal")
	}
	if err := tt.Click("启动参数"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" --custom")
	want := a.shortcutEditors[101].draft.Options
	if want == "-windowed" {
		t.Fatal("draft not edited")
	}
	if err := tt.Click("封面配置"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("当前设置") {
		t.Fatal("artwork tab did not render")
	}
	if err := tt.Click("查看 Death Stranding · 死亡搁浅"); err != nil {
		t.Fatal(err)
	}
	if a.detailID != "two" {
		t.Fatal("artwork still targets previous game", a.detailID)
	}
	if err := tt.Click("启动配置"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("查看 Hades · 黑帝斯"); err != nil {
		t.Fatal(err)
	}
	if a.shortcutEditors[101].draft.Options != want {
		t.Fatal("draft lost")
	}
	if a.shortcutEditors[102].draft.Options != "--example" {
		t.Fatal("draft leaked across games")
	}
	if a.snapshot.Shortcuts[0].Options != "-windowed" {
		t.Fatal("editing mutated saved shortcut")
	}
	if err := tt.Click("重置草稿"); err != nil {
		t.Fatal(err)
	}
	if a.shortcutEditorFor(a.snapshot.Shortcuts[0]).draft.Options != "-windowed" {
		t.Fatal("reset failed")
	}
}

func TestShortcutWorkspaceExternalAndDeleteSelection(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	tt := nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click("查看 External shortcut"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("外部快捷方式") || tt.HasText("保存启动设置") {
		t.Fatal("external shortcut became editable")
	}
	if err := tt.Click("勾选 Hades · 黑帝斯"); err != nil {
		t.Fatal(err)
	}
	if a.shortcutID != 103 {
		t.Fatal("delete checkbox changed focused detail")
	}
	if err := tt.Click("删除所选 (1)"); err != nil {
		t.Fatal(err)
	}
	if !a.deleteOpen || len(a.deleteIDs) != 1 || a.deleteIDs[0] != 101 {
		t.Fatal("wrong delete target", a.deleteIDs)
	}
}

func TestShortcutWorkspacePreviewArtifacts(t *testing.T) {
	dir := previewDir()
	if dir == "" {
		t.Skip("optional preview")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{1100, 1280} {
		for _, tab := range []int{0, 1} {
			a := shortcutWorkspaceApp(t)
			a.shortcutTab = tab
			a.selectShortcut(a.snapshot.Shortcuts[0])
			if tab == 1 {
				a.key = "mock"
				a.artGames = []artwork.Game{{ID: 1, Name: "Hades"}, {ID: 2, Name: "Hades II"}, {ID: 3, Name: "Hades alternative"}}
				a.artGameChoice = artGameLabel(a.artGames[0])
				a.artStatus = map[model.ArtworkKind]service.ArtworkStatus{}
				a.artCurrent = map[model.ArtworkKind]*nativeui.Bitmap{}
				im := image.NewRGBA(image.Rect(0, 0, 120, 180))
				for y := 0; y < 180; y++ {
					for x := 0; x < 120; x++ {
						im.SetRGBA(x, y, color.RGBA{30, 100, 150, 255})
					}
				}
				a.artCurrent[model.Portrait] = nativeui.NewBitmap(im)
				a.artStatus[model.Portrait] = service.ArtworkStatus{Paths: []string{"sample.png"}}
				state := a.artState()
				state.loaded = true
				for id := 1; id <= 8; id++ {
					state.artImages = append(state.artImages, artwork.Image{ID: id, URL: "mock"})
					state.thumbs[id] = nativeui.NewBitmap(im)
				}
				state.selectedID = 1
				state.selectedURL = "mock"
				state.previewLabel = "已选择：SteamGridDB #1"
			}
			height := 900
			if width == 1100 {
				height = 800
			}
			tt := nativeui.NewTester(a.view, width, height)
			name := "shortcuts-launch"
			if tab == 1 {
				name = "shortcuts-artwork"
			}
			if width == 1100 {
				name += "-compact"
			}
			f, err := os.Create(filepath.Join(dir, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
