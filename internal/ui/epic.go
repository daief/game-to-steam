package ui

import (
	"fmt"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

// An association alone is insufficient: a shortcut removed outside the app
// must be shown as available for import again.
func (a *app) importedGame(id string) bool {
	m, ok := a.snapshot.Managed[id]
	if ok {
		for _, sc := range a.snapshot.Shortcuts {
			if sc.AppID == m.AppID {
				return true
			}
		}
	}
	return false
}

func (a *app) visibleEpicGames() []model.Game {
	var games []model.Game
	for _, g := range a.snapshot.Games {
		imported := a.importedGame(g.ID)
		if a.epicFilter == "未导入" && imported || a.epicFilter == "已导入" && !imported {
			continue
		}
		if !a.matches(g.Name) && !a.matches(g.InstallDir) {
			continue
		}
		games = append(games, g)
	}
	return games
}

func (a *app) selectVisibleEpic(selected bool) {
	if a.busy {
		return
	}
	for _, g := range a.visibleEpicGames() {
		a.selectedGames[g.ID] = selected
	}
}

func (a *app) importOne(g model.Game) {
	if a.busy {
		return
	}
	g.ForceEpic = a.force[g.ID]
	a.imports = []service.ImportRequest{{Game: g}}
	a.importIndex = 0
	a.nextAdoption()
}

func (a *app) epicLibraryView(c *nativeui.Context) {
	t := c.Theme()
	imported := 0
	for _, g := range a.snapshot.Games {
		if a.importedGame(g.ID) {
			imported++
		}
	}
	visible := a.visibleEpicGames()
	nativeui.Row(c).WidthPercent(100).Gap(12).AlignItems(nativeui.Center).Children(func() {
		nativeui.Column(c).Grow(1).Gap(6).Children(func() {
			nativeui.Row(c).Gap(10).AlignItems(nativeui.Center).Children(func() {
				nativeui.Text(c, "Epic 游戏管理").FontSize(20).Bold()
				badge(c, fmt.Sprintf("已扫描 %d 款游戏 · %d 款尚未导入 Steam", len(a.snapshot.Games), len(a.snapshot.Games)-imported), t.Accent)
			})
			nativeui.Text(c, "自动检索本机已安装的 Epic 游戏，选择单项或批量导入 Steam。").FontSize(12).TextColor(t.TextMuted)
		})
		nativeui.Button(c, "重新扫描").Disabled(a.busy).OnClick(a.refresh)
		nativeui.Button(c, "全选可见").Disabled(a.busy || len(visible) == 0).OnClick(func() { a.selectVisibleEpic(true) })
	})
	nativeui.Row(c).WidthPercent(100).Gap(6).AlignItems(nativeui.Center).Children(func() {
		for i, filter := range []string{"全部", "未导入", "已导入"} {
			count := []int{len(a.snapshot.Games), len(a.snapshot.Games) - imported, imported}[i]
			button := nativeui.Button
			if a.epicFilter == filter || a.epicFilter == "" && i == 0 {
				button = nativeui.PrimaryButton
			}
			button(c, fmt.Sprintf("%s (%d)", filter, count)).Height(30).Radius(3).Disabled(a.busy).OnClick(func() { a.epicFilter = filter })
		}
		nativeui.Box(c).Grow(1)
		search := nativeui.TextInput(c.Key("epic-search"), &a.query).Label("搜索游戏").Placeholder("搜索名称或安装目录… Ctrl+F").Width(290).Disabled(a.busy)
		if !a.busy && !a.adoptOpen && !a.deleteOpen && !a.launchOpen && !a.epicAuthOpen && !a.epicForgetOpen {
			if c.Shortcut(nativeui.Ctrl, nativeui.KeyF) {
				search.Focus()
			}
			if search.Shortcut(0, nativeui.KeyEscape) {
				a.query = ""
			}
		}
	})
	// Shared widths keep the sticky heading and every row on the same tracks.
	nativeui.Column(c).WidthPercent(100).Grow(1).Border(1, t.Border).Radius(6).Children(func() {
		nativeui.Row(c).WidthPercent(100).Height(38).Padding(8, 12).Gap(12).AlignItems(nativeui.Center).Background(railColor).Children(func() {
			all := len(visible) > 0
			for _, g := range visible {
				all = all && a.selectedGames[g.ID]
			}
			nativeui.Checkbox(c.Key("select-all"), &all, "").Label("选择当前列表").Width(20).Disabled(a.busy || len(visible) == 0).OnChange(func() { a.selectVisibleEpic(all) })
			nativeui.Text(c, "游戏名称").Width(170).FontSize(12).TextColor(t.TextMuted)
			nativeui.Text(c, "安装目录").Grow(1).FontSize(12).TextColor(t.TextMuted)
			nativeui.Text(c, "启动方式").Width(152).FontSize(12).TextColor(t.TextMuted)
			nativeui.Text(c, "导入状态").Width(66).FontSize(12).TextColor(t.TextMuted)
			nativeui.Text(c, "操作").Width(132).FontSize(12).TextColor(t.TextMuted)
		})
		nativeui.Scroll(c).WidthPercent(100).Grow(1).Children(func() {
			nativeui.Column(c).WidthPercent(100).Children(func() {
				for _, g := range visible {
					a.gameRow(c, g)
				}
				if len(visible) == 0 {
					nativeui.Column(c).WidthPercent(100).Padding(32).Gap(8).Children(func() {
						nativeui.Text(c, "没有符合条件的游戏").Bold()
						nativeui.Text(c, "尝试切换筛选、调整搜索词，或重新扫描本机游戏。").TextColor(t.TextMuted)
					})
				}
				if a.gameSelectionCount() > 0 {
					nativeui.Box(c).Height(96).Shrink(0)
				}
			})
		})
		if a.gameSelectionCount() > 0 {
			a.epicBatchBar(c)
		}
	})
	nativeui.Text(c, fmt.Sprintf("显示 %d / %d 款", len(visible), len(a.snapshot.Games))).FontSize(12).TextColor(t.TextMuted)
	nativeui.Text(c, a.snapshot.EpicAuth.Label()+" · 导入前请退出 Steam；需要认证的游戏请先在设置中登录，无云存档同步。").FontSize(11).TextColor(t.TextMuted)
}

func (a *app) gameRow(c *nativeui.Context, g model.Game) {
	t := c.Theme()
	present := a.importedGame(g.ID)
	fill := railColor
	if a.selectedGames[g.ID] {
		fill = t.SurfaceHover
	}
	row := nativeui.Row(c.Key(g.ID)).WidthPercent(100).MinHeight(64).Padding(8, 12).Gap(12).AlignItems(nativeui.Center).Background(fill).Border(1, t.Border).BorderWidth(0, 0, 1, 0)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		selected := a.selectedGames[g.ID]
		nativeui.Checkbox(c.Key("select"), &selected, "").Label("选择 " + g.Name).Width(20).Disabled(a.busy).OnChange(func() { a.selectedGames[g.ID] = selected })
		nativeui.Row(c).Width(170).Gap(10).AlignItems(nativeui.Center).Children(func() {
			nativeui.Box(c).Width(28).Height(38).Background(studioList).Border(1, t.Border).Radius(3).Center().Children(func() { nativeui.Icon(c, studioGameIcon).Width(20).Height(20).TextColor(t.TextMuted) })
			nativeui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				nativeui.Text(c, g.Name).Bold().SingleLine().WidthPercent(100).Tooltip(g.Name)
				if present {
					nativeui.Text(c, fmt.Sprintf("AppID: %d", a.snapshot.Managed[g.ID].AppID)).Font("Cascadia Code").FontSize(11).TextColor(t.TextMuted)
				} else {
					nativeui.Text(c, "Epic 本机游戏").FontSize(11).TextColor(t.TextMuted)
				}
			})
		})
		nativeui.Text(c, g.InstallDir).Grow(1).MinWidth(0).SingleLine().Font("Cascadia Code").FontSize(11).TextColor(t.TextMuted).Tooltip(g.InstallDir)
		nativeui.Column(c).Width(152).Gap(6).Children(func() {
			via := g
			via.ForceEpic = a.force[g.ID]
			label := "EXE 直接启动"
			if via.ViaEpic() {
				label = "Epic 独立认证直启"
			}
			if present {
				label = "已保存的启动配置"
			}
			nativeui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
			force := a.force[g.ID]
			nativeui.Checkbox(c.Key("force"), &force, "强制开启 Epic 认证").FontSize(11).Disabled(a.busy).OnChange(func() {
				if present {
					a.openLaunch(g.ID)
					a.launchDraft.ForceEpic = force
				} else {
					a.force[g.ID] = force
				}
			})
		})
		nativeui.Box(c).Width(66).Children(func() {
			if present {
				badge(c, "已导入", t.Success)
			} else {
				badge(c, "未导入", t.Accent)
			}
		})
		nativeui.Row(c).Width(132).Gap(6).Children(func() {
			if present {
				nativeui.Button(c, "封面").Label("封面与启动").Disabled(a.busy).OnClick(func() { a.openDetail(g.ID) })
				nativeui.Button(c, "设置").Label("启动设置").Disabled(a.busy).OnClick(func() { a.openLaunch(g.ID) })
			} else {
				nativeui.PrimaryButton(c, "导入").Disabled(a.busy || a.snapshot.Profile.AccountID == "").OnClick(func() { a.importOne(g) })
			}
		})
	})
}

