//go:build windows

package epicauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SecureDir removes inherited ACLs and permits only this user and SYSTEM.
func SecureDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + u.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

type Store struct{ Dir string }

// Lock is cancellable; closing the returned file releases the cross-process lock.
func (s Store) Lock(ctx context.Context) (func(), error) {
	if err := SecureDir(s.Dir); err != nil {
		return nil, fmt.Errorf("创建受限认证目录失败: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(s.Dir, "auth.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
		if err == nil {
			return func() { _ = f.Close() }, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func crypt(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 || len(data) > maxResponse {
		return nil, fmt.Errorf("认证数据长度无效")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, fmt.Errorf("当前用户 DPAPI 认证数据处理失败")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}

func (s Store) Load() (Session, error) {
	b, err := os.ReadFile(filepath.Join(s.Dir, "session.dpapi"))
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, fmt.Errorf("桥接尚未登录，请在客户端设置中登录 Epic，或运行 epic-bridge.exe --auth-login")
	}
	if err != nil {
		return Session{}, err
	}
	plain, err := crypt(b, false)
	if err != nil {
		return Session{}, err
	}
	defer clear(plain)
	var v Session
	if json.Unmarshal(plain, &v) != nil {
		return Session{}, fmt.Errorf("桥接认证数据损坏，请重新登录")
	}
	return v, nil
}
func (s Store) Save(v Session) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("序列化会话失败")
	}
	defer clear(plain)
	b, err := crypt(plain, true)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, "session-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	src, _ := windows.UTF16PtrFromString(f.Name())
	dst, _ := windows.UTF16PtrFromString(filepath.Join(s.Dir, "session.dpapi"))
	return windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
func (s Store) Forget() error {
	err := os.Remove(filepath.Join(s.Dir, "session.dpapi"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// WithSession commits any rotated refresh token before the caller requests game credentials.
func WithSession(ctx context.Context, store Store, c *Client, fn func(Session) error) error {
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	s, err := store.Load()
	if err != nil {
		return err
	}
	if s.AccessToken == "" || time.Until(s.ExpiresAt) < 2*time.Minute {
		s, err = c.Refresh(ctx, s)
		if err != nil {
			return err
		}
		if err = store.Save(s); err != nil {
			return fmt.Errorf("无法保存刷新会话，停止启动: %w", err)
		}
	}
	return fn(s)
}
