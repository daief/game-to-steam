//go:build windows

// epic-bridge is a standalone experimental console launcher, not the desktop UI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"game-to-steam/internal/bridge"
	"game-to-steam/internal/epic"
	"game-to-steam/internal/model"
	"game-to-steam/internal/system"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "桥接失败：", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("manifest-dir", epic.DefaultManifestPath(), "Epic 清单目录")
	id := flag.String("game", "", "Epic 游戏标识（用 --list 查看）")
	list := flag.Bool("list", false, "只读列出已安装游戏，不启动 Epic")
	launcher := flag.String("launcher", "", "EpicGamesLauncher.exe 路径；默认自动发现")
	watch := flag.String("watch-exe", "", "实际游戏 EXE 路径（可相对安装目录）；默认跟踪安装目录内与清单同名的 EXE")
	timeout := flag.Duration("startup-timeout", 5*time.Minute, "等待游戏出现的最长时间")
	grace := flag.Duration("exit-grace", 5*time.Second, "游戏进程消失后的宽限期")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("不接受额外位置参数；使用 --help 查看用法")
	}
	if !*list && *id == "" {
		return fmt.Errorf("请先运行 --list，再指定 --game 游戏标识")
	}
	games, warnings, err := epic.Scan(system.NormalizePath(*dir))
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	if *list {
		for _, g := range games {
			fmt.Printf("%s\n  --game %s\n  EXE: %s\n", g.Name, g.ID, g.Executable)
		}
		return nil
	}
	var selected model.Game
	for _, g := range games {
		if g.ID == *id {
			selected = g
			break
		}
	}
	if selected.ID == "" {
		return fmt.Errorf("游戏不在已安装清单中，请运行 --list 检查标识")
	}
	if *launcher == "" {
		*launcher = system.EpicLauncherPath()
	}
	*launcher = system.NormalizePath(*launcher)
	if *watch != "" {
		*watch = system.NormalizePath(*watch)
		if !filepath.IsAbs(*watch) {
			*watch = filepath.Join(selected.InstallDir, *watch)
		}
		info, err := os.Stat(*watch)
		if err != nil || info.IsDir() {
			return fmt.Errorf("跟踪 EXE 不存在或不是文件: %s", *watch)
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(base, "game-to-steam", "bridge", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(logDir, "session-"+time.Now().Format("20060102-150405")+"-*.log")
	if err != nil {
		return err
	}
	defer file.Close()
	logger := log.New(io.MultiWriter(os.Stderr, file), "", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("experimental bridge pid=%d log=%s", os.Getpid(), file.Name())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err = bridge.Run(ctx, bridge.Config{Game: selected, Launcher: *launcher, WatchExe: *watch, StartupTimeout: *timeout, ExitGrace: *grace}, bridge.Native{Log: logger}, logger)
	if err != nil {
		logger.Printf("failed: %v", err)
	}
	return err
}
