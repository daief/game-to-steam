package ui

import (
	"context"
	"time"

	"game-to-steam/internal/system"
	"github.com/egoist/mygo"
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) watchSteam(ctx context.Context, win *mygo.Window) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		running, err := system.SteamRunning()
		win.Update(func() { a.steamChecked, a.steamRunning, a.steamStateError = true, running, err })
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *app) steamRoot() string {
	if a.snapshot.Profile.Root != "" {
		return a.snapshot.Profile.Root
	}
	return a.snapshot.Settings.SteamPath
}

func (a *app) steamButtons(c *nativeui.Context) {
	if a.steamStateError != nil {
		nativeui.Text(c, "状态检测失败").FontSize(11).TextColor(c.Theme().Warning)
	}
	disabled := a.busy || a.steamAction || !a.steamChecked || a.steamStateError != nil || a.steamRoot() == ""
	nativeui.Row(c).WidthPercent(100).Padding(4, 0, 0, 0).Gap(8).Children(func() {
		if a.steamRunning {
			nativeui.Button(c, "退出").Label("退出 Steam").Grow(1).MinWidth(0).Height(26).FontSize(11).Padding(4, 6).TextColor(c.Theme().Danger).Disabled(disabled).OnClick(func() { a.controlSteam(true) })
			nativeui.Button(c, "打开").Label("打开 Steam").Grow(1).MinWidth(0).Height(26).FontSize(11).Padding(4, 6).Disabled(disabled).OnClick(func() { a.controlSteam(false) })
		} else {
			nativeui.Button(c, "启动").Label("启动 Steam").Grow(1).Height(26).FontSize(11).Padding(4, 6).Disabled(disabled).OnClick(func() { a.controlSteam(false) })
		}
	})
}

func (a *app) controlSteam(quit bool) {
	if a.win == nil || a.busy || a.steamAction || !a.steamChecked || a.steamStateError != nil {
		return
	}
	a.steamAction = true
	root, win := a.steamRoot(), a.win
	go func() {
		err := system.ControlSteam(root, quit)
		win.Update(func() {
			a.steamAction = false
			if err != nil {
				a.status = err.Error()
			} else if quit {
				a.status = "已请求 Steam 退出"
			} else {
				a.status = "已请求打开 Steam"
			}
		})
	}()
}
