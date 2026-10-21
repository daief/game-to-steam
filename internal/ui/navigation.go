package ui

import (
	"path/filepath"

	"game-to-steam/internal/system"
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) navigation(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Column(c).Width(200).HeightPercent(100).Padding(8).Gap(8).Background(railColor).Border(1, t.Border).BorderWidth(0, 1, 0, 0).Children(func() {
		nativeui.Row(c).WidthPercent(100).Padding(10, 8).Gap(10).AlignItems(nativeui.Center).Children(func() {
			nativeui.Box(c).Width(30).Height(30).Radius(4).Background(t.SurfacePressed).Border(1, t.Border).Center().Children(func() {
				nativeui.Icon(c, studioGameIcon).Width(20).Height(20).TextColor(t.Accent)
			})
			nativeui.Column(c).Gap(4).Children(func() {
				nativeui.Text(c, "Game to\nSteam").FontSize(20).Bold()
				nativeui.Text(c, "v"+Version+" · 桌面版").FontSize(11).TextColor(t.TextMuted)
			})
		})
		nativeui.Box(c).WidthPercent(100).Height(1).Background(t.Border)
		for i, page := range []string{"Epic 游戏", "Steam 快捷方式", "设置"} {
			if i == 0 || i == 2 {
				label := "游戏管理"
				if i == 2 {
					label = "系统与设置"
				}
				nativeui.Text(c, label).Padding(12, 8).FontSize(11).TextColor(t.TextMuted)
			}
			fill, color := railColor, t.TextMuted
			if a.page == page {
				fill, color = t.SurfacePressed, t.Text
			}
			nativeui.Row(c).WidthPercent(100).Height(40).Padding(10).Gap(10).Radius(6).Background(fill).AlignItems(nativeui.Center).Children(func() {
				icon := studioGameIcon
				if i == 1 {
					icon = shortcutNavIcon
				}
				if i == 2 {
					icon = settingsNavIcon
				}
				nativeui.Icon(c, icon).Width(18).Height(18).TextColor(color)
				nativeui.Button(c, page).Grow(1).Height(28).Padding(0).TextAlign(nativeui.Start).Justify(nativeui.Start).Background(fill).BorderWidth(0).TextColor(color).Disabled(a.busy || a.adoptOpen || a.deleteOpen || a.launchOpen || a.epicAuthOpen || a.epicForgetOpen).OnClick(func() { a.page = page; a.query = ""; a.detailID = "" })
			})
		}
		nativeui.Box(c).Grow(1)
		nativeui.Column(c).WidthPercent(100).Padding(10).Gap(8).Border(1, t.Border).Radius(4).Children(func() {
			nativeui.Text(c, "Steam · 当前账号").FontSize(11).TextColor(t.Accent)
			label := a.snapshot.Profile.Name
			if label == "" {
				label = "未检测到账号"
			}
			nativeui.Text(c, label).SingleLine().WidthPercent(100).Tooltip(label).Bold()
			nativeui.Text(c, "写入前请完全退出 Steam").FontSize(11).TextColor(t.TextMuted)
		})
		nativeui.Button(c, "打开 Steam").WidthPercent(100).Height(34).Disabled(a.busy || a.snapshot.Profile.Root == "").OnClick(func() {
			if err := system.Open(filepath.Join(a.snapshot.Profile.Root, "steam.exe")); err != nil {
				a.status = err.Error()
			}
		})
	})
}

var shortcutNavIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" d="M10 7H7a5 5 0 0 0 0 10h3m4-10h3a5 5 0 0 1 0 10h-3M7 12h10"/></svg>`))
var settingsNavIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" d="m9 3 6 0 1 3 3 1 2 5-2 5-3 1-1 3H9l-1-3-3-1-2-5 2-5 3-1z"/><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>`))
