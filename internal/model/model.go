// Package model contains the data shared by the UI and application services.
package model

type Game struct {
	ID, Name, InstallDir, Executable, LaunchCommand string
	Namespace, CatalogID, AppName                   string
	Managed, HasDLC, ForceEpic                      bool
	RequiresAuth, RequiresOwnershipToken            bool
}

func (g Game) ViaEpic() bool {
	return g.Managed || g.HasDLC || g.RequiresAuth || g.RequiresOwnershipToken || g.ForceEpic
}

type Shortcut struct {
	AppID                              uint32
	Name, Exe, StartDir, Options, Icon string
	Tags                               []string
}

// LaunchSettings edits the actual Steam command, rather than an extra suffix.
type LaunchSettings struct {
	ForceEpic              bool
	Exe, StartDir, Options string
}

type SteamProfile struct {
	Root, AccountID, Name, ConfigDir, GridDir string
}

type ManagedGame struct {
	Game    Game
	AppID   uint32
	Adopted bool
}

type Settings struct {
	Network          NetworkSettings `json:"network"`
	SteamPath        string          `json:"steamPath"`
	EpicManifestPath string          `json:"epicManifestPath"`
	EpicLauncherPath string          `json:"epicLauncherPath"`
}

const (
	EpicAuthenticated = "authenticated"
)

type NetworkSettings struct {
	Mode     string `json:"mode"`
	ProxyURL string `json:"proxyURL"`
}

type State struct {
	Settings Settings `json:"settings"`
	// Account IDs separate associations for different Steam accounts.
	Accounts map[string]map[string]ManagedGame `json:"accounts"`
}

type ArtworkKind string

const (
	Portrait  ArtworkKind = "portrait"
	Landscape ArtworkKind = "landscape"
	Hero      ArtworkKind = "hero"
)

var ArtworkKinds = []ArtworkKind{Portrait, Landscape, Hero}

func (k ArtworkKind) Label() string {
	switch k {
	case Portrait:
		return "竖版封面"
	case Landscape:
		return "横版封面"
	case Hero:
		return "详情背景"
	}
	return string(k)
}

type Result struct {
	Name, Message string
	Failed        bool
}
