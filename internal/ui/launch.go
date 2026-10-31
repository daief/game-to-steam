package ui

// Imported games share the Steam editor, including drafts already being edited.
func (a *app) openLaunch(id string) {
	if a.busy {
		return
	}
	m, ok := a.snapshot.Managed[id]
	if !ok {
		return
	}
	for _, sc := range a.snapshot.Shortcuts {
		if sc.AppID == m.AppID {
			a.page, a.query, a.shortcutTab = "Steam 快捷方式", a.shortcutQuery, 0
			a.selectShortcut(sc)
			return
		}
	}
	a.status = "快捷方式已不存在，请刷新"
}
