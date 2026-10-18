package epicauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sessionFixture() Session {
	return Session{AccessToken: "secret-access", RefreshToken: "secret-refresh", AccountID: "account", DisplayName: "玩家", ExpiresAt: time.Now().Add(time.Hour).UTC(), RefreshExpiresAt: time.Now().Add(24 * time.Hour).UTC()}
}

func TestLoginRefreshExchangeOwnership(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/account/api/oauth/token":
			user, password, ok := r.BasicAuth()
			if !ok || user != clientID || password != clientSecret {
				t.Error("missing app auth")
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "authorization_code" && r.Form.Get("code") != "test-code" {
				t.Error("code")
			}
			if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("refresh_token") != "secret-refresh" {
				t.Error("refresh")
			}
			s := sessionFixture()
			if r.Form.Get("grant_type") == "refresh_token" {
				s.RefreshToken = "rotated"
			}
			_ = json.NewEncoder(w).Encode(s)
		case "/account/api/oauth/exchange":
			if r.Header.Get("Authorization") != "bearer secret-access" {
				t.Error("bearer")
			}
			fmt.Fprint(w, `{"code":"game-exchange"}`)
		case "/ecommerceintegration/api/public/platforms/EPIC/identities/account/ownershipToken":
			_ = r.ParseForm()
			if r.Form.Get("nsCatalogItemId") != "namespace:catalog" {
				t.Error("ownership body")
			}
			fmt.Fprint(w, "signed-ownership")
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := New(server.Client().Transport)
	c.account = server.URL
	c.commerce = server.URL
	ctx := context.Background()
	s, err := c.Login(ctx, "test-code")
	if err != nil {
		t.Fatal(err)
	}
	s, err = c.Refresh(ctx, s)
	if err != nil || s.RefreshToken != "rotated" {
		t.Fatal(s.RefreshToken, err)
	}
	if code, err := c.Exchange(ctx, s); err != nil || code != "game-exchange" {
		t.Fatal(code, err)
	}
	if b, err := c.Ownership(ctx, s, "namespace", "catalog"); err != nil || string(b) != "signed-ownership" {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}

func TestResponseFailuresAreBoundedAndRedacted(t *testing.T) {
	for _, kind := range []string{"unauthorized", "malformed", "oversized", "redirect", "account-change", "empty-ownership"} {
		t.Run(kind, func(t *testing.T) {
			redirected := false
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "unauthorized":
					w.WriteHeader(401)
					fmt.Fprint(w, `{"errorMessage":"secret-access secret-refresh test-code"}`)
				case "malformed":
					fmt.Fprint(w, `{"access_token":"secret-access"}`)
				case "oversized":
					fmt.Fprint(w, strings.Repeat("x", maxResponse+1))
				case "redirect":
					http.Redirect(w, r, target.URL+"/secret-access", 302)
				case "account-change":
					s := sessionFixture()
					s.AccountID = "different"
					json.NewEncoder(w).Encode(s)
				case "empty-ownership":
				}
			}))
			defer server.Close()
			c := New(server.Client().Transport)
			c.account = server.URL
			c.commerce = server.URL
			var err error
			if kind == "empty-ownership" {
				_, err = c.Ownership(context.Background(), sessionFixture(), "ns", "catalog")
			} else if kind == "account-change" {
				_, err = c.Refresh(context.Background(), sessionFixture())
			} else {
				_, err = c.Login(context.Background(), "test-code")
			}
			if err == nil || redirected {
				t.Fatal("accepted failed or redirected auth", err)
			}
			for _, s := range []string{"secret-access", "secret-refresh", "test-code"} {
				if strings.Contains(err.Error(), s) {
					t.Fatal("secret in error")
				}
			}
		})
	}
}

func TestParseCode(t *testing.T) {
	for _, s := range []string{"authorization-code", `{"authorizationCode":"authorization-code"}`, ` "authorization-code" `} {
		if code, err := ParseCode(s); err != nil || code != "authorization-code" {
			t.Fatal(code, err)
		}
	}
	for _, s := range []string{"", `{"errorMessage":"private"}`, "abc\nsecret", `{broken}`} {
		if _, err := ParseCode(s); err == nil {
			t.Fatal("accepted", s)
		}
	}
	if !strings.HasPrefix(LoginURL(), "https://www.epicgames.com/id/login?") {
		t.Fatal("login destination")
	}
}
