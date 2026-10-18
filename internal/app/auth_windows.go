//go:build windows

package app

import (
	"context"
	"path/filepath"

	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/network"
)

func (s *Service) authStore() epicauth.Store {
	return epicauth.Store{Dir: filepath.Join(s.Dir, "bridge", "auth")}
}

func (s *Service) EpicAuthStatus() epicauth.Status { return s.authStore().Status() }

func (s *Service) LoginEpic(ctx context.Context, input string) error {
	code, err := epicauth.ParseCode(input)
	if err != nil {
		return err
	}
	t, err := network.Transport(s.State.Settings.Network)
	if err != nil {
		return err
	}
	defer t.CloseIdleConnections()
	store := s.authStore()
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	session, err := epicauth.New(t).Login(ctx, code)
	if err != nil {
		return err
	}
	return store.Save(session)
}

// ForgetEpic removes only the independent local session. No remote logout.
func (s *Service) ForgetEpic(ctx context.Context) error {
	store := s.authStore()
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return store.Forget()
}
