// Package bridge implements an experimental Epic launch/session monitor.
// It never edits Steam files or terminates game/launcher processes.
package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"game-to-steam/internal/epic"
	"game-to-steam/internal/model"
)

type Process struct {
	PID, ParentPID uint32
	Created        uint64
	Path           string
}

type Backend interface {
	Snapshot(names []string) ([]Process, error)
	Launch(launcher, uri string) (uint32, error)
}

type Config struct {
	Game     model.Game
	Launcher string
	// Start overrides the launcher URI path for the authenticated experiment.
	// Called only after path validation and the already-running-game check.
	Start     func(context.Context) (uint32, error)
	ExtraArgs []string // Authenticated mode only: forwarded to game after validation.
	// WatchExe overrides the manifest's executable with a real game executable.
	// Empty means same basename anywhere beneath this game's install directory.
	WatchExe       string
	StartupTimeout time.Duration
	ExitGrace      time.Duration
	PollInterval   time.Duration
}

type Logger interface{ Printf(string, ...any) }

func (c Config) defaults() Config {
	if c.StartupTimeout == 0 {
		c.StartupTimeout = 5 * time.Minute
	}
	if c.ExitGrace == 0 {
		c.ExitGrace = 5 * time.Second
	}
	if c.PollInterval == 0 {
		c.PollInterval = 500 * time.Millisecond
	}
	return c
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(strings.ToLower(filepath.Clean(root)), strings.ToLower(filepath.Clean(path)))
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (c Config) matches(p Process) bool {
	if !inside(c.Game.InstallDir, p.Path) {
		return false
	}
	if c.WatchExe != "" {
		return strings.EqualFold(filepath.Clean(c.WatchExe), filepath.Clean(p.Path))
	}
	return strings.EqualFold(filepath.Base(c.Game.Executable), filepath.Base(p.Path))
}

func (c Config) validate() error {
	if c.Game.ID == "" || !filepath.IsAbs(c.Game.InstallDir) || (c.Start == nil && !filepath.IsAbs(c.Launcher)) {
		return fmt.Errorf("需要游戏标识、绝对安装目录及 Epic 启动器路径")
	}
	if c.WatchExe == "" && c.Game.Executable == "" {
		return fmt.Errorf("清单没有游戏 EXE，请通过 --watch-exe 指定实际游戏程序")
	}
	if c.WatchExe != "" && (!filepath.IsAbs(c.WatchExe) || !inside(c.Game.InstallDir, c.WatchExe)) {
		return fmt.Errorf("跟踪 EXE 必须位于该游戏安装目录内")
	}
	if c.StartupTimeout <= 0 || c.ExitGrace <= 0 || c.PollInterval <= 0 {
		return fmt.Errorf("等待时长和轮询间隔必须大于零")
	}
	return nil
}

func games(c Config, snapshot []Process) []Process {
	var result []Process
	for _, p := range snapshot {
		if c.matches(p) {
			result = append(result, p)
		}
	}
	return result
}

// Run refuses to attach to an already-running game. Once a new game is seen,
// it stays alive until matching processes are absent for the entire grace period.
// Cancellation/timeout only ends the bridge, never kills Epic or the game.
func Run(ctx context.Context, config Config, backend Backend, log Logger) error {
	c := config.defaults()
	if err := c.validate(); err != nil {
		return err
	}
	name := filepath.Base(c.Game.Executable)
	if c.WatchExe != "" {
		name = filepath.Base(c.WatchExe)
	}
	names := []string{name, "EpicGamesLauncher.exe"}
	before, err := backend.Snapshot(names)
	if err != nil {
		return fmt.Errorf("启动前进程检查失败: %w", err)
	}
	if len(games(c, before)) != 0 {
		return fmt.Errorf("游戏已在运行，请先退出游戏再测试桥接")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Printf("game=%s id=%s install=%s watch=%s", c.Game.Name, c.Game.ID, c.Game.InstallDir, name)
	for _, p := range before {
		log.Printf("baseline pid=%d parent=%d image=%s", p.PID, p.ParentPID, p.Path)
	}
	var pid uint32
	if c.Start != nil {
		pid, err = c.Start(ctx)
	} else {
		pid, err = backend.Launch(c.Launcher, epic.LaunchURL(c.Game))
	}
	if err != nil {
		if c.Start != nil {
			return fmt.Errorf("认证直启失败（不回退到 Epic 启动器）: %w", err)
		}
		return fmt.Errorf("隔离启动 Epic 失败（不回退到普通子进程启动）: %w", err)
	}
	log.Printf("launch-request pid=%d; waiting for game (timeout=%s)", pid, c.StartupTimeout)
	started := time.Now()
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()
	seen := false
	var absentSince time.Time
	known := map[Process]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := backend.Snapshot(names)
		if err != nil {
			return fmt.Errorf("进程跟踪失败: %w", err)
		}
		alive := games(c, snapshot)
		current := map[Process]bool{}
		for _, p := range alive {
			current[p] = true
			if !known[p] {
				log.Printf("game-start pid=%d parent=%d created=%d image=%s", p.PID, p.ParentPID, p.Created, p.Path)
			}
		}
		for p := range known {
			if !current[p] {
				log.Printf("game-exit pid=%d image=%s", p.PID, p.Path)
			}
		}
		known = current
		if len(alive) > 0 {
			seen = true
			absentSince = time.Time{}
		} else if seen {
			if absentSince.IsZero() {
				absentSince = time.Now()
				log.Printf("no game processes; grace=%s", c.ExitGrace)
			}
			if time.Since(absentSince) >= c.ExitGrace {
				for _, p := range snapshot {
					log.Printf("remaining pid=%d parent=%d image=%s", p.PID, p.ParentPID, p.Path)
				}
				log.Printf("session finished; exiting bridge without closing Epic")
				return nil
			}
		} else if time.Since(started) >= c.StartupTimeout {
			return fmt.Errorf("等待游戏启动超时；若停留在登录界面或游戏进程名不同，请检查日志并指定 --watch-exe")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
