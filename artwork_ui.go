package main

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	"game-to-steam/internal/steam"
)

func bitmap(im image.Image, maxW, maxH int) *ui.Bitmap {
	if im == nil {
		return nil
	}
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW && h <= maxH {
		return ui.NewBitmap(im)
	}
	scale := min(float64(maxW)/float64(w), float64(maxH)/float64(h))
	target := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
	draw.CatmullRom.Scale(target, target.Bounds(), im, b, draw.Over, nil)
	return ui.NewBitmap(target)
}
func (a *app) kind() model.ArtworkKind {
	for _, k := range model.ArtworkKinds {
		if k.Label() == a.artKind {
			return k
		}
	}
	return model.Portrait
}
func (a *app) openDetail(id string) {
	m, ok := a.snapshot.Managed[id]
	if !ok {
		return
	}
	a.detailID = id
	a.artQuery = m.Game.Name
	a.artGames = nil
	a.artGameChoice = ""
	a.artImages = nil
	a.thumbs = map[int]*ui.Bitmap{}
	a.preview = nil
	a.previewData = nil
	a.previewLabel = ""
	a.force[id] = m.Game.ForceEpic
	a.artKind = model.Portrait.Label()
	a.loadCurrent()
}

func (a *app) loadCurrent() {
	m, ok := a.snapshot.Managed[a.detailID]
	if !ok {
		return
	}
	kind := a.kind()
	grid := a.snapshot.Profile.GridDir
	a.run("读取当前图片", func(ctx context.Context) (func(), error) {
		paths, err := steam.ArtFiles(grid, m.AppID, kind)
		if err != nil {
			return nil, err
		}
		var bm *ui.Bitmap
		label := "尚无图片"
		if len(paths) > 0 {
			_, im, e := artwork.Local(paths[0])
			if e != nil {
				return nil, e
			}
			bm = bitmap(im, 640, 360)
			label = "当前图片：" + filepath.Base(paths[0])
		}
		return func() { a.preview = bm; a.previewData = nil; a.previewLabel = label }, nil
	})
}

func (a *app) detailView(c *ui.Context) {
	m, ok := a.snapshot.Managed[a.detailID]
	if !ok {
		ui.Text(c, "游戏管理关联已不存在，请返回并刷新")
		ui.Button(c, "返回游戏列表").Disabled(a.busy).OnClick(func() { a.detailID = "" })
		return
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Button(c, "返回游戏列表").Disabled(a.busy).OnClick(func() { a.detailID = "" })
		ui.Text(c, m.Game.Name).FontSize(18).Bold().Grow(1)
		ui.Button(c, "启动设置").Disabled(a.busy).OnClick(func() { a.openLaunch(a.detailID) })
	})
	if m.Adopted {
		ui.Text(c, "此条目已接管，原配置保留；保存启动设置时才会更新命令。").FontSize(12)
	}
	ui.Row(c).Gap(12).Children(func() {
		kinds := []string{}
		for _, k := range model.ArtworkKinds {
			kinds = append(kinds, k.Label())
		}
		ui.Select(c, &a.artKind, kinds).Disabled(a.busy).OnChange(func() {
			a.artImages = nil
			a.thumbs = map[int]*ui.Bitmap{}
			a.previewData = nil
			a.preview = nil
			a.loadCurrent()
		})
		ui.Button(c, "选择本地图片").Disabled(a.busy).OnClick(a.chooseLocal)
		ui.Button(c, "自动补齐缺失图片").Disabled(a.busy || a.key == "").OnClick(func() {
			id, key := a.detailID, a.key
			a.run("补齐缺失封面", func(ctx context.Context) (func(), error) {
				results, err := a.svc.FillArtwork(ctx, id, key)
				return func() { a.results = results; a.previewLabel = "补图完成，可点击查看当前图片" }, err
			})
		})
		ui.Button(c, "查看当前图片").Disabled(a.busy).OnClick(a.loadCurrent)
	})
	ui.Row(c).Gap(10).Children(func() {
		ui.TextInput(c, &a.artQuery).Label("封面搜索词").Placeholder("SteamGridDB 游戏名称").Disabled(a.busy).Grow(1)
		ui.Button(c, "搜索图片来源").Disabled(a.busy || a.key == "" || a.artQuery == "").OnClick(a.searchArt)
	})
	if len(a.artGames) > 0 {
		ui.Row(c).Gap(10).Children(func() {
			options := []string{}
			for _, g := range a.artGames {
				options = append(options, artGameLabel(g))
			}
			ui.Select(c, &a.artGameChoice, options).Disabled(a.busy).Grow(1)
			ui.Button(c, "浏览候选图片").Disabled(a.busy).OnClick(a.browseArt)
		})
	}
	ui.Row(c).Gap(16).Grow(1).WidthPercent(100).Children(func() {
		ui.Scroll(c).Width(250).HeightPercent(100).Children(func() {
			ui.Column(c).Gap(8).WidthPercent(100).Children(func() {
				if len(a.artImages) == 0 {
					ui.Text(c, "搜索并选择图片来源，或使用本地图片。").TextColor(c.Theme().TextMuted)
				}
				for _, candidate := range a.artImages {
					ui.Column(c.Key(strconv.Itoa(candidate.ID))).Gap(5).Padding(8).Background(c.Theme().Surface).WidthPercent(100).Children(func() {
						if thumb := a.thumbs[candidate.ID]; thumb != nil {
							ui.Image(c, thumb).Width(200).Height(105).Fit(ui.Contain)
						}
						ui.Button(c, fmt.Sprintf("预览 #%d · %d×%d", candidate.ID, candidate.Width, candidate.Height)).Disabled(a.busy).OnClick(func() { a.previewCandidate(candidate) })
					})
				}
			})
		})
		ui.Column(c).Grow(1).Gap(10).Children(func() {
			ui.Text(c, a.previewLabel).FontSize(12)
			if a.preview != nil {
				ui.Image(c, a.preview).WidthPercent(100).Height(280).Fit(ui.Contain)
			}
			ui.PrimaryButton(c, "应用所选图片").Disabled(a.busy || len(a.previewData) == 0).OnClick(a.applyPreview)
			ui.Text(c, "应用会替换此类型的已有图片；修改前自动备份。需要 Steam 已退出。").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Scroll(c).MaxHeight(130).WidthPercent(100).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					for _, r := range a.results {
						ui.Text(c, r.Name+"："+r.Message).FontSize(12)
					}
				})
			})
		})
	})
}

