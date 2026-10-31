// Package ui implements the native desktop interface. UI state stays on MyGo's
// main thread. Slow work runs in one goroutine and returns through Window.Update.
package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	nativeui "github.com/egoist/mygo/ui"

	"game-to-steam/assets"
	service "game-to-steam/internal/app"
	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	"game-to-steam/internal/system"
)

// Version falls back for plain go builds; MyGo packages supply their version.
var Version = "0.2.3"

type app struct {
	epicAuthOpen, epicForgetOpen            bool
	epicAuthCode                            string
	win                                     *mygo.Window
	svc                                     *service.Service
	snapshot                                service.Snapshot
	page, query, status                     string
	epicFilter                              string
	epicQuery, shortcutQuery                string
	importOpen, resultsOpen                 bool
	shortcutID                              uint32
	shortcutChosen                          bool
	shortcutTab                             int
	shortcutEditors                         map[uint32]*shortcutEditor
	busy                                    bool
	cancel                                  context.CancelFunc
	results                                 []model.Result
	selectedGames                           map[string]bool
	selectedShortcuts                       map[uint32]bool
	settings                                model.Settings
	key                                     string
	deleteOpen, deleteImages                bool
	deleteIDs                               []uint32
	adoptOpen                               bool
	imports                                 []service.ImportRequest
	importIndex                             int
	adoptChoice                             string
	adoptCandidates                         []model.Shortcut
	detailID, artQuery                      string
	artGames                                []artwork.Game
	artGameChoice                           string
	artStates                               map[model.ArtworkKind]*artState
	artStatus                               map[model.ArtworkKind]service.ArtworkStatus
	artTab                                  int
	artCurrent                              map[model.ArtworkKind]*nativeui.Bitmap
	artLoading                              bool
	artError                                string
	artRequest                              uint64
	detailRequest                           uint64
	artCancel                               context.CancelFunc
	iconRequest                             uint64
	shortcutIcons                           map[uint32]*nativeui.Bitmap
	steamChecked, steamRunning, steamAction bool
	steamStateError                         error
}

func newApp() *app {
	return &app{page: "Epic 游戏", status: "准备就绪", selectedGames: map[string]bool{}, selectedShortcuts: map[uint32]bool{}, artStates: map[model.ArtworkKind]*artState{}}
}

// run serializes service access; workers return UI changes instead of writing
// fields read by the view. This is the Go goroutine/main-thread boundary.
func (a *app) run(title string, job func(context.Context) (func(), error)) {
	if a.busy || a.win == nil {
		return
	}
	a.busy = true
	a.status = title
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go func() {
		update, err := job(ctx)
		a.win.Update(func() {
			defer cancel()
			a.busy = false
			a.cancel = nil
			if update != nil {
				update()
			}
			if err != nil {
				a.status = title + "失败：" + err.Error()
			} else {
				a.status = title + "：完成"
			}
		})
	}()
}

func (a *app) accept(snapshot service.Snapshot) {
	a.snapshot = snapshot
	a.settings = snapshot.Settings
	a.loadShortcutIcons()
}
func (a *app) refresh() {
	a.run("刷新游戏与 Steam 快捷方式", func(ctx context.Context) (func(), error) {
		snap := a.svc.Refresh()
		return func() {
			a.accept(snap)
			a.selectedGames = map[string]bool{}
			a.selectedShortcuts = map[uint32]bool{}
			a.results = nil
		}, nil
	})
}

