package ui

import (
	"context"
	"fmt"
	"strings"

	"game-to-steam/internal/bridge"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
	"golang.org/x/sys/windows"
)

// Each shortcut keeps its draft when the user switches games or detail tabs.
// original remains the optimistic-concurrency baseline until a successful save.
type shortcutEditor struct {
	id            string
	original      model.Shortcut
	draft         model.LaunchSettings
	advanced      bool
	gameOptions   string
	bridgeOptions []string
	optionsError  error
}

func (a *app) shortcutManagedID(appID uint32) string {
	for id, m := range a.snapshot.Managed {
		if m.AppID == appID {
			return id
		}
	}
	return ""
}

func (a *app) currentShortcut() (model.Shortcut, bool) {
	if a.shortcutChosen {
		for _, sc := range a.snapshot.Shortcuts {
			if sc.AppID == a.shortcutID {
				return sc, true
			}
		}
	}
	return model.Shortcut{}, false
}

func (a *app) shortcutEditorFor(sc model.Shortcut) *shortcutEditor {
	if a.shortcutEditors == nil {
		a.shortcutEditors = map[uint32]*shortcutEditor{}
	}
	if e := a.shortcutEditors[sc.AppID]; e != nil {
		return e
	}
	id := a.shortcutManagedID(sc.AppID)
	force := a.snapshot.Managed[id].Game.ForceEpic
	e := &shortcutEditor{id: id, original: sc, draft: model.LaunchSettings{Exe: sc.Exe, StartDir: sc.StartDir, Options: sc.Options, ForceEpic: force}}
	e.syncOptions()
	a.shortcutEditors[sc.AppID] = e
	return e
}

func (a *app) selectShortcut(sc model.Shortcut) {
	if a.busy {
		return
	}
	a.shortcutID, a.shortcutChosen = sc.AppID, true
	a.shortcutEditorFor(sc)
	if a.shortcutTab == 1 {
		a.openShortcutArtwork(sc)
	}
}

func (a *app) openShortcutArtwork(sc model.Shortcut) {
	id := a.shortcutManagedID(sc.AppID)
	if id != "" && a.detailID != id {
		a.openDetail(id)
	} else if id != "" && !a.artLoading && !a.artState().loaded && a.key != "" {
		a.searchArt()
	}
}

func shortcutListWidth(width float32) float32 {
	if width < 1280 {
		return 280
	}
	return 300
}

