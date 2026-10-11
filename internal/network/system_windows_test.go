//go:build windows

package network

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/sys/windows"
)

// Use real WinHTTP with a local PAC server, without changing Windows settings.
func TestWindowsPACResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		if r.URL.Path == "/direct.pac" {
			_, _ = w.Write([]byte(`function FindProxyForURL(url, host) { return "DIRECT"; }`))
			return
		}
		_, _ = w.Write([]byte(`function FindProxyForURL(url, host) { return "PROXY 127.0.0.1:7890"; }`))
	}))
	defer server.Close()
	for _, entry := range []struct{ route, want string }{{"/proxy.pac", "http://127.0.0.1:7890"}, {"/direct.pac", ""}} {
		pac, e := windows.UTF16PtrFromString(server.URL + entry.route)
		if e != nil {
			t.Fatal(e)
		}
		req, _ := http.NewRequest(http.MethodGet, "https://www.steamgriddb.com/test", nil)
		proxy, e := resolveSystemConfig(req, ieConfig{AutoURL: pac})
		if e != nil {
			t.Fatal(e)
		}
		got := ""
		if proxy != nil {
			got = proxy.String()
		}
		if got != entry.want {
			t.Fatalf("%s => %s", entry.route, got)
		}
	}
}