func (a *app) view(c *nativeui.Context) {
	c.SetTheme(midnightTheme)
	w, h := c.Size()
	if a.page == "Epic 游戏" {
		a.epicQuery = a.query
	} else if a.page == "Steam 快捷方式" {
		a.shortcutQuery = a.query
	}
	root := nativeui.Row(c).Fill().AlignItems(nativeui.Stretch).Background(c.Theme().Background)
	root.Children(func() {
		a.navigation(c)
		height := h
		nativeui.Column(c.Key("workspace")).Grow(1).Height(height).MinWidth(0).MinHeight(0).Children(func() {
			nativeui.Row(c).WidthPercent(100).Height(40).Shrink(0).Padding(6, 16).Gap(12).AlignItems(nativeui.Center).Background(studioList).Children(func() {
				nativeui.Text(c, a.page).Bold().Grow(1)
				if a.steamChecked && a.steamRunning && a.steamStateError == nil {
					nativeui.Text(c, "Steam 运行时无法保存，请退出后再保存").FontSize(11).TextColor(c.Theme().Warning)
				}
				if a.busy {
					nativeui.Button(c, "取消后续任务").Height(28).OnClick(func() {
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
			})
			body := nativeui.Column(c.Key("body")).WidthPercent(100).Grow(1).MinHeight(0).Gap(10).Padding(12, 16)
			if a.page == "Steam 快捷方式" {
				body.Padding(0).Gap(0)
			}
			body.Children(func() {
				if a.svc == nil {
					nativeui.Text(c, "初始化失败，请处理错误后重新打开")
					return
				}
				if a.page == "Steam 快捷方式" {
					a.shortcutWorkspace(c)
				} else if a.page == "设置" {
					a.settingsView(c)
				} else {
					a.libraryView(c)
				}
			})
			nativeui.Row(c).WidthPercent(100).Height(32).Shrink(0).Padding(4, 16).Gap(12).AlignItems(nativeui.Center).Background(railColor).Children(func() {
				color := c.Theme().TextMuted
				if strings.Contains(a.status, "失败") {
					color = c.Theme().Danger
				}
				nativeui.Text(c, a.status).SingleLine().FontSize(11).TextColor(color).Grow(1).MinWidth(0)
				if len(a.results)+len(a.snapshot.Warnings) > 0 {
					nativeui.Button(c, fmt.Sprintf("操作详情 (%d)", len(a.results)+len(a.snapshot.Warnings))).Height(24).FontSize(11).OnClick(func() { a.resultsOpen = true })
				}
			})
		})
	})
	if a.epicAuthOpen {
		a.epicAuthDialog(c)
	} else {
		a.epicAuthCode = ""
	}
	if a.epicForgetOpen {
		a.epicForgetDialog(c)
	}
	a.dialogs(c)
	a.importDialog(c)
	nativeui.Modal(c.Key("results"), &a.resultsOpen, func() {
		nativeui.Text(c, "操作详情").FontSize(18).Bold()
		nativeui.Scroll(c).Width(min(w-96, 640)).MaxHeight(h - 180).Children(func() {
			nativeui.Column(c).WidthPercent(100).Gap(8).Children(func() {
				for _, warning := range a.snapshot.Warnings {
					nativeui.Text(c, warning).WidthPercent(100).TextColor(c.Theme().Warning)
				}
				for _, r := range a.results {
					nativeui.Text(c, r.Name+"："+r.Message).WidthPercent(100)
				}
			})
		})
		nativeui.Button(c, "关闭").OnClick(func() { a.resultsOpen = false })
	})
}

func (a *app) libraryView(c *nativeui.Context) { a.epicLibraryView(c) }

func (a *app) matches(name string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(a.query)))
}
func (a *app) gameSelectionCount() int {
	n := 0
	for _, g := range a.snapshot.Games {
		if a.selectedGames[g.ID] {
			n++
		}
	}
	return n
}
func (a *app) shortcutSelectionCount() int {
	n := 0
	for _, s := range a.snapshot.Shortcuts {
		if a.selectedShortcuts[s.AppID] {
			n++
		}
	}
	return n
}

func (a *app) prepareImport() {
	if a.busy {
		return
	}
	a.imports = nil
	for _, g := range a.snapshot.Games {
		if a.selectedGames[g.ID] {
			a.imports = append(a.imports, service.ImportRequest{Game: g})
		}
	}
	a.beginImport()
}
func (a *app) nextAdoption() {
	if a.busy {
		return
	}
	for a.importIndex < len(a.imports) {
		req := a.imports[a.importIndex]
		if m, ok := a.snapshot.Managed[req.Game.ID]; ok {
			found := false
			for _, sc := range a.snapshot.Shortcuts {
				if sc.AppID == m.AppID {
					found = true
				}
			}
			if found {
				a.importIndex++
				continue
			}
		}
		candidates := service.Candidates(req.Game, a.snapshot.Shortcuts)
		if len(candidates) > 0 {
			a.adoptCandidates = candidates
			a.adoptChoice = shortcutChoice(candidates[0])
			a.adoptOpen = true
			return
		}
		a.importIndex++
	}
	a.adoptOpen = false
	requests := append([]service.ImportRequest{}, a.imports...)
	key := a.key
	a.run("导入游戏（需要 Steam 已退出）", func(ctx context.Context) (func(), error) {
		results, err := a.svc.Import(ctx, requests, key)
		snap := a.svc.Refresh()
		return func() { a.accept(snap); a.results = results; a.selectedGames = map[string]bool{} }, err
	})
}
func shortcutChoice(s model.Shortcut) string {
	return fmt.Sprintf("%s · AppID %d · %s", s.Name, s.AppID, s.Exe)
}

func (a *app) prepareDelete() {
	if a.busy {
		return
	}
	a.deleteIDs = nil
	for _, s := range a.snapshot.Shortcuts {
		if a.selectedShortcuts[s.AppID] {
			a.deleteIDs = append(a.deleteIDs, s.AppID)
		}
	}
	a.deleteImages = true
	a.deleteOpen = true
}
func (a *app) dialogs(c *nativeui.Context) {
	nativeui.Modal(c.Key("delete-dialog"), &a.deleteOpen, func() {
		nativeui.Text(c, fmt.Sprintf("删除 %d 个 Steam 快捷方式？", len(a.deleteIDs))).FontSize(20).Bold()
		nativeui.Text(c, "删除后游戏本体仍然保留。")
		nativeui.Text(c, "已管理游戏的累计时长将在重新导入时恢复。")
		nativeui.Scroll(c).Width(dialogWidth(c)).MaxHeight(160).Children(func() {
			nativeui.Column(c).Gap(4).Children(func() {
				for _, sc := range a.snapshot.Shortcuts {
					for _, id := range a.deleteIDs {
						if sc.AppID == id {
							nativeui.Text(c, sc.Name)
						}
					}
				}
			})
		})
		nativeui.Checkbox(c, &a.deleteImages, "同时清理这些条目的 Steam 自定义图片").Disabled(a.busy)
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "取消删除").Disabled(a.busy).OnClick(func() { a.deleteOpen = false })
			nativeui.Button(c, "确认删除").TextColor(c.Theme().Danger).Disabled(a.busy).OnClick(func() {
				if a.busy {
					return
				}
				ids := append([]uint32{}, a.deleteIDs...)
				cleanup := a.deleteImages
				a.deleteOpen = false
				a.run("删除快捷方式", func(ctx context.Context) (func(), error) {
					err := a.svc.Delete(ids, cleanup)
					snap := a.svc.Refresh()
					return func() { a.accept(snap); a.selectedShortcuts = map[uint32]bool{} }, err
				})
			})
		})
	})
	nativeui.Modal(c.Key("adopt-dialog"), &a.adoptOpen, func() {
		if a.importIndex >= len(a.imports) {
			return
		}
		nativeui.Text(c, "发现已有快捷方式："+a.imports[a.importIndex].Game.Name).FontSize(19).Bold()
		nativeui.Text(c, "同名可能是不同游戏，请核对路径。接管会保留原配置和图片。")
		choices := []string{}
		for _, s := range a.adoptCandidates {
			choices = append(choices, shortcutChoice(s))
		}
		nativeui.Select(c, &a.adoptChoice, choices).Width(dialogWidth(c)).Disabled(a.busy)
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "取消本次导入").Disabled(a.busy).OnClick(func() { a.adoptOpen = false; a.imports = nil })
			nativeui.Button(c, "跳过此游戏").Disabled(a.busy).OnClick(func() {
				if a.busy {
					return
				}
				a.imports[a.importIndex].Skip = true
				a.importIndex++
				a.nextAdoption()
			})
			nativeui.PrimaryButton(c, "确认接管所选条目").Disabled(a.busy).OnClick(func() {
				if a.busy {
					return
				}
				for _, sc := range a.adoptCandidates {
					if shortcutChoice(sc) == a.adoptChoice {
						a.imports[a.importIndex].AdoptID = sc.AppID
					}
				}
				a.importIndex++
				a.nextAdoption()
			})
		})
	})
}

