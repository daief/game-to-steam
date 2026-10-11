// Package epic reads installed games without authenticating to Epic.
package epic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"game-to-steam/internal/model"
)

type manifest struct {
	DisplayName, InstallLocation, LaunchExecutable, LaunchCommand string
	CatalogNamespace, CatalogItemId, AppName                      string
	BIsManaged                                                    bool  `json:"bIsManaged"`
	BIsIncompleteInstall                                          bool  `json:"bIsIncompleteInstall"`
	BIsApplication                                                *bool `json:"bIsApplication"`
	ExpectingDLCInstalled                                         map[string]bool
}

func DefaultManifestPath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "Epic", "EpicGamesLauncher", "Data", "Manifests")
}

func Scan(dir string) ([]model.Game, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取 Epic 清单目录 %s: %w", dir, err)
	}
	games := []model.Game{}
	warnings := []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".item") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		var m manifest
		if err == nil {
			err = json.Unmarshal(data, &m)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		if m.BIsIncompleteInstall || (m.BIsApplication != nil && !*m.BIsApplication) {
			continue
		}
		if m.DisplayName == "" || m.AppName == "" || m.CatalogNamespace == "" || m.CatalogItemId == "" || m.InstallLocation == "" {
			warnings = append(warnings, entry.Name()+": 缺少游戏标识或安装路径")
			continue
		}
		if stat, err := os.Stat(m.InstallLocation); err != nil || !stat.IsDir() {
			warnings = append(warnings, m.DisplayName+": 安装目录不存在")
			continue
		}
		if m.LaunchExecutable == "" && !m.BIsManaged {
			continue
		}
		exe := ""
		if m.LaunchExecutable != "" {
			rel := strings.ReplaceAll(m.LaunchExecutable, `\`, string(filepath.Separator))
			rel = strings.ReplaceAll(rel, "/", string(filepath.Separator))
			if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(filepath.Clean(rel), ".."+string(filepath.Separator)) {
				warnings = append(warnings, m.DisplayName+": 无效的可执行文件路径")
				continue
			}
			exe = filepath.Join(m.InstallLocation, rel)
		}
		id := m.CatalogNamespace + ":" + m.CatalogItemId + ":" + m.AppName
		if seen[id] {
			continue
		}
		seen[id] = true
		games = append(games, model.Game{ID: id, Name: m.DisplayName, InstallDir: m.InstallLocation, Executable: exe, LaunchCommand: m.LaunchCommand, Namespace: m.CatalogNamespace, CatalogID: m.CatalogItemId, AppName: m.AppName, Managed: m.BIsManaged, HasDLC: len(m.ExpectingDLCInstalled) > 0})
	}
	sort.Slice(games, func(i, j int) bool { return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name) })
	return games, warnings, nil
}

func LaunchURL(g model.Game) string {
	return "com.epicgames.launcher://apps/" + url.QueryEscape(g.ID) + "?action=launch&silent=true"
}

func MakeShortcut(g model.Game, launcher string) (model.Shortcut, error) {
	s := model.Shortcut{Name: g.Name, Exe: quote(g.Executable), StartDir: quote(g.InstallDir), Options: g.LaunchCommand, Icon: g.Executable, Tags: []string{"Epic", "game-to-steam"}}
	if g.ViaEpic() {
		if info, err := os.Stat(launcher); err != nil || info.IsDir() {
			return s, fmt.Errorf("找不到 Epic 启动器，请在设置中指定 EpicGamesLauncher.exe")
		}
		s.Exe = quote(launcher)
		s.StartDir = quote(filepath.Dir(launcher))
		s.Options = LaunchURL(g)
	} else if info, err := os.Stat(g.Executable); err != nil || info.IsDir() {
		return s, fmt.Errorf("游戏 EXE 不存在: %s", g.Executable)
	}
	return s, nil
}

func quote(s string) string { return `"` + strings.Trim(s, `"`) + `"` }
