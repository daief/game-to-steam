package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	service "game-to-steam/internal/app"
	"game-to-steam/internal/artwork"
	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func artworkTestApp(t *testing.T) *app {
	a := testApp(t)
	a.snapshot.Managed["one"] = model.ManagedGame{Game: a.snapshot.Games[0], AppID: 123}
	a.openDetail("one")
	a.page = "Steam 快捷方式"
	a.shortcutChosen = true
	a.shortcutID = 123
	a.shortcutTab = 1
	a.snapshot.Shortcuts = []model.Shortcut{{AppID: 123, Name: "Alpha"}}
	a.artStatus = map[model.ArtworkKind]service.ArtworkStatus{}
	for _, k := range model.ArtworkKinds {
		a.artStatus[k] = service.ArtworkStatus{}
	}
	return a
}
func TestArtworkCardsPreserveSelectionsAndBusy(t *testing.T) {
	a := artworkTestApp(t)
	portrait := a.artState()
	portrait.previewData = []byte("portrait")
	portrait.previewKind = model.Portrait
	portrait.artImages = []artwork.Image{{ID: 1}}
	tt := nativeui.NewTester(a.view, 1100, 800)
	for i, k := range model.ArtworkKinds {
		if err := tt.Click(k.Label()); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText("待保存") && !tt.HasText(artPurpose(k)) {
			t.Fatal("missing purpose", k)
		}
		if a.kind() != k {
			t.Fatal(a.kind())
		}
		if i > 0 && len(a.artState().previewData) != 0 {
			t.Fatal("selection leaked across kinds")
		}
	}
	if err := tt.Click("竖版封面"); err != nil {
		t.Fatal(err)
	}
	if a.artState() != portrait || string(portrait.previewData) != "portrait" {
		t.Fatal("selection lost")
	}
	a.busy = true // also exercises the previous frame's enabled input
	if err := tt.Click("详情背景"); err != nil {
		t.Fatal(err)
	}
	if a.kind() != model.Portrait {
		t.Fatal("busy tab changed")
	}
	a.searchArt()
	a.openDetail("one")
	if a.artState() != portrait {
		t.Fatal("busy action cleared draft")
	}
}

