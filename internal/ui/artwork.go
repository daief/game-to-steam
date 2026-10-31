package ui

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/egoist/mygo"
	nativeui "github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	service "game-to-steam/internal/app"
	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
)

func bitmap(im image.Image, maxW, maxH int) *nativeui.Bitmap {
	if im == nil {
		return nil
	}
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW && h <= maxH {
		return nativeui.NewBitmap(im)
	}
	scale := min(float64(maxW)/float64(w), float64(maxH)/float64(h))
	target := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
	draw.CatmullRom.Scale(target, target.Bounds(), im, b, draw.Over, nil)
	return nativeui.NewBitmap(target)
}

type artState struct {
	artImages    []artwork.Image
	thumbs       map[int]*nativeui.Bitmap
	preview      *nativeui.Bitmap
	previewData  []byte
	previewLabel string
	previewKind  model.ArtworkKind
	loaded       bool
	selectedID   int
	selectedURL  string
	thumbErrors  map[int]string
}

func (a *app) kind() model.ArtworkKind { return model.ArtworkKinds[a.artTab] }
func (a *app) artState() *artState {
	if a.artStates == nil {
		a.artStates = map[model.ArtworkKind]*artState{}
	}
	k := a.kind()
	if a.artStates[k] == nil {
		a.artStates[k] = &artState{thumbs: map[int]*nativeui.Bitmap{}}
	}
	return a.artStates[k]
}
func (a *app) clearArtChoices() { a.artStates = map[model.ArtworkKind]*artState{} }
func artPurpose(k model.ArtworkKind) string {
	switch k {
	case model.Portrait:
		return "用于游戏库竖向卡片"
	case model.Landscape:
		return "用于游戏库横向卡片"
	case model.Hero:
		return "用于详情页顶部 hero 背景"
	default:
		return ""
	}
}
func (a *app) artStatusLabel(k model.ArtworkKind) string {
	status, ok := a.artStatus[k]
	if !ok {
		return "读取中"
	}
	if status.Err != nil {
		return "读取失败"
	}
	if len(status.Paths) > 0 {
		return "已设置"
	}
	return "未设置"
}
func (a *app) openDetail(id string) {
	if a.busy {
		return
	}
	m, ok := a.snapshot.Managed[id]
	if !ok {
		return
	}
	a.cancelArtwork()
	a.detailRequest++
	a.detailID = id
	a.artQuery = artwork.SearchQuery(m.Game)
	a.artGames = nil
	a.artGameChoice = ""
	a.clearArtChoices()
	a.artStatus = nil
	a.artCurrent = nil
	a.results = nil
	a.artTab = 0
	a.loadCurrent()
}

// Read installed images separately from the candidate selections.
func readCurrentArtwork(svc *service.Service, id string) (map[model.ArtworkKind]service.ArtworkStatus, map[model.ArtworkKind]*nativeui.Bitmap) {
	statuses := svc.ArtworkStatus(id)
	current := map[model.ArtworkKind]*nativeui.Bitmap{}
	for _, kind := range model.ArtworkKinds {
		status := statuses[kind]
		if status.Err != nil || len(status.Paths) == 0 {
			continue
		}
		_, im, err := artwork.Local(status.Paths[0])
		if err != nil {
			status.Err = err
			statuses[kind] = status
			continue
		}
		current[kind] = bitmap(im, 240, 140)
	}
	return statuses, current
}
func (a *app) loadCurrent() {
	if a.win == nil || a.busy {
		return
	}
	id, win, svc, request := a.detailID, a.win, *a.svc, a.detailRequest
	go func() {
		statuses, current := readCurrentArtwork(&svc, id)
		win.Update(func() {
			if a.detailID != id || a.detailRequest != request {
				return
			}
			a.artStatus, a.artCurrent = statuses, current
			if a.page == "Steam 快捷方式" && a.shortcutTab == 1 && a.key != "" && !a.artLoading && !a.artState().loaded && a.artGameChoice == "" && a.artError == "" {
				a.searchArt()
			}
		})
	}()
}