// The bar belongs to the viewport, outside its scrolling content.
func (a *app) epicBatchBar(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Row(c.Key("epic-batch-bar")).Label("批量导入悬浮栏").Absolute().Left(24).Right(24).Bottom(16).Height(64).Padding(12, 20).Gap(16).AlignItems(nativeui.Center).Radius(32).Background(t.SurfacePressed).Border(1, t.Accent).Shadow(0, 6, 20, 0, nativeui.RGBA(0, 0, 0, 0.5)).Children(func() {
		badge(c, fmt.Sprintf("%d", a.gameSelectionCount()), t.Accent)
		nativeui.Column(c).Grow(1).Gap(4).Children(func() {
			nativeui.Text(c, fmt.Sprintf("已选择 %d 款游戏", a.gameSelectionCount())).Bold()
			nativeui.Text(c, "导入前请完全退出 Steam").FontSize(11).TextColor(t.TextMuted)
		})
		nativeui.Button(c, "取消选择").Disabled(a.busy).OnClick(func() { a.selectedGames = map[string]bool{} })
		nativeui.PrimaryButton(c, fmt.Sprintf("导入所选 (%d)", a.gameSelectionCount())).Disabled(a.busy || a.snapshot.Profile.AccountID == "").OnClick(a.prepareImport)
	})
}
