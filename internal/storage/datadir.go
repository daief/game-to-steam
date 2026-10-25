package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir migrates legacy data before either the client or bridge opens it.
func DataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Game2Steam")
	if err := migrateDir(filepath.Join(base, "game-to-steam"), dir); err != nil {
		return "", fmt.Errorf("迁移旧数据目录失败: %w", err)
	}
	return dir, nil
}

func migrateDir(old, dest string) error {
	entries, err := os.ReadDir(old)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(old, entry.Name())
		dst := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := migrateDir(src, dst); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Lstat(dst); err == nil {
			if entry.Name() != "window-state.json" {
				return fmt.Errorf("新旧目录存在同名文件，请先处理冲突: %s", dst)
			}
			dst = filepath.Join(dest, "window-state.legacy.json")
			if _, err := os.Lstat(dst); !os.IsNotExist(err) {
				return fmt.Errorf("旧窗口状态备份已存在或不可访问: %s", dst)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return os.Remove(old)
}
