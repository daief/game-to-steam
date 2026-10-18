//go:build windows

package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"game-to-steam/internal/epicauth"
	"golang.org/x/sys/windows"
)

// startGame preserves Steam's environment and ordinary parent/child relationship.
// It never returns command lines in errors, and cancellation never kills the game.
func startGame(exe, dir string, args []string) (uint32, error) {
	pid, h, err := syscall.StartProcess(exe, append([]string{exe}, args...), &syscall.ProcAttr{
		Dir: dir, Env: os.Environ(), Files: []uintptr{0, 0, 0}, Sys: &syscall.SysProcAttr{NoInheritHandles: true},
	})
	if err != nil {
		return 0, fmt.Errorf("创建游戏进程失败（系统错误，不记录认证命令）")
	}
	// A successfully created process must remain tracked even if handle release fails.
	_ = windows.CloseHandle(windows.Handle(h))
	return uint32(pid), nil
}

func RunAuthenticated(ctx context.Context, c Config, n Native, store epicauth.Store, client *epicauth.Client, locale string, log Logger) error {
	return runAuthenticated(ctx, c, n, startGame, store, client, locale, log)
}

func runAuthenticated(ctx context.Context, c Config, backend Backend, start func(string, string, []string) (uint32, error), store epicauth.Store, client *epicauth.Client, locale string, log Logger) error {
	g := c.Game
	if !filepath.IsAbs(g.Executable) || !inside(g.InstallDir, g.Executable) {
		return fmt.Errorf("游戏入口必须是安装目录内的绝对路径")
	}
	info, err := os.Stat(g.Executable)
	if err != nil || info.IsDir() {
		return fmt.Errorf("游戏入口 EXE 不存在")
	}
	// Validate static parameters before requesting any credentials.
	if _, err = epicauth.GameArguments(g, epicauth.Session{AccountID: "validation"}, "validation", "validation", locale, c.ExtraArgs...); err != nil {
		return err
	}
	var tokenDir string
	launched := false
	c.Start = func(ctx context.Context) (pid uint32, err error) {
		err = epicauth.WithSession(ctx, store, client, func(s epicauth.Session) error {
			var ownership string
			if g.RequiresOwnershipToken {
				log.Printf("authenticated-launch: requesting ownership token")
				b, e := client.Ownership(ctx, s, g.Namespace, g.CatalogID)
				if e != nil {
					return e
				}
				defer clear(b)
				tokenDir, e = os.MkdirTemp(store.Dir, "launch-ovt-")
				if e != nil {
					return e
				}
				if e = epicauth.SecureDir(tokenDir); e != nil {
					return e
				}
				ownership = filepath.Join(tokenDir, "ownership.ovt")
				if e = os.WriteFile(ownership, b, 0600); e != nil {
					return fmt.Errorf("写入受限所有权令牌失败")
				}
			}
			log.Printf("authenticated-launch: requesting exchange code")
			code, e := client.Exchange(ctx, s)
			if e != nil {
				return e
			}
			args, e := epicauth.GameArguments(g, s, code, ownership, locale, c.ExtraArgs...)
			if e != nil {
				return e
			}
			if e = ctx.Err(); e != nil {
				return e
			}
			pid, e = start(g.Executable, g.InstallDir, args)
			launched = e == nil
			return e
		})
		return pid, err
	}
	log.Printf("launch-mode=authenticated; ordinary game child; Steam environment inherited; no Epic launch")
	err = Run(ctx, c, backend, log)
	// On cancellation, timeout or tracking failure the game may still need this file.
	// Never delete unrelated directories or a previous session's token files.
	if tokenDir != "" {
		if err == nil || !launched {
			if e := os.RemoveAll(tokenDir); e != nil {
				log.Printf("ownership token cleanup failed; retained in restricted launch directory")
			}
		} else {
			log.Printf("ownership token retained in restricted launch directory because session completion is unknown")
		}
	}
	return err
}
