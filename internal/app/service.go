// Package app coordinates operations. The UI calls these methods from a single
// background job at a time; this package never owns UI widgets.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"game-to-steam/internal/artwork"
	"game-to-steam/internal/bridge"
	"game-to-steam/internal/epic"
	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/model"
	"game-to-steam/internal/network"
	"game-to-steam/internal/steam"
	"game-to-steam/internal/storage"
	"game-to-steam/internal/system"
)

type Service struct {
	Dir       string
	State     model.State
	Profile   model.SteamProfile
	stateFile storage.Snapshot
	Preflight func() error
	// BridgePath is a runtime dependency, not a persisted launch preference.
	BridgePath string
}
type Snapshot struct {
	EpicAuth  epicauth.Status
	Games     []model.Game
	Shortcuts []model.Shortcut
	Managed   map[string]model.ManagedGame
	Profile   model.SteamProfile
	Settings  model.Settings
	Warnings  []string
}

func New(dir string) (*Service, error) {
	s, err := storage.Load(dir)
	if err != nil {
		return nil, err
	}
	file, err := storage.Read(filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	if s.Settings.SteamPath == "" {
		s.Settings.SteamPath = system.SteamPath()
	}
	if s.Settings.EpicManifestPath == "" {
		s.Settings.EpicManifestPath = system.EpicManifestPath()
		if s.Settings.EpicManifestPath == "" {
			s.Settings.EpicManifestPath = epic.DefaultManifestPath()
		}
	}
	s.Settings = normalizeSettings(s.Settings)
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	bridgePath := filepath.Join(filepath.Dir(executable), "epic-bridge.exe")
	if override := os.Getenv("GAME_TO_STEAM_BRIDGE_PATH"); override != "" {
		bridgePath, err = filepath.Abs(override)
		if err != nil {
			return nil, err
		}
	}
	return &Service{Dir: dir, State: s, stateFile: file, Preflight: system.RequireSteamStopped, BridgePath: bridgePath}, nil
}

func (s *Service) makeShortcut(g model.Game) (model.Shortcut, error) {
	if !g.ViaEpic() {
		return epic.MakeShortcut(g, "")
	}
	return bridge.MakeAuthenticatedShortcut(g, s.BridgePath, s.State.Settings.EpicManifestPath)
}

func (s *Service) Refresh() Snapshot {
	out := Snapshot{Settings: s.State.Settings, Managed: map[string]model.ManagedGame{}, EpicAuth: s.authStore().Status()}
	latest, stateErr := storage.Read(s.stateFile.Path)
	if stateErr != nil {
		out.Warnings = append(out.Warnings, stateErr.Error())
		return out
	}
	if latest.Exists != s.stateFile.Exists || !bytes.Equal(latest.Data, s.stateFile.Data) {
		state, err := storage.Load(s.Dir)
		if err != nil {
			out.Warnings = append(out.Warnings, err.Error())
			return out
		}
		// A deleted state file is not silently treated as permission to replace
		// earlier associations. Restarting explicitly initializes a fresh state.
		if !latest.Exists {
			out.Warnings = append(out.Warnings, "本地状态文件已被移除，请重新启动客户端")
			return out
		}
		state.Settings = normalizeSettings(state.Settings)
		s.State, s.stateFile = state, latest
		out.Settings = state.Settings
	}
	games, warnings, e := epic.Scan(s.State.Settings.EpicManifestPath)
	out.Games = games
	out.Warnings = warnings
	if e != nil {
		out.Warnings = append(out.Warnings, e.Error())
	}
	p, e := steam.RecentProfile(s.State.Settings.SteamPath)
	if e != nil {
		out.Warnings = append(out.Warnings, e.Error())
		s.Profile = model.SteamProfile{}
		return out
	}
	s.Profile = p
	out.Profile = p
	root, _, e := s.readShortcuts()
	if e != nil {
		out.Warnings = append(out.Warnings, e.Error())
		return out
	}
	out.Shortcuts = steam.List(root)
	for id, entry := range s.State.Accounts[p.AccountID] {
		out.Managed[id] = entry
	}
	for i, g := range out.Games {
		if m, ok := out.Managed[g.ID]; ok {
			out.Games[i].ForceEpic = m.Game.ForceEpic
		}
	}
	return out
}

func (s *Service) readShortcuts() (*steam.Node, storage.Snapshot, error) {
	if s.Profile.AccountID == "" {
		return nil, storage.Snapshot{}, fmt.Errorf("请先刷新并确定 Steam 目标账号")
	}
	f, e := storage.Read(filepath.Join(s.Profile.ConfigDir, "shortcuts.vdf"))
	if e != nil {
		return nil, f, e
	}
	if !f.Exists {
		return steam.NewRoot(), f, nil
	}
	root, e := steam.Parse(f.Data)
	if e != nil {
		return nil, f, fmt.Errorf("无法安全读取 %s: %w", f.Path, e)
	}
	return root, f, nil
}
func cloneState(state model.State) model.State {
	b, _ := json.Marshal(state)
	var out model.State
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Service) managed(state *model.State) map[string]model.ManagedGame {
	if state.Accounts == nil {
		state.Accounts = map[string]map[string]model.ManagedGame{}
	}
	m := state.Accounts[s.Profile.AccountID]
	if m == nil {
		m = map[string]model.ManagedGame{}
		state.Accounts[s.Profile.AccountID] = m
	}
	return m
}

func (s *Service) check() error {
	if e := s.Preflight(); e != nil {
		return e
	}
	p, e := steam.RecentProfile(s.State.Settings.SteamPath)
	if e != nil {
		return e
	}
	if p.AccountID != s.Profile.AccountID || p.Root != s.Profile.Root {
		return fmt.Errorf("最近使用的 Steam 账号发生变化，请刷新后重试")
	}
	return nil
}
func (s *Service) commit(state model.State, changes []storage.Change) error {
	b, e := storage.StateBytes(state)
	if e != nil {
		return e
	}
	changes = append(changes, storage.Change{Before: s.stateFile, Data: b})
	_, e = storage.Commit(filepath.Join(s.Dir, "backups"), changes, s.check)
	if e != nil {
		return e
	}
	s.State = state
	s.stateFile, e = storage.Read(s.stateFile.Path)
	return e
}

func (s *Service) SaveSettings(settings model.Settings) error {
	settings = normalizeSettings(settings)
	if err := network.Validate(settings.Network); err != nil {
		return err
	}
	state := cloneState(s.State)
	state.Settings = settings
	b, e := storage.StateBytes(state)
	if e != nil {
		return e
	}
	_, e = storage.Commit(filepath.Join(s.Dir, "backups"), []storage.Change{{Before: s.stateFile, Data: b}}, nil)
	if e != nil {
		return e
	}
	s.State = state
	s.stateFile, e = storage.Read(s.stateFile.Path)
	return e
}

func normalizeSettings(settings model.Settings) model.Settings {
	settings.Network = network.Normalize(settings.Network)
	settings.SteamPath = system.NormalizePath(settings.SteamPath)
	settings.EpicManifestPath = system.NormalizePath(settings.EpicManifestPath)
	settings.EpicLauncherPath = system.NormalizePath(settings.EpicLauncherPath)
	return settings
}

func SamePath(a, b string) bool {
	normalize := func(p string) string {
		return strings.ToLower(filepath.Clean(strings.ReplaceAll(strings.Trim(p, `"`), "/", `\`)))
	}
	return normalize(a) == normalize(b)
}
func Candidates(g model.Game, shortcuts []model.Shortcut) []model.Shortcut {
	var out []model.Shortcut
	for _, sc := range shortcuts {
		decoded, _ := url.QueryUnescape(sc.Options)
		if (g.Executable != "" && SamePath(sc.Exe, g.Executable)) || strings.Contains(decoded, "com.epicgames.launcher://apps/"+g.ID+"?") || bridge.ShortcutGameID(sc) == g.ID || strings.EqualFold(strings.TrimSpace(sc.Name), strings.TrimSpace(g.Name)) {
			out = append(out, sc)
		}
	}
	return out
}

type ImportRequest struct {
	Game    model.Game
	AdoptID uint32
	Skip    bool
}

func (s *Service) Import(ctx context.Context, requests []ImportRequest, key string) ([]model.Result, error) {
	if e := s.check(); e != nil {
		return nil, e
	}
	root, before, e := s.readShortcuts()
	if e != nil {
		return nil, e
	}
	state := cloneState(s.State)
	managed := s.managed(&state)
	results := []model.Result{}
	var imported []model.ManagedGame
	for _, req := range requests {
		g := req.Game
		result := model.Result{Name: g.Name}
		if req.Skip {
			result.Message = "已跳过"
			results = append(results, result)
			continue
		}
		current := steam.List(root)
		if existing, ok := managed[g.ID]; ok {
			found := false
			for _, sc := range current {
				if sc.AppID == existing.AppID {
					found = true
				}
			}
			if found {
				result.Message = "已管理，无需重复导入"
				results = append(results, result)
				continue
			}
		}
		candidates := Candidates(g, current)
		if req.AdoptID != 0 {
			found := false
			count := 0
			for _, sc := range current {
				if sc.AppID == req.AdoptID {
					count++
				}
			}
			for _, sc := range candidates {
				if sc.AppID == req.AdoptID {
					found = true
				}
			}
			if !found || count != 1 {
				result.Failed = true
				result.Message = "接管候选已变化，请刷新"
			} else {
				conflict := false
				for id, m := range managed {
					if id != g.ID && m.AppID == req.AdoptID {
						conflict = true
					}
				}
				if conflict {
					result.Failed = true
					result.Message = "该快捷方式已关联其他游戏"
				} else {
					managed[g.ID] = model.ManagedGame{Game: g, AppID: req.AdoptID, Adopted: true}
					result.Message = "已接管，保留原配置和图片"
				}
			}
		} else if len(candidates) > 0 {
			result.Failed = true
			result.Message = "发现重复候选，需要确认接管或跳过"
		} else {
			sc, err := s.makeShortcut(g)
			if err == nil {
				sc.AppID = steam.AppID(sc.Exe, sc.Name)
				if history, ok := state.PlaytimeHistory[s.Profile.AccountID][g.ID]; ok {
					sc.AppID = history.AppID
				}
				for _, old := range current {
					if old.AppID == sc.AppID {
						err = fmt.Errorf("AppID 冲突，停止导入该游戏")
					}
				}
			}
			if err == nil {
				err = steam.Upsert(root, sc)
			}
			if err != nil {
				result.Failed = true
				result.Message = err.Error()
			} else {
				entry := model.ManagedGame{Game: g, AppID: sc.AppID}
				managed[g.ID] = entry
				imported = append(imported, entry)
				result.Message = "导入成功"
			}
		}
		results = append(results, result)
	}
	changes := []storage.Change{{Before: before, Data: steam.Encode(root)}}
	var restore map[string]uint64
	for _, entry := range imported {
		if history, ok := state.PlaytimeHistory[s.Profile.AccountID][entry.Game.ID]; ok {
			if restore == nil {
				restore = map[string]uint64{}
			}
			for id, minutes := range history.Minutes {
				restore[id] = minutes
			}
		}
	}
	if len(restore) > 0 {
		config, err := storage.Read(filepath.Join(s.Profile.ConfigDir, "localconfig.vdf"))
		if err != nil {
			return nil, err
		}
		data, err := steam.RestorePlaytime(config.Data, restore)
		if err != nil {
			return nil, fmt.Errorf("无法恢复累计时长，未导入: %w", err)
		}
		changes = append(changes, storage.Change{Before: config, Data: data})
	}
	if e = s.commit(state, changes); e != nil {
		return results, e
	}
	if key == "" {
		if len(imported) > 0 {
			results = append(results, model.Result{Name: "封面", Message: "未设置 API Key，快捷方式已导入，可稍后补图"})
		}
		return results, nil
	}
	for _, entry := range imported {
		if e = ctx.Err(); e != nil {
			return results, e
		}
		artResults, err := s.FillArtwork(ctx, entry.Game.ID, key)
		results = append(results, artResults...)
		if err != nil {
			results = append(results, model.Result{Name: entry.Game.Name + " / 封面", Message: err.Error(), Failed: true})
		}
	}
	return results, nil
}

func (s *Service) Delete(ids []uint32, cleanup bool) error {
	if e := s.check(); e != nil {
		return e
	}
	root, before, e := s.readShortcuts()
	if e != nil {
		return e
	}
	selected := map[uint32]bool{}
	counts := map[uint32]int{}
	for _, sc := range steam.List(root) {
		counts[sc.AppID]++
	}
	for _, id := range ids {
		if counts[id] != 1 {
			return fmt.Errorf("快捷方式已变化或 AppID 重复，请刷新")
		}
		selected[id] = true
	}
	steam.Remove(root, selected)
	changes := []storage.Change{{Before: before, Data: steam.Encode(root)}}
	if cleanup {
		for _, id := range ids {
			paths, err := steam.AllArtFiles(s.Profile.GridDir, id)
			if err != nil {
				return err
			}
			for _, path := range paths {
				f, err := storage.Read(path)
				if err != nil {
					return err
				}
				changes = append(changes, storage.Change{Before: f, Delete: true})
			}
		}
	}
	state := cloneState(s.State)
	managed := s.managed(&state)
	var playtimeConfig *storage.Snapshot
	for id, m := range managed {
		if selected[m.AppID] {
			if playtimeConfig == nil {
				config, err := storage.Read(filepath.Join(s.Profile.ConfigDir, "localconfig.vdf"))
				if err != nil {
					return err
				}
				playtimeConfig = &config
				// Include the source in the transaction backup and conflict checks.
				if config.Exists {
					changes = append(changes, storage.Change{Before: config, Data: config.Data})
				}
			}
			minutes, err := steam.ReadPlaytime(playtimeConfig.Data, m.AppID)
			if err != nil {
				return fmt.Errorf("无法保存累计时长，未删除: %w", err)
			}
			if state.PlaytimeHistory == nil {
				state.PlaytimeHistory = map[string]map[string]model.SteamPlaytime{}
			}
			if state.PlaytimeHistory[s.Profile.AccountID] == nil {
				state.PlaytimeHistory[s.Profile.AccountID] = map[string]model.SteamPlaytime{}
			}
			history := state.PlaytimeHistory[s.Profile.AccountID][id]
			if history.AppID != m.AppID || history.Minutes == nil {
				history = model.SteamPlaytime{AppID: m.AppID, Minutes: map[string]uint64{}}
			}
			for key, value := range minutes {
				if value > history.Minutes[key] {
					history.Minutes[key] = value
				}
			}
			state.PlaytimeHistory[s.Profile.AccountID][id] = history
			delete(managed, id)
		}
	}
	return s.commit(state, changes)
}

func (s *Service) UpdateLaunch(id string, force bool) error {
	if e := s.check(); e != nil {
		return e
	}
	state := cloneState(s.State)
	managed := s.managed(&state)
	m, ok := managed[id]
	if !ok {
		return fmt.Errorf("该游戏尚未由本客户端管理")
	}
	root, before, e := s.readShortcuts()
	if e != nil {
		return e
	}
	var existing *model.Shortcut
	for _, sc := range steam.List(root) {
		if sc.AppID == m.AppID {
			copy := sc
			existing = &copy
		}
	}
	if existing == nil {
		return fmt.Errorf("快捷方式已被移除，请刷新")
	}
	// Resolve the current installation again: Epic updates and moves may change
	// the executable after the original import.
	games, _, scanErr := epic.Scan(s.State.Settings.EpicManifestPath)
	if scanErr != nil {
		return scanErr
	}
	currentGame := false
	for _, g := range games {
		if g.ID == id {
			m.Game = g
			currentGame = true
			break
		}
	}
	if !currentGame {
		return fmt.Errorf("游戏已不在 Epic 安装清单中，无法更新启动策略")
	}
	m.Game.ForceEpic = force
	generated, e := s.makeShortcut(m.Game)
	if e != nil {
		return e
	}
	existing.Exe = generated.Exe
	existing.StartDir = generated.StartDir
	existing.Options = generated.Options
	if e = steam.Upsert(root, *existing); e != nil {
		return e
	}
	managed[id] = m
	return s.commit(state, []storage.Change{{Before: before, Data: steam.Encode(root)}})
}

// PreviewLaunch only generates a draft. Steam may remain running while previewing.
func (s *Service) PreviewLaunch(id string, force bool) (model.LaunchSettings, error) {
	var out model.LaunchSettings
	if _, ok := s.State.Accounts[s.Profile.AccountID][id]; !ok {
		return out, fmt.Errorf("该游戏尚未由本客户端管理")
	}
	games, _, err := epic.Scan(s.State.Settings.EpicManifestPath)
	if err != nil {
		return out, err
	}
	for _, g := range games {
		if g.ID == id {
			g.ForceEpic = force
			sc, e := s.makeShortcut(g)
			if e != nil {
				return out, e
			}
			return model.LaunchSettings{ForceEpic: force, Exe: sc.Exe, StartDir: sc.StartDir, Options: sc.Options}, nil
		}
	}
	return out, fmt.Errorf("游戏已不在 Epic 安装清单中，可手动编辑现有命令")
}

// SaveLaunch preserves identity and untouched fields. The original command is
// compared again so an open editor cannot overwrite another program's changes.
func (s *Service) SaveLaunch(id string, draft model.LaunchSettings, original model.Shortcut) error {
	if e := s.check(); e != nil {
		return e
	}
	if strings.TrimSpace(draft.Exe) == "" {
		return fmt.Errorf("启动程序不能为空")
	}
	for _, field := range []string{draft.Exe, draft.StartDir, draft.Options} {
		if strings.ContainsAny(field, "\x00\r\n") {
			return fmt.Errorf("启动命令不能包含换行或 NUL 字符")
		}
	}
	state := cloneState(s.State)
	managed := s.managed(&state)
	m, ok := managed[id]
	if !ok {
		return fmt.Errorf("该游戏尚未由本客户端管理")
	}
	if original.AppID != m.AppID {
		return fmt.Errorf("游戏关联已变化，请重新打开启动设置")
	}
	root, before, e := s.readShortcuts()
	if e != nil {
		return e
	}
	var existing model.Shortcut
	count := 0
	for _, sc := range steam.List(root) {
		if sc.AppID == m.AppID {
			existing = sc
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("快捷方式已移除或 AppID 重复，请刷新")
	}
	if existing.Exe != original.Exe || existing.StartDir != original.StartDir || existing.Options != original.Options {
		return fmt.Errorf("启动命令已被外部修改，请重新打开启动设置")
	}
	existing.Exe = draft.Exe
	existing.StartDir = draft.StartDir
	existing.Options = draft.Options
	if e = steam.Upsert(root, existing); e != nil {
		return e
	}
	m.Game.ForceEpic = draft.ForceEpic
	managed[id] = m
	return s.commit(state, []storage.Change{{Before: before, Data: steam.Encode(root)}})
}

func (s *Service) ApplyArtwork(id string, kind model.ArtworkKind, data []byte, onlyMissing bool) error {
	if e := s.check(); e != nil {
		return e
	}
	m, ok := s.State.Accounts[s.Profile.AccountID][id]
	if !ok {
		return fmt.Errorf("仅支持已导入或接管的游戏")
	}
	validKind := false
	for _, k := range model.ArtworkKinds {
		if k == kind {
			validKind = true
		}
	}
	if !validKind {
		return fmt.Errorf("无效图片类型")
	}
	if _, _, e := artwork.Normalize(data); e != nil {
		return e
	}
	root, shortcutsBefore, e := s.readShortcuts()
	if e != nil {
		return e
	}
	count := 0
	for _, sc := range steam.List(root) {
		if sc.AppID == m.AppID {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("快捷方式已变化或 AppID 重复，请刷新")
	}
	paths, e := steam.ArtFiles(s.Profile.GridDir, m.AppID, kind)
	if e != nil {
		return e
	}
	if onlyMissing && len(paths) > 0 {
		return nil
	}
	target := filepath.Join(s.Profile.GridDir, steam.ArtBase(m.AppID, kind)+".png")
	before, e := storage.Read(target)
	if e != nil {
		return e
	}
	changes := []storage.Change{{Before: before, Data: data}}
	for _, p := range paths {
		if p == target {
			continue
		}
		f, err := storage.Read(p)
		if err != nil {
			return err
		}
		changes = append(changes, storage.Change{Before: f, Delete: true})
	}
	// Include the shortcut snapshot so removing a shortcut concurrently invalidates the write.
	check := func() error {
		if e := s.check(); e != nil {
			return e
		}
		if e := s.stateFile.Unchanged(); e != nil {
			return e
		}
		return shortcutsBefore.Unchanged()
	}
	_, e = storage.Commit(filepath.Join(s.Dir, "backups"), changes, check)
	return e
}

func (s *Service) FillArtwork(ctx context.Context, id, key string) ([]model.Result, error) {
	m, ok := s.State.Accounts[s.Profile.AccountID][id]
	if !ok {
		return nil, fmt.Errorf("游戏未被管理")
	}
	if e := s.check(); e != nil {
		return nil, e
	}
	client, clientErr := artwork.NewWithNetwork(key, filepath.Join(s.Dir, "cache"), s.State.Settings.Network)
	if clientErr != nil {
		return nil, clientErr
	}
	query := artwork.SearchQuery(m.Game)
	games, e := client.Search(ctx, query)
	if e != nil {
		return nil, e
	}
	var matched *artwork.Game
	for _, g := range games {
		if strings.EqualFold(strings.TrimSpace(g.Name), strings.TrimSpace(query)) {
			if matched != nil {
				return nil, fmt.Errorf("存在多个同名匹配，请手动选择图片来源")
			}
			copy := g
			matched = &copy
		}
	}
	if matched == nil && len(games) > 0 {
		matched = &games[0]
	}
	if matched == nil {
		return nil, fmt.Errorf("搜索 %q 没有结果，请在封面管理中手动搜索选择", query)
	}
	results := []model.Result{}
	for _, kind := range model.ArtworkKinds {
		result := model.Result{Name: m.Game.Name + " / " + kind.Label()}
		paths, err := steam.ArtFiles(s.Profile.GridDir, m.AppID, kind)
		if err == nil && len(paths) > 0 {
			result.Message = "已有图片，已保留"
			results = append(results, result)
			continue
		}
		var data []byte
		if err == nil {
			images, lookupErr := client.Images(ctx, matched.ID, kind)
			err = lookupErr
			if err == nil {
				if len(images) == 0 {
					err = fmt.Errorf("没有可用图片")
				} else {
					data, _, err = client.Download(ctx, images[0].URL)
				}
			}
		}
		if err == nil {
			err = s.ApplyArtwork(id, kind, data, true)
		}
		if err != nil {
			result.Failed = true
			result.Message = err.Error()
		} else {
			result.Message = "已补图"
		}
		results = append(results, result)
	}
	return results, nil
}

func DataDir() (string, error) {
	return storage.DataDir()
}

// ArtworkStatus is a read-only snapshot of files matching Steam's artwork names.
type ArtworkStatus struct {
	Paths []string
	Err   error
}

func (s *Service) ArtworkStatus(id string) map[model.ArtworkKind]ArtworkStatus {
	result := map[model.ArtworkKind]ArtworkStatus{}
	m, ok := s.State.Accounts[s.Profile.AccountID][id]
	for _, kind := range model.ArtworkKinds {
		if !ok {
			result[kind] = ArtworkStatus{Err: fmt.Errorf("游戏未被管理")}
			continue
		}
		paths, err := steam.ArtFiles(s.Profile.GridDir, m.AppID, kind)
		result[kind] = ArtworkStatus{Paths: paths, Err: err}
	}
	return result
}
