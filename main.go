// game-to-steam is a native Go desktop application. UI state stays on MyGo's
// main thread. Slow work runs in one goroutine and returns through Window.Update.
package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	"game-to-steam/internal/network"
	"game-to-steam/internal/system"
)

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
	artCurrent                   map[model.ArtworkKind]*ui.Bitmap
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

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(180).HeightPercent(100).Padding(18).Gap(12).Background(t.Surface).Children(func() {
			ui.Text(c, "game-to-steam").FontSize(18).Bold()
			ui.Text(c, "游戏库工具 · 0.1.0").TextColor(t.TextMuted)
			for _, page := range []string{"Epic 游戏", "Steam 快捷方式", "设置"} {
				ui.Button(c, page).WidthPercent(100).Disabled(a.busy || a.adoptOpen || a.deleteOpen || a.launchOpen || a.epicAuthOpen || a.epicForgetOpen).OnClick(func() { a.page = page; a.query = ""; a.detailID = "" })
			}
			ui.Box(c).Grow(1)
			ui.Text(c, "目标 Steam 账号").TextColor(t.TextMuted)
			label := "未检测到"
			if a.snapshot.Profile.AccountID != "" {
				label = a.snapshot.Profile.Name + "\n" + a.snapshot.Profile.AccountID
			}
			ui.Text(c, label)
			ui.Text(c, "修改前请手动退出 Steam").FontSize(12).TextColor(t.TextMuted)
			ui.Button(c, "打开 Steam").Disabled(a.busy || a.snapshot.Profile.Root == "").OnClick(func() {
				if err := system.Open(filepath.Join(a.snapshot.Profile.Root, "steam.exe")); err != nil {
					a.status = err.Error()
				}
			})
		})
		ui.Column(c).Grow(1).HeightPercent(100).Padding(24).Gap(14).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				title := a.page
				if a.detailID != "" {
					title = "封面与启动方式"
				}
				ui.Text(c, title).FontSize(25).Bold().Grow(1)
				if a.busy {
					ui.Text(c, "处理中…").TextColor(t.TextMuted)
					ui.Button(c, "取消后续任务").OnClick(func() {
						if a.cancel != nil {
							a.cancel()
						}
					})
				}
				ui.Button(c, "刷新").Disabled(a.busy || a.svc == nil || a.detailID != "").OnClick(a.refresh)
			})
			ui.Text(c, a.status).TextColor(t.TextMuted)
			if a.svc == nil {
				ui.Text(c, "初始化失败，请根据上方提示处理本地数据后重新打开客户端")
				return
			}
			if a.detailID != "" {
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
				ui.Scroll(c).Height(120).WidthPercent(100).Children(func() {
					ui.Column(c).Gap(5).Children(func() {
						for _, warning := range a.snapshot.Warnings {
							ui.Text(c, "提示："+warning).FontSize(12)
						}
						for _, r := range a.results {
							prefix := "✓ "
							if r.Failed {
								prefix = "! "
							}
							ui.Text(c, prefix+r.Name+"："+r.Message).FontSize(12)
						}
					})
				})
			}
		})
	})
	a.dialogs(c)
	a.launchDialog(c)
}

func (a *app) libraryView(c *ui.Context) {
	isEpic := a.page == "Epic 游戏"
	if isEpic {
		ui.Text(c, a.snapshot.EpicAuth.Label()+"；需要认证的游戏请先在设置中登录。无云存档同步。").FontSize(12).TextColor(c.Theme().TextMuted)
	}
	ui.TextInput(c, &a.query).Label("搜索游戏").Placeholder("输入游戏名称筛选").Disabled(a.busy).WidthPercent(100)
	ui.Row(c).Gap(10).Children(func() {
		ui.Button(c, "全选可见").Disabled(a.busy).OnClick(func() {
			if isEpic {
				for _, g := range a.snapshot.Games {
					if a.matches(g.Name) {
						a.selectedGames[g.ID] = true
					}
				}
			} else {
				for _, s := range a.snapshot.Shortcuts {
					if a.matches(s.Name) {
						a.selectedShortcuts[s.AppID] = true
					}
				}
			}
		})
		ui.Button(c, "取消选择").Disabled(a.busy).OnClick(func() { a.selectedGames = map[string]bool{}; a.selectedShortcuts = map[uint32]bool{} })
		if isEpic {
			ui.PrimaryButton(c, fmt.Sprintf("导入所选 (%d)", a.gameSelectionCount())).Disabled(a.busy || a.gameSelectionCount() == 0 || a.snapshot.Profile.AccountID == "").OnClick(a.prepareImport)
		} else {
			ui.PrimaryButton(c, fmt.Sprintf("删除所选 (%d)", a.shortcutSelectionCount())).Disabled(a.busy || a.shortcutSelectionCount() == 0).OnClick(a.prepareDelete)
		}
	})
	ui.Scroll(c).Grow(1).WidthPercent(100).Children(func() {
		ui.Column(c).Gap(8).WidthPercent(100).Children(func() {
			visible := 0
			if isEpic {
				for _, g := range a.snapshot.Games {
					if !a.matches(g.Name) {
						continue
					}
					visible++
					a.gameRow(c, g)
				}
			} else {
				for _, s := range a.snapshot.Shortcuts {
					if !a.matches(s.Name) {
						continue
					}
					visible++
					a.shortcutRow(c, s)
				}
			}
			if visible == 0 {
				ui.Text(c, "没有可显示的游戏。可刷新列表或检查设置中的路径。").TextColor(c.Theme().TextMuted)
			}
		})
	})
}
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

