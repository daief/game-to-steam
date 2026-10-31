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
	a.imports = []service.ImportRequest{{Game: g}}
	a.beginImport()
}

func (a *app) epicLibraryView(c *nativeui.Context) {
	nativeui.Column(c.Key("epic-content")).WidthPercent(100).Grow(1).MinHeight(0).Gap(12).Children(func() {
		t := c.Theme()
		imported := 0
		for _, g := range a.snapshot.Games {
			if a.importedGame(g.ID) {
				imported++
			}
		}
		visible := a.visibleEpicGames()
		nativeui.Row(c).WidthPercent(100).Gap(6).AlignItems(nativeui.Center).Children(func() {
			for i, filter := range []string{"全部", "未导入", "已导入"} {
				count := []int{len(a.snapshot.Games), len(a.snapshot.Games) - imported, imported}[i]
				button := nativeui.Button
				if a.epicFilter == filter || a.epicFilter == "" && i == 0 {
					button = nativeui.PrimaryButton
				}
				button(c, fmt.Sprintf("%s (%d)", filter, count)).Height(30).Disabled(a.busy).OnClick(func() { a.epicFilter = filter })
			}
			nativeui.Box(c).Grow(1)
			search := nativeui.TextInput(c.Key("epic-search"), &a.query).Label("搜索游戏").Placeholder("搜索名称或安装目录… Ctrl+F").Width(260).MinWidth(80).Disabled(a.busy)
			if !a.busy && !a.importOpen && !a.adoptOpen && !a.deleteOpen && !a.epicAuthOpen && !a.epicForgetOpen {
				if c.Shortcut(nativeui.Ctrl, nativeui.KeyF) {
					search.Focus()
				}
				if search.Shortcut(0, nativeui.KeyEscape) {
					a.query = ""
				}
			}
			nativeui.Button(c, "刷新").Disabled(a.busy).OnClick(a.refresh)
		})
		nativeui.Column(c).WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
			nativeui.Row(c).WidthPercent(100).Height(36).Padding(6, 12).Gap(12).AlignItems(nativeui.Center).Background(railColor).Children(func() {
				all := len(visible) > 0
				for _, g := range visible {
					all = all && a.selectedGames[g.ID]
				}
				nativeui.Checkbox(c.Key("select-all"), &all, "").Label("选择当前列表").Width(20).Disabled(a.busy || len(visible) == 0).OnChange(func() { a.selectVisibleEpic(all) })
				nativeui.Text(c, "游戏").Grow(1).FontSize(12).TextColor(t.TextMuted)
				nativeui.Text(c, "导入状态").Width(76).FontSize(12).TextColor(t.TextMuted)
				nativeui.Text(c, "操作").Width(72).FontSize(12).TextColor(t.TextMuted)
			})
			nativeui.Scroll(c.Key("epic-games")).Label("Epic 游戏列表").WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
				nativeui.Column(c).WidthPercent(100).Children(func() {
					for _, g := range visible {
						a.gameRow(c, g)
					}
					if len(visible) == 0 {
						nativeui.Text(c, "没有符合条件的游戏").Padding(24).TextColor(t.TextMuted)
					}
				})
			})
		})
		if a.gameSelectionCount() > 0 {
			a.epicBatchBar(c)
		}
	})
}

func (a *app) gameRow(c *nativeui.Context, g model.Game) {
	t := c.Theme()
	present := a.importedGame(g.ID)
	fill := t.Background
	if a.selectedGames[g.ID] {
		fill = t.SurfacePressed
	}
	nativeui.Row(c.Key(g.ID)).WidthPercent(100).MinHeight(68).Padding(10, 12).Gap(12).AlignItems(nativeui.Center).Background(fill).Border(1, t.Border).BorderWidth(0, 0, 1, 0).Children(func() {
		selected := a.selectedGames[g.ID]
		nativeui.Checkbox(c.Key("select"), &selected, "").Label("选择 " + g.Name).Width(20).Disabled(a.busy).OnChange(func() { a.selectedGames[g.ID] = selected })
		nativeui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			nativeui.Text(c, g.Name).Bold().SingleLine().WidthPercent(100)
			nativeui.Text(c, g.InstallDir).SingleLine().WidthPercent(100).FontSize(11).TextColor(t.TextMuted)
		})
		nativeui.Row(c).Width(76).AlignItems(nativeui.Center).Children(func() {
			if present {
				badge(c, "已导入", t.Success)
			} else {
				nativeui.Text(c, "未导入").FontSize(11).TextColor(t.TextMuted)
			}
		})
		nativeui.Row(c).Width(72).Children(func() {
			if present {
				nativeui.Button(c, "查看").Label("查看 " + g.Name).Disabled(a.busy).OnClick(func() { a.openLaunch(g.ID) })
			} else {
				nativeui.PrimaryButton(c, "导入").Label("导入 " + g.Name).Disabled(a.busy || a.snapshot.Profile.AccountID == "").OnClick(func() { a.importOne(g) })
			}
		})
	})
}

func (a *app) epicBatchBar(c *nativeui.Context) {
	nativeui.Row(c.Key("epic-batch-bar")).Label("批量导入操作栏").WidthPercent(100).Padding(8, 0).Gap(12).AlignItems(nativeui.Center).Children(func() {
		nativeui.Text(c, fmt.Sprintf("已选择 %d 款", a.gameSelectionCount())).Grow(1)
		nativeui.Button(c, "取消选择").Disabled(a.busy).OnClick(func() { a.selectedGames = map[string]bool{} })
		nativeui.PrimaryButton(c, fmt.Sprintf("导入所选 (%d)", a.gameSelectionCount())).Disabled(a.busy || a.snapshot.Profile.AccountID == "").OnClick(a.prepareImport)
	})
}
