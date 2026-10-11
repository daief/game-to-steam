package steam

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strconv"

	"game-to-steam/internal/model"
)

// Node retains field order and raw scalar bytes, including fields we do not edit.
type Node struct {
	Type     byte
	Key      string
	Value    []byte
	Children []*Node
}

func Parse(data []byte) (*Node, error) {
	r := bytes.NewReader(data)
	nodes, err := parseNodes(r, 0)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 || len(nodes) != 1 || nodes[0].Type != 0 || nodes[0].Key != "shortcuts" {
		return nil, fmt.Errorf("无效的 shortcuts.vdf 根对象")
	}
	for _, n := range nodes[0].Children {
		if n.Type != 0 {
			return nil, fmt.Errorf("无效的快捷方式对象")
		}
	}
	return nodes[0], nil
}

func cstring(r *bytes.Reader) ([]byte, error) {
	var b []byte
	for {
		c, e := r.ReadByte()
		if e != nil {
			return nil, e
		}
		if c == 0 {
			return b, nil
		}
		b = append(b, c)
	}
}

func parseNodes(r *bytes.Reader, depth int) ([]*Node, error) {
	if depth > 32 {
		return nil, fmt.Errorf("VDF 嵌套过深")
	}
	var nodes []*Node
	for {
		t, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("VDF 缺少结束标记: %w", err)
		}
		if t == 8 {
			return nodes, nil
		}
		key, err := cstring(r)
		if err != nil {
			return nil, err
		}
		n := &Node{Type: t, Key: string(key)}
		switch t {
		case 0:
			n.Children, err = parseNodes(r, depth+1)
		case 1:
			n.Value, err = cstring(r)
		case 2, 3, 4, 6:
			n.Value = make([]byte, 4)
			_, err = io.ReadFull(r, n.Value)
		case 7, 10:
			n.Value = make([]byte, 8)
			_, err = io.ReadFull(r, n.Value)
		case 5:
			for {
				var pair [2]byte
				_, err = io.ReadFull(r, pair[:])
				if err != nil || pair == [2]byte{} {
					break
				}
				n.Value = append(n.Value, pair[:]...)
			}
		default:
			return nil, fmt.Errorf("不支持的 VDF 字段类型 %d，停止修改", t)
		}
		if err != nil {
			return nil, fmt.Errorf("VDF 字段 %s: %w", n.Key, err)
		}
		nodes = append(nodes, n)
	}
}

func Encode(root *Node) []byte {
	var b bytes.Buffer
	writeNode(&b, root)
	b.WriteByte(8)
	return b.Bytes()
}
func writeNode(b *bytes.Buffer, n *Node) {
	b.WriteByte(n.Type)
	b.WriteString(n.Key)
	b.WriteByte(0)
	if n.Type == 0 {
		for _, c := range n.Children {
			writeNode(b, c)
		}
		b.WriteByte(8)
		return
	}
	b.Write(n.Value)
	if n.Type == 1 {
		b.WriteByte(0)
	}
	if n.Type == 5 {
		b.Write([]byte{0, 0})
	}
}

func NewRoot() *Node { return &Node{Key: "shortcuts", Type: 0} }
func (n *Node) field(key string) *Node {
	for _, c := range n.Children {
		if c.Key == key {
			return c
		}
	}
	return nil
}
func (n *Node) text(key string) string {
	if c := n.field(key); c != nil && c.Type == 1 {
		return string(c.Value)
	}
	return ""
}
func (n *Node) number(key string) uint32 {
	if c := n.field(key); c != nil && c.Type == 2 && len(c.Value) == 4 {
		return binary.LittleEndian.Uint32(c.Value)
	}
	return 0
}
func (n *Node) setText(key, value string) { n.set(&Node{Type: 1, Key: key, Value: []byte(value)}) }
func (n *Node) setNumber(key string, value uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, value)
	n.set(&Node{Type: 2, Key: key, Value: b})
}
func (n *Node) set(value *Node) {
	for i, c := range n.Children {
		if c.Key == value.Key {
			n.Children[i] = value
			return
		}
	}
	n.Children = append(n.Children, value)
}

func AppID(exe, name string) uint32 { return crc32.ChecksumIEEE([]byte(exe+name)) | 0x80000000 }

func List(root *Node) []model.Shortcut {
	items := make([]model.Shortcut, 0, len(root.Children))
	for _, n := range root.Children {
		s := model.Shortcut{AppID: n.number("appid"), Name: n.text("AppName"), Exe: n.text("Exe"), StartDir: n.text("StartDir"), Options: n.text("LaunchOptions"), Icon: n.text("icon")}
		if tags := n.field("tags"); tags != nil {
			for _, t := range tags.Children {
				if t.Type == 1 {
					s.Tags = append(s.Tags, string(t.Value))
				}
			}
		}
		items = append(items, s)
	}
	return items
}

func Upsert(root *Node, s model.Shortcut) error {
	var n *Node
	for _, c := range root.Children {
		if c.number("appid") == s.AppID {
			if n != nil {
				return fmt.Errorf("AppID 重复，无法安全修改")
			}
			n = c
		}
	}
	if n == nil {
		n = &Node{Type: 0}
		root.Children = append(root.Children, n)
		for _, key := range []string{"IsHidden", "AllowDesktopConfig", "AllowOverlay", "OpenVR", "Devkit", "DevkitOverrideAppID", "LastPlayTime"} {
			value := uint32(0)
			if key == "AllowDesktopConfig" || key == "AllowOverlay" {
				value = 1
			}
			n.setNumber(key, value)
		}
		n.setText("ShortcutPath", "")
		n.setText("FlatpakAppID", "")
		n.setText("DevkitGameID", "")
	}
	n.setNumber("appid", s.AppID)
	n.setText("AppName", s.Name)
	n.setText("Exe", s.Exe)
	n.setText("StartDir", s.StartDir)
	n.setText("LaunchOptions", s.Options)
	n.setText("icon", s.Icon)
	tags := &Node{Type: 0, Key: "tags"}
	for i, t := range s.Tags {
		tags.Children = append(tags.Children, &Node{Type: 1, Key: strconv.Itoa(i), Value: []byte(t)})
	}
	n.set(tags)
	reindex(root)
	return nil
}

func Remove(root *Node, ids map[uint32]bool) {
	kept := root.Children[:0]
	for _, n := range root.Children {
		if !ids[n.number("appid")] {
			kept = append(kept, n)
		}
	}
	root.Children = kept
	reindex(root)
}
func reindex(root *Node) {
	for i, n := range root.Children {
		n.Key = strconv.Itoa(i)
	}
}
