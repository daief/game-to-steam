package system

import "testing"

func TestNormalizePath(t *testing.T) {
	for input, want := range map[string]string{"c:/apps/steam": `C:\apps\steam`, `C:\ProgramData\Epic\EpicGamesLauncher\Data\Manifests`: `C:\ProgramData\Epic\EpicGamesLauncher\Data\Manifests`, "": "", ` "d:/Games/Epic Launcher/" `: `D:\Games\Epic Launcher`, `\\server\share\Epic`: `\\server\share\Epic`} {
		if got := NormalizePath(input); got != want {
			t.Errorf("%q => %q, want %q", input, got, want)
		}
	}
}
func TestLauncherFromCommand(t *testing.T) {
	want := `C:\apps\Epic Games\Launcher\Portal\Binaries\Win64\EpicGamesLauncher.exe`
	for _, command := range []string{`"` + want + `" %1`, `"` + want + `" "%1"`, want + ` %1`} {
		if got := launcherFromCommand(command); got != want {
			t.Errorf("%q => %q", command, got)
		}
	}
	for _, command := range []string{"", `"missing quote`, `"C:\other.exe" %1`} {
		if got := launcherFromCommand(command); got != "" {
			t.Fatal(got)
		}
	}
}
