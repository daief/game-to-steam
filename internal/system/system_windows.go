//go:build windows

// Package system contains the Windows-specific discovery and credential APIs.
package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func regString(root registry.Key, path, name string) string {
	k, e := registry.OpenKey(root, path, registry.READ)
	if e != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue(name)
	return v
}
func NormalizePath(p string) string {
	p = strings.TrimSpace(strings.Trim(strings.TrimSpace(p), `"`))
	if p == "" {
		return ""
	}
	p = filepath.Clean(strings.ReplaceAll(p, "/", `\`))
	if len(p) >= 2 && p[1] == ':' && p[0] >= 'a' && p[0] <= 'z' {
		p = strings.ToUpper(p[:1]) + p[1:]
	}
	return p
}
func SteamPath() string {
	return NormalizePath(regString(registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"))
}
func EpicManifestPath() string {
	p := regString(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Epic Games\EpicGamesLauncher`, "AppDataPath")
	if p != "" {
		return filepath.Join(p, "Manifests")
	}
	return ""
}
func EpicLauncherPath() string {
	// The URI handler records custom installations even when InstallLocation
	// is absent. Read only; never execute the registry command.
	for _, root := range []registry.Key{registry.CLASSES_ROOT, registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key := `com.epicgames.launcher\shell\open\command`
		if root != registry.CLASSES_ROOT {
			key = `SOFTWARE\Classes\` + key
		}
		candidate := launcherFromCommand(regString(root, key, ""))
		if info, e := os.Stat(candidate); candidate != "" && e == nil && !info.IsDir() {
			return candidate
		}
	}
	p := regString(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Epic Games\EpicGamesLauncher`, "InstallLocation")
	for _, base := range []string{p, filepath.Join(os.Getenv("ProgramFiles(x86)"), "Epic Games", "Launcher"), filepath.Join(os.Getenv("ProgramFiles"), "Epic Games", "Launcher")} {
		if base == "" {
			continue
		}
		for _, arch := range []string{"Win64", "Win32"} {
			exe := filepath.Join(base, "Portal", "Binaries", arch, "EpicGamesLauncher.exe")
			if stat, e := os.Stat(exe); e == nil && !stat.IsDir() {
				return exe
			}
		}
	}
	return ""
}

func launcherFromCommand(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) {
		end := strings.Index(command[1:], `"`)
		if end < 0 {
			return ""
		}
		command = command[1 : end+1]
	} else {
		end := strings.Index(strings.ToLower(command), ".exe")
		if end < 0 {
			return ""
		}
		command = command[:end+4]
	}
	path := NormalizePath(command)
	if !strings.EqualFold(filepath.Base(path), "EpicGamesLauncher.exe") {
		return ""
	}
	return path
}

func RequireSteamStopped() error {
	h, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return fmt.Errorf("无法检查 Steam 进程: %w", e)
	}
	defer windows.CloseHandle(h)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	e = windows.Process32First(h, &entry)
	for e == nil {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "steam.exe") {
			return fmt.Errorf("Steam 正在运行。请在 Steam 菜单中选择退出，退出后重试")
		}
		e = windows.Process32Next(h, &entry)
	}
	if !errors.Is(e, windows.ERROR_NO_MORE_FILES) {
		return fmt.Errorf("无法完成进程检查: %w", e)
	}
	return nil
}

func Open(path string) error {
	op, _ := windows.UTF16PtrFromString("open")
	target, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	return windows.ShellExecute(0, op, target, nil, nil, windows.SW_SHOWNORMAL)
}

const credentialTarget = "game-to-steam/SteamGridDB"

type credential struct {
	Flags, Type             uint32
	TargetName, Comment     *uint16
	LastWritten             windows.Filetime
	CredentialBlobSize      uint32
	CredentialBlob          *byte
	Persist, AttributeCount uint32
	Attributes              uintptr
	TargetAlias, UserName   *uint16
}

var advapi = windows.NewLazySystemDLL("advapi32.dll")
var credRead = advapi.NewProc("CredReadW")
var credWrite = advapi.NewProc("CredWriteW")
var credDelete = advapi.NewProc("CredDeleteW")
var credFree = advapi.NewProc("CredFree")

func ReadKey() (string, error) {
	target, _ := windows.UTF16PtrFromString(credentialTarget)
	var ptr *credential
	ok, _, e := credRead.Call(uintptr(unsafe.Pointer(target)), 1, 0, uintptr(unsafe.Pointer(&ptr)))
	if ok == 0 {
		if errors.Is(e, windows.ERROR_NOT_FOUND) {
			return "", nil
		}
		return "", fmt.Errorf("无法读取 Windows 凭据: %w", e)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(ptr)))
	return string(unsafe.Slice(ptr.CredentialBlob, ptr.CredentialBlobSize)), nil
}
func SaveKey(key string) error {
	target, _ := windows.UTF16PtrFromString(credentialTarget)
	if key == "" {
		ok, _, e := credDelete.Call(uintptr(unsafe.Pointer(target)), 1, 0)
		if ok == 0 && !errors.Is(e, windows.ERROR_NOT_FOUND) {
			return fmt.Errorf("无法删除 Windows 凭据: %w", e)
		}
		return nil
	}
	if len(key) > 2560 {
		return fmt.Errorf("API Key 过长")
	}
	data := []byte(key)
	user, _ := windows.UTF16PtrFromString("SteamGridDB")
	c := credential{Type: 1, TargetName: target, CredentialBlobSize: uint32(len(data)), CredentialBlob: &data[0], Persist: 2, UserName: user}
	ok, _, e := credWrite.Call(uintptr(unsafe.Pointer(&c)), 0)
	if ok == 0 {
		return fmt.Errorf("无法保存 Windows 凭据: %w", e)
	}
	return nil
}
