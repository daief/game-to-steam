//go:build windows

package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"game-to-steam/internal/epic"
	"game-to-steam/internal/model"
	"golang.org/x/sys/windows"
)

// MakeShortcut retains Epic metadata/icon while routing launch through the bridge.
func MakeShortcut(g model.Game, launcher, executable, manifestDir string) (model.Shortcut, error) {
	sc, err := epic.MakeShortcut(g, launcher)
	if err != nil {
		return sc, err
	}
	if !g.ViaEpic() {
		return sc, nil
	}
	if !filepath.IsAbs(executable) {
		return sc, fmt.Errorf("桥接程序必须使用绝对路径")
	}
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		return sc, fmt.Errorf("找不到桥接程序 %s；请将 epic-bridge.exe 与客户端放在同一目录，或运行 mise run build", executable)
	}
	if g.Executable == "" {
		return sc, fmt.Errorf("清单没有游戏 EXE，无法自动生成桥接跟踪命令")
	}
	if !filepath.IsAbs(manifestDir) {
		return sc, fmt.Errorf("Epic 清单目录必须使用绝对路径")
	}
	sc.Exe = `"` + executable + `"`
	sc.StartDir = `"` + filepath.Dir(executable) + `"`
	sc.Options = windows.ComposeCommandLine([]string{"--game", g.ID, "--manifest-dir", manifestDir, "--launcher", launcher})
	return sc, nil
}

// MakeAuthenticatedShortcut does not require an Epic launcher installation or
// a current login: credentials are acquired at launch, never written to Steam.
func MakeAuthenticatedShortcut(g model.Game, executable, manifestDir string) (model.Shortcut, error) {
	if !g.ViaEpic() {
		return epic.MakeShortcut(g, "")
	}
	if !filepath.IsAbs(executable) {
		return model.Shortcut{}, fmt.Errorf("桥接程序必须使用绝对路径")
	}
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		return model.Shortcut{}, fmt.Errorf("找不到桥接程序 %s；请将 epic-bridge.exe 与客户端放在同一目录，或运行 mise run build", executable)
	}
	if !filepath.IsAbs(g.Executable) || !inside(g.InstallDir, g.Executable) {
		return model.Shortcut{}, fmt.Errorf("游戏入口必须位于安装目录内")
	}
	if info, err := os.Stat(g.Executable); err != nil || info.IsDir() {
		return model.Shortcut{}, fmt.Errorf("游戏入口 EXE 不存在")
	}
	if !filepath.IsAbs(manifestDir) {
		return model.Shortcut{}, fmt.Errorf("Epic 清单目录必须使用绝对路径")
	}
	return model.Shortcut{Name: g.Name, Exe: `"` + executable + `"`, StartDir: `"` + filepath.Dir(executable) + `"`,
		Options: windows.ComposeCommandLine([]string{"--game", g.ID, "--manifest-dir", manifestDir, "--launch-mode", model.EpicAuthenticated}),
		Icon:    g.Executable, Tags: []string{"Epic", "game-to-steam"}}, nil
}

// ShortcutGameID identifies only this bridge's CLI, not arbitrary programs with --game.
func ShortcutGameID(sc model.Shortcut) string {
	if !strings.EqualFold(filepath.Base(strings.Trim(sc.Exe, `"`)), "epic-bridge.exe") {
		return ""
	}
	args, err := windows.DecomposeCommandLine("bridge " + sc.Options)
	if err != nil {
		return ""
	}
	for i := 1; i < len(args); i++ {
		if args[i] == "--game" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "--game=") {
			return strings.TrimPrefix(args[i], "--game=")
		}
	}
	return ""
}