func (a *app) cancelArtwork() {
	if a.artCancel != nil {
		a.artCancel()
		a.artCancel = nil
	}
	a.artRequest++
	a.artLoading = false
	a.artError = ""
}

// Network reads use saved settings only; service mutations stay in run's serial job.
func (a *app) runArtwork(title string, job func(context.Context) (func(), error)) {
	a.cancelArtwork()
	if a.win == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.artCancel, a.artLoading, a.artError = cancel, true, ""
	request, id, win := a.artRequest, a.detailID, a.win
	go func() {
		update, err := job(ctx)
		win.Update(func() {
			defer cancel()
			if err != nil {
				err = fmt.Errorf("%s失败：%w", title, err)
			}
			a.finishArtwork(request, id, update, err)
		})
	}()
}

func (a *app) finishArtwork(request uint64, id string, update func(), err error) {
	if request != a.artRequest || id != a.detailID {
		return
	}
	a.artLoading, a.artCancel = false, nil
	if update != nil {
		update()
	}
	if err != nil {
		a.artError = err.Error()
	}
}

func (a *app) selectArtKind(index int) {
	if a.busy {
		return
	}
	searching := a.artLoading
	a.cancelArtwork()
	a.artTab = index
	if a.artState().loaded || a.key == "" {
		return
	}
	if a.artGameChoice != "" {
		a.browseArt()
	} else if searching {
		a.searchArt()
	}
}
func (a *app) selectArtSource(label string) {
	if a.busy || label == a.artGameChoice {
		return
	}
	a.artGameChoice = label
	a.clearArtChoices()
	a.browseArt()
}
func (a *app) fillArt() {
	if a.busy {
		return
	}
	id, key := a.detailID, a.key
	a.run("补齐缺失封面", func(ctx context.Context) (func(), error) {
		results, err := a.svc.FillArtwork(ctx, id, key)
		statuses, current := readCurrentArtwork(a.svc, id)
		return func() { a.results = results; a.artStatus = statuses; a.artCurrent = current }, err
	})
}
func (a *app) artworkCandidates(c *nativeui.Context) {
	state := a.artState()
	if a.artLoading {
		nativeui.Column(c.Key("art-gallery")).Label("候选图片列表").Grow(1).MinHeight(0).WidthPercent(100).Center().Children(func() {
			nativeui.Spinner(c).Label("正在加载封面").Size(24, 24)
		})
		return
	}
	_, height := c.Size()
	thumbHeight := artThumbHeight(a.kind())
	if height < 700 {
		thumbHeight = min(thumbHeight, 150)
	}

	nativeui.Scroll(c.Key("art-gallery")).Label("候选图片列表").Grow(1).MinHeight(0).WidthPercent(100).Children(func() {
		nativeui.Column(c).Gap(10).WidthPercent(100).Children(func() {
			if len(state.thumbErrors) > 0 {
				nativeui.Text(c, fmt.Sprintf("%d 张图片加载失败，可选择后点击保存下载原图。", len(state.thumbErrors))).FontSize(12)
			}
			if state.previewLabel != "" && !state.hasSelection() {
				nativeui.Text(c, state.previewLabel).FontSize(12)
			}
			if a.artError != "" {
				nativeui.Text(c, a.artError).WidthPercent(100).TextColor(c.Theme().Danger)
			}
			if !state.loaded && !a.artLoading && a.artError == "" {
				nativeui.Text(c, "搜索游戏或选择本地图片").TextColor(c.Theme().TextMuted)
			}
			if state.loaded && len(state.artImages) == 0 {
				nativeui.Text(c, "暂无图片 · 更换来源或选择本地图片").FontSize(12)
			}
			if state.selectedID == 0 && state.preview != nil && len(state.previewData) > 0 {
				nativeui.Image(c, state.preview).Width(200).Height(120).Fit(nativeui.Contain)
			}
			w, _ := c.Size()
			available := w - 160 - shortcutListWidth(w) - 32
			columns := max(1, int((available+10)/(artCardWidth(a.kind())+10)))
			if a.kind() == model.Portrait {
				thumbHeight = max(1, ((available-10*float32(columns-1))/float32(columns)-12)*1.5)
			}
			nativeui.Grid(c).Columns(columns).Gap(10).WidthPercent(100).Children(func() {
				for _, candidate := range state.artImages {
					border := c.Theme().Border
					if state.selectedID == candidate.ID {
						border = c.Theme().Accent
					}
					label := fmt.Sprintf("选择 #%d", candidate.ID)
					if state.selectedID == candidate.ID {
						label = fmt.Sprintf("已选择 #%d", candidate.ID)
					}
					nativeui.ButtonBase(c.Key(strconv.Itoa(candidate.ID))).Label(label).Border(2, border).Radius(4).MinWidth(0).Padding(4).Background(c.Theme().Surface).Disabled(a.busy).OnClick(func() { a.selectCandidate(candidate) }).Children(func() {
						if thumb := state.thumbs[candidate.ID]; thumb != nil {
							nativeui.Image(c, thumb).WidthPercent(100).Height(thumbHeight).Fit(nativeui.Contain)
						} else {
							nativeui.Text(c, "缩略图不可用").WidthPercent(100).Height(thumbHeight).TextColor(c.Theme().TextMuted).TextAlign(nativeui.Center)
						}
					})
				}
			})
			for _, r := range a.results {
				nativeui.Text(c, r.Name+"："+r.Message).FontSize(12)
			}
		})
	})
}