func (a *app) settingsView(c *nativeui.Context) {
	w, _ := c.Size()
	columns := 1
	if w >= 1100 {
		columns = 2
	}
	nativeui.Scroll(c.Key("settings-scroll")).Grow(1).MinHeight(0).WidthPercent(100).Children(func() {
		nativeui.Column(c).Gap(16).WidthPercent(100).Children(func() {
			section(c, "路径", "", func() {
				a.settingsPath(c, "Steam 安装目录", &a.settings.SteamPath, system.SteamPath)
				a.settingsPath(c, "Epic 安装清单目录", &a.settings.EpicManifestPath, detectedManifestPath)
			})
			nativeui.Grid(c).WidthPercent(100).Columns(columns).Gap(16).Children(func() {
				nativeui.Column(c).Grow(1).MinWidth(0).Children(func() {
					section(c, "Epic 认证", "", func() { a.epicAuthSettings(c) })
				})
				nativeui.Column(c).Grow(1).MinWidth(0).Children(func() {
					section(c, "SteamGridDB 封面服务", "", func() {
						nativeui.Row(c).WidthPercent(100).AlignItems(nativeui.Center).Children(func() {
							nativeui.Text(c, "API Key（可选）").FontSize(12).Grow(1)
							nativeui.Link(c, "获取密钥 ↗", "https://www.steamgriddb.com/profile/preferences/api").FontSize(11)
						})
						nativeui.TextInput(c, &a.key).Password().Background(studioList).Height(36).Label("SteamGridDB API Key").Disabled(a.busy).WidthPercent(100)
						nativeui.Text(c, "在线搜图需要 API Key").FontSize(12).TextColor(c.Theme().TextMuted)
					})
				})
			})
			a.networkSettings(c)
			nativeui.Row(c).WidthPercent(100).Padding(12, 16).Gap(12).AlignItems(nativeui.Center).Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(6).Children(func() {
				nativeui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					nativeui.Text(c, "数据与备份").FontSize(12).Bold()
					nativeui.Text(c, a.svc.Dir).WidthPercent(100).SingleLine().Font("Cascadia Code").FontSize(11).TextColor(c.Theme().TextMuted)
				})
				nativeui.Button(c, "打开目录").Label("打开数据与备份目录").Height(30).Padding(4, 12).OnClick(func() {
					if err := system.Open(a.svc.Dir); err != nil {
						a.status = err.Error()
					}
				})
			})
		})
	})
	nativeui.Row(c).WidthPercent(100).Padding(12, 0).Gap(12).AlignItems(nativeui.Center).Border(1, c.Theme().Border).BorderWidth(1, 0, 0, 0).Children(func() {
		nativeui.Box(c).Grow(1)
		nativeui.PrimaryButton(c, "保存设置").Height(36).Disabled(a.busy).OnClick(func() {
			settings := a.settings
			key := strings.TrimSpace(a.key)
			a.run("保存设置", func(ctx context.Context) (func(), error) {
				err := a.svc.SaveSettings(settings)
				if err == nil {
					err = system.SaveKey(key)
				}
				snap := a.svc.Refresh()
				return func() {
					if err == nil {
						a.accept(snap)
						a.key = key
					}
				}, err
			})
		})
	})
}

