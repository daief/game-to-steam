//go:build windows

package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Native struct{ Log Logger }

var user32 = windows.NewLazySystemDLL("user32.dll")
var getShellWindow = user32.NewProc("GetShellWindow")
var getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")

func desktopPID() (uint32, error) {
	hwnd, _, _ := getShellWindow.Call()
	if hwnd == 0 {
		return 0, fmt.Errorf("找不到桌面 Shell；需要普通用户的交互桌面")
	}
	var pid uint32
	ok, _, err := getWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if ok == 0 || pid == 0 {
		return 0, fmt.Errorf("读取桌面进程失败: %v", err)
	}
	return pid, nil
}

func cleanEnvironment(env []string) []string {
	result := make([]string, 0, len(env))
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		// Do not pass Steam's session/app identifiers to the launcher.
		if !strings.HasPrefix(strings.ToLower(key), "steam") {
			result = append(result, value)
		}
	}
	return result
}

func startDesktop(exe string, args []string) (uint32, uint32, error) {
	pid, err := desktopPID()
	if err != nil {
		return 0, 0, err
	}
	var ours, theirs uint32
	if err = windows.ProcessIdToSessionId(uint32(os.Getpid()), &ours); err != nil {
		return 0, 0, err
	}
	if err = windows.ProcessIdToSessionId(pid, &theirs); err != nil {
		return 0, 0, err
	}
	if ours != theirs {
		return 0, 0, fmt.Errorf("桌面 Shell 与桥接不属于同一会话")
	}
	h, err := windows.OpenProcess(windows.PROCESS_CREATE_PROCESS, false, pid)
	if err != nil {
		return 0, 0, fmt.Errorf("无法使用桌面进程作为父进程: %w", err)
	}
	defer windows.CloseHandle(h)
	child, processHandle, err := syscall.StartProcess(exe, append([]string{exe}, args...), &syscall.ProcAttr{
		Dir: filepath.Dir(exe), Env: cleanEnvironment(os.Environ()),
		// Windows requires exactly three stdio entries, even without inheritance.
		Files: []uintptr{0, 0, 0},
		Sys:   &syscall.SysProcAttr{ParentProcess: syscall.Handle(h), NoInheritHandles: true, CreationFlags: windows.CREATE_NO_WINDOW},
	})
	if err != nil {
		return 0, 0, err
	}
	// Windows has no zombie processes; releasing our handle does not stop Epic.
	err = windows.CloseHandle(windows.Handle(processHandle))
	return uint32(child), pid, err
}

func (n Native) Launch(launcher, uri string) (uint32, error) {
	if !strings.EqualFold(filepath.Base(launcher), "EpicGamesLauncher.exe") || !filepath.IsAbs(launcher) {
		return 0, fmt.Errorf("需要 EpicGamesLauncher.exe 的绝对路径")
	}
	if !strings.HasPrefix(uri, "com.epicgames.launcher://apps/") || strings.ContainsAny(uri, "\x00\r\n") {
		return 0, fmt.Errorf("无效的 Epic 游戏启动 URI")
	}
	pid, parent, err := startDesktop(launcher, []string{uri})
	if err == nil {
		n.Log.Printf("desktop-launch parent=%d child=%d (Steam environment removed)", parent, pid)
	}
	return pid, err
}

func (n Native) Snapshot(names []string) ([]Process, error) {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil {
		return nil, err
	}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	var result []Process
	for err = windows.Process32First(h, &entry); err == nil; err = windows.Process32Next(h, &entry) {
		match := false
		for _, name := range names {
			if strings.EqualFold(name, windows.UTF16ToString(entry.ExeFile[:])) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		var candidateSession uint32
		if e := windows.ProcessIdToSessionId(entry.ProcessID, &candidateSession); e != nil {
			if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
				continue
			}
			return nil, fmt.Errorf("无法确认候选进程会话 pid=%d: %w", entry.ProcessID, e)
		}
		if candidateSession != session {
			continue
		}
		p, e := inspect(entry)
		if errors.Is(e, windows.ERROR_INVALID_PARAMETER) {
			continue
		} // exited during enumeration
		if e != nil {
			return nil, fmt.Errorf("无法读取候选进程 pid=%d（请勿以管理员运行游戏）: %w", entry.ProcessID, e)
		}
		result = append(result, p)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return result, nil
}

func inspect(entry windows.ProcessEntry32) (Process, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
	if err != nil {
		return Process{}, err
	}
	defer windows.CloseHandle(h)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err = windows.QueryFullProcessImageName(h, 0, &buffer[0], &size); err != nil {
		return Process{}, err
	}
	var creation, exit, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return Process{}, err
	}
	return Process{PID: entry.ProcessID, ParentPID: entry.ParentProcessID, Path: windows.UTF16ToString(buffer[:size]), Created: uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)}, nil
}
