package ui

import (
	"fmt"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
	"strings"
)

// Recessed controls and library rail share the global midnight palette.
var studioList = nativeui.Hex("#090F19")
var studioSelected = nativeui.Hex("#233140")

func studioPanel(c *nativeui.Context, title, subtitle string, body func()) {
	nativeui.Column(c).WidthPercent(100).Padding(16).Gap(10).Radius(6).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
		nativeui.Row(c).WidthPercent(100).Padding(0, 0, 0, 12).Gap(10).AlignItems(nativeui.Center).Border(1, c.Theme().Border).BorderWidth(0, 0, 1, 0).Children(func() {
			nativeui.Text(c, "◇").FontSize(18).TextColor(c.Theme().Accent)
			nativeui.Text(c, title).FontSize(15).Bold().Grow(1)
		})
		if subtitle != "" {
			nativeui.Text(c, subtitle).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		body()
	})
}

func (a *app) studioArtworkEditor(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Row(c).WidthPercent(100).Gap(4).Border(1, t.Border).BorderWidth(0, 0, 1, 0).Children(func() {
		for i, k := range model.ArtworkKinds {
			color, border := t.TextMuted, t.Border
			if a.kind() == k {
				color, border = t.Accent, t.Accent
			}
			nativeui.Button(c, k.Label()).Padding(6, 18).Height(34).Radius(0).Background(t.Background).TextColor(color).Border(2, border).BorderWidth(0, 0, 2, 0).Disabled(a.busy).OnClick(func() { a.selectArtKind(i) })
		}
	})
	nativeui.Text(c, "当前设置 · Steam 客户端实际生效图片").FontSize(12).TextColor(t.TextMuted)
	nativeui.Row(c).WidthPercent(100).Padding(12).Gap(16).Background(t.Surface).Border(1, t.Border).Radius(6).Children(func() {
		nativeui.Column(c).Width(86).Gap(4).Children(func() {
			if bm := a.artCurrent[a.kind()]; bm != nil {
				nativeui.Image(c, bm).Width(86).Height(84).Fit(nativeui.Contain)
			} else {
				nativeui.Box(c).Width(86).Height(84).Background(studioList).Border(1, t.Border).Radius(4).Center().Children(func() { nativeui.Text(c, "未设置").FontSize(11).TextColor(t.TextMuted) })
			}
			nativeui.Text(c, a.artStatusLabel(a.kind())).FontSize(11).TextColor(t.Accent)
		})
		nativeui.Column(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
			nativeui.Text(c, "本地缓存路径").FontSize(13).Bold()
			path := strings.Join(a.artStatus[a.kind()].Paths, "\n")
			if path == "" {
				path = "尚未设置此类封面"
			}
			nativeui.Text(c, path).WidthPercent(100).SingleLine().Tooltip(path).Font("Cascadia Code").FontSize(11).Padding(10).Background(studioList).Border(1, t.Border).Radius(4).TextColor(t.TextMuted)
			nativeui.Row(c).Gap(8).Wrap().Children(func() {
				nativeui.Button(c, "自动补齐").Label("自动补齐缺失图片").Disabled(a.busy || a.key == "").OnClick(a.fillArt)
				nativeui.Button(c, "从本地选择文件…").Label("本地图片").Disabled(a.busy).OnClick(a.chooseLocal)
			})
		})
	})
	nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Children(func() {
		nativeui.Text(c, "STEAMGRIDDB 在线图库").FontSize(12).Bold().TextColor(t.TextMuted)
		badge(c, fmt.Sprintf("%d 张候选", len(a.artState().artImages)), t.Success)

		nativeui.TextInput(c, &a.artQuery).Label("封面搜索词").Placeholder("搜索游戏名称…").Grow(1).MinWidth(80).Disabled(a.busy)
		nativeui.Button(c, "搜索").Disabled(a.busy || a.key == "" || strings.TrimSpace(a.artQuery) == "").OnClick(a.searchArt)
	})
	if len(a.artGames) > 1 {
		nativeui.ScrollHorizontal(c).Height(36).Shrink(0).WidthPercent(100).Label("游戏来源列表").Children(func() {
			nativeui.Row(c).Gap(6).AlignItems(nativeui.Center).Children(func() {
				nativeui.Text(c, "游戏来源").FontSize(11).TextColor(t.TextMuted)
				for _, g := range a.artGames {
					label := artGameLabel(g)
					button := nativeui.Button
					if label == a.artGameChoice {
						button = nativeui.PrimaryButton
					}
					button(c, label).Shrink(0).Disabled(a.busy).OnClick(func() { a.selectArtSource(label) })
				}
			})
		})
	}
	a.artworkCandidates(c, 80)
	nativeui.Column(c).WidthPercent(100).Gap(10).Children(func() {
		nativeui.Text(c, "写入封面需要更新 Steam 缓存目录，请先退出 Steam。").FontSize(11).Padding(10).WidthPercent(100).Background(nativeui.Hex("#30232A")).TextColor(t.Warning).Border(1, nativeui.Hex("#71414B")).Radius(4)
		nativeui.Row(c).WidthPercent(100).Gap(12).AlignItems(nativeui.Center).Children(func() {
			nativeui.Text(c, artPurpose(a.kind())).FontSize(11).TextColor(t.TextMuted).Grow(1)
			nativeui.PrimaryButton(c, "下载并写入 Steam 封面").Label("保存").Disabled(a.busy || !a.artState().hasSelection()).OnClick(a.applyPreview)
		})
	})
}

var studioGameIcon = nativeui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" d="M7 6h10c2 0 3 2 3.5 5l1 6c.3 2-2 3-3.2 1.5L15 15H9l-3.3 3.5C4.5 20 2.2 19 2.5 17l1-6C4 8 5 6 7 6zM8 9v5m-2.5-2.5h5M16 10h.01M18 13h.01"/></svg>`))