// Run initializes the application and runs the native UI event loop.
// Call it from the main goroutine.
func Run() error {
	mygo.App.SetName("Game2Steam")
	if packagedVersion := mygo.App.Version(); packagedVersion != "" {
		Version = packagedVersion
	}
	mygo.App.SetVersion(Version)
	a := newApp()
	dir, err := service.DataDir()
	if err == nil {
		a.svc, err = service.New(dir)
	}
	if err != nil {
		a.status = err.Error()
	}
	if a.svc != nil {
		a.settings = a.svc.State.Settings
		a.key, err = system.ReadKey()
		if err != nil {
			a.status = err.Error()
		}
	}
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Game2Steam", Width: 980, Height: 600, MinWidth: 980, MinHeight: 600, StateKey: "main", Content: nativeui.View(a.view)})
		if err := a.win.SetIcon(assets.Icon); err != nil {
			a.status = "设置窗口图标失败: " + err.Error()
		}
		// MyGo restores saved outer bounds without clamping them to new minima.
		// Ensure old window state cannot collapse the artwork browser on startup.
		ensureMainWindowSize(a.win)

		steamContext, stopSteamWatch := context.WithCancel(context.Background())
		a.win.OnClosed(stopSteamWatch)
		go a.watchSteam(steamContext, a.win)

		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.busy {
				e.PreventDefault()
				a.status = "请等待当前操作完成后关闭窗口"
			} else {
				a.cancelArtwork()
			}
		})
		if a.svc != nil {
			a.refresh()
		}
	})
	return mygo.App.Run()
}

func ensureMainWindowSize(win *mygo.Window) {
	width, height := win.ContentSize()
	if width < 980 || height < 600 {
		win.SetContentSize(max(width, 980), max(height, 600))
	}
}
