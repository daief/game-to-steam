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
var Version = "0.2.1"

type app struct {
	epicAuthOpen, epicForgetOpen bool
	epicAuthCode                 string
	launchOpen                   bool
	launchID                     string
	launchDraft                  model.LaunchSettings
	launchOriginal               model.Shortcut
	launchGeneratedForce         bool
	win                          *mygo.Window
	svc                          *service.Service
	snapshot                     service.Snapshot
	page, query, status          string
	epicFilter                   string
	shortcutID                   uint32
	shortcutChosen               bool
	shortcutTab                  int
	shortcutEditors              map[uint32]*shortcutEditor
	busy                         bool
	cancel                       context.CancelFunc
	results                      []model.Result
	selectedGames                map[string]bool
	selectedShortcuts            map[uint32]bool
	force                        map[string]bool
	settings                     model.Settings
	key                          string
	deleteOpen, deleteImages     bool
	deleteIDs                    []uint32
	adoptOpen                    bool
	imports                      []service.ImportRequest
	importIndex                  int
	adoptChoice                  string
	adoptCandidates              []model.Shortcut
	detailID, artQuery           string
	artGames                     []artwork.Game
	artGameChoice                string
	artStates                    map[model.ArtworkKind]*artState
	artStatus                    map[model.ArtworkKind]service.ArtworkStatus
	artTab                       int
	artCurrent                   map[model.ArtworkKind]*nativeui.Bitmap
}

func newApp() *app {
	return &app{page: "Epic 游戏", status: "准备就绪", selectedGames: map[string]bool{}, selectedShortcuts: map[uint32]bool{}, force: map[string]bool{}, artStates: map[model.ArtworkKind]*artState{}}
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
				a.status = err.Error()
			} else {
				a.status = title + "：完成"
			}
		})
	}()
}

