//go:build windows

package epicauth

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Status contains only presentation-safe local metadata, never credentials.
type Status struct {
	State, DisplayName          string
	ExpiresAt, RefreshExpiresAt time.Time
}

// Status reads an atomically replaced session without locking, creating files,
// refreshing tokens, or communicating with Epic.
func (s Store) Status() Status {
	if _, err := os.Stat(filepath.Join(s.Dir, "session.dpapi")); errors.Is(err, os.ErrNotExist) {
		return Status{State: "missing"}
	}
	v, err := s.Load()
	if err != nil {
		return Status{State: "invalid"}
	}
	state := "ready"
	if v.RefreshToken == "" || !validID(v.AccountID) || time.Now().After(v.RefreshExpiresAt) {
		state = "expired"
	} else if v.AccessToken == "" || time.Until(v.ExpiresAt) < 2*time.Minute {
		state = "refresh"
	}
	return Status{State: state, DisplayName: v.DisplayName, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

func (s Status) Label() string {
	switch s.State {
	case "ready":
		return "已登录：" + s.DisplayName + "（本地状态）"
	case "refresh":
		return "已登录：" + s.DisplayName + "（启动时自动刷新）"
	case "expired":
		return "登录已过期，请重新登录"
	case "invalid":
		return "本地认证不可用，请重新登录"
	default:
		return "尚未登录 Epic 独立认证"
	}
}