func (a *app) shortcutWorkspace(c *nativeui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	listWidth := shortcutListWidth(w)
	nativeui.Row(c).WidthPercent(100).Grow(1).MinHeight(0).Gap(0).Children(func() {
		list := nativeui.Column(c.Key("shortcut-list")).Width(listWidth).HeightPercent(100).MinHeight(0).Padding(12).Gap(8).Background(studioList).Border(1, t.Border).BorderWidth(0, 1, 0, 0)
		list.Children(func() {
			nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Children(func() {
				nativeui.Text(c, "快捷列表").FontSize(14).Bold().Grow(1)
				nativeui.Text(c, fmt.Sprintf("%d", len(a.snapshot.Shortcuts))).FontSize(12).TextColor(t.TextMuted)
				nativeui.Button(c, "").Label("刷新").Size(30, 30).Padding(6).Disabled(a.busy).OnClick(a.refresh).Children(func() { nativeui.Icon(c, refreshIcon).Size(16, 16) })
			})
			search := nativeui.TextInput(c.Key("shortcut-search"), &a.query).Label("搜索游戏").Placeholder("搜索快捷方式… Ctrl+F").WidthPercent(100).Disabled(a.busy)
			if !a.busy && !a.deleteOpen && !a.importOpen && !a.adoptOpen && !a.epicAuthOpen && !a.epicForgetOpen {
				if c.Shortcut(nativeui.Ctrl, nativeui.KeyF) {
					search.Focus()
				}
				if search.Shortcut(0, nativeui.KeyEscape) {
					a.query = ""
				}
			}
			nativeui.Row(c).Gap(6).Children(func() {
				nativeui.Button(c, "全选可见").FontSize(11).Disabled(a.busy).OnClick(func() {
					for _, sc := range a.snapshot.Shortcuts {
						if a.matches(sc.Name) {
							a.selectedShortcuts[sc.AppID] = true
						}
					}
				})
				nativeui.Button(c, "取消选择").FontSize(11).Disabled(a.busy || a.shortcutSelectionCount() == 0).OnClick(func() { a.selectedShortcuts = map[uint32]bool{} })
			})
			nativeui.Scroll(c.Key("shortcut-scroll")).Label("快捷方式列表").WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
				nativeui.Column(c).WidthPercent(100).Gap(8).Children(func() {
					n := 0
					for _, sc := range a.snapshot.Shortcuts {
						if !a.matches(sc.Name) {
							continue
						}
						n++
						a.shortcutListItem(c, sc)
					}
					if n == 0 {
						nativeui.Text(c, "没有符合条件的快捷方式").FontSize(12).TextColor(t.TextMuted)
					}
				})
			})
			nativeui.Row(c).WidthPercent(100).Padding(8, 0).Gap(8).AlignItems(nativeui.Center).Border(1, t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
				nativeui.Text(c, fmt.Sprintf("已勾选 %d 项", a.shortcutSelectionCount())).Grow(1).FontSize(11).TextColor(t.TextMuted)
				nativeui.Button(c, fmt.Sprintf("删除所选 (%d)", a.shortcutSelectionCount())).Height(28).FontSize(11).TextColor(t.Danger).Disabled(a.busy || a.shortcutSelectionCount() == 0).OnClick(a.prepareDelete)
			})
		})

		nativeui.Column(c.Key("shortcut-detail")).Grow(1).MinWidth(0).MinHeight(0).HeightPercent(100).Padding(12, 16).Gap(8).Children(func() {
			sc, ok := a.currentShortcut()
			if !ok {
				nativeui.Column(c).Fill().AlignItems(nativeui.Center).Justify(nativeui.Center).Gap(12).Children(func() {
					nativeui.Text(c, "选择一个游戏").FontSize(22).Bold()
				})
				return
			}
			nativeui.Row(c).WidthPercent(100).Height(36).Shrink(0).Gap(10).AlignItems(nativeui.Center).Children(func() {
				nativeui.Text(c, sc.Name).FontSize(20).Bold().SingleLine().Grow(1).MinWidth(0)
				for i, label := range []string{"启动配置", "封面配置"} {
					color, border := t.TextMuted, t.Border
					if a.shortcutTab == i {
						color, border = t.Accent, t.Accent
					}
					nativeui.Button(c, label).Height(36).Radius(0).Background(t.Background).TextColor(color).Border(2, border).BorderWidth(0, 0, 2, 0).Disabled(a.busy).OnClick(func() {
						if a.busy {
							return
						}
						a.shortcutTab = i
						if i == 1 {
							a.openShortcutArtwork(sc)
						}
					})
				}
			})
			if a.shortcutManagedID(sc.AppID) == "" {
				a.externalShortcutDetails(c, sc)
				return
			}
			if a.shortcutTab == 0 {
				a.shortcutLaunchView(c, sc)
			} else {
				if a.detailID != a.shortcutManagedID(sc.AppID) {
					nativeui.Button(c, "加载封面配置").Disabled(a.busy).OnClick(func() { a.openShortcutArtwork(sc) })
					return
				}
				a.studioArtworkEditor(c)
			}
		})
	})
}

