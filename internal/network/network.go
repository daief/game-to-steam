// Package network gives all artwork requests an explicit, shared proxy policy.
package network

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"game-to-steam/internal/model"
)

const (
	Direct = "direct"
	System = "system"
	Custom = "custom"
)

func Normalize(s model.NetworkSettings) model.NetworkSettings {
	if s.Mode == "" {
		s.Mode = System
	}
	s.ProxyURL = strings.TrimSpace(s.ProxyURL)
	return s
}
func Validate(s model.NetworkSettings) error {
	s = Normalize(s)
	switch s.Mode {
	case Direct, System:
		return nil
	case Custom:
		_, e := parseProxy(s.ProxyURL)
		return e
	}
	return fmt.Errorf("未知网络模式")
}
func parseProxy(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("代理地址格式不正确，例如 http://127.0.0.1:7890")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("代理地址需使用 http、https、socks5 或 socks5h")
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("代理端口必须为 1–65535")
		}
	}
	return u, nil
}
func Transport(s model.NetworkSettings) (*http.Transport, error) {
	s = Normalize(s)
	if e := Validate(s); e != nil {
		return nil, e
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	switch s.Mode {
	case Direct:
		t.Proxy = nil
	case Custom:
		u, _ := parseProxy(s.ProxyURL)
		t.Proxy = http.ProxyURL(u)
	case System:
		t.Proxy = systemProxy
	}
	return t, nil
}

// Windows lists may specify per-protocol servers. A HTTPS request through an
// HTTP proxy uses CONNECT; https= does not mean the proxy itself speaks TLS.
func proxyFor(target *url.URL, servers, bypass string) (*url.URL, error) {
	if bypassHost(target, bypass) {
		return nil, nil
	}
	var fallback, selected string
	for _, entry := range strings.FieldsFunc(servers, func(r rune) bool { return r == ';' || r == ' ' || r == '\t' }) {
		if k, v, ok := strings.Cut(entry, "="); ok {
			if strings.EqualFold(k, target.Scheme) {
				selected = v
				break
			}
			if strings.EqualFold(k, "socks") {
				fallback = "socks5://" + v
			}
		} else if fallback == "" {
			fallback = entry
		}
	}
	if selected == "" {
		selected = fallback
	}
	if selected == "" {
		return nil, nil
	}
	if !strings.Contains(selected, "://") {
		selected = "http://" + selected
	}
	return parseProxy(selected)
}
func bypassHost(target *url.URL, list string) bool {
	host := strings.ToLower(target.Hostname())
	full := strings.ToLower(target.Host)
	for _, rule := range strings.Split(list, ";") {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if rule == "<local>" {
			if !strings.Contains(host, ".") && net.ParseIP(host) == nil {
				return true
			}
			continue
		}
		if ok, _ := path.Match(rule, host); ok {
			return true
		}
		if ok, _ := path.Match(rule, full); ok {
			return true
		}
	}
	return false
}
