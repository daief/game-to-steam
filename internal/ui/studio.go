package ui

import (
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
	"strings"
)

// Recessed controls and library rail share the global midnight palette.
var studioList = nativeui.Hex("#11161D")

func (a *app) studioArtworkEditor(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Row(c).WidthPercent(100).Gap(6).AlignItems(nativeui.Center).Children(func() {
		for i, k := range model.ArtworkKinds {
			button := nativeui.Button
			if a.kind() == k {
				button = nativeui.PrimaryButton
			}
			button(c, k.Label()).Height(30).FontSize(12).Padding(4, 8).Disabled(a.busy).OnClick(func() { a.selectArtKind(i) })
		}
		query := a.artQuery
		nativeui.TextInput(c.Key("art-search"), &query).Label("封面搜索词").Placeholder("搜索游戏名称").Grow(1).MinWidth(120).Disabled(a.busy).OnChange(func() {
			if !a.busy {
				a.artQuery = query
			}
		})
		searchLabel := "搜索"
		if a.artLoading {
			searchLabel = ""
		}
		search := nativeui.Button(c, searchLabel).Label("搜索").Width(48).Height(30).Padding(4, 8).Disabled(a.busy || a.artLoading || a.key == "" || strings.TrimSpace(a.artQuery) == "").OnClick(a.searchArt)
		if a.artLoading {
			search.Children(func() { nativeui.Spinner(c).Label("封面搜索中").Size(16, 16) })
		}
		nativeui.Button(c, "本地图片").Padding(4, 8).Disabled(a.busy).OnClick(a.chooseLocal)
	})
	if a.key == "" {
		nativeui.Row(c).Gap(8).AlignItems(nativeui.Center).Children(func() {
			nativeui.Text(c, "在线搜图需要 API Key").FontSize(11).TextColor(t.TextMuted)
			nativeui.Button(c, "前往设置").Height(26).OnClick(func() { a.page = "设置" })
		})
	}
	if len(a.artGames) > 1 {
		nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Children(func() {
			nativeui.Text(c, "游戏来源").FontSize(11).TextColor(t.TextMuted)
			choices := make([]string, 0, len(a.artGames))
			for _, g := range a.artGames {
				choices = append(choices, artGameLabel(g))
			}
			choice := a.artGameChoice
			nativeui.Select(c.Key("art-source"), &choice, choices).Grow(1).MinWidth(0).Disabled(a.busy).OnChange(func() { a.selectArtSource(choice) })
		})
	}
	a.artworkCandidates(c)
	nativeui.Row(c).WidthPercent(100).Height(60).Shrink(0).Gap(10).AlignItems(nativeui.Center).Border(1, t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
		if bm := a.artCurrent[a.kind()]; bm != nil {
			nativeui.Image(c, bm).Width(42).Height(46).Fit(nativeui.Contain)
		}
		nativeui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			nativeui.Text(c, "当前设置："+a.artStatusLabel(a.kind())).FontSize(11).TextColor(t.TextMuted)
			label := artPurpose(a.kind())
			if a.artState().hasSelection() {
				label = "待保存"
			}
			nativeui.Text(c, label).SingleLine().WidthPercent(100).FontSize(11).TextColor(t.TextMuted)
		})
		nativeui.Button(c, "补齐缺图").Label("自动补齐缺失图片").Disabled(a.busy || a.key == "").OnClick(a.fillArt)
		nativeui.PrimaryButton(c, "保存").Disabled(a.busy || !a.artState().hasSelection()).OnClick(a.applyPreview)
	})
}
