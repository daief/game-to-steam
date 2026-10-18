//go:build windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"game-to-steam/internal/epicauth"
)

func TestLocalEpicAuthStatusAndForget(t *testing.T) {
	s, _ := fixture(t)
	if s.EpicAuthStatus().State != "missing" {
		t.Fatal("expected absent auth")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "bridge")); !os.IsNotExist(err) {
		t.Fatal("status must not create auth files")
	}
	store := s.authStore()
	unlock, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(epicauth.Session{AccountID: "account", DisplayName: "User", AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour), RefreshExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	unlock()
	if s.EpicAuthStatus().State != "ready" || s.Refresh().EpicAuth.DisplayName != "User" {
		t.Fatal("status not surfaced")
	}
	if err = s.LoginEpic(context.Background(), "bad"); err == nil {
		t.Fatal("accepted malformed code")
	}
	if s.EpicAuthStatus().State != "ready" {
		t.Fatal("failed login removed working auth")
	}
	if err = s.ForgetEpic(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.EpicAuthStatus().State != "missing" {
		t.Fatal("forget failed")
	}
}
