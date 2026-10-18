//go:build windows

package epicauth

import (
	"fmt"
	"strings"

	"game-to-steam/internal/model"
	"golang.org/x/sys/windows"
)

func GameArguments(g model.Game, s Session, code, ownershipPath, locale string, extra ...string) ([]string, error) {
	if code == "" || !validID(g.AppName) || !validID(g.Namespace) || !validID(s.AccountID) || strings.ContainsAny(s.DisplayName+code+ownershipPath, "\x00\r\n") {
		return nil, fmt.Errorf("启动认证字段无效")
	}
	if len(locale) > 85 || locale == "" || strings.ContainsAny(locale, " \t\r\n\x00") {
		return nil, fmt.Errorf("语言参数无效")
	}
	args, err := windows.DecomposeCommandLine("game " + g.LaunchCommand)
	if err != nil {
		return nil, fmt.Errorf("游戏清单启动参数无效")
	}
	args = args[1:]
	args = append(args, extra...)
	for _, a := range args {
		if strings.ContainsAny(a, "\x00\r\n") {
			return nil, fmt.Errorf("游戏参数包含无效字符")
		}
		key, _, _ := strings.Cut(strings.ToLower(strings.TrimLeft(a, "-/")), "=")
		if strings.HasPrefix(key, "auth_") || strings.HasPrefix(key, "epic") {
			return nil, fmt.Errorf("清单参数包含认证字段，停止启动以避免覆盖")
		}
	}
	args = append(args, "-AUTH_LOGIN=unused", "-AUTH_PASSWORD="+code, "-AUTH_TYPE=exchangecode", "-epicapp="+g.AppName, "-epicenv=Prod", "-EpicPortal", "-epicusername="+s.DisplayName, "-epicuserid="+s.AccountID, "-epiclocale="+locale, "-epicsandboxid="+g.Namespace)
	if g.RequiresOwnershipToken && ownershipPath == "" {
		return nil, fmt.Errorf("游戏需要所有权令牌")
	}
	if ownershipPath != "" {
		args = append(args, "-epicovt="+ownershipPath)
	}
	return args, nil
}
