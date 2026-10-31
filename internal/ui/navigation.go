package ui

import (
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) navigation(c *nativeui.Context) {
	container := nativeui.Column(c.Key("navigation")).Width(160).Shrink(0).HeightPercent(100).Padding(8).Gap(8).Background(railColor)
	container.Children(func() {
		nativeui.Row(c).WidthPercent(100).Padding(10, 0).Gap(5).AlignItems(nativeui.End).Children(func() {
			nativeui.Text(c, "Game2Steam").FontSize(15).Bold()
			nativeui.Text(c, "v"+Version).FontSize(11).TextColor(c.Theme().TextMuted)
		})
		for _, page := range []string{"Epic 游戏", "Steam 快捷方式", "设置"} {
			fill, color := railColor, c.Theme().TextMuted
			if a.page == page {
				fill, color = c.Theme().SurfacePressed, c.Theme().Text
			}
			nativeui.Button(c.Key(page), page).WidthPercent(100).Height(36).TextAlign(nativeui.Start).Justify(nativeui.Start).Background(fill).BorderWidth(0).TextColor(color).Disabled(a.busy || a.importOpen || a.adoptOpen || a.deleteOpen || a.epicAuthOpen || a.epicForgetOpen).OnClick(func() {
				a.page = page
				if page == "Epic 游戏" {
					a.query = a.epicQuery
				} else {
					a.query = a.shortcutQuery
				}
			})
		}
		nativeui.Box(c).Grow(1)
		label := a.snapshot.Profile.Name
		if label == "" {
			label = "未检测到账号"
		}
		nativeui.Column(c).Label("Steam 账号与操作").WidthPercent(100).Padding(12).Gap(10).Border(1, c.Theme().Border).Radius(4).Children(func() {
			nativeui.Text(c, "Steam 当前账号").FontSize(11).TextColor(c.Theme().TextMuted)
			nativeui.Text(c, label).SingleLine().WidthPercent(100).FontSize(12).Bold()
			a.steamButtons(c)
		})
	})
}

var settingsNavIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" d="m9 3 6 0 1 3 3 1 2 5-2 5-3 1-1 3H9l-1-3-3-1-2-5 2-5 3-1z"/><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>`))