func (a *app) shortcutListItem(c *nativeui.Context, sc model.Shortcut) {
	t := c.Theme()
	border, fill := t.Border, t.Surface
	if a.shortcutChosen && a.shortcutID == sc.AppID {
		border, fill = t.Accent, t.SurfacePressed
	}
	nativeui.Column(c.Key(fmt.Sprintf("shortcut-%d", sc.AppID))).Label("游戏卡片 "+sc.Name).WidthPercent(100).Padding(10).Gap(5).Background(fill).Border(2, border).BorderWidth(0, 0, 0, 2).Disabled(a.busy).OnClick(func() { a.selectShortcut(sc) }).Children(func() {
		nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Children(func() {
			selected := a.selectedShortcuts[sc.AppID]
			nativeui.Checkbox(c.Key("delete-select"), &selected, "").Label("勾选 " + sc.Name).Width(18).Disabled(a.busy).OnChange(func() { a.selectedShortcuts[sc.AppID] = selected })
			nativeui.Box(c).Width(36).Height(36).Center().Children(func() {
				if icon := a.shortcutIcons[sc.AppID]; icon != nil {
					nativeui.Image(c, icon).Size(32, 32).Fit(nativeui.Contain)
				} else {
					nativeui.Text(c, string([]rune(sc.Name + "?")[:1])).FontSize(18).TextColor(t.TextMuted)
				}
			})
			nativeui.Button(c, sc.Name).Padding(0).Height(20).Label("查看 " + sc.Name).Grow(1).MinWidth(0).SingleLine().Bold().TextAlign(nativeui.Start).Justify(nativeui.Start).Background(fill).BorderWidth(0).Disabled(a.busy).OnClick(func() { a.selectShortcut(sc) })
		})
		nativeui.Text(c, fmt.Sprintf("AppID: %d", sc.AppID)).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) externalShortcutDetails(c *nativeui.Context, sc model.Shortcut) {
	nativeui.Scroll(c.Key("launch-scroll")).Label("启动配置表单").WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
		nativeui.Column(c).WidthPercent(100).Gap(14).Children(func() {
			section(c, "外部快捷方式", "只读 · 导入时接管后可编辑", func() {
				for _, field := range []struct{ label, value string }{{"启动程序", sc.Exe}, {"工作目录", sc.StartDir}, {"启动参数", sc.Options}} {
					nativeui.Text(c, field.label).TextColor(c.Theme().TextMuted)
					nativeui.Text(c, field.value).WidthPercent(100).Font("Cascadia Code").FontSize(12)
				}
			})
		})
	})
}

func (e *shortcutEditor) syncOptions() {
	e.gameOptions, e.bridgeOptions, e.optionsError = e.draft.Options, nil, nil
	if bridge.ShortcutGameID(model.Shortcut{Exe: e.draft.Exe, Options: e.draft.Options}) == "" {
		return
	}
	args, err := windows.DecomposeCommandLine("bridge " + e.draft.Options)
	if err != nil {
		e.optionsError = err
		return
	}
	args = args[1:]
	for i, arg := range args {
		if arg == "--" {
			e.bridgeOptions = args[:i]
			e.gameOptions = windows.ComposeCommandLine(args[i+1:])
			return
		}
	}
	e.bridgeOptions = args
	e.gameOptions = ""
}

func (e *shortcutEditor) updateGameOptions() {
	e.optionsError = nil
	if e.bridgeOptions == nil {
		e.draft.Options = e.gameOptions
		return
	}
	args, err := windows.DecomposeCommandLine("game " + e.gameOptions)
	if err != nil {
		e.optionsError = err
		return
	}
	full := append([]string{}, e.bridgeOptions...)
	if len(args) > 1 {
		full = append(full, "--")
		full = append(full, args[1:]...)
	}
	e.draft.Options = windows.ComposeCommandLine(full)
}

