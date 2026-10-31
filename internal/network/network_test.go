package network

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"game-to-steam/internal/model"
)

func TestModesAndCustomProxyRouting(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://environment.invalid:1234")
	direct, e := Transport(model.NetworkSettings{Mode: Direct})
	if e != nil || direct.Proxy != nil {
		t.Fatal("direct reads environment proxy", e)
	}
	if Normalize(model.NetworkSettings{}).Mode != System {
		t.Fatal("old settings should default to system")
	}
	hits := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Host != "remote.invalid" {
			t.Error(r.URL)
		}
		w.WriteHeader(204)
	}))
	defer proxy.Close()
	transport, e := Transport(model.NetworkSettings{Mode: Custom, ProxyURL: strings.TrimPrefix(proxy.URL, "http://")})
	if e != nil {
		t.Fatal(e)
	}
	defer transport.CloseIdleConnections()
	response, e := (&http.Client{Transport: transport}).Get("http://remote.invalid/image")
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if hits != 1 || response.StatusCode != 204 {
		t.Fatal("proxy not used")
	}
	for _, raw := range []string{"", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:99999", "user@host:7890", "ftp://host:1", "http://user:password@host:1", "http://host:99999", "http://host/path"} {
		if Validate(model.NetworkSettings{Mode: Custom, ProxyURL: raw}) == nil {
			t.Fatal("accepted", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1:7890", "[::1]:7890", "http://127.0.0.1:7890", "https://proxy:443", "socks5://127.0.0.1:1080", "socks5h://[::1]:1080"} {
		if e := Validate(model.NetworkSettings{Mode: Custom, ProxyURL: raw}); e != nil {
			t.Fatal(raw, e)
		}
	}
}

func TestSystemServerSelectionAndBypass(t *testing.T) {
	for _, tc := range []struct{ target, servers, bypass, want string }{
		{"https://example.com/image", "http=first:8080;https=second:7890", "", "http://second:7890"},
		{"https://example.com/image", "127.0.0.1:7890", "", "http://127.0.0.1:7890"},
		{"https://example.com/image", "socks=127.0.0.1:1080", "", "socks5://127.0.0.1:1080"},
		{"https://example.com/image", "http=first:8080", "", ""},
		{"http://server/image", "proxy:1", "<local>", ""},
		{"https://api.example.com/image", "proxy:1", "*.example.com;localhost", ""},
		{"https://other.com/image", "proxy:1", "*.example.com", "http://proxy:1"},
		{"https://example.com/image", "", "", ""},
	} {
		target, _ := url.Parse(tc.target)
		proxy, e := proxyFor(target, tc.servers, tc.bypass)
		if e != nil {
			t.Fatal(e)
		}
		got := ""
		if proxy != nil {
			got = proxy.String()
		}
		if got != tc.want {
			t.Errorf("%+v => %s", tc, got)
		}
	}
}
