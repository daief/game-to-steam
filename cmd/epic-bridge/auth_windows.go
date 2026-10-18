//go:build windows

package main

import (
	"context"
	"fmt"
	"time"

	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/network"
	"game-to-steam/internal/storage"
	"game-to-steam/internal/system"
)

func authClient(appDir string) (*epicauth.Client, error) {
	state, err := storage.Load(appDir)
	if err != nil {
		return nil, err
	}
	transport, err := network.Transport(state.Settings.Network)
	if err != nil {
		return nil, err
	}
	return epicauth.New(transport), nil
}

func authAction(ctx context.Context, appDir string, store epicauth.Store, login, status bool) error {
	if login {
		fmt.Println("独立桥接登录：不会导入或改写 Epic 客户端登录信息。请确认网页中是拥有游戏的账号。")
		fmt.Println("请在浏览器完成登录，然后复制 JSON 响应中的 authorizationCode（不要分享给他人）。")
		u := epicauth.LoginURL()
		fmt.Println(u)
		if err := system.Open(u); err != nil {
			fmt.Println("无法自动打开浏览器，请手动访问上面的 Epic 链接。")
		}
		code, err := epicauth.ReadCode()
		if err != nil {
			return err
		}
		client, err := authClient(appDir)
		if err != nil {
			return err
		}
		unlock, err := store.Lock(ctx)
		if err != nil {
			return err
		}
		defer unlock()
		s, err := client.Login(ctx, code)
		if err != nil {
			return err
		}
		if err = store.Save(s); err != nil {
			return err
		}
		fmt.Printf("桥接登录成功：%s；会话有效期至 %s\n", s.DisplayName, s.ExpiresAt.Local().Format(time.RFC3339))
		return nil
	}
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if status {
		s, err := store.Load()
		if err != nil {
			return err
		}
		fmt.Printf("桥接账号：%s\n访问令牌有效期：%s\n刷新令牌有效期：%s\n", s.DisplayName, s.ExpiresAt.Local().Format(time.RFC3339), s.RefreshExpiresAt.Local().Format(time.RFC3339))
		return nil
	}
	if err = store.Forget(); err != nil {
		return err
	}
	fmt.Println("已删除桥接本地认证；未注销 Epic 客户端。")
	return nil
}
