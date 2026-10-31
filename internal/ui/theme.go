package ui

import nativeui "github.com/egoist/mygo/ui"

// Shared workbench palette for native pages and dialogs.
var midnightTheme = func() *nativeui.Theme {
	t := nativeui.DarkTheme()
	t.Background = nativeui.Hex("#171C24")
	t.Surface = nativeui.Hex("#222A35")
	t.SurfaceHover = nativeui.Hex("#232A37")
	t.SurfacePressed = nativeui.Hex("#203147")
	t.Border = nativeui.Hex("#394553")
	t.Text = nativeui.Hex("#E7EDF4")
	t.TextMuted = nativeui.Hex("#9BA6B2")
	t.Accent = nativeui.Hex("#78BCEA")
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

var railColor = nativeui.Hex("#1D242E")

func badge(c *nativeui.Context, label string, color nativeui.Color) {
	nativeui.Row(c).AlignSelf(nativeui.Center).AlignItems(nativeui.Center).Justify(nativeui.Center).Padding(4, 8).Radius(4).Background(railColor).Children(func() {
		nativeui.Text(c, label).FontSize(11).TextColor(color)
	})
}

func section(c *nativeui.Context, title, description string, content func()) {
	panel := nativeui.Column(c).Label(title+"分组").WidthPercent(100).Padding(16).Gap(8).Radius(6).Background(c.Theme().Surface).Border(1, c.Theme().Border)
	if title == "Epic 认证" || title == "SteamGridDB 封面服务" {
		panel.HeightPercent(100)
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

func dialogWidth(c *nativeui.Context) float32 { w, _ := c.Size(); return min(w-96, 640) }

var refreshIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" d="M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1"/></svg>`))

var helpIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M9.5 9a2.5 2.5 0 1 1 4 2c-1 .6-1.5 1-1.5 2v.5M12 17h.01" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>`))
