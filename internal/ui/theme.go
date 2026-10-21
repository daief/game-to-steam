package ui

import nativeui "github.com/egoist/mygo/ui"

// Midnight Gaming Fluent tokens shared by every native page and dialog.
var midnightTheme = func() *nativeui.Theme {
	t := nativeui.DarkTheme()
	t.Background = nativeui.Hex("#0C131F")
	t.Surface = nativeui.Hex("#19202C")
	t.SurfaceHover = nativeui.Hex("#232A37")
	t.SurfacePressed = nativeui.Hex("#203147")
	t.Border = nativeui.Hex("#3E484E")
	t.Text = nativeui.Hex("#DCE2F4")
	t.TextMuted = nativeui.Hex("#9BA6B2")
	t.Accent = nativeui.Hex("#67C9F5")
	t.AccentHover = nativeui.Hex("#93DEFF")
	t.AccentPressed = nativeui.Hex("#45B6EB")
	t.AccentText = nativeui.Hex("#09101A")
	t.Success = nativeui.Hex("#57C7A2")
	t.Warning = nativeui.Hex("#F0BE68")
	t.Danger = nativeui.Hex("#EF797E")
	t.Selection = nativeui.Hex("#203147")
	t.Focus = nativeui.RGBA(103, 201, 245, 0.4)
	t.FontSize = 13
	t.Font = "Microsoft YaHei UI"
	return t
}()

var railColor = nativeui.Hex("#151C28")

func badge(c *nativeui.Context, label string, color nativeui.Color) {
	nativeui.Box(c).Padding(4, 8).Radius(4).Background(railColor).Children(func() {
		nativeui.Text(c, label).FontSize(11).TextColor(color)
	})
}

func section(c *nativeui.Context, title, description string, content func()) {
	panel := nativeui.Column(c).WidthPercent(100).Padding(16).Gap(8).Radius(6).Background(c.Theme().Surface).Border(1, c.Theme().Border)
	if title == "Epic 独立认证 (DPAPI)" || title == "SteamGridDB 封面服务" {
		panel.MinHeight(214)
	}
	panel.Children(func() {
		nativeui.Row(c).WidthPercent(100).Gap(10).AlignItems(nativeui.Center).Children(func() {
			nativeui.Icon(c, settingsNavIcon).Width(18).Height(18).TextColor(c.Theme().Accent)
			nativeui.Text(c, title).FontSize(15).Bold()
		})
		if description != "" {
			nativeui.Text(c, description).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		content()
	})
}
