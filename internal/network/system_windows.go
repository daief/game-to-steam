//go:build windows

package network

import (
	"fmt"
	"net/http"
	"net/url"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Structures follow the WinHTTP ABI; pointer alignment is supplied by Go.
type ieConfig struct {
	AutoDetect             uint32
	AutoURL, Proxy, Bypass *uint16
}
type autoOptions struct {
	Flags, Detect            uint32
	URL                      *uint16
	Reserved                 uintptr
	ReservedFlags, AutoLogon uint32
}
type proxyInfo struct {
	Access        uint32
	Proxy, Bypass *uint16
}

var winhttp = windows.NewLazySystemDLL("winhttp.dll")
var getConfig = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
var httpOpen = winhttp.NewProc("WinHttpOpen")
var httpClose = winhttp.NewProc("WinHttpCloseHandle")
var getProxy = winhttp.NewProc("WinHttpGetProxyForUrl")
var setTimeouts = winhttp.NewProc("WinHttpSetTimeouts")
var globalFree = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalFree")

func freeString(p *uint16) {
	if p != nil {
		globalFree.Call(uintptr(unsafe.Pointer(p)))
	}
}

func systemProxy(req *http.Request) (*url.URL, error) {
	if e := req.Context().Err(); e != nil {
		return nil, e
	}
	var cfg ieConfig
	ok, _, e := getConfig.Call(uintptr(unsafe.Pointer(&cfg)))
	if ok == 0 {
		return nil, fmt.Errorf("无法读取 Windows 系统代理: %w", e)
	}
	defer freeString(cfg.AutoURL)
	defer freeString(cfg.Proxy)
	defer freeString(cfg.Bypass)
	return resolveSystemConfig(req, cfg)
}

func resolveSystemConfig(req *http.Request, cfg ieConfig) (*url.URL, error) {
	if cfg.AutoURL != nil || cfg.AutoDetect != 0 {
	agent, _ := windows.UTF16PtrFromString("game-to-steam/0.2.1")
		h, _, err := httpOpen.Call(uintptr(unsafe.Pointer(agent)), 1, 0, 0, 0)
		if h == 0 {
			return nil, fmt.Errorf("无法初始化系统代理解析: %w", err)
		}
		defer httpClose.Call(h)
		setTimeouts.Call(h, 5000, 5000, 5000, 5000)
		options := autoOptions{}
		if cfg.AutoURL != nil {
			options.Flags = 2
			options.URL = cfg.AutoURL
		} else {
			options.Flags = 1
			options.Detect = 3
		}
		target, err := windows.UTF16PtrFromString(req.URL.String())
		if err != nil {
			return nil, err
		}
		var info proxyInfo
		ok, _, err := getProxy.Call(h, uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&info)))
		defer freeString(info.Proxy)
		defer freeString(info.Bypass)
		if e := req.Context().Err(); e != nil {
			return nil, e
		}
		if ok != 0 {
			if info.Access == 1 {
				return nil, nil
			}
			return proxyFor(req.URL, windows.UTF16PtrToString(info.Proxy), windows.UTF16PtrToString(info.Bypass))
		}
		// Automatically detecting settings is commonly enabled with no WPAD
		// server. Only that specific absence falls back to the manual setting.
		if cfg.AutoURL != nil || err != syscall.Errno(12180) {
			return nil, fmt.Errorf("系统自动代理解析失败: %w", err)
		}
	}
	return proxyFor(req.URL, windows.UTF16PtrToString(cfg.Proxy), windows.UTF16PtrToString(cfg.Bypass))
}
