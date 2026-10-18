//go:build windows

package epicauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"game-to-steam/internal/model"
	"golang.org/x/sys/windows"
)

func TestDPAPIStoreAndLock(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "auth")}
	unlock, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := sessionFixture()
	if err = store.Save(s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(store.Dir, "session.dpapi"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), s.AccessToken) || strings.Contains(string(b), s.RefreshToken) {
		t.Fatal("plaintext on disk")
	}
	loaded, err := store.Load()
	if err != nil || loaded.RefreshToken != s.RefreshToken {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if release, err := store.Lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatal("concurrent lock accepted", err)
	}
	if err = store.Forget(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(); err == nil {
		t.Fatal("forgotten session still readable")
	}
	unlock()
	release, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err = os.WriteFile(filepath.Join(store.Dir, "session.dpapi"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(); err == nil {
		t.Fatal("accepted corruption")
	}
}

func TestGameArguments(t *testing.T) {
	g := model.Game{AppName: "app", Namespace: "ns", LaunchCommand: `-SaveToUserDir -UserDir="游戏 存档"`, RequiresOwnershipToken: true}
	args, err := GameArguments(g, sessionFixture(), "exchange-code", `C:\安全目录\ownership.ovt`, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	args2, err := windows.DecomposeCommandLine(windows.ComposeCommandLine(args))
	if err != nil || strings.Join(args2, "|") != strings.Join(args, "|") {
		t.Fatal("quoting", err)
	}
	for _, expected := range []string{"-UserDir=游戏 存档", "-AUTH_PASSWORD=exchange-code", "-epicusername=玩家", "-epiclocale=zh-CN", `-epicovt=C:\安全目录\ownership.ovt`} {
		found := false
		for _, a := range args {
			found = found || a == expected
		}
		if !found {
			t.Error("missing", expected)
		}
	}
	for _, bad := range []string{"-AUTH_PASSWORD=secret", "/epicuserid other", "--EpicPortal", "-epiclocale=other"} {
		g.LaunchCommand = bad
		if _, err = GameArguments(g, sessionFixture(), "exchange-code", "file", "en-US"); err == nil {
			t.Error("accepted override", bad)
		}
	}
	g.LaunchCommand = ""
	if args, err = GameArguments(g, sessionFixture(), "exchange-code", "token.ovt", "zh-CN", "-windowed", "-UserDir=中文 路径"); err != nil || !strings.Contains(strings.Join(args, "|"), "-UserDir=中文 路径") {
		t.Fatal(args, err)
	}
	if _, err = GameArguments(g, sessionFixture(), "exchange-code", "token.ovt", "zh-CN", "--AUTH_PASSWORD=override"); err == nil {
		t.Fatal("accepted auth override from extra args")
	}
	if _, err = GameArguments(g, sessionFixture(), "exchange-code", "", "en-US"); err == nil {
		t.Fatal("missing ownership")
	}
}

func TestLocalStatusStates(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "auth")}
	if store.Status().State != "missing" {
		t.Fatal("expected missing")
	}
	unlock, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	s := sessionFixture()
	for _, state := range []string{"ready", "refresh", "expired"} {
		if state == "refresh" {
			s.ExpiresAt = time.Now().Add(-time.Minute)
		}
		if state == "expired" {
			s.RefreshExpiresAt = time.Now().Add(-time.Minute)
		}
		if err = store.Save(s); err != nil {
			t.Fatal(err)
		}
		got := store.Status()
		if got.State != state || strings.Contains(got.Label(), s.RefreshToken) {
			t.Fatal(got)
		}
	}
}