func (a *app) accept(snapshot service.Snapshot) {
	a.snapshot = snapshot
	a.settings = snapshot.Settings
	for _, g := range snapshot.Games {
		a.force[g.ID] = g.ForceEpic
	}
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
	t := c.Theme()
	nativeui.Row(c).Fill().Background(t.Background).Children(func() {
		a.navigation(c)
		workspace := nativeui.Column(c).Grow(1).HeightPercent(100).Padding(0).Gap(0)
		if a.page == "Steam 快捷方式" {
			workspace.Padding(0).Gap(0)
		}
		workspace.Children(func() {
			nativeui.Row(c).WidthPercent(100).Height(40).Padding(8, 24).Gap(12).AlignItems(nativeui.Center).Background(studioList).Border(1, t.Border).BorderWidth(0, 0, 1, 0).Children(func() {
				nativeui.Text(c, a.page).FontSize(12).Bold().TextColor(t.Accent)
				nativeui.Text(c, "本地游戏库管理").FontSize(11).TextColor(t.TextMuted).Grow(1)
				if a.busy {
					nativeui.Button(c, "取消后续任务").Height(24).OnClick(func() {
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
			})
			body := nativeui.Column(c).WidthPercent(100).Grow(1).MinHeight(0).Padding(16, 24).Gap(10)
			if a.page == "Steam 快捷方式" {
				body.Padding(0).Gap(0)
			}
			body.Children(func() {
				if a.page != "Steam 快捷方式" && (a.page != "Epic 游戏" || a.detailID != "") {
					nativeui.Row(c).Gap(12).AlignItems(nativeui.Center).Children(func() {
						title := a.page
						if a.page == "设置" {
							title = "全局设置"
						}
						if a.page == "Epic 游戏" {
							title = "Epic 游戏管理"
						}
						if a.detailID != "" && a.page != "Steam 快捷方式" {
							title = "封面与启动方式"
						}
						nativeui.Text(c, title).FontSize(20).Bold().Grow(1)
						if a.busy {
							nativeui.Text(c, "处理中…").TextColor(t.TextMuted)
							nativeui.Button(c, "取消后续任务").OnClick(func() {
								if a.cancel != nil {
									a.cancel()
								}
							})
						}
						if a.detailID != "" {
							nativeui.Button(c, "刷新").Disabled(a.busy || a.svc == nil || (a.detailID != "" && a.page != "Steam 快捷方式")).OnClick(a.refresh)
						}
					})
				}
				if a.svc == nil {
					nativeui.Text(c, "初始化失败，请根据上方提示处理本地数据后重新打开客户端")
					return
				}
				if a.page == "Steam 快捷方式" {
					a.shortcutWorkspace(c)
				} else if a.detailID != "" {
					a.detailView(c)
				} else if a.page == "设置" {
					a.settingsView(c)
				} else {
					a.libraryView(c)
				}
				if a.epicAuthOpen {
					a.epicAuthDialog(c)
				} else {
					a.epicAuthCode = ""
				}
				if a.epicForgetOpen {
					a.epicForgetDialog(c)
				}
				if a.detailID == "" && (len(a.results) > 0 || len(a.snapshot.Warnings) > 0) {
					nativeui.Scroll(c).Height(120).WidthPercent(100).Children(func() {
						nativeui.Column(c).Gap(5).Children(func() {
							for _, warning := range a.snapshot.Warnings {
								nativeui.Text(c, "提示："+warning).FontSize(12)
							}
							for _, r := range a.results {
								prefix := "✓ "
								if r.Failed {
									prefix = "! "
								}
								nativeui.Text(c, prefix+r.Name+"："+r.Message).FontSize(12)
							}
						})
					})
				}
				footer := nativeui.Row(c).WidthPercent(100).Padding(4, 0).Gap(12).Height(28)
				if a.page == "Steam 快捷方式" {
					footer.Padding(4, 12).Radius(0).Height(28)
				}
				footer.Children(func() {
					nativeui.Text(c, a.status).FontSize(12).TextColor(t.TextMuted).Grow(1)
					badge(c, fmt.Sprintf("Epic %d · Steam %d", len(a.snapshot.Games), len(a.snapshot.Shortcuts)), t.Accent)
				})
			})
		})
	})
	a.dialogs(c)
	a.launchDialog(c)
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
			g.ForceEpic = a.force[g.ID]
			a.imports = append(a.imports, service.ImportRequest{Game: g})
		}
	}
	a.importIndex = 0
	a.nextAdoption()
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
		nativeui.Text(c, "已管理的 Epic 游戏会保留本地累计时长，供重新导入时恢复。")
		nativeui.Text(c, "已管理的 Epic 游戏会保留本地累计时长，供重新导入时恢复。")
		nativeui.Scroll(c).Width(620).MaxHeight(180).Children(func() {
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
		nativeui.Select(c, &a.adoptChoice, choices).Width(660).Disabled(a.busy)
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
	nativeui.Scroll(c).Grow(1).WidthPercent(100).Children(func() {
		nativeui.Column(c).Gap(16).WidthPercent(100).Children(func() {
			section(c, "基础路径设置 (Core Paths)", "", func() {
				nativeui.Text(c, "Steam 安装目录")
				nativeui.TextInput(c, &a.settings.SteamPath).Label("Steam 安装目录").Background(studioList).Font("Cascadia Code").Height(36).Disabled(a.busy).WidthPercent(100)
				nativeui.Text(c, "Epic 安装清单目录")
				nativeui.TextInput(c, &a.settings.EpicManifestPath).Label("Epic 安装清单目录").Background(studioList).Font("Cascadia Code").Height(36).Disabled(a.busy).WidthPercent(100)
			})
			nativeui.Row(c).WidthPercent(100).Gap(16).AlignItems(nativeui.Start).Children(func() {
				nativeui.Column(c).Grow(1).MinWidth(0).Children(func() {
					section(c, "Epic 独立认证 (DPAPI)", "", func() { a.epicAuthSettings(c) })
				})
				nativeui.Column(c).Grow(1).MinWidth(0).Children(func() {
					section(c, "SteamGridDB 封面服务", "", func() {
						nativeui.Row(c).WidthPercent(100).AlignItems(nativeui.Center).Children(func() {
							nativeui.Text(c, "API Key（可选）").FontSize(12).Grow(1)
							nativeui.Link(c, "获取密钥 ↗", "https://www.steamgriddb.com/profile/preferences/api").FontSize(11)
						})
						nativeui.TextInput(c, &a.key).Password().Background(studioList).Height(36).Label("SteamGridDB API Key").Disabled(a.busy).WidthPercent(100)
						nativeui.Text(c, "密钥保存到 Windows 凭据管理器。未填写时仍可导入及选择本地图片。").FontSize(12).TextColor(c.Theme().TextMuted)
					})
				})
			})
			a.networkSettings(c)
			nativeui.Row(c).WidthPercent(100).Padding(12, 16).Gap(12).AlignItems(nativeui.Center).Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(6).Children(func() {
				nativeui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					nativeui.Text(c, "数据与备份").FontSize(12).Bold()
					nativeui.Text(c, a.svc.Dir).WidthPercent(100).SingleLine().Tooltip(a.svc.Dir).Font("Cascadia Code").FontSize(11).TextColor(c.Theme().TextMuted)
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
		nativeui.Text(c, "更改将在保存后应用").FontSize(12).TextColor(c.Theme().TextMuted).Grow(1)
		nativeui.PrimaryButton(c, "保存设置").Height(36).Disabled(a.busy).OnClick(func() {
			settings := a.settings
			key := strings.TrimSpace(a.key)
			a.run("保存设置", func(ctx context.Context) (func(), error) {
				err := a.svc.SaveSettings(settings)
				if err == nil {
					err = system.SaveKey(key)
				}
				snap := a.svc.Refresh()
				return func() { a.accept(snap); a.key = key }, err
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
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Game2Steam", Width: 1280, Height: 900, MinWidth: 1100, MinHeight: 800, StateKey: "main", Content: nativeui.View(a.view)})
		if err := a.win.SetIcon(assets.Icon); err != nil {
			a.status = "设置窗口图标失败: " + err.Error()
		}
		// MyGo restores saved outer bounds without clamping them to new minima.
		// Ensure old window state cannot collapse the artwork browser on startup.
		ensureMainWindowSize(a.win)

		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.busy {
				e.PreventDefault()
				a.status = "请等待当前操作完成后关闭窗口"
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
	if width < 1100 || height < 800 {
		win.SetContentSize(max(width, 1100), max(height, 800))
	}
}