func artGameLabel(g artwork.Game) string { return fmt.Sprintf("%s · SGDB %d", g.Name, g.ID) }
func (a *app) searchArt() {
	query, key := a.artQuery, a.key
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.run("搜索 SteamGridDB", func(ctx context.Context) (func(), error) {
		client, err := artwork.NewWithNetwork(key, cache, networkSettings)
		if err != nil {
			return nil, err
		}
		games, err := client.Search(ctx, query)
		return func() {
			a.artGames = games
			a.artImages = nil
			a.thumbs = map[int]*ui.Bitmap{}
			if len(games) > 0 {
				a.artGameChoice = artGameLabel(games[0])
			} else {
				a.artGameChoice = ""
				a.previewLabel = "没有搜索结果，请调整搜索词或选择本地图片"
			}
		}, err
	})
}
func (a *app) browseArt() {
	gameID := 0
	for _, g := range a.artGames {
		if artGameLabel(g) == a.artGameChoice {
			gameID = g.ID
		}
	}
	if gameID == 0 {
		return
	}
	kind, key := a.kind(), a.key
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.run("加载候选图片", func(ctx context.Context) (func(), error) {
		client, err := artwork.NewWithNetwork(key, cache, networkSettings)
		if err != nil {
			return nil, err
		}
		images, err := client.Images(ctx, gameID, kind)
		if err != nil {
			return nil, err
		}
		thumbs := map[int]*ui.Bitmap{}
		if len(images) > 20 {
			images = images[:20]
		}
		for i, candidate := range images {
			if i >= 6 {
				break
			}
			if candidate.Thumb == "" {
				continue
			}
			_, im, e := client.Download(ctx, candidate.Thumb)
			if e == nil {
				thumbs[candidate.ID] = bitmap(im, 200, 110)
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return func() {
			a.artImages = images
			a.thumbs = thumbs
			a.previewData = nil
			a.previewLabel = fmt.Sprintf("找到 %d 张候选图片，点击预览后应用", len(images))
		}, nil
	})
}
func (a *app) previewCandidate(candidate artwork.Image) {
	key := a.key
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.run("下载预览图片", func(ctx context.Context) (func(), error) {
		client, err := artwork.NewWithNetwork(key, cache, networkSettings)
		if err != nil {
			return nil, err
		}
		data, im, err := client.Download(ctx, candidate.URL)
		if err != nil {
			return nil, err
		}
		bm := bitmap(im, 640, 360)
		return func() {
			a.preview = bm
			a.previewData = data
			a.previewLabel = fmt.Sprintf("待应用：SteamGridDB #%d", candidate.ID)
		}, nil
	})
}
func (a *app) chooseLocal() {
	a.run("选择本地图片", func(ctx context.Context) (func(), error) {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择 PNG 或 JPEG 图片", Filters: []mygo.FileFilter{{Name: "静态图片", Extensions: []string{"png", "jpg", "jpeg"}}}})
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, nil
		}
		data, im, err := artwork.Local(paths[0])
		if err != nil {
			return nil, err
		}
		bm := bitmap(im, 640, 360)
		return func() {
			a.preview = bm
			a.previewData = data
			a.previewLabel = "待应用：" + filepath.Base(paths[0])
		}, nil
	})
}
func (a *app) applyPreview() {
	id, kind := a.detailID, a.kind()
	data := append([]byte{}, a.previewData...)
	a.run("应用图片", func(ctx context.Context) (func(), error) {
		err := a.svc.ApplyArtwork(id, kind, data, false)
		return func() {
			if err == nil {
				a.previewData = nil
				a.previewLabel = "图片已应用"
			}
		}, err
	})
}
