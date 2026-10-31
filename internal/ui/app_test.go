package ui

import (
	"context"
	"strings"
	"testing"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func TestBusyRejectsOperations(t *testing.T) {
	a := testApp(t)
	a.selectedGames["one"] = true
	a.selectedShortcuts[1] = true
	a.busy = true
	a.prepareImport()
	a.prepareDelete()
	a.nextAdoption()
	a.run("must not run", func(context.Context) (func(), error) {
		t.Error("busy app started a job")
		return nil, nil
	})
	if a.imports != nil || a.deleteIDs != nil || a.adoptOpen || a.deleteOpen {
		t.Fatal("busy operation changed pending requests")
	}
}

func TestImportRequiresBridgeLogin(t *testing.T) {
	a := testApp(t)
	a.importOne(a.snapshot.Games[0])
	tt := nativeui.NewTester(a.view, 1100, 780)
	if !a.importNeedsLogin() {
		t.Fatal("bridge import did not require login")
	}
	if err := tt.Click("前往 Epic 登录"); err != nil {
		t.Fatal(err)
	}
	if a.importOpen || !a.epicAuthOpen || a.page != "设置" {
		t.Fatal("login action did not open settings")
	}
}

func TestEpicLoginBusyTransitionPreservesCode(t *testing.T) {
	a := testApp(t)
	a.epicAuthOpen = true
	a.epicAuthCode = "mock-code"
	tt := nativeui.NewTester(a.view, 1100, 780)
	// The previous frame enabled this button. The task starts before the next input.
	a.busy = true
	if err := tt.Click("确认 Epic 登录"); err != nil {
		t.Fatal(err)
	}
	if a.epicAuthCode != "mock-code" || !a.epicAuthOpen {
		t.Fatal("busy submit consumed the authorization code")
	}
}

func TestEpicLoginSubmitClearsCode(t *testing.T) {
	a := testApp(t)
	a.epicAuthOpen = true
	a.epicAuthCode = "mock-code"
	// No native window: run rejects the job, so this checks the UI draft only
	// and cannot send an authentication request.
	tt := nativeui.NewTester(a.view, 1100, 780)
	if err := tt.Click("确认 Epic 登录"); err != nil {
		t.Fatal(err)
	}
	if a.epicAuthCode != "" {
		t.Fatal("submitted authorization code retained in the UI")
	}
}

func testApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.svc = &service.Service{Dir: t.TempDir()}
	a.snapshot = service.Snapshot{Games: []model.Game{{ID: "one", Name: "Alpha"}, {ID: "two", Name: "Beta"}}, Shortcuts: []model.Shortcut{{AppID: 1, Name: "External"}}, Managed: map[string]model.ManagedGame{}, Profile: model.SteamProfile{AccountID: "1", Name: "Test"}}
	return a
}
func TestSearchAndVisibleSelection(t *testing.T) {
	a := testApp(t)
	tt := nativeui.NewTester(a.view, 1100, 780)
	if e := tt.Click("搜索游戏"); e != nil {
		t.Fatal(e)
	}
	tt.Type("Alpha")
	if e := tt.Click("选择当前列表"); e != nil {
		t.Fatal(e)
	}
	if !a.selectedGames["one"] || a.selectedGames["two"] {
		t.Fatal(a.selectedGames)
	}
	if e := tt.Click("取消选择"); e != nil {
		t.Fatal(e)
	}
	if a.gameSelectionCount() != 0 {
		t.Fatal("selection not cleared")
	}
}
func TestDeleteConfirmationDefaultsAndCancel(t *testing.T) {
	a := testApp(t)
	a.page = "Steam 快捷方式"
	a.selectedShortcuts[1] = true
	a.deleteImages = false
	tt := nativeui.NewTester(a.view, 1100, 780)
	if e := tt.Click("删除所选 (1)"); e != nil {
		t.Fatal(e)
	}
	if !a.deleteOpen || !a.deleteImages {
		t.Fatal("image cleanup should default to selected")
	}
	if e := tt.Click("同时清理这些条目的 Steam 自定义图片"); e != nil {
		t.Fatal(e)
	}
	if a.deleteImages {
		t.Fatal("cleanup could not be deselected")
	}
	if e := tt.Click("取消删除"); e != nil {
		t.Fatal(e)
	}
	if a.deleteOpen {
		t.Fatal("dialog still open")
	}
}
func TestAdoptionRequiresConfirmation(t *testing.T) {
	a := testApp(t)
	a.snapshot.EpicAuth.State = "ready"
	a.snapshot.Shortcuts = []model.Shortcut{{AppID: 5, Name: "Alpha", Exe: "other.exe"}}
	a.selectedGames["one"] = true
	tt := nativeui.NewTester(a.view, 1100, 780)
	if e := tt.Click("导入所选 (1)"); e != nil {
		t.Fatal(e)
	}
	if e := tt.Click("确认导入"); e != nil {
		t.Fatal(e)
	}
	if !a.adoptOpen || a.imports[0].AdoptID != 0 {
		t.Fatal("adoption was not deferred")
	}
	if e := tt.Click("取消本次导入"); e != nil {
		t.Fatal(e)
	}
	if a.adoptOpen || a.imports != nil {
		t.Fatal("cancel failed")
	}
}

