package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"game-to-steam/internal/model"
)

// textVDF is deliberately read-only; it never rewrites Steam login information.
type textVDF struct {
	value    string
	children map[string]*textVDF
}

func parseText(data string) (*textVDF, error) {
	tokens := []string{}
	for i := 0; i < len(data); {
		if unicode.IsSpace(rune(data[i])) {
			i++
			continue
		}
		if i+1 < len(data) && data[i:i+2] == "//" {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		if data[i] == '{' || data[i] == '}' {
			tokens = append(tokens, string(data[i]))
			i++
			continue
		}
		if data[i] != '"' {
			return nil, fmt.Errorf("无效的文本 VDF")
		}
		i++
		var b strings.Builder
		closed := false
		for i < len(data) {
			ch := data[i]
			i++
			if ch == '"' {
				closed = true
				break
			}
			if ch == '\\' && i < len(data) {
				ch = data[i]
				i++
			}
			b.WriteByte(ch)
		}
		if !closed {
			return nil, fmt.Errorf("VDF 字符串未结束")
		}
		tokens = append(tokens, b.String())
	}
	pos := 0
	var object func(bool) (*textVDF, error)
	object = func(nested bool) (*textVDF, error) {
		n := &textVDF{children: map[string]*textVDF{}}
		for pos < len(tokens) {
			key := tokens[pos]
			pos++
			if key == "}" {
				if !nested {
					return nil, fmt.Errorf("多余的结束括号")
				}
				return n, nil
			}
			if key == "{" || pos >= len(tokens) {
				return nil, fmt.Errorf("无效的 VDF 键值")
			}
			value := tokens[pos]
			pos++
			if value == "{" {
				child, e := object(true)
				if e != nil {
					return nil, e
				}
				n.children[key] = child
			} else {
				if value == "}" {
					return nil, fmt.Errorf("缺少 VDF 值")
				}
				n.children[key] = &textVDF{value: value}
			}
		}
		if nested {
			return nil, fmt.Errorf("VDF 对象未结束")
		}
		return n, nil
	}
	return object(false)
}
func (n *textVDF) get(key string) string {
	if c := n.children[key]; c != nil {
		return c.value
	}
	return ""
}

func RecentProfile(root string) (model.SteamProfile, error) {
	var out model.SteamProfile
	data, err := os.ReadFile(filepath.Join(root, "config", "loginusers.vdf"))
	if err != nil {
		return out, fmt.Errorf("无法读取 Steam 登录账号: %w", err)
	}
	tree, err := parseText(string(data))
	if err != nil {
		return out, err
	}
	users := tree.children["users"]
	if users == nil {
		return out, fmt.Errorf("Steam 登录记录没有 users")
	}
	var chosenID string
	var chosen *textVDF
	latest := int64(-1)
	recentCount := 0
	for id, u := range users.children {
		if u.get("MostRecent") == "1" {
			chosenID = id
			chosen = u
			recentCount++
		}
	}
	if recentCount > 1 {
		return out, fmt.Errorf("多个 Steam 账号被标记为最近使用，无法安全选择")
	}
	if recentCount == 0 {
		tie := false
		for id, u := range users.children {
			ts, e := strconv.ParseInt(u.get("Timestamp"), 10, 64)
			if e != nil || ts <= 0 {
				continue
			}
			if ts > latest {
				chosenID = id
				chosen = u
				latest = ts
				tie = false
			} else if ts == latest {
				tie = true
			}
		}
		if tie {
			chosen = nil
		}
	}
	if chosen == nil {
		return out, fmt.Errorf("无法确定最近使用的 Steam 账号，请先登录 Steam")
	}
	steamID, err := strconv.ParseUint(chosenID, 10, 64)
	if err != nil || steamID < 76561197960265728 || steamID > 76561197960265728+4294967295 {
		return out, fmt.Errorf("无效的 SteamID")
	}
	account := strconv.FormatUint(steamID-76561197960265728, 10)
	config := filepath.Join(root, "userdata", account, "config")
	name := chosen.get("PersonaName")
	if name == "" {
		name = chosen.get("AccountName")
	}
	return model.SteamProfile{Root: root, AccountID: account, Name: name, ConfigDir: config, GridDir: filepath.Join(config, "grid")}, nil
}