func (a *app) gameRow(c *ui.Context, g model.Game) {
	t := c.Theme()
	managed, ok := a.snapshot.Managed[g.ID]
	present := false
	if ok {
		for _, sc := range a.snapshot.Shortcuts {
			if sc.AppID == managed.AppID {
				present = true
			}
		}
	}
	ui.Column(c.Key(g.ID)).Padding(12).Gap(6).WidthPercent(100).Background(t.Surface).Radius(6).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			selected := a.selectedGames[g.ID]
			ui.Checkbox(c.Key("select"), &selected, g.Name).Disabled(a.busy).Grow(1).OnChange(func() { a.selectedGames[g.ID] = selected })
			if present {
				ui.Text(c, "已管理").TextColor(t.TextMuted)
				ui.Button(c, "启动设置").Disabled(a.busy).OnClick(func() { a.openLaunch(g.ID) })
				ui.Button(c, "封面与启动").Disabled(a.busy).OnClick(func() { a.openDetail(g.ID) })
			}
		})
		ui.Text(c, g.InstallDir).FontSize(12).TextColor(t.TextMuted)
		force := a.force[g.ID]
		ui.Checkbox(c.Key("force"), &force, "强制开启 Epic 认证").Disabled(a.busy).OnChange(func() {
			if present {
				a.openLaunch(g.ID)
				a.launchDraft.ForceEpic = force
			} else {
				a.force[g.ID] = force
			}
		})
		via := g
		via.ForceEpic = force
		label := "自动策略：直接运行游戏 EXE"
		if via.ViaEpic() {
			label = "启动策略：独立认证后直接启动游戏（不启动 Epic 客户端）"
		}
		if present {
			label = "当前命令可在“启动设置”中查看和编辑"
		}
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	})
}

