package ui

import (
	"os"
	"path/filepath"
	"testing"

	nativeui "github.com/egoist/mygo/ui"
)

func TestSettingsDirectoryDraftValidation(t *testing.T) {
	dir := t.TempDir()
	if got, err := directoryDraft(`"` + dir + `"`); err != nil || got != dir {
		t.Fatal(got, err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", file, filepath.Join(dir, "missing")} {
		if _, err := directoryDraft(path); err == nil {
			t.Fatal("invalid directory accepted", path)
		}
	}
}

func TestSettingsInputWidthsAndPathActions(t *testing.T) {
	for _, size := range [][2]int{{980, 600}, {1600, 1000}} {
		a := testApp(t)
		a.page = "设置"
		a.settings.Network.Mode = "custom"
		tt := nativeui.NewTester(a.view, size[0], size[1])
		for _, label := range []string{"Steam 安装目录", "Epic 安装清单目录"} {
			input := assertVisible(t, tt, label+"输入", size)
			if input.W > 761 {
				t.Fatal("path input exceeds width cap", input)
			}
			for _, action := range []string{"扫描", "选择"} {
				button := assertVisible(t, tt, action+label, size)
				if button.X < input.X+input.W || button.Y < input.Y-2 || button.Y > input.Y+2 {
					t.Fatal("path action not after input", input, button)
				}
			}
		}
		if size[0] == 1600 {
			input, _ := tt.Find("代理地址输入")
			if input.W <= 0 || input.W > 421 {
				t.Fatal("proxy input exceeds width cap", input)
			}
		}
	}
}
