//go:build windows

package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"game-to-steam/internal/epicauth"
)

type authTransport func(*http.Request) (*http.Response, error)

func (f authTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthenticatedLifecycle(t *testing.T) {
	for _, scenario := range []string{"complete", "cancel", "existing", "missing-exe", "auth-error", "create-failed"} {
		t.Run(scenario, func(t *testing.T) {
			c := testConfig(t)
			c.Launcher = ""
			c.Game.AppName = "app"
			c.Game.Namespace = "ns"
			c.Game.CatalogID = "item"
			c.Game.RequiresOwnershipToken = true
			if scenario != "missing-exe" {
				if err := os.WriteFile(c.Game.Executable, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := epicauth.Store{Dir: filepath.Join(t.TempDir(), "auth")}
			unlock, err := store.Lock(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			s := epicauth.Session{AccessToken: "old-access", RefreshToken: "old-refresh", AccountID: "account", DisplayName: "玩家", ExpiresAt: time.Now().Add(-time.Hour), RefreshExpiresAt: time.Now().Add(time.Hour)}
			if err = store.Save(s); err != nil {
				t.Fatal(err)
			}
			unlock()
			calls := []string{}
			client := epicauth.New(authTransport(func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.URL.Path)
				body := ""
				status := 200
				if scenario == "auth-error" {
					status = 401
					body = "secret-exchange secret-ovt"
				} else {
					switch {
					case strings.HasSuffix(r.URL.Path, "/token"):
						s.AccessToken = "new-access"
						s.RefreshToken = "new-refresh"
						s.ExpiresAt = time.Now().Add(time.Hour)
						b, _ := json.Marshal(s)
						body = string(b)
					case strings.HasSuffix(r.URL.Path, "/ownershipToken"):
						stored, e := store.Load()
						if e != nil || stored.RefreshToken != "new-refresh" {
							t.Error("rotation not saved before requesting credentials", e)
						}
						body = "secret-ovt"
					case strings.HasSuffix(r.URL.Path, "/exchange"):
						body = `{"code":"secret-exchange"}`
					default:
						t.Error("unexpected endpoint", r.URL.Path)
					}
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
			}))
			p := Process{PID: 88, Path: c.Game.Executable, Created: 1}
			f := &fakeBackend{snapshots: [][]Process{{}, {p}, {}}}
			if scenario == "existing" {
				f.snapshots = [][]Process{{p}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := false
			ovtPath := ""
			start := func(exe, dir string, args []string) (uint32, error) {
				started = true
				if exe != c.Game.Executable || dir != c.Game.InstallDir {
					t.Error("wrong launch target")
				}
				for _, arg := range args {
					if strings.HasPrefix(arg, "-epicovt=") {
						ovtPath = strings.TrimPrefix(arg, "-epicovt=")
					}
				}
				b, e := os.ReadFile(ovtPath)
				if e != nil || string(b) != "secret-ovt" {
					t.Error("missing token file", e)
				}
				if scenario == "cancel" {
					cancel()
				}
				if scenario == "create-failed" {
					return 0, fmt.Errorf("fixture launch failure")
				}
				return p.PID, nil
			}
			var output bytes.Buffer
			err = runAuthenticated(ctx, c, f, start, store, client, "zh-CN", log.New(&output, "", 0))
			if (err == nil) != (scenario == "complete") {
				t.Fatal("unexpected result", err)
			}
			if f.launches != 0 {
				t.Fatal("launched Epic fallback")
			}
			if scenario == "existing" || scenario == "missing-exe" {
				if len(calls) != 0 || started {
					t.Fatal("credentials requested before preflight")
				}
			}
			if scenario == "auth-error" && started {
				t.Fatal("started after failed auth")
			}
			if scenario == "complete" || scenario == "cancel" || scenario == "create-failed" {
				if !started || len(calls) != 3 || !strings.HasSuffix(calls[1], "ownershipToken") || !strings.HasSuffix(calls[2], "exchange") {
					t.Fatal(calls)
				}
				_, e := os.Stat(ovtPath)
				if scenario == "cancel" {
					if e != nil {
						t.Fatal("deleted live session token", e)
					}
				} else if !os.IsNotExist(e) {
					t.Fatal("token was not cleaned", e)
				}
			}
			logs := output.String()
			if err != nil {
				logs += err.Error()
			}
			for _, secret := range []string{"new-access", "new-refresh", "secret-exchange", "secret-ovt"} {
				if strings.Contains(logs, secret) {
					t.Fatal("secret leaked")
				}
			}
		})
	}
}

// Opt-in, creates only a copy of the Go test process; no Epic or game involved.
func TestAuthenticatedNativeHelper(t *testing.T) {
	if os.Getenv("EPIC_BRIDGE_AUTH_HELPER") == "1" {
		if os.Getenv("SteamAppId") != "123" {
			os.Exit(2)
		}
		if os.WriteFile(os.Getenv("EPIC_BRIDGE_AUTH_MARKER"), []byte("ok"), 0600) != nil {
			os.Exit(3)
		}
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	if os.Getenv("GAME_TO_STEAM_BRIDGE_NATIVE_TEST") != "1" {
		t.Skip("opt-in game-child check")
	}
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("EPIC_BRIDGE_AUTH_HELPER", "1")
	t.Setenv("EPIC_BRIDGE_AUTH_MARKER", marker)
	t.Setenv("SteamAppId", "123")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := startGame(exe, filepath.Dir(exe), []string{"-test.run=^TestAuthenticatedNativeHelper$"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		p, e := os.FindProcess(int(pid))
		if e == nil {
			_ = p.Kill()
			_ = p.Release()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, e := os.ReadFile(marker); e == nil && string(b) == "ok" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper or inherited Steam environment failed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	processes, err := (Native{Log: log.Default()}).Snapshot([]string{filepath.Base(exe)})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range processes {
		if p.PID == pid {
			if p.ParentPID != uint32(os.Getpid()) {
				t.Fatal("not ordinary child", p)
			}
			return
		}
	}
	t.Fatal("helper not found")
}
