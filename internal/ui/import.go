package ui

import (
	"fmt"

	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) beginImport() {
	if len(a.imports) == 0 {
		return
	}
	a.importIndex, a.importOpen = 0, true
}

func (a *app) importNeedsLogin() bool {
	if a.snapshot.EpicAuth.State == "ready" || a.snapshot.EpicAuth.State == "refresh" {
		return false
	}
	for _, request := range a.imports {
		game := request.Game
		if !a.importedGame(game.ID) {
			return true
		}
	}
	return false
}

func (a *app) importDialog(c *nativeui.Context) {
	nativeui.Modal(c.Key("import-dialog"), &a.importOpen, func() {
		nativeui.Text(c, fmt.Sprintf("导入 %d 款游戏", len(a.imports))).FontSize(20).Bold()
		nativeui.Scroll(c).Width(dialogWidth(c)).MaxHeight(140).Children(func() {
			nativeui.Column(c).WidthPercent(100).Gap(6).Children(func() {
				for _, request := range a.imports {
					nativeui.Text(c, request.Game.Name).WidthPercent(100)
				}
			})
		})
		nativeui.Text(c, "不支持云存档同步").FontSize(11).TextColor(c.Theme().TextMuted)
		needsLogin := a.importNeedsLogin()
		if needsLogin {
			nativeui.Text(c, "所选游戏需要登录 Epic").TextColor(c.Theme().Warning)
			nativeui.Button(c, "前往 Epic 登录").OnClick(func() { a.importOpen = false; a.page = "设置"; a.epicAuthOpen = true })
		}
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "取消本次导入").Disabled(a.busy).OnClick(func() { a.importOpen = false; a.imports = nil })
			nativeui.PrimaryButton(c, "确认导入").Disabled(a.busy || needsLogin).OnClick(func() {
				if a.busy || a.importNeedsLogin() {
					return
				}
				for i := range a.imports {
					a.imports[i].Game.ForceEpic = true
				}
				a.importOpen = false
				a.nextAdoption()
			})
		})
	})
}