func TestLaunchEditorLoadsActualCommandAndKeepsChangesLocal(t *testing.T) {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{Game: model.Game{ID: "one", Name: "Alpha"}, AppID: 1}
	a.snapshot.Shortcuts[0] = model.Shortcut{AppID: 1, Name: "Custom name", Exe: `"C:\custom.exe"`, Options: "--original"}
	a.openLaunch("one")
	tt := nativeui.NewTester(a.view, 1100, 780)
	e := a.shortcutEditors[1]
	if e.draft.Options != "--original" || e.draft.Exe != `"C:\custom.exe"` {
		t.Fatal("editor did not load Steam fields")
	}
	if err := tt.Click("启动参数"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" --added")
	if !strings.Contains(e.draft.Options, "--added") {
		t.Fatal("draft was not edited")
	}
	if tt.HasText("强制 Epic 认证") || tt.HasText("自动判断") {
		t.Fatal("obsolete launch choices remain")
	}
	if a.snapshot.Shortcuts[0].Options != "--original" {
		t.Fatal("draft changed saved shortcut")
	}
}

func TestNetworkSettingsModeSelection(t *testing.T) {
	a := testApp(t)
	a.page = "设置"
	a.settings.Network.Mode = "custom"
	a.settings.Network.ProxyURL = "http://127.0.0.1:7890"
	tt := nativeui.NewTester(a.view, 1100, 780)
	if e := tt.Click("代理地址"); e != nil {
		t.Fatal(e)
	}
	if e := tt.Click("直连"); e != nil {
		t.Fatal(e)
	}
	if a.settings.Network.Mode != "direct" {
		t.Fatal(a.settings.Network)
	}
	if e := tt.Click("自定义代理"); e != nil {
		t.Fatal(e)
	}
	if a.settings.Network.Mode != "custom" || a.settings.Network.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatal("mode switch lost proxy draft", a.settings.Network)
	}
	if e := tt.Click("系统代理"); e != nil {
		t.Fatal(e)
	}
	if a.settings.Network.Mode != "system" {
		t.Fatal(a.settings.Network)
	}
}

func TestEpicLoginDialogClearsCodeOnCancel(t *testing.T) {
	a := testApp(t)
	a.epicAuthOpen = true
	a.epicAuthCode = "sensitive-code"
	tt := nativeui.NewTester(a.view, 1100, 780)
	if !tt.HasText("登录 Epic 独立认证") {
		t.Fatal("missing login dialog")
	}
	if tt.HasText("sensitive-code") {
		t.Fatal("authorization code rendered as plain text")
	}
	if err := tt.Click("取消登录"); err != nil {
		t.Fatal(err)
	}
	if a.epicAuthOpen || a.epicAuthCode != "" {
		t.Fatal("auth draft not cleared")
	}
}

func TestEpicLoginDialogOnlyShowsRelevantErrors(t *testing.T) {
	a := testApp(t)
	a.epicAuthOpen = true
	a.status = "刷新游戏与 Steam 快捷方式：完成"
	tt := nativeui.NewTester(a.view, 980, 600)
	if tt.HasText("使用已保存的代理设置") {
		t.Fatal("redundant proxy explanation")
	}
	a.status = "Epic 独立认证失败：授权码已过期"
	tt.Frame()
	if !tt.HasText(a.status) {
		t.Fatal("login failure hidden")
	}
}

func TestEpicAuthSettingsAndForgetConfirmation(t *testing.T) {
	a := testApp(t)
	a.page = "设置"
	a.snapshot.EpicAuth = epicauth.Status{State: "ready", DisplayName: "Test user"}
	tt := nativeui.NewTester(a.view, 1100, 1600)
	if !tt.HasText("已登录：Test user（本地状态）") {
		t.Fatal("missing auth status")
	}
	if err := tt.Click("移除桥接登录"); err != nil {
		t.Fatal(err)
	}
	if !a.epicForgetOpen {
		t.Fatal("forget requires confirmation")
	}
	if err := tt.Click("保留登录"); err != nil {
		t.Fatal(err)
	}
	if a.epicForgetOpen {
		t.Fatal("forget cancel failed")
	}
	if tt.HasText("官方 Epic 启动器") || tt.HasText("Epic 启动模式") || tt.HasText("EpicGamesLauncher.exe 路径") {
		t.Fatal("removed launcher controls still visible")
	}
}
