package ui

import (
	"game-to-steam/internal/network"
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) networkSettings(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Column(c).WidthPercent(100).Padding(16).Gap(14).Radius(6).Background(t.Surface).Border(1, t.Border).Children(func() {
		nativeui.Row(c).WidthPercent(100).Gap(12).AlignItems(nativeui.Start).Children(func() {
			nativeui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				nativeui.Row(c).Gap(10).AlignItems(nativeui.Center).Children(func() {
					nativeui.Icon(c, settingsNavIcon).Width(18).Height(18).TextColor(t.Accent)
					nativeui.Text(c, "网络与代理配置").FontSize(15).Bold()
				})
				nativeui.Text(c, "用于 Epic 认证与 SteamGridDB 图片请求").FontSize(11).TextColor(t.TextMuted)
			})
			nativeui.Row(c).Padding(3).Gap(2).Radius(4).Background(studioList).Border(1, t.Border).Children(func() {
				mode := network.Normalize(a.settings.Network).Mode
				for _, item := range []struct{ label, value string }{{"直连", network.Direct}, {"跟随系统代理", network.System}, {"指定自定义代理", network.Custom}} {
					fill, color := studioList, t.TextMuted
					if mode == item.value {
						fill, color = t.Accent, t.AccentText
					}
					nativeui.Button(c, item.label).Height(26).Padding(4, 10).FontSize(11).Radius(3).Background(fill).TextColor(color).BorderWidth(0).Disabled(a.busy).OnClick(func() {
						if !a.busy {
							a.settings.Network.Mode = item.value
						}
					})
				}
			})
		})
		nativeui.Row(c).WidthPercent(100).Gap(16).AlignItems(nativeui.Center).Children(func() {
			if a.settings.Network.Mode == network.Custom {
				nativeui.Column(c).Width(420).Gap(6).Children(func() {
					nativeui.Text(c, "代理地址（协议 / 主机 / 端口）").FontSize(12)
					nativeui.TextInput(c, &a.settings.Network.ProxyURL).Label("代理地址").Placeholder("http://127.0.0.1:7890").WidthPercent(100).Height(34).Background(studioList).Font("Cascadia Code").Disabled(a.busy)
				})
				nativeui.Text(c, "支持 HTTP、HTTPS、SOCKS5、SOCKS5H").Grow(1).MinWidth(0).FontSize(11).TextColor(t.TextMuted)
			} else {
				hint := "直接连接网络，不使用代理。"
				if network.Normalize(a.settings.Network).Mode == network.System {
					hint = "使用 Windows 系统代理，支持自动配置。"
				}
				nativeui.Text(c, hint).FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}