func TestArtworkTypeSwitchPreservesLocalDraftWithoutSource(t *testing.T) {
	a := artworkTestApp(t)
	a.key = "mock"
	state := a.artState()
	state.previewData = []byte("local image")
	state.previewKind = model.Portrait
	a.selectArtKind(2)
	a.selectArtKind(0)
	if a.artState() != state || string(state.previewData) != "local image" {
		t.Fatal("switching types discarded the local selection")
	}
}
func TestArtworkSourceResetAndActualStatus(t *testing.T) {
	a := artworkTestApp(t)
	for i, k := range model.ArtworkKinds {
		a.artTab = i
		a.artState().previewData = []byte("pending")
		if a.artStatusLabel(k) != "未设置" {
			t.Fatal("preview counted as installed")
		}
		if artPurpose(k) == "" {
			t.Fatal("missing purpose")
		}
	}
	a.artStatus[model.Hero] = service.ArtworkStatus{Paths: []string{"123_hero.png"}}
	a.artStatus[model.Landscape] = service.ArtworkStatus{Err: errors.New("read failed")}
	if a.artStatusLabel(model.Hero) != "已设置" || a.artStatusLabel(model.Landscape) != "读取失败" {
		t.Fatal(a.artStatus)
	}
	a.searchArt()
	if len(a.artStates) != 0 {
		t.Fatal("search retained source drafts")
	}
	a.artState().previewData = []byte("new")
	a.openDetail("one")
	if a.artTab != 0 || len(a.artState().previewData) != 0 {
		t.Fatal("detail did not reset")
	}
}
func TestArtworkPreviewArtifacts(t *testing.T) {
	dir := previewDir()
	if dir == "" {
		t.Skip("optional preview")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := artworkTestApp(t)
	a.key = "mock"
	a.artGames = []artwork.Game{{ID: 1, Name: "示例游戏"}}
	a.artGameChoice = artGameLabel(a.artGames[0])
	sizes := [][2]int{{200, 300}, {300, 140}, {400, 130}}
	for i, k := range model.ArtworkKinds {
		a.artTab = i
		state := a.artState()
		im := image.NewRGBA(image.Rect(0, 0, sizes[i][0], sizes[i][1]))
		for y := 0; y < im.Bounds().Dy(); y++ {
			for x := 0; x < im.Bounds().Dx(); x++ {
				im.SetRGBA(x, y, color.RGBA{30, 100, 150, 255})
			}
		}
		state.preview = bitmap(im, 640, 360)
		state.previewLabel = "待应用：示例图片"
		state.previewData = []byte("mock")
		state.previewKind = k
		state.artImages = []artwork.Image{{ID: 1, Width: sizes[i][0], Height: sizes[i][1]}}
		state.thumbs[1] = bitmap(im, 200, 110)
		state.loaded = true
		state.selectedID = 1
		for id := 2; id <= 6; id++ {
			state.artImages = append(state.artImages, artwork.Image{ID: id, Width: sizes[i][0], Height: sizes[i][1]})
			state.thumbs[id] = bitmap(im, 200, 240)
		}
		if a.artCurrent == nil {
			a.artCurrent = map[model.ArtworkKind]*nativeui.Bitmap{}
		}
		a.artCurrent[k] = bitmap(im, 240, 140)
		a.artStatus[k] = service.ArtworkStatus{Paths: []string{"sample.png"}}
		tt := nativeui.NewTester(a.view, 1100, 800)
		f, err := os.Create(filepath.Join(dir, "artwork-"+string(k)+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestArtworkChangeSourceClearsAllDrafts(t *testing.T) {
	a := artworkTestApp(t)
	a.artGames = []artwork.Game{{ID: 1, Name: "First"}, {ID: 2, Name: "Second"}}
	a.artGameChoice = artGameLabel(a.artGames[0])
	for i := range model.ArtworkKinds {
		a.artTab = i
		a.artState().previewData = []byte("old source")
	}
	tt := nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click(artGameLabel(a.artGames[0])); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(artGameLabel(a.artGames[1])); err != nil {
		t.Fatal(err)
	}
	if a.artGameChoice != artGameLabel(a.artGames[1]) {
		t.Fatal("source did not change")
	}
	for _, state := range a.artStates {
		if len(state.previewData) > 0 || len(state.artImages) > 0 {
			t.Fatal("old source retained")
		}
	}
}

func TestArtworkBusySourceAndSearchInput(t *testing.T) {
	a := artworkTestApp(t)
	a.artGames = []artwork.Game{{ID: 1, Name: "First"}, {ID: 2, Name: "Second"}}
	a.artGameChoice = artGameLabel(a.artGames[0])
	tt := nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click(artGameLabel(a.artGames[0])); err != nil {
		t.Fatal(err)
	}
	a.busy = true
	if err := tt.Click(artGameLabel(a.artGames[1])); err != nil {
		t.Fatal(err)
	}
	if a.artGameChoice != artGameLabel(a.artGames[0]) {
		t.Fatal("busy source changed")
	}
	a.busy = false
	tt = nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click("封面搜索词"); err != nil {
		t.Fatal(err)
	}
	previous := a.artQuery
	a.busy = true
	tt.Type("blocked")
	if a.artQuery != previous {
		t.Fatal("busy search changed")
	}
}

func TestAllCandidateThumbnailsDoNotDownloadOriginals(t *testing.T) {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 20, 30))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "original") {
			t.Error("browsing requested an original", r.URL)
			http.Error(w, "unexpected", 500)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/broken") {
			http.Error(w, "unavailable", 404)
			return
		}
		_, _ = w.Write(pngData.Bytes())
	}))
	defer server.Close()
	client := artwork.New("mock", t.TempDir())
	client.BaseURL = server.URL
	images := []artwork.Image{}
	for i := 1; i <= 20; i++ {
		candidate := artwork.Image{ID: i, URL: fmt.Sprintf("%s/original/%d", server.URL, i), Thumb: fmt.Sprintf("%s/thumb/%d", server.URL, i)}
		if i == 7 {
			candidate.Thumb = ""
		}
		if i == 8 {
			candidate.Thumb = server.URL + "/broken/thumb"
		}
		if i == 9 {
			candidate.Thumb = server.URL + "/broken/thumb"
			candidate.URL = server.URL + "/broken/original"
		}
		images = append(images, candidate)
	}
	thumbs, failures, err := loadArtThumbnails(context.Background(), client, images)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(failures[9], "404") {
		t.Fatal("missing download failure", failures)
	}
	for _, candidate := range images {
		if candidate.ID >= 7 && candidate.ID <= 9 {
			if thumbs[candidate.ID] != nil {
				t.Fatal("failed image not marked missing")
			}
			continue
		}
		if thumbs[candidate.ID] == nil {
			t.Fatal("missing candidate thumbnail", candidate.ID)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := loadArtThumbnails(ctx, client, images); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestArtworkLayoutUsesCurrentCardsAndSingleSave(t *testing.T) {
	a := artworkTestApp(t)
	current := nativeui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 20, 30)))
	a.artCurrent = map[model.ArtworkKind]*nativeui.Bitmap{model.Portrait: current}
	state := a.artState()
	state.preview = nativeui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 10, 20)))
	state.previewData = []byte("pending")
	state.previewKind = model.Portrait
	tt := nativeui.NewTester(a.view, 1100, 800)
	for _, label := range []string{"当前设置", "竖版封面", "横版封面", "详情背景", "搜索", "保存"} {
		if !tt.HasText(label) {
			t.Fatal("missing layout element", label)
		}
	}
	for _, label := range []string{"浏览候选图片", "查看当前图片", "应用所选", "Logo"} {
		if tt.HasText(label) {
			t.Fatal("obsolete step", label)
		}
	}
	if a.artCurrent[model.Portrait] != current {
		t.Fatal("pending image replaced installed card")
	}
}
func TestReadCurrentArtworkLoadsAllKindsWithoutChangingDrafts(t *testing.T) {
	a := artworkTestApp(t)
	grid := t.TempDir()
	a.svc.Profile = model.SteamProfile{AccountID: "1", GridDir: grid}
	a.svc.State.Accounts = map[string]map[string]model.ManagedGame{"1": a.snapshot.Managed}
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 20, 30)))
	for _, name := range []string{"123p.png", "123_hero.png"} {
		if err := os.WriteFile(filepath.Join(grid, name), data.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(grid, "123.png"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	a.artState().previewData = []byte("pending")
	statuses, current := readCurrentArtwork(a.svc, "one")
	for _, k := range []model.ArtworkKind{model.Portrait, model.Hero} {
		if statuses[k].Err != nil || current[k] == nil {
			t.Fatal("current image missing", k, statuses[k])
		}
	}
	if statuses[model.Landscape].Err == nil || current[model.Landscape] != nil {
		t.Fatal("corrupt landscape accepted")
	}
	if string(a.artState().previewData) != "pending" {
		t.Fatal("draft lost")
	}
}

func TestArtworkManySourcesLayout(t *testing.T) {
	a := artworkTestApp(t)
	for i := 1; i <= 30; i++ {
		a.artGames = append(a.artGames, artwork.Game{ID: i, Name: fmt.Sprintf("Game %d", i)})
	}
	a.artGameChoice = artGameLabel(a.artGames[0])
	im := image.NewRGBA(image.Rect(0, 0, 120, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 120; x++ {
			im.SetRGBA(x, y, color.RGBA{30, 100, 150, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/grids/") {
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"url":"` + "http://" + r.Host + `/image","thumb":"` + "http://" + r.Host + `/image"}]}`))
			return
		}
		_, _ = w.Write(b.Bytes())
	}))
	defer server.Close()
	client := artwork.New("mock", t.TempDir())
	client.BaseURL = server.URL
	state, err := fetchArtCandidates(context.Background(), client, 1, model.Portrait)
	if err != nil {
		t.Fatal(err)
	}
	a.artStates[model.Portrait] = state
	tt := nativeui.NewTester(a.view, 1100, 800)
	rect, ok := tt.Find("候选图片列表")
	if !ok {
		t.Fatal("missing image list")
	}
	t.Log("image viewport", rect)
	source, _ := tt.Find("游戏来源")
	t.Log("source viewport", source)
	if source.H > 44 {
		t.Fatal("sources took over the vertical image list", source)
	}
	if state.thumbs[1] == nil {
		t.Fatal("downloaded candidate has no bitmap")
	}
	// The rendered image must be visible inside the image-list viewport.
	candidate, _ := tt.Find("选择 #1")
	pixel := tt.Image().At(int(candidate.X+candidate.W/2), int(rect.Y+50))
	red, green, blue, _ := pixel.RGBA()
	if red>>8 != 30 || green>>8 != 100 || blue>>8 != 150 {
		t.Fatal("downloaded image not visible", pixel)
	}
	if dir := previewDir(); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "artwork-many-sources.png"))
		if f != nil {
			_ = png.Encode(f, tt.Image())
			f.Close()
		}
	}
	if rect.H < 260 {
		t.Errorf("image viewport collapsed: %v", rect)
	}
}

