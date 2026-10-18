package main

import (
	"strings"
	"testing"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/model"
	"github.com/egoist/mygo/ui"
)

func testApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.svc = &service.Service{Dir: t.TempDir()}
	a.snapshot = service.Snapshot{Games: []model.Game{{ID: "one", Name: "Alpha"}, {ID: "two", Name: "Beta"}}, Shortcuts: []model.Shortcut{{AppID: 1, Name: "External"}}, Managed: map[string]model.ManagedGame{}, Profile: model.SteamProfile{AccountID: "1", Name: "Test"}}
	return a
}
func TestSearchAndVisibleSelection(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1100, 780)
	if e := tt.Click("搜索游戏"); e != nil {
		t.Fatal(e)
	}
	tt.Type("Alpha")
	if e := tt.Click("全选可见"); e != nil {
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
	tt := ui.NewTester(a.view, 1100, 780)
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
	a.snapshot.Shortcuts = []model.Shortcut{{AppID: 5, Name: "Alpha", Exe: "other.exe"}}
	a.selectedGames["one"] = true
	tt := ui.NewTester(a.view, 1100, 780)
	if e := tt.Click("导入所选 (1)"); e != nil {
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

func TestLaunchEditorLoadsActualCommandAndKeepsCancelledChangesLocal(t *testing.T) {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{Game: model.Game{ID: "one", Name: "Alpha"}, AppID: 1}
	a.snapshot.Shortcuts[0] = model.Shortcut{AppID: 1, Name: "Custom name", Exe: `"C:\custom.exe"`, StartDir: `"C:\"`, Options: "--original"}
	a.openLaunch("one")
	tt := ui.NewTester(a.view, 1100, 780)
	if !tt.HasText("桥接需先在设置中登录 Epic，不启动官方客户端，也不提供云存档同步。") {
		t.Fatal("missing authentication/cloud-save guidance")
	}
	if a.launchDraft.Options != "--original" || a.launchDraft.Exe != `"C:\custom.exe"` {
		t.Fatal("editor did not load Steam fields")
	}
	if err := tt.Click("启动参数"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" --added")
	if !strings.Contains(a.launchDraft.Options, "--added") {
		t.Fatal(a.launchDraft.Options)
	}
	if err := tt.Click("强制开启 Epic 认证（当前快捷方式）"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("强制选项已改变，请先按策略生成命令，再调整参数。") {
		t.Fatal("missing generation guidance")
	}
	if err := tt.Click("取消编辑"); err != nil {
		t.Fatal(err)
	}
	if a.launchOpen || a.snapshot.Shortcuts[0].Options != "--original" {
		t.Fatal("cancel changed shortcut")
	}
}

func TestNetworkSettingsModeSelection(t *testing.T) {
	a := testApp(t)
	a.page = "设置"
	a.settings.Network.Mode = "custom"
	a.settings.Network.ProxyURL = "http://127.0.0.1:7890"
	tt := ui.NewTester(a.view, 1100, 780)
	if e := tt.Click("代理地址"); e != nil {
		t.Fatal(e)
	}
	if e := tt.Click("网络模式"); e != nil {
		t.Fatal(e)
	}
	if e := tt.Click("直连"); e != nil {
		t.Fatal(e)
	}
	if a.settings.Network.Mode != "direct" {
		t.Fatal(a.settings.Network)
	}
}

func TestEpicLoginDialogClearsCodeOnCancel(t *testing.T) {
	a := testApp(t)
	a.epicAuthOpen = true
	a.epicAuthCode = "sensitive-code"
	tt := ui.NewTester(a.view, 1100, 780)
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

func TestEpicAuthSettingsAndForgetConfirmation(t *testing.T) {
	a := testApp(t)
	a.page = "设置"
	a.snapshot.EpicAuth = epicauth.Status{State: "ready", DisplayName: "Test user"}
	tt := ui.NewTester(a.view, 1100, 1600)
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