func (a *app) shortcutLaunchView(c *nativeui.Context, sc model.Shortcut) {
	e := a.shortcutEditorFor(sc)
	nativeui.Scroll(c.Key("launch-scroll")).Label("启动配置表单").WidthPercent(100).Grow(1).MinHeight(0).Children(func() {
		nativeui.Column(c).WidthPercent(100).Padding(12, 0).Gap(16).Children(func() {
			nativeui.Column(c).WidthPercent(100).Gap(8).Children(func() {
				nativeui.Text(c, "游戏参数").FontSize(13).Bold()
				nativeui.TextArea(c.Key("game-options"), &e.gameOptions).Label("启动参数").Placeholder("例如 -windowed").WidthPercent(100).Height(84).Background(studioList).Disabled(a.busy).OnChange(e.updateGameOptions)
				if e.optionsError != nil {
					nativeui.Text(c, e.optionsError.Error()).WidthPercent(100).TextColor(c.Theme().Danger)
				}
			})
			label := "高级设置"
			if e.advanced {
				label = "收起高级设置"
			}
			nativeui.Button(c, label).Height(30).AlignSelf(nativeui.Start).Disabled(a.busy).OnClick(func() { e.advanced = !e.advanced })
			if e.advanced {
				nativeui.Column(c).WidthPercent(100).Gap(8).Padding(12, 0).Border(1, c.Theme().Border).BorderWidth(1, 0, 0, 0).Children(func() {
					nativeui.Row(c).WidthPercent(100).Gap(12).AlignItems(nativeui.Center).Children(func() {
						nativeui.Button(c, "重置").Disabled(a.busy).OnClick(func() {
							id := e.id
							a.run("生成启动命令草稿", func(ctx context.Context) (func(), error) {
								draft, err := a.svc.PreviewLaunch(id)
								return func() {
									if err == nil {
										e.draft = draft
										e.syncOptions()
									}
								}, err
							})
						})
					})
					for _, field := range []struct {
						label string
						value *string
					}{{"启动程序", &e.draft.Exe}, {"工作目录", &e.draft.StartDir}} {
						nativeui.Row(c).Gap(6).AlignItems(nativeui.Center).Children(func() {
							nativeui.Text(c, field.label).FontSize(12).TextColor(c.Theme().TextMuted)
							if field.label == "启动程序" {
								nativeui.Icon(c, helpIcon).Label("启动程序说明").Size(14, 14).TextColor(c.Theme().TextMuted).Tooltip("通过 bridge 程序可无需打开 Epic 启动器直接进入游戏")
							}
						})
						path := strings.Trim(*field.value, `"`)
						nativeui.TextInput(c.Key(field.label), &path).Label(field.label + "路径").WidthPercent(100).Background(studioList).Disabled(a.busy).OnChange(func() {
							if a.busy {
								return
							}
							*field.value = strings.Trim(path, `"`)
							if strings.TrimSpace(*field.value) == "" {
								*field.value = ""
							} else {
								*field.value = `"` + *field.value + `"`
							}
							e.syncOptions()
						})
					}
					nativeui.Text(c, "完整启动参数").FontSize(12).TextColor(c.Theme().TextMuted)
					nativeui.TextArea(c.Key("raw-options"), &e.draft.Options).Label("完整启动参数编辑").WidthPercent(100).Height(100).Background(studioList).Disabled(a.busy).OnChange(e.syncOptions)
				})
			}
		})
	})
	nativeui.Row(c).WidthPercent(100).Height(48).Shrink(0).Gap(10).AlignItems(nativeui.Center).Border(1, c.Theme().Border).BorderWidth(1, 0, 0, 0).Children(func() {
		nativeui.Box(c).Grow(1)
		if e.optionsError != nil || e.draft.Exe != e.original.Exe || e.draft.StartDir != e.original.StartDir || e.draft.Options != e.original.Options {
			nativeui.Text(c, "未保存").FontSize(11).TextColor(c.Theme().Warning)
		}
		nativeui.PrimaryButton(c, "保存启动设置").Disabled(a.busy || e.optionsError != nil || strings.TrimSpace(e.draft.Exe) == "").OnClick(func() {
			id, draft, original := e.id, e.draft, e.original
			a.run("保存启动设置", func(ctx context.Context) (func(), error) {
				if err := a.svc.SaveLaunch(id, draft, original); err != nil {
					return nil, err
				}
				snap := a.svc.Refresh()
				return func() { a.accept(snap); delete(a.shortcutEditors, sc.AppID) }, nil
			})
		})
	})
}
