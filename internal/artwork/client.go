// Package artwork implements SteamGridDB search and bounded static-image downloads.
package artwork

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"game-to-steam/internal/model"
	"game-to-steam/internal/network"
	"game-to-steam/internal/storage"
)

const maxImageBytes = 20 << 20

type Game struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Image struct {
	ID     int    `json:"id"`
	URL    string `json:"url"`
	Thumb  string `json:"thumb"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type Client struct {
	Key, BaseURL, CacheDir string
	HTTP                   *http.Client
}

func New(key, cache string) *Client {
	c, _ := NewWithNetwork(key, cache, model.NetworkSettings{Mode: network.Direct})
	return c
}

func NewWithNetwork(key, cache string, settings model.NetworkSettings) (*Client, error) {
	transport, e := network.Transport(settings)
	if e != nil {
		return nil, e
	}
	return &Client{Key: key, CacheDir: cache, BaseURL: "https://www.steamgriddb.com/api/v2", HTTP: &http.Client{Transport: transport, Timeout: 25 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	if strings.TrimSpace(c.Key) == "" {
		return fmt.Errorf("请先在设置中保存 SteamGridDB API Key")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("User-Agent", "game-to-steam/0.1.0")
	res, e := c.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("图片服务连接失败或超时：%w", e)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return fmt.Errorf("SteamGridDB API Key 无效或无权访问")
	}
	if res.StatusCode == 429 {
		return fmt.Errorf("SteamGridDB 请求过于频繁，请稍后重试")
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("SteamGridDB 返回 HTTP %d", res.StatusCode)
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, &envelope); e != nil {
		return fmt.Errorf("SteamGridDB 响应无效")
	}
	if !envelope.Success {
		return fmt.Errorf("SteamGridDB 请求失败")
	}
	return json.Unmarshal(envelope.Data, out)
}
func (c *Client) Search(ctx context.Context, name string) ([]Game, error) {
	var games []Game
	e := c.get(ctx, "/search/autocomplete/"+url.PathEscape(strings.TrimSpace(name)), &games)
	return games, e
}
func (c *Client) Images(ctx context.Context, id int, kind model.ArtworkKind) ([]Image, error) {
	endpoint := "grids"
	query := "?types=static&nsfw=false&humor=false&mimes=image%2Fpng,image%2Fjpeg"
	switch kind {
	case model.Portrait:
		query += "&dimensions=600x900"
	case model.Landscape:
		query += "&dimensions=460x215,920x430"
	case model.Hero:
		endpoint = "heroes"
	default:
		return nil, fmt.Errorf("未知图片类型")
	}
	var images []Image
	e := c.get(ctx, "/"+endpoint+"/game/"+strconv.Itoa(id)+query, &images)
	return images, e
}

// Normalize validates before decoding, to limit both file and pixel allocations.
// Encoding a still PNG avoids extension/mime mismatches and animation surprises.
func Normalize(data []byte) ([]byte, image.Image, error) { return normalize(data, false) }

func normalize(data []byte, thumbnail bool) ([]byte, image.Image, error) {
	if len(data) > maxImageBytes {
		return nil, nil, fmt.Errorf("图片超过 20 MB")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "png" && format != "jpeg" && !(thumbnail && format == "webp")) {
		return nil, nil, fmt.Errorf("请选择有效的 PNG 或 JPEG 静态图片")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 24_000_000 {
		return nil, nil, fmt.Errorf("图片尺寸超过 2400 万像素")
	}
	im, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return nil, nil, fmt.Errorf("图片无法解码: %w", e)
	}
	var b bytes.Buffer
	if e = png.Encode(&b, im); e != nil {
		return nil, nil, e
	}
	if b.Len() > maxImageBytes {
		return nil, nil, fmt.Errorf("转换后的图片超过 20 MB，请选择更小的图片")
	}
	return b.Bytes(), im, nil
}
func Local(path string) ([]byte, image.Image, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxImageBytes+1))
	if e != nil {
		return nil, nil, e
	}
	return Normalize(b)
}

func (c *Client) Download(ctx context.Context, rawURL string) ([]byte, image.Image, error) {
	return c.download(ctx, rawURL, false)
}

// DownloadThumbnail accepts static WebP thumbnails and caches a PNG. Full-size
// downloads and local imports keep the existing PNG/JPEG restriction.
func (c *Client) DownloadThumbnail(ctx context.Context, rawURL string) ([]byte, image.Image, error) {
	return c.download(ctx, rawURL, true)
}
func (c *Client) download(ctx context.Context, rawURL string, thumbnail bool) ([]byte, image.Image, error) {
	u, e := url.Parse(rawURL)
	if e != nil {
		return nil, nil, fmt.Errorf("无效的图片地址")
	}
	// Test clients use a local httptest server. Production only fetches the SGDB CDN.
	if c.BaseURL == "https://www.steamgriddb.com/api/v2" && (u.Scheme != "https" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".steamgriddb.com") || u.User != nil || u.Port() != "") {
		return nil, nil, fmt.Errorf("图片不是 SteamGridDB HTTPS CDN 资源")
	}
	hash := fmt.Sprintf("%x.png", sha256.Sum256([]byte(rawURL)))
	cache := filepath.Join(c.CacheDir, hash)
	if c.CacheDir != "" {
		if data, im, e := Local(cache); e == nil {
			return data, im, nil
		}
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if e != nil {
		return nil, nil, e
	}
	res, e := c.HTTP.Do(req)
	if e != nil {
		return nil, nil, fmt.Errorf("图片下载失败或超时：%w", e)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, nil, fmt.Errorf("图片下载返回 HTTP %d", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, maxImageBytes+1))
	if e != nil {
		return nil, nil, e
	}
	data, im, e := normalize(b, thumbnail)
	if e != nil {
		return nil, nil, e
	}
	if c.CacheDir != "" {
		// The decoded image remains usable if the optional cache is unwritable
		// or another thumbnail request is replacing the same cached file.
		_ = storage.Replace(cache, data)
	}
	return data, im, nil
}
