package artwork

import (
	"game-to-steam/internal/model"
	"testing"
)

func TestSearchQueryUsesReadableEnglishNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		game model.Game
		want string
	}{
		{"English title", model.Game{Name: "Hogwarts Legacy", InstallDir: `C:\Games\different`}, "Hogwarts Legacy"},
		{"Hogwarts localized", model.Game{Name: "霍格沃茨之遗", InstallDir: `C:\Program Files\Epic Games\HogwartsLegacy`, Executable: `C:\Program Files\Epic Games\HogwartsLegacy\HogwartsLegacy.exe`}, "Hogwarts Legacy"},
		{"random install suffix", model.Game{Name: "《赤痕·：夜之仪式》", InstallDir: `C:\Program Files\Epic Games\BloodstainedftBhx`, Executable: `C:\Program Files\Epic Games\BloodstainedftBhx\BloodstainedRotN.exe`}, "Bloodstained"},
		{"Unreal shipping exe", model.Game{Name: "游戏", InstallDir: `D:\中文游戏`, Executable: `D:\中文游戏\Binaries\Win64\ExampleGame-Win64-Shipping.exe`}, "Example Game"},
		{"trailing slash", model.Game{Name: "游戏", InstallDir: `D:\Games\Borderlands3\`}, "Borderlands 3"},
		{"opaque id not title", model.Game{Name: "游戏", InstallDir: `D:\Games\fa4240e57a3c46b39f169041b7811293`, AppName: "fa4240e57a3c46b39f169041b7811293"}, "游戏"},
		{"no title evidence", model.Game{Name: "游戏", InstallDir: `D:\游戏`, Executable: `D:\游戏\game.exe`}, "游戏"},
		{"directory authoritative", model.Game{Name: "游戏", InstallDir: `D:\Games\Control`, Executable: `D:\Games\Control\Control_DX12.exe`}, "Control"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SearchQuery(tc.game); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
