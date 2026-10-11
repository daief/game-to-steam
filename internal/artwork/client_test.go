package artwork

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
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
