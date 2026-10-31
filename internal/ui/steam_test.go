package ui

import (
	"errors"
	"testing"

	nativeui "github.com/egoist/mygo/ui"
)

func TestSteamCardReflectsRunningState(t *testing.T) {
	a := testApp(t)
	a.steamChecked = true
	tt := nativeui.NewTester(a.view, 980, 600)
	if !tt.HasText("启动 Steam") || tt.HasText("退出 Steam") {
		t.Fatal("stopped Steam has wrong controls")
	}
	if tt.HasText("Steam 运行时无法保存，请退出后再保存") {
		t.Fatal("warning shown while Steam stopped")
	}
	a.steamRunning = true
	tt.Frame()
	if !tt.HasText("退出 Steam") || !tt.HasText("打开 Steam") || tt.HasText("启动 Steam") {
		t.Fatal("running Steam has wrong controls")
	}
	if !tt.HasText("Steam 运行时无法保存，请退出后再保存") {
		t.Fatal("missing warning while Steam running")
	}
	card, _ := tt.Find("Steam 账号与操作")
	for _, label := range []string{"退出 Steam", "打开 Steam"} {
		button, ok := tt.Find(label)
		if !ok || button.X < card.X || button.Y < card.Y || button.X+button.W > card.X+card.W || button.Y+button.H > card.Y+card.H {
			t.Fatal("Steam action outside account card", label, button, card)
		}
		if button.H > 28 {
			t.Fatal("Steam action not compact", button)
		}
	}
	a.steamStateError = errors.New("process lookup failed")
	tt.Frame()
	if !tt.HasText("状态检测失败") {
		t.Fatal("unknown state silently treated as stopped")
	}
	a.steamStateError = nil
	a.steamRunning = false
	tt.Frame()
	if !tt.HasText("启动 Steam") || tt.HasText("退出 Steam") {
		t.Fatal("exited Steam did not restore launch action")
	}
}