func (a *app) shortcutRow(c *ui.Context, s model.Shortcut) {
	t := c.Theme()
	ui.Column(c.Key(fmt.Sprint(s.AppID) + s.Name)).Padding(12).Gap(5).WidthPercent(100).Background(t.Surface).Radius(6).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			selected := a.selectedShortcuts[s.AppID]
			ui.Checkbox(c.Key("select"), &selected, s.Name).Disabled(a.busy).Grow(1).OnChange(func() { a.selectedShortcuts[s.AppID] = selected })
			for id, m := range a.snapshot.Managed {
				if m.AppID == s.AppID {
					ui.Button(c, "启动设置").Disabled(a.busy).OnClick(func() { a.openLaunch(id) })
					ui.Button(c, "封面与启动").Disabled(a.busy).OnClick(func() { a.openDetail(id) })
					break
				}
			}
		})
		ui.Text(c, fmt.Sprintf("AppID %d · %s", s.AppID, s.Exe)).FontSize(12).TextColor(t.TextMuted)
	})
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
func (a *app) dialogs(c *ui.Context) {
	ui.Modal(c.Key("delete-dialog"), &a.deleteOpen, func() {
		ui.Text(c, fmt.Sprintf("删除 %d 个 Steam 快捷方式？", len(a.deleteIDs))).FontSize(20).Bold()
		ui.Text(c, "删除后游戏本体仍然保留。")
		ui.Scroll(c).Width(620).MaxHeight(180).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				for _, sc := range a.snapshot.Shortcuts {
					for _, id := range a.deleteIDs {
						if sc.AppID == id {
							ui.Text(c, sc.Name)
						}
					}
				}
			})
		})
		ui.Checkbox(c, &a.deleteImages, "同时清理这些条目的 Steam 自定义图片").Disabled(a.busy)
		ui.Row(c).Gap(12).Children(func() {
			ui.Button(c, "取消删除").Disabled(a.busy).OnClick(func() { a.deleteOpen = false })
			ui.PrimaryButton(c, "确认删除").Disabled(a.busy).OnClick(func() {
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
	ui.Modal(c.Key("adopt-dialog"), &a.adoptOpen, func() {
		if a.importIndex >= len(a.imports) {
			return
		}
		ui.Text(c, "发现已有快捷方式："+a.imports[a.importIndex].Game.Name).FontSize(19).Bold()
		ui.Text(c, "同名可能是不同游戏，请核对路径。接管会保留原配置和图片。")
		choices := []string{}
		for _, s := range a.adoptCandidates {
			choices = append(choices, shortcutChoice(s))
		}
		ui.Select(c, &a.adoptChoice, choices).Width(660).Disabled(a.busy)
		ui.Row(c).Gap(12).Children(func() {
			ui.Button(c, "取消本次导入").Disabled(a.busy).OnClick(func() { a.adoptOpen = false; a.imports = nil })
			ui.Button(c, "跳过此游戏").Disabled(a.busy).OnClick(func() {
				if a.busy {
					return
				}
				a.imports[a.importIndex].Skip = true
				a.importIndex++
				a.nextAdoption()
			})
			ui.PrimaryButton(c, "确认接管所选条目").Disabled(a.busy).OnClick(func() {
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

func (a *app) settingsView(c *ui.Context) {
	ui.Scroll(c).Grow(1).WidthPercent(100).Children(func() {
		ui.Column(c).Gap(10).WidthPercent(100).Children(func() {
			ui.Text(c, "自动发现的路径可手动修改。只操作最近使用的 Steam 账号。").TextColor(c.Theme().TextMuted)
			ui.Text(c, "Steam 安装目录")
			ui.TextInput(c, &a.settings.SteamPath).Label("Steam 安装目录").Disabled(a.busy).WidthPercent(100)
			ui.Text(c, "Epic 安装清单目录")
			ui.TextInput(c, &a.settings.EpicManifestPath).Label("Epic 安装清单目录").Disabled(a.busy).WidthPercent(100)
			ui.Text(c, "网络")
			mode := networkLabel(a.settings.Network.Mode)
			ui.Select(c, &mode, []string{"直连", "系统代理", "指定代理地址"}).Label("网络模式").Disabled(a.busy).OnChange(func() {
				switch mode {
				case "直连":
					a.settings.Network.Mode = network.Direct
				case "指定代理地址":
					a.settings.Network.Mode = network.Custom
				default:
					a.settings.Network.Mode = network.System
				}
			})
			if a.settings.Network.Mode == network.Custom {
				ui.TextInput(c, &a.settings.Network.ProxyURL).Label("代理地址").Placeholder("http://127.0.0.1:7890").Disabled(a.busy).WidthPercent(100)
			}
			ui.Text(c, "保存后用于 Epic 认证和封面请求；系统代理读取 Windows 设置，包含自动配置。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, "SteamGridDB API Key（可选）")
			ui.TextInput(c, &a.key).Password().Label("SteamGridDB API Key").Disabled(a.busy).WidthPercent(100)
			ui.Link(c, "获取 SteamGridDB API Key", "https://www.steamgriddb.com/profile/preferences/api")
			ui.Text(c, "密钥保存到 Windows 凭据管理器。未填写时仍可导入及选择本地图片。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.PrimaryButton(c, "保存设置").Disabled(a.busy).OnClick(func() {
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
			a.epicAuthSettings(c)
			ui.Text(c, "数据与备份目录："+a.svc.Dir).FontSize(12)
			ui.Button(c, "打开数据与备份目录").OnClick(func() {
				if err := system.Open(a.svc.Dir); err != nil {
					a.status = err.Error()
				}
			})
		})
	})
}

func networkLabel(mode string) string {
	switch mode {
	case network.Direct:
		return "直连"
	case network.Custom:
		return "指定代理地址"
	}
	return "系统代理"
}

func main() {
	mygo.App.SetName("game-to-steam")
	mygo.App.SetVersion("0.1.0")
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
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "game-to-steam", Width: 1280, Height: 900, MinWidth: 1100, MinHeight: 800, StateKey: "main", Content: ui.View(a.view)})
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
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func ensureMainWindowSize(win *mygo.Window) {
	width, height := win.ContentSize()
	if width < 1100 || height < 800 {
		win.SetContentSize(max(width, 1100), max(height, 800))
	}
}
