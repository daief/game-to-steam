package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"game-to-steam/internal/model"
)

func TestBinaryRoundTripAndPreservation(t *testing.T) {
	root := NewRoot()
	sc := model.Shortcut{AppID: AppID(`"C:\Games\游戏.exe"`, "游戏"), Name: "游戏", Exe: `"C:\Games\游戏.exe"`, Tags: []string{"用户分类"}}
	if err := Upsert(root, sc); err != nil {
		t.Fatal(err)
	}
	root.Children[0].setText("FutureField", "keep me")
	root.Children[0].set(&Node{Type: 7, Key: "future64", Value: []byte{1, 2, 3, 4, 5, 6, 7, 8}})
	data := Encode(root)
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, Encode(parsed)) {
		t.Fatal("roundtrip changed bytes")
	}
	items := List(parsed)
	if len(items) != 1 || items[0].Name != "游戏" || items[0].Tags[0] != "用户分类" {
		t.Fatalf("bad shortcut: %+v", items)
	}
	items[0].Options = "new option"
	if err := Upsert(parsed, items[0]); err != nil {
		t.Fatal(err)
	}
	if parsed.Children[0].text("FutureField") != "keep me" || !bytes.Equal(parsed.Children[0].field("future64").Value, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("unknown fields lost")
	}
	if AppID("exe", "name") != uint32(0x8d266ab7) {
		t.Fatal("incorrect IEEE CRC32 shortcut ID")
	}
}
func TestRejectMalformedBinary(t *testing.T) {
	data := Encode(NewRoot())
	for _, bad := range [][]byte{nil, []byte("garbage"), data[:len(data)-1], append(append([]byte{}, data...), 0), {0, 's', 0, 99, 'x', 0, 8, 8}} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
}
func TestRemovePreservesOtherShortcuts(t *testing.T) {
	r := NewRoot()
	_ = Upsert(r, model.Shortcut{AppID: 1, Name: "delete"})
	_ = Upsert(r, model.Shortcut{AppID: 2, Name: "keep", Tags: []string{"tag"}})
	r.Children[1].setText("unknown", "unchanged")
	Remove(r, map[uint32]bool{1: true})
	if len(r.Children) != 1 || r.Children[0].Key != "0" || r.Children[0].text("unknown") != "unchanged" {
		t.Fatal("wrong removal")
	}
}
func TestRecentProfile(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "config"), 0700)
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "config", "loginusers.vdf"), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`"users" { "76561197960265729" { "PersonaName" "最近账号" "MostRecent" "1" "Timestamp" "1" } "76561197960265730" { "PersonaName" "其他账号" "Timestamp" "9" } }`)
	p, err := RecentProfile(dir)
	if err != nil || p.AccountID != "1" || p.Name != "最近账号" {
		t.Fatalf("%+v %v", p, err)
	}
	write(`"users" { "76561197960265729" { "Timestamp" "5" } "76561197960265730" { "Timestamp" "5" } }`)
	if _, err = RecentProfile(dir); err == nil {
		t.Fatal("accepted ambiguous account")
	}
	write(`"users" { "76561197960265729" { "MostRecent" "1" } "76561197960265730" { "MostRecent" "1" } }`)
	if _, err = RecentProfile(dir); err == nil {
		t.Fatal("accepted two recent accounts")
	}
	write(`"users" { "bogus" { "MostRecent" "1" } }`)
	if _, err = RecentProfile(dir); err == nil {
		t.Fatal("accepted invalid steam ID")
	}
}
func TestArtworkCleanupBoundary(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"123.png", "123p.jpg", "123_hero.png", "123_logo.png", "123-icon.ico", "123_bigpicture.png", "1234.png", "123p-other.png", "source.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := AllArtFiles(dir, 123)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f) == "1234.png" || filepath.Base(f) == "123p-other.png" || filepath.Base(f) == "source.jpg" {
			t.Fatal("unsafe cleanup", f)
		}
	}
	if len(files) != 6 {
		t.Fatalf("expected 6 images, got %v", files)
	}
}
