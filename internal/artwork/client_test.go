package artwork

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"game-to-steam/internal/model"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 16, 12))); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}

func TestConfiguredProxyUsedForSearchAndDownload(t *testing.T) {
	data := testPNG(t)
	calls := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Host == "api.invalid" {
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Error("API key missing")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"name":"Game"}]}`))
		} else if r.URL.Host == "images.invalid" {
			if r.Header.Get("Authorization") != "" {
				t.Error("key leaked to image host")
			}
			_, _ = w.Write(data)
		} else {
			t.Error(r.URL)
		}
	}))
	defer proxy.Close()
	c, e := NewWithNetwork("test-key", "", model.NetworkSettings{Mode: "custom", ProxyURL: proxy.URL})
	if e != nil {
		t.Fatal(e)
	}
	c.BaseURL = "http://api.invalid"
	defer c.HTTP.CloseIdleConnections()
	if _, e = c.Search(context.Background(), "Game"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = c.Download(context.Background(), "http://images.invalid/image.png"); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal("search and image did not share proxy", calls)
	}
}
func TestSearchImagesAndDownload(t *testing.T) {
	pngData := testPNG(t)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/"):
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Error("missing auth")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":10,"name":"游戏"}]}`))
		case strings.HasPrefix(r.URL.Path, "/grids/"):
			if r.URL.Query().Get("dimensions") != "600x900" || r.URL.Query().Get("types") != "static" {
				t.Error("wrong image filters")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"url":"` + server.URL + `/image","width":16,"height":12}]}`))
		case r.URL.Path == "/image":
			if r.Header.Get("Authorization") != "" {
				t.Error("credential sent to image CDN")
			}
			_, _ = w.Write(pngData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New("secret", t.TempDir())
	c.BaseURL = server.URL
	games, e := c.Search(context.Background(), "游戏")
	if e != nil || len(games) != 1 {
		t.Fatalf("%v %v", games, e)
	}
	images, e := c.Images(context.Background(), 10, model.Portrait)
	if e != nil || len(images) != 1 {
		t.Fatalf("%v %v", images, e)
	}
	b, im, e := c.Download(context.Background(), images[0].URL)
	if e != nil || len(b) == 0 || im.Bounds().Dx() != 16 {
		t.Fatalf("download %v", e)
	}
	server.Close()
	if _, _, e = c.Download(context.Background(), images[0].URL); e != nil {
		t.Fatal("cache was not used", e)
	}
}
func TestFailures(t *testing.T) {
	for _, code := range []int{401, 403, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
			defer server.Close()
			c := New("secret", "")
			c.BaseURL = server.URL
			_, e := c.Search(context.Background(), "test")
			if e == nil || strings.Contains(e.Error(), "secret") {
				t.Fatal("bad error", e)
			}
		})
	}
	if _, e := New("", "").Search(context.Background(), "test"); e == nil {
		t.Fatal("missing key accepted")
	}
	if _, _, e := Normalize([]byte("not an image")); e == nil {
		t.Fatal("bad image accepted")
	}
	if _, _, e := Normalize(make([]byte, maxImageBytes+1)); e == nil {
		t.Fatal("oversize accepted")
	}
	if _, _, e := New("", "").Download(context.Background(), "http://127.0.0.1/private"); e == nil {
		t.Fatal("unexpected URL accepted")
	}
}
func TestEmptyInvalidAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "empty") {
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
			return
		}
		if strings.Contains(r.URL.Path, "bad") {
			_, _ = w.Write([]byte("invalid"))
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	c := New("key", "")
	c.BaseURL = server.URL
	games, e := c.Search(context.Background(), "empty")
	if e != nil || len(games) != 0 {
		t.Fatal(games, e)
	}
	if _, e = c.Search(context.Background(), "bad"); e == nil {
		t.Fatal("invalid JSON accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, e = c.Search(ctx, "timeout"); e == nil {
		t.Fatal("timeout not propagated")
	}
}

func TestAllArtworkEndpoints(t *testing.T) {
	for _, tc := range []struct {
		kind             model.ArtworkKind
		path, dimensions string
	}{
		{model.Portrait, "/grids/game/42", "600x900"},
		{model.Landscape, "/grids/game/42", "460x215,920x430"},
		{model.Hero, "/heroes/game/42", ""},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.URL.Query().Get("dimensions") != tc.dimensions || r.URL.Query().Get("types") != "static" {
					t.Errorf("wrong endpoint or filters: %s", r.URL)
				}
				_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
			}))
			defer server.Close()
			c := New("mock", "")
			c.BaseURL = server.URL
			if _, err := c.Images(context.Background(), 42, tc.kind); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// thumbnail.webp is the Go x/image gopher-doc.1bpp.lossless.webp fixture;
// its BSD license is included beside it.
func TestWebPThumbnailIsDecodedAndCachedAsPNG(t *testing.T) {
	webp, err := os.ReadFile("testdata/thumbnail.webp")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(webp) }))
	client := New("mock", t.TempDir())
	client.BaseURL = server.URL
	data, im, err := client.DownloadThumbnail(context.Background(), server.URL+"/thumb.webp")
	if err != nil || im == nil {
		t.Fatal("WebP thumbnail failed", err)
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal("cache is not PNG", err)
	}
	if _, _, err = Normalize(webp); err == nil {
		t.Fatal("local format policy changed")
	}
	server.Close()
	if _, _, err = client.DownloadThumbnail(context.Background(), server.URL+"/thumb.webp"); err != nil {
		t.Fatal("cache not used", err)
	}
}

func TestDownloadStillWorksWhenCacheCannotBeWritten(t *testing.T) {
	data := testPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	cache := t.TempDir() + "/file"
	if err := os.WriteFile(cache, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	client := New("mock", cache)
	client.BaseURL = server.URL
	if _, im, err := client.DownloadThumbnail(context.Background(), server.URL+"/thumb"); err != nil || im == nil {
		t.Fatal("cache failure hid valid image", err)
	}
}

func TestConcurrentThumbnailCacheWritesDoNotHideImages(t *testing.T) {
	webp, err := os.ReadFile("testdata/thumbnail.webp")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(webp) }))
	defer server.Close()
	client := New("mock", t.TempDir())
	client.BaseURL = server.URL
	var requests sync.WaitGroup
	for i := 0; i < 8; i++ {
		requests.Go(func() {
			if _, im, err := client.DownloadThumbnail(context.Background(), server.URL+"/shared.webp"); err != nil || im == nil {
				t.Error("concurrent image lost", err)
			}
		})
	}
	requests.Wait()
}
