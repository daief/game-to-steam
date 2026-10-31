package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func TestResponsiveArtworkPreservesDraftAndScrolls(t *testing.T) {
	a := shortcutWorkspaceApp(t)
	a.selectShortcut(a.snapshot.Shortcuts[0])
	a.shortcutEditors[101].draft.Options = "--unsaved"
	a.shortcutTab = 1
	a.openShortcutArtwork(a.snapshot.Shortcuts[0])
	a.key = "mock"
	state := a.artState()
	state.loaded = true
	for id := 1; id <= 20; id++ {
		state.artImages = append(state.artImages, artwork.Image{ID: id, URL: "mock"})
		state.thumbs[id] = nativeui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 40, 60)))
	}
	tt := nativeui.NewTester(a.view, 1600, 900)
	if err := tt.Click("选择 #1"); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{980, 600}, {1100, 800}, {1600, 900}, {980, 600}} {
		tt.SetSize(size[0], size[1])
		assertVisible(t, tt, "保存", size)
		viewport := assertVisible(t, tt, "候选图片列表", size)
		first := assertVisible(t, tt, "已选择 #1", size)
		if viewport.H < 220 || first.Y+first.H > viewport.Y+viewport.H {
			t.Fatalf("gallery cannot show a full row at %v: %v, %v", size, viewport, first)
		}
		if a.artState() != state || state.selectedID != 1 || a.shortcutEditors[101].draft.Options != "--unsaved" {
			t.Fatal("resize discarded pending changes")
		}
	}
	viewport, _ := tt.Find("候选图片列表")
	before, _ := tt.Find("选择 #20")
	tt.Scroll(viewport.X+viewport.W/2, viewport.Y+viewport.H/2, 0, 500)
	after, _ := tt.Find("选择 #20")
	if after.Y >= before.Y {
		t.Fatal("gallery wheel did not scroll")
	}
	if !tt.HasText("查看 Hades · 黑帝斯") {
		t.Fatal("fixed list disappeared")
	}
	if err := tt.Click("查看 Hades · 黑帝斯"); err != nil {
		t.Fatal(err)
	}
	if a.artState() != state || state.selectedID != 1 {
		t.Fatal("returning to game lost artwork selection")
	}
}

func assertVisible(t *testing.T, tt *nativeui.Tester, label string, size [2]int) nativeui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
		t.Fatalf("%q outside %v: %v (found %v)", label, size, r, ok)
	}
	return r
}

func TestResponsivePagesAndPreviews(t *testing.T) {
	for _, size := range [][2]int{{980, 600}, {1100, 800}, {1600, 900}} {
		for _, page := range []string{"Epic 游戏", "Steam 快捷方式", "设置"} {
			t.Run(fmt.Sprintf("%s-%dx%d", page, size[0], size[1]), func(t *testing.T) {
				a := shortcutWorkspaceApp(t)
				a.page = page
				a.settings.Network.Mode = "custom"
				a.settings.Network.ProxyURL = "http://127.0.0.1:7890"
				a.settings.SteamPath = `C:\Program Files (x86)\Steam`
				a.settings.EpicManifestPath = `C:\ProgramData\Epic\EpicGamesLauncher\Data\Manifests`
				tt := nativeui.NewTester(a.view, size[0], size[1])
				if tt.HasText("Steam 运行时无法保存，请退出后再保存") {
					t.Fatal("Steam warning shown without running process")
				}
				switch page {
				case "Epic 游戏":
					assertVisible(t, tt, "Epic 游戏列表", size)
					assertVisible(t, tt, "查看 Alpha", size)
					if tt.HasText("强制开启 Epic 认证") || tt.HasText("启动设置") {
						t.Fatal("Epic duplicated editor controls")
					}
				case "Steam 快捷方式":
					assertVisible(t, tt, "快捷方式列表", size)
				case "设置":
					assertVisible(t, tt, "保存设置", size)
					r, _ := tt.Find("Steam 安装目录")
					tt.Scroll(r.X+40, r.Y+50, 0, 600)
					assertVisible(t, tt, "代理地址", size)
				}
				if dir := previewDir(); dir != "" {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%dx%d.png", page, size[0], size[1])))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					closeErr := f.Close()
					if err != nil {
						t.Fatal(err)
					}
					if closeErr != nil {
						t.Fatal(closeErr)
					}
				}
			})
		}
	}
}

func TestEpicViewOpensSteamEditor(t *testing.T) {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{AppID: 1}
	tt := nativeui.NewTester(a.view, 980, 600)
	if err := tt.Click("查看 Alpha"); err != nil {
		t.Fatal(err)
	}
	if a.page != "Steam 快捷方式" || !a.shortcutChosen || a.shortcutID != 1 {
		t.Fatal("imported game did not open Steam editor")
	}
	assertVisible(t, tt, "保存启动设置", [2]int{980, 600})
}

func TestImportConfirmationAppliesForceToSelection(t *testing.T) {
	a := testApp(t)
	a.snapshot.EpicAuth.State = "ready"
	a.selectedGames["one"], a.selectedGames["two"] = true, true
	a.prepareImport()
	tt := nativeui.NewTester(a.view, 980, 600)
	if !a.importOpen || a.adoptOpen {
		t.Fatal("import bypassed confirmation")
	}
	assertVisible(t, tt, "确认导入", [2]int{980, 600})
	if err := tt.Click("确认导入"); err != nil {
		t.Fatal(err)
	}
	if a.importOpen || len(a.imports) != 2 {
		t.Fatal("confirmation lost requests")
	}
	for _, request := range a.imports {
		if !request.Game.ForceEpic {
			t.Fatal("force was not applied to every selected game")
		}
	}
}

func TestResponsiveDialogs(t *testing.T) {
	for _, kind := range []string{"auth", "delete", "adopt", "import"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			label := ""
			switch kind {
			case "auth":
				a.epicAuthOpen = true
				label = "取消登录"
			case "delete":
				a.selectedShortcuts[1] = true
				a.prepareDelete()
				label = "确认删除"
			case "adopt":
				a.snapshot.Shortcuts[0].Name = "Alpha"
				a.importOne(a.snapshot.Games[0])
				a.importOpen = false
				a.nextAdoption()
				label = "确认接管所选条目"
			case "import":
				a.importOne(a.snapshot.Games[0])
				label = "确认导入"
			}
			tt := nativeui.NewTester(a.view, 980, 600)
			assertVisible(t, tt, label, [2]int{980, 600})
		})
	}
}
