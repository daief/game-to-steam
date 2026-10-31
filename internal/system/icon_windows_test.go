package system

import (
	"path/filepath"
	"testing"
)

func TestFileIconReadsProjectICO(t *testing.T) {
	if FileIcon(filepath.Join(t.TempDir(), "missing.exe")) != nil {
		t.Fatal("missing icon accepted")
	}
	icon := FileIcon(filepath.Join("..", "..", "assets", "icon.ico"))
	if icon == nil || icon.Bounds().Dx() != 32 || icon.Bounds().Dy() != 32 {
		t.Fatal("failed to extract Windows icon")
	}
}
