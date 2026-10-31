package ui

import (
	"path/filepath"
	"strings"

	"game-to-steam/internal/model"
	"game-to-steam/internal/system"
	nativeui "github.com/egoist/mygo/ui"
)

func shortcutIconPaths(sc model.Shortcut, managed map[string]model.ManagedGame) []string {
	paths := []string{sc.Icon}
	for _, m := range managed {
		if m.AppID == sc.AppID {
			paths = append(paths, m.Game.Executable)
			break
		}
	}
	if !strings.EqualFold(filepath.Base(strings.Trim(sc.Exe, `"`)), "epic-bridge.exe") {
		paths = append(paths, strings.Trim(sc.Exe, `"`))
	}
	return paths
}

func (a *app) loadShortcutIcons() {
	if a.win == nil {
		return
	}
	a.iconRequest++
	request, win := a.iconRequest, a.win
	paths := map[uint32][]string{}
	for _, sc := range a.snapshot.Shortcuts {
		paths[sc.AppID] = shortcutIconPaths(sc, a.snapshot.Managed)
	}
	go func() {
		icons := map[uint32]*nativeui.Bitmap{}
		for id, candidates := range paths {
			for _, path := range candidates {
				if path == "" {
					continue
				}
				if im := system.FileIcon(path); im != nil {
					icons[id] = bitmap(im, 48, 48)
					break
				}
			}
		}
		win.Update(func() {
			if a.iconRequest == request {
				a.shortcutIcons = icons
			}
		})
	}()
}
