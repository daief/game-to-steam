package ui

import (
	"context"
	"fmt"
	"game-to-steam/internal/epic"
	"game-to-steam/internal/network"
	"game-to-steam/internal/system"
	"github.com/egoist/mygo"
	nativeui "github.com/egoist/mygo/ui"
	"os"
	"strings"
)

func (a *app) networkSettings(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Column(c).WidthPercent(100).Padding(16).Gap(14).Radius(6).Background(t.Surface).Border(1, t.Border).Children(func() {
		nativeui.Row(c).WidthPercent(100).Wrap().Gap(12).AlignItems(nativeui.Center).Children(func() {
			nativeui.Column(c).MinWidth(160).Gap(4).Children(func() {
				nativeui.Row(c).Gap(10).AlignItems(nativeui.Center).Children(func() {
					nativeui.Icon(c, settingsNavIcon).Width(18).Height(18).TextColor(t.Accent)
					nativeui.Text(c, "网络与代理").FontSize(15).Bold()
				})
			})
			nativeui.Row(c).Padding(3).Gap(2).Radius(4).Background(studioList).Border(1, t.Border).Children(func() {
				mode := network.Normalize(a.settings.Network).Mode
				for _, item := range []struct{ label, value string }{{"直连", network.Direct}, {"系统代理", network.System}, {"自定义代理", network.Custom}} {
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
		nativeui.Row(c).WidthPercent(100).Wrap().Gap(12).AlignItems(nativeui.Center).Children(func() {
			if a.settings.Network.Mode == network.Custom {
				nativeui.Column(c).Grow(1).MinWidth(200).MaxWidth(420).Gap(6).Children(func() {
					nativeui.Text(c, "代理地址").FontSize(12)
					address := strings.TrimPrefix(a.settings.Network.ProxyURL, "http://")
					nativeui.TextInput(c, &address).Label("代理地址输入").Placeholder("127.0.0.1:7890").OnChange(func() {
						if !a.busy {
							a.settings.Network.ProxyURL = address
						}
					}).WidthPercent(100).Height(34).Background(studioList).Font("Cascadia Code").Disabled(a.busy)
				})
			}
		})
	})
}

func detectedManifestPath() string {
	if path := system.EpicManifestPath(); path != "" {
		return path
	}
	return epic.DefaultManifestPath()
}

func directoryDraft(path string) (string, error) {
	path = system.NormalizePath(path)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("未找到有效目录，请手动选择")
	}
	return path, nil
}

func (a *app) settingsPath(c *nativeui.Context, label string, value *string, detect func() string) {
	nativeui.Text(c, label)
	nativeui.Row(c).WidthPercent(100).MaxWidth(880).Gap(8).AlignItems(nativeui.Center).Children(func() {
		nativeui.TextInput(c.Key(label), value).Label(label + "输入").Grow(1).MinWidth(0).MaxWidth(760).Height(36).Background(studioList).Disabled(a.busy)
		nativeui.Button(c, "扫描").Label("扫描" + label).Height(34).Disabled(a.busy).OnClick(func() {
			a.run("扫描"+label, func(ctx context.Context) (func(), error) {
				path, err := directoryDraft(detect())
				if err != nil {
					return nil, err
				}
				return func() { *value = path }, nil
			})
		})
		nativeui.Button(c, "选择").Label("选择" + label).Height(34).Disabled(a.busy).OnClick(func() {
			initial := system.NormalizePath(*value)
			a.run("选择"+label, func(ctx context.Context) (func(), error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择" + label, Directory: true, DefaultPath: initial})
				if err != nil || len(paths) == 0 {
					return nil, err
				}
				path, err := directoryDraft(paths[0])
				if err != nil {
					return nil, err
				}
				return func() { *value = path }, nil
			})
		})
	})
}