func artGameLabel(g artwork.Game) string { return fmt.Sprintf("%s · SGDB %d", g.Name, g.ID) }
func (a *app) searchArt() {
	if a.busy || a.artLoading {
		return
	}
	a.clearArtChoices()
	a.artGames = nil
	a.artGameChoice = ""
	query, key := a.artQuery, a.key
	kind := a.kind()
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.runArtwork("搜索 SteamGridDB", func(ctx context.Context) (func(), error) {
		client, err := artwork.NewWithNetwork(key, cache, networkSettings)
		if err != nil {
			return nil, err
		}
		games, err := client.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		var candidates *artState
		if len(games) > 0 {
			candidates, err = fetchArtCandidates(ctx, client, games[0].ID, kind)
		}
		return func() {
			a.artGames = games
			if len(games) > 0 {
				a.artGameChoice = artGameLabel(games[0])
				if candidates != nil {
					a.artStates[kind] = candidates
				}
			} else {
				a.artState().loaded = true
				a.artState().previewLabel = "没有搜索结果，请调整搜索词或选择本地图片"
			}
		}, err
	})
}
func (a *app) browseArt() {
	if a.busy {
		return
	}
	state := a.artState()
	kind := a.kind()

	gameID := 0
	for _, g := range a.artGames {
		if artGameLabel(g) == a.artGameChoice {
			gameID = g.ID
		}
	}
	if gameID == 0 {
		return
	}
	key := a.key
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.runArtwork("加载候选图片", func(ctx context.Context) (func(), error) {
		client, err := artwork.NewWithNetwork(key, cache, networkSettings)
		if err != nil {
			return nil, err
		}
		candidates, err := fetchArtCandidates(ctx, client, gameID, kind)
		if err != nil {
			return nil, err
		}
		return func() {
			state.loaded = true
			state.artImages = candidates.artImages
			state.thumbs = candidates.thumbs
			state.thumbErrors = candidates.thumbErrors
		}, nil
	})
}
func (s *artState) hasSelection() bool { return s.selectedURL != "" || len(s.previewData) > 0 }
func (a *app) selectCandidate(candidate artwork.Image) {
	if a.busy {
		return
	}
	if candidate.URL == "" {
		a.status = "此图片缺少原图地址，请选择其他图片"
		return
	}
	state := a.artState()
	state.selectedID = candidate.ID
	state.selectedURL = candidate.URL
	state.preview = state.thumbs[candidate.ID]
	state.previewData = nil
	state.previewKind = a.kind()
	state.previewLabel = fmt.Sprintf("已选择：SteamGridDB #%d", candidate.ID)
}
func (a *app) chooseLocal() {
	if a.busy {
		return
	}
	a.cancelArtwork()
	state := a.artState()
	kind := a.kind()

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
			state.preview = bm
			state.selectedID = 0
			state.selectedURL = ""
			state.previewData = data
			state.previewKind = kind
			state.previewLabel = "待应用：" + filepath.Base(paths[0])
		}, nil
	})
}
func (a *app) applyPreview() {
	if a.busy {
		return
	}
	state := a.artState()
	kind := a.kind()

	id := a.detailID
	if !state.hasSelection() || state.previewKind != kind {
		return
	}
	data := append([]byte{}, state.previewData...)
	url, key := state.selectedURL, a.key
	cache := filepath.Join(a.svc.Dir, "cache")
	networkSettings := a.snapshot.Settings.Network
	a.run("保存图片", func(ctx context.Context) (func(), error) {
		var client *artwork.Client
		var err error
		if url != "" {
			client, err = artwork.NewWithNetwork(key, cache, networkSettings)
			if err != nil {
				return nil, err
			}
		}
		err = applyArtSelection(ctx, client, url, data, func(data []byte) error { return a.svc.ApplyArtwork(id, kind, data, false) })
		statuses, current := readCurrentArtwork(a.svc, id)
		return func() {
			a.artStatus = statuses
			a.artCurrent = current
			if err == nil {
				state.previewData = nil
				state.selectedID = 0
				state.selectedURL = ""
				state.previewLabel = "图片已应用"
			}
		}, err
	})
}

