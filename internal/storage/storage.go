// Package storage keeps settings, associations and recoverable write backups.
package storage

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"game-to-steam/internal/model"
)

func Load(dir string) (model.State, error) {
	s := model.State{Accounts: map[string]map[string]model.ManagedGame{}}
	b, e := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(e, os.ErrNotExist) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("本地状态损坏，停止修改: %w", e)
	}
	if s.Accounts == nil {
		s.Accounts = map[string]map[string]model.ManagedGame{}
	}
	return s, nil
}
func StateBytes(s model.State) ([]byte, error) { return json.MarshalIndent(s, "", "  ") }

type Snapshot struct {
	Path   string
	Data   []byte
	Exists bool
}

func Read(path string) (Snapshot, error) {
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return Snapshot{Path: path}, nil
	}
	if e != nil {
		return Snapshot{}, e
	}
	return Snapshot{Path: path, Data: b, Exists: true}, nil
}
func (s Snapshot) Unchanged() error {
	current, e := Read(s.Path)
	if e != nil {
		return e
	}
	if current.Exists != s.Exists || sha256.Sum256(current.Data) != sha256.Sum256(s.Data) {
		return fmt.Errorf("文件已被外部修改，请刷新后重试: %s", s.Path)
	}
	return nil
}

type Change struct {
	Before Snapshot
	Data   []byte
	Delete bool
}
type backupRecord struct {
	Path, File string
	Exists     bool
}

// Commit backs up every target before touching one. preflight is called again
// immediately before replacement, to reject a newly started Steam process.
func Commit(backupRoot string, changes []Change, preflight func() error) (string, error) {
	return commit(backupRoot, changes, preflight, Replace)
}

func commit(backupRoot string, changes []Change, preflight func() error, replace func(string, []byte) error) (string, error) {
	if len(changes) == 0 {
		return "", nil
	}
	if preflight != nil {
		if e := preflight(); e != nil {
			return "", e
		}
	}
	for _, c := range changes {
		if e := c.Before.Unchanged(); e != nil {
			return "", e
		}
	}
	dir, e := os.MkdirTemp(backupRoot, time.Now().Format("20060102-150405")+"-")
	if errors.Is(e, os.ErrNotExist) {
		if e = os.MkdirAll(backupRoot, 0700); e != nil {
			return "", e
		}
		dir, e = os.MkdirTemp(backupRoot, time.Now().Format("20060102-150405")+"-")
	}
	if e != nil {
		return "", e
	}
	records := []backupRecord{}
	for i, c := range changes {
		file := fmt.Sprintf("%03d.bin", i)
		if c.Before.Exists {
			if e = os.WriteFile(filepath.Join(dir, file), c.Before.Data, 0600); e != nil {
				return dir, e
			}
		}
		records = append(records, backupRecord{c.Before.Path, file, c.Before.Exists})
	}
	manifest, e := json.MarshalIndent(records, "", "  ")
	if e != nil {
		return dir, e
	}
	if e = os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0600); e != nil {
		return dir, e
	}
	if preflight != nil {
		if e = preflight(); e != nil {
			return dir, e
		}
	}
	for _, c := range changes {
		if e = c.Before.Unchanged(); e != nil {
			return dir, e
		}
	}
	for i, c := range changes {
		if c.Delete {
			if c.Before.Exists {
				e = os.Remove(c.Before.Path)
			}
		} else {
			e = replace(c.Before.Path, c.Data)
		}
		if e != nil {
			original := e
			var restoreErr error
			for j := i; j >= 0; j-- {
				b := changes[j].Before
				if b.Exists {
					restoreErr = errors.Join(restoreErr, Replace(b.Path, b.Data))
				} else {
					err := os.Remove(b.Path)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						restoreErr = errors.Join(restoreErr, err)
					}
				}
			}
			return dir, fmt.Errorf("写入失败: %w；恢复结果: %v；备份: %s", original, restoreErr, dir)
		}
	}
	return dir, nil
}

func Replace(path string, data []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".game-to-steam-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return replaceFile(tmp, path)
}
