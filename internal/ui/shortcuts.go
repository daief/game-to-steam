package ui

import (
	"context"
	"fmt"
	"strings"

	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

// Each shortcut keeps its draft when the user switches games or detail tabs.
// original remains the optimistic-concurrency baseline until a successful save.
type shortcutEditor struct {
	id             string
	original       model.Shortcut
	draft          model.LaunchSettings
	generatedForce bool
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
	e := &shortcutEditor{id: id, original: sc, draft: model.LaunchSettings{Exe: sc.Exe, StartDir: sc.StartDir, Options: sc.Options, ForceEpic: force}, generatedForce: force}
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
	}
}

func (a *app) shortcutWorkspace(c *nativeui.Context) {
	t := c.Theme()
	nativeui.Row(c).WidthPercent(100).Grow(1).Gap(0).Children(func() {
		nativeui.Column(c).Width(300).HeightPercent(100).Padding(8).Gap(12).Background(studioList).Border(1, t.Border).BorderWidth(0, 1, 0, 0).Children(func() {
			nativeui.Row(c).WidthPercent(100).AlignItems(nativeui.Center).Children(func() {
				nativeui.Text(c, "快捷列表").FontSize(14).Bold().Grow(1)
				badge(c, fmt.Sprintf("%d 款", len(a.snapshot.Shortcuts)), t.Accent)
				nativeui.Button(c, "↻").Label("刷新").Tooltip("重新扫描快捷方式").Disabled(a.busy).OnClick(a.refresh)
			})
			search := nativeui.TextInput(c.Key("shortcut-search"), &a.query).Label("搜索游戏").Placeholder("搜索快捷方式… Ctrl+F").WidthPercent(100).Disabled(a.busy)
			if !a.busy && !a.deleteOpen && !a.launchOpen {
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
			nativeui.Scroll(c).WidthPercent(100).Grow(1).Children(func() {
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
		nativeui.Column(c).Grow(1).MinWidth(0).HeightPercent(100).Padding(20).Gap(10).Children(func() {
			sc, ok := a.currentShortcut()
			if !ok {
				nativeui.Column(c).Fill().AlignItems(nativeui.Center).Justify(nativeui.Center).Gap(12).Children(func() {
					nativeui.Text(c, "选择一个游戏").FontSize(22).Bold()
					nativeui.Text(c, "从左侧快捷列表选择游戏，配置启动方式与封面。").FontSize(12).TextColor(t.TextMuted)
				})
				return
			}
			nativeui.Column(c).WidthPercent(100).Gap(6).Children(func() {
				nativeui.Text(c, sc.Name).FontSize(20).Bold().SingleLine().WidthPercent(100).Tooltip(sc.Name)
				nativeui.Row(c).Gap(8).AlignItems(nativeui.Center).Children(func() {
					badge(c, fmt.Sprintf("AppID: %d", sc.AppID), t.TextMuted)
					if a.shortcutManagedID(sc.AppID) != "" {
						badge(c, "Epic · 已管理", t.Success)
					} else {
						badge(c, "外部快捷方式", t.TextMuted)
					}
				})
			})
			nativeui.Row(c).WidthPercent(100).Gap(20).Border(1, t.Border).BorderWidth(0, 0, 1, 0).Children(func() {
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
				a.artworkEditor(c, true)
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
	nativeui.Column(c.Key(fmt.Sprintf("shortcut-%d", sc.AppID))).WidthPercent(100).Padding(10).Gap(5).Radius(6).Background(fill).Border(1, border).Children(func() {
		nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Children(func() {
			selected := a.selectedShortcuts[sc.AppID]
			nativeui.Checkbox(c.Key("delete-select"), &selected, "").Label("勾选 " + sc.Name).Width(18).Disabled(a.busy).OnChange(func() { a.selectedShortcuts[sc.AppID] = selected })
			nativeui.Box(c).Width(30).Height(30).Border(1, t.Border).Radius(3).Center().Children(func() { nativeui.Icon(c, studioGameIcon).Width(20).Height(20).TextColor(t.Accent) })
			nativeui.Button(c, sc.Name).Padding(0).Height(20).Label("查看 " + sc.Name).Grow(1).MinWidth(0).SingleLine().Bold().TextAlign(nativeui.Start).Justify(nativeui.Start).Background(fill).BorderWidth(0).Tooltip(sc.Name).Disabled(a.busy).OnClick(func() { a.selectShortcut(sc) })
		})
		nativeui.Text(c, fmt.Sprintf("AppID: %d", sc.AppID)).Font("Cascadia Code").FontSize(11).TextColor(t.TextMuted)
		nativeui.Text(c, shortcutExecutableName(sc.Exe)).SingleLine().WidthPercent(100).Padding(5, 6).Radius(3).Background(studioList).Font("Cascadia Code").FontSize(11).TextColor(t.TextMuted).Tooltip(sc.Exe).Disabled(a.busy).OnClick(func() { a.selectShortcut(sc) })
	})
}

func (a *app) externalShortcutDetails(c *nativeui.Context, sc model.Shortcut) {
	nativeui.Scroll(c).WidthPercent(100).Grow(1).Children(func() {
		nativeui.Column(c).WidthPercent(100).Gap(14).Children(func() {
			section(c, "外部快捷方式", "此条目未由本工具导入或接管，目前可查看和删除。匹配 Epic 游戏后，可在导入时确认接管。", func() {
				for _, field := range []struct{ label, value string }{{"启动程序", sc.Exe}, {"工作目录", sc.StartDir}, {"启动参数", sc.Options}} {
					nativeui.Text(c, field.label).TextColor(c.Theme().TextMuted)
					nativeui.Text(c, field.value).WidthPercent(100).Font("Cascadia Code").FontSize(12)
				}
			})
		})
	})
}

func (a *app) shortcutLaunchView(c *nativeui.Context, sc model.Shortcut) {
	e := a.shortcutEditorFor(sc)
	pending := e.generatedForce != e.draft.ForceEpic
	nativeui.Scroll(c).WidthPercent(100).Grow(1).Children(func() {
		nativeui.Column(c).WidthPercent(100).Gap(20).Children(func() {
			studioPanel(c, "基础启动策略", "编辑当前 Steam 快捷方式，保留 AppID、名称、图片与标签。", func() {
				nativeui.Text(c, "选择启动策略").FontSize(12).TextColor(c.Theme().TextMuted)
				nativeui.Row(c).WidthPercent(100).Gap(12).Children(func() {
					for _, force := range []bool{false, true} {
						border, fill := c.Theme().Border, c.Theme().Surface
						if e.draft.ForceEpic == force {
							border, fill = c.Theme().Accent, studioSelected
						}
						nativeui.Column(c.Key(fmt.Sprint(force))).Grow(1).MinWidth(0).Height(100).Padding(12).Gap(8).Background(fill).Border(1, border).Radius(6).Disabled(a.busy).OnClick(func() {
							if !a.busy {
								e.draft.ForceEpic = force
							}
						}).Children(func() {
							label, desc := "自动判断启动方式", "按游戏清单判断，支持的游戏使用 EXE 直启；需要认证时使用独立桥接。"
							if force {
								label, desc = "强制 Epic 独立认证", "通过独立认证桥接直接启动游戏，不启动官方 Epic 客户端。"
							}
							value := e.draft.ForceEpic
							nativeui.Radio(c, &value, force, label).FontSize(12).Disabled(a.busy).OnChange(func() {
								if !a.busy {
									e.draft.ForceEpic = value
								}
							})
							nativeui.Text(c, desc).WidthPercent(100).FontSize(12).TextColor(c.Theme().TextMuted)
						})
					}
				})
				nativeui.Row(c).WidthPercent(100).Gap(10).AlignItems(nativeui.Center).Children(func() {
					nativeui.Text(c, "生成会替换草稿，请先生成再调整参数。").FontSize(11).TextColor(c.Theme().TextMuted).Grow(1)
					nativeui.Button(c, "按策略生成命令").Disabled(a.busy).OnClick(func() {
						id, force := e.id, e.draft.ForceEpic
						a.run("生成启动命令草稿", func(ctx context.Context) (func(), error) {
							draft, err := a.svc.PreviewLaunch(id, force)
							return func() {
								if err == nil {
									e.draft = draft
									e.generatedForce = force
								}
							}, err
						})
					})
				})
				for _, field := range []struct {
					label string
					value *string
				}{{"启动程序", &e.draft.Exe}, {"工作目录", &e.draft.StartDir}} {
					nativeui.Text(c, field.label).FontSize(12).TextColor(c.Theme().TextMuted)
					nativeui.TextInput(c, field.value).Label(field.label).WidthPercent(100).Font("Cascadia Code").Background(studioList).Disabled(a.busy)
				}
			})
			studioPanel(c, "高级命令行与参数预览", "Launch Options · 完整启动参数", func() {
				nativeui.TextArea(c, &e.draft.Options).Label("启动参数").WidthPercent(100).Height(52).Background(studioList).Font("Cascadia Code").Disabled(a.busy)
				nativeui.Text(c, "认证桥接的游戏 flag 放在 -- 之后，例如 -- -windowed；裸 EXE 参数直接传给游戏。").FontSize(11).TextColor(c.Theme().TextMuted)
				nativeui.Text(c, "最终写入 Steam 的命令预览（只读）").FontSize(12).TextColor(c.Theme().TextMuted)
				nativeui.Box(c).WidthPercent(100).Padding(12).Background(studioList).Border(1, c.Theme().Border).Radius(4).Children(func() {
					nativeui.Text(c, strings.TrimSpace(e.draft.Exe+" "+e.draft.Options)).WidthPercent(100).Font("Cascadia Code").FontSize(12).TextColor(c.Theme().Accent)
				})
			})
			if pending {
				nativeui.Text(c, "强制选项已改变，请先按策略生成命令，再调整参数。").FontSize(12).TextColor(c.Theme().Warning)
			}
			nativeui.Text(c, "桥接需先在设置中登录 Epic，不启动官方客户端，也不提供云存档同步。").FontSize(11).TextColor(c.Theme().TextMuted)
		})
	})
	nativeui.Row(c).WidthPercent(100).Gap(10).AlignItems(nativeui.Center).Children(func() {
		nativeui.Text(c, "保存前请退出 Steam").FontSize(11).TextColor(c.Theme().Warning).Grow(1)
		nativeui.Button(c, "重置草稿").Disabled(a.busy).OnClick(func() { delete(a.shortcutEditors, sc.AppID) })
		nativeui.PrimaryButton(c, "保存启动设置").Disabled(a.busy || pending || strings.TrimSpace(e.draft.Exe) == "").OnClick(func() {
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

func shortcutExecutableName(exe string) string {
	parts := strings.Split(strings.ReplaceAll(strings.Trim(exe, "\""), "\\", "/"), "/")
	return parts[len(parts)-1]
}