func TestCandidateClickSelectsWithoutDownloadAndSaveDownloads(t *testing.T) {
	var calls atomic.Int32
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 12, 18)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = w.Write(encoded.Bytes()) }))
	defer server.Close()
	a := artworkTestApp(t)
	candidate := artwork.Image{ID: 42, URL: server.URL + "/original.png"}
	state := a.artState()
	state.artImages = []artwork.Image{candidate}
	state.loaded = true
	state.thumbs[42] = nativeui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 12, 18)))
	state.previewData = []byte("old local selection")
	tt := nativeui.NewTester(a.view, 1100, 800)
	if err := tt.Click("选择 #42"); err != nil {
		t.Fatal(err)
	}
	if a.busy || calls.Load() != 0 || !tt.HasText("已选择 #42") {
		t.Fatal("selection started download or did not select")
	}
	if state.selectedURL != candidate.URL || len(state.previewData) != 0 || !state.hasSelection() {
		t.Fatal("invalid pending selection")
	}
	a.selectArtKind(2)
	a.selectArtKind(0)
	if state.selectedURL != candidate.URL {
		t.Fatal("selection lost on type switch")
	}
	client := artwork.New("mock", t.TempDir())
	client.BaseURL = server.URL
	applied := false
	err := applyArtSelection(context.Background(), client, state.selectedURL, nil, func(data []byte) error {
		applied = true
		if calls.Load() != 1 {
			t.Error("apply happened before download")
		}
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			t.Error(err)
		}
		return nil
	})
	if err != nil || !applied || calls.Load() != 1 {
		t.Fatal("save did not download and apply", err, calls.Load())
	}
}
func TestSaveDownloadFailureDoesNotApplyAndLocalNeedsNoDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", 502) }))
	defer server.Close()
	client := artwork.New("mock", t.TempDir())
	client.BaseURL = server.URL
	applied := false
	err := applyArtSelection(context.Background(), client, server.URL+"/failed", nil, func([]byte) error { applied = true; return nil })
	if err == nil || applied {
		t.Fatal("download failure applied image", err)
	}
	err = applyArtSelection(context.Background(), nil, "", []byte("local"), func(data []byte) error {
		if string(data) != "local" {
			t.Fatal("local data changed")
		}
		applied = true
		return nil
	})
	if err != nil || !applied {
		t.Fatal("local save failed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = applyArtSelection(ctx, nil, "", []byte("local"), func([]byte) error { t.Error("canceled save applied image"); return nil }); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestArtworkDetailPrefillsEnglishQueryWithoutRenamingGame(t *testing.T) {
	a := artworkTestApp(t)
	managed := a.snapshot.Managed["one"]
	managed.Game.Name = "霍格沃茨之遗"
	managed.Game.InstallDir = `C:\Games\HogwartsLegacy`
	managed.Game.Executable = `C:\Games\HogwartsLegacy\HogwartsLegacy.exe`
	a.snapshot.Managed["one"] = managed
	a.openDetail("one")
	if a.artQuery != "Hogwarts Legacy" {
		t.Fatal(a.artQuery)
	}
	if a.snapshot.Managed["one"].Game.Name != "霍格沃茨之遗" {
		t.Fatal("display name changed")
	}
}

func TestArtworkCandidatesWrapWithWindowWidth(t *testing.T) {
	a := artworkTestApp(t)
	state := a.artState()
	state.loaded = true
	for id := 1; id <= 20; id++ {
		state.artImages = append(state.artImages, artwork.Image{ID: id, URL: fmt.Sprintf("https://images.invalid/%d", id)})
		state.thumbs[id] = nativeui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 120, 180)))
	}
	columns := func(width int) int {
		tt := nativeui.NewTester(a.view, width, 900)
		first, ok := tt.Find("选择 #1")
		if !ok {
			t.Fatal("missing first candidate")
		}
		if delta := first.H - ((first.W-12)*1.5 + 12); delta < -1 || delta > 1 {
			t.Fatal("portrait card leaves excess space around image", first)
		}
		count := 0
		for id := 1; id <= 20; id++ {
			r, ok := tt.Find(fmt.Sprintf("选择 #%d", id))
			if ok && r.Y == first.Y {
				count++
			}
		}
		return count
	}
	narrow, wide := columns(1100), columns(1600)
	if count := columns(1280); count != 4 {
		t.Fatalf("expected four portrait columns at 1280px, got %d", count)
	}
	if narrow < 2 || wide <= narrow {
		t.Fatalf("columns do not adapt: %d -> %d", narrow, wide)
	}
	if len(model.ArtworkKinds) != 3 {
		t.Fatal("Logo remains in supported kinds")
	}
}
