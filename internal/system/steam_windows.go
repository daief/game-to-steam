package system

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func steamExitCommand(root string) (*exec.Cmd, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("Steam 安装目录必须是绝对路径")
	}
	return exec.Command(filepath.Join(root, "steam.exe"), "-shutdown"), nil
}

func ControlSteam(root string, quit bool) error {
	cmd, err := steamExitCommand(root)
	if err != nil {
		return err
	}
	if !quit {
		return Open(cmd.Path)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("请求 Steam 退出失败：%w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
