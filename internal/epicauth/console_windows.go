//go:build windows

package epicauth

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ReadCode only accepts an interactive console, with echo disabled (no shell history).
func ReadCode() (string, error) {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return "", fmt.Errorf("请在交互式 PowerShell 终端执行 --auth-login")
	}
	if err := windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return "", err
	}
	defer windows.SetConsoleMode(h, mode)
	fmt.Fprint(os.Stderr, "粘贴 authorizationCode 或 JSON 响应（不回显），按 Enter：")
	defer fmt.Fprintln(os.Stderr)
	var all []uint16
	for len(all) < 16384 {
		buf := make([]uint16, 1024)
		var n uint32
		if err := windows.ReadConsole(h, &buf[0], uint32(len(buf)), &n, nil); err != nil {
			return "", fmt.Errorf("读取授权码失败")
		}
		if n == 0 {
			return "", fmt.Errorf("未输入授权码")
		}
		all = append(all, buf[:n]...)
		if strings.ContainsRune(string(utf16.Decode(buf[:n])), '\n') {
			break
		}
	}
	defer clear(all)
	if len(all) >= 16384 {
		return "", fmt.Errorf("授权响应过长")
	}
	return ParseCode(string(utf16.Decode(all)))
}

func DefaultLocale() string {
	var buf [85]uint16
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return "en-US"
	}
	return windows.UTF16ToString(buf[:])
}
