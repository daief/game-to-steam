package steam

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// configNode tracks byte offsets so changing minutes never rewrites unrelated
// Steam preferences, unknown fields, comments, or escaping.
type configNode struct {
	key, value        string
	start, end, close int
	object            bool
	children          []*configNode
}

func parseConfig(data string) (*configNode, error) {
	i := 0
	skip := func() {
		for i < len(data) {
			if unicode.IsSpace(rune(data[i])) {
				i++
				continue
			}
			if strings.HasPrefix(data[i:], "//") {
				for i < len(data) && data[i] != '\n' {
					i++
				}
				continue
			}
			break
		}
	}
	quoted := func() (string, int, int, error) {
		skip()
		start := i
		if i >= len(data) || data[i] != '"' {
			return "", 0, 0, fmt.Errorf("localconfig.vdf 字符串无效")
		}
		i++
		var b strings.Builder
		for i < len(data) {
			c := data[i]
			i++
			if c == '"' {
				return b.String(), start, i, nil
			}
			if c == '\\' && i < len(data) {
				c = data[i]
				i++
			}
			b.WriteByte(c)
		}
		return "", 0, 0, fmt.Errorf("localconfig.vdf 字符串未结束")
	}
	var object func(bool, int) (*configNode, error)
	object = func(nested bool, depth int) (*configNode, error) {
		if depth > 64 {
			return nil, fmt.Errorf("localconfig.vdf 嵌套过深")
		}
		n := &configNode{object: true}
		for {
			skip()
			if i == len(data) {
				if nested {
					return nil, fmt.Errorf("localconfig.vdf 缺少结束括号")
				}
				n.close = i
				return n, nil
			}
			if data[i] == '}' {
				if !nested {
					return nil, fmt.Errorf("localconfig.vdf 多余的结束括号")
				}
				n.close = i
				i++
				return n, nil
			}
			key, _, _, err := quoted()
			if err != nil {
				return nil, err
			}
			skip()
			var child *configNode
			if i < len(data) && data[i] == '{' {
				i++
				child, err = object(true, depth+1)
			} else {
				child = &configNode{}
				child.value, child.start, child.end, err = quoted()
			}
			if err != nil {
				return nil, err
			}
			child.key = key
			n.children = append(n.children, child)
		}
	}
	return object(false, 0)
}

func (n *configNode) lookup(key string) (*configNode, error) {
	var found *configNode
	for _, c := range n.children {
		if strings.EqualFold(c.key, key) {
			if found != nil {
				return nil, fmt.Errorf("localconfig.vdf 字段 %s 重复", key)
			}
			found = c
		}
	}
	return found, nil
}

var appsPath = []string{"UserLocalConfigStore", "Software", "Valve", "Steam", "Apps"}

func configPath(n *configNode, path []string) (*configNode, error) {
	for _, key := range path {
		var err error
		n, err = n.lookup(key)
		if err != nil || n == nil {
			return nil, err
		}
		if !n.object {
			return nil, fmt.Errorf("localconfig.vdf 字段 %s 不是对象", key)
		}
	}
	return n, nil
}

// ReadPlaytime captures only cumulative minutes, never the rolling two-week
// counter (which would become stale while the shortcut is deleted).
func ReadPlaytime(data []byte, id uint32) (map[string]uint64, error) {
	minutes := map[string]uint64{}
	if len(data) == 0 {
		return minutes, nil
	}
	root, err := parseConfig(string(data))
	if err != nil {
		return nil, err
	}
	apps, err := configPath(root, appsPath)
	if err != nil || apps == nil {
		return minutes, err
	}
	for _, key := range []string{strconv.FormatUint(uint64(id), 10), strconv.FormatInt(int64(int32(id)), 10)} {
		app, err := configPath(apps, []string{key})
		if err != nil {
			return nil, err
		}
		if app == nil {
			continue
		}
		field, err := app.lookup("Playtime")
		if err != nil {
			return nil, err
		}
		if field == nil {
			continue
		}
		value, err := strconv.ParseUint(field.value, 10, 64)
		if err != nil || field.object {
			return nil, fmt.Errorf("localconfig.vdf 累计时长无效")
		}
		minutes[key] = value
	}
	return minutes, nil
}

// RestorePlaytime restores a lower bound without replacing newer Steam data.
func RestorePlaytime(data []byte, minutes map[string]uint64) ([]byte, error) {
	for key, value := range minutes {
		if _, err := strconv.ParseInt(key, 10, 64); err != nil {
			return nil, fmt.Errorf("无效的时长 AppID")
		}
		var err error
		data, err = restoreMinutes(data, append(append([]string{}, appsPath...), key, "Playtime"), value)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func restoreMinutes(data []byte, path []string, value uint64) ([]byte, error) {
	root, err := parseConfig(string(data))
	if err != nil {
		return nil, err
	}
	n := root
	for i, key := range path {
		child, err := n.lookup(key)
		if err != nil {
			return nil, err
		}
		if child == nil {
			insert := "\"Playtime\"\t\"" + strconv.FormatUint(value, 10) + "\"\n"
			for j := len(path) - 2; j >= i; j-- {
				insert = "\"" + path[j] + "\"\n{\n" + insert + "}\n"
			}
			return []byte(string(data[:n.close]) + "\n" + insert + string(data[n.close:])), nil
		}
		if i == len(path)-1 {
			current, err := strconv.ParseUint(child.value, 10, 64)
			if err != nil || child.object {
				return nil, fmt.Errorf("localconfig.vdf 累计时长无效")
			}
			if current >= value {
				return data, nil
			}
			return []byte(string(data[:child.start]) + "\"" + strconv.FormatUint(value, 10) + "\"" + string(data[child.end:])), nil
		}
		if !child.object {
			return nil, fmt.Errorf("localconfig.vdf 字段 %s 不是对象", key)
		}
		n = child
	}
	return data, nil
}
