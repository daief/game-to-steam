package steam

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlaytimePreservesUnrelatedBytesAndNewerMinutes(t *testing.T) {
	data := []byte(`// keep comment
"UserLocalConfigStore" { "Software" { "Valve" { "Steam" { "apps" {
"-2147483647" { "Playtime" "125" "Playtime2wks" "7" "unknown" "a\\b" }
"2147483649" { "Playtime" "126" }
"42" { "Playtime" "900" }
} } } } }`)
	minutes, err := ReadPlaytime(data, 2147483649)
	if err != nil || minutes["-2147483647"] != 125 || minutes["2147483649"] != 126 {
		t.Fatalf("%v %v", minutes, err)
	}
	got, err := RestorePlaytime(data, map[string]uint64{"-2147483647": 130, "2147483649": 100})
	want := bytes.Replace(data, []byte(`"Playtime" "125"`), []byte(`"Playtime" "130"`), 1)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unexpected rewrite: %s %v", got, err)
	}
}

func TestPlaytimeRestoresMissingPaths(t *testing.T) {
	for _, data := range []string{"", `"UserLocalConfigStore" { "other" "keep" }`, `"UserLocalConfigStore" { "Software" { "Valve" { "Steam" { "Apps" { "-2147483647" { "LastPlayed" "123" } } } } } }`} {
		got, err := RestorePlaytime([]byte(data), map[string]uint64{"-2147483647": 81})
		if err != nil {
			t.Fatal(err)
		}
		minutes, err := ReadPlaytime(got, 2147483649)
		if err != nil || minutes["-2147483647"] != 81 {
			t.Fatalf("%s %v", got, err)
		}
		if strings.Contains(data, `"other" "keep"`) && !bytes.Contains(got, []byte(`"other" "keep"`)) {
			t.Fatal("lost unrelated preference")
		}
	}
}

func TestPlaytimeRejectsAmbiguousOrMalformedConfig(t *testing.T) {
	for _, data := range []string{`"UserLocalConfigStore" {`, `"UserLocalConfigStore" {} "userlocalconfigstore" {}`, `"UserLocalConfigStore" "wrong"`} {
		if _, err := ReadPlaytime([]byte(data), 2147483649); err == nil {
			t.Fatalf("accepted %s", data)
		}
		if _, err := RestorePlaytime([]byte(data), map[string]uint64{"-2147483647": 81}); err == nil {
			t.Fatalf("rewrote %s", data)
		}
	}
}