func artThumbHeight(kind model.ArtworkKind) float32 {
	if kind == model.Portrait {
		return 190
	}
	return 110
}

// Load every displayed candidate, with bounded parallel downloads. A missing or
// unusable thumbnail remains selectable; only Save downloads original images.
func loadArtThumbnails(ctx context.Context, client *artwork.Client, images []artwork.Image) (map[int]*nativeui.Bitmap, map[int]string, error) {
	loaded := make([]*nativeui.Bitmap, len(images))
	failures := make([]string, len(images))
	var workers sync.WaitGroup
	for worker := 0; worker < min(4, len(images)); worker++ {
		workers.Go(func() {
			for i := worker; i < len(images); i += 4 {
				if ctx.Err() != nil {
					return
				}
				candidate := images[i]
				var im image.Image
				err := fmt.Errorf("缩略图地址缺失")
				if candidate.Thumb != "" {
					_, im, err = client.DownloadThumbnail(ctx, candidate.Thumb)
				}

				if err == nil && im != nil {
					loaded[i] = bitmap(im, 200, 240)
				} else if err != nil {
					failures[i] = err.Error()
				}
			}
		})
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	thumbs := map[int]*nativeui.Bitmap{}
	errors := map[int]string{}
	for i, message := range failures {
		if message != "" {
			errors[images[i].ID] = message
		}
	}
	for i, bm := range loaded {
		if bm != nil {
			thumbs[images[i].ID] = bm
		}
	}
	return thumbs, errors, nil
}

func fetchArtCandidates(ctx context.Context, client *artwork.Client, gameID int, kind model.ArtworkKind) (*artState, error) {
	images, err := client.Images(ctx, gameID, kind)
	if err != nil {
		return nil, err
	}
	if len(images) > 20 {
		images = images[:20]
	}
	thumbs, thumbErrors, err := loadArtThumbnails(ctx, client, images)
	if err != nil {
		return nil, err
	}
	return &artState{loaded: true, artImages: images, thumbs: thumbs, thumbErrors: thumbErrors}, nil
}

// Save is the only online-selection path that downloads and applies an original.
func applyArtSelection(ctx context.Context, client *artwork.Client, url string, data []byte, apply func([]byte) error) error {
	if url != "" {
		var err error
		data, _, err = client.Download(ctx, url)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return apply(data)
}

func artCardWidth(kind model.ArtworkKind) float32 {
	if kind == model.Portrait {
		return 160
	}
	return 260
}
