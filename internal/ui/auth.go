package ui

import (
	"context"
	"strings"

	"game-to-steam/internal/epicauth"
	"game-to-steam/internal/system"
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) epicAuthSettings(c *nativeui.Context) {

	nativeui.Column(c).WidthPercent(100).Padding(10).Gap(6).Background(studioList).Border(1, c.Theme().Border).Radius(4).Children(func() {
		nativeui.Text(c, a.snapshot.EpicAuth.Label()).FontSize(12).Bold().TextColor(c.Theme().Accent)
	})
	nativeui.Text(c, "游戏更新使用官方客户端；不支持云存档同步。").FontSize(12).WidthPercent(100)
	nativeui.Row(c).WidthPercent(100).Gap(8).AlignItems(nativeui.Center).Wrap().Children(func() {
		nativeui.PrimaryButton(c, "登录 / 更换 Epic 账号").Height(30).FontSize(11).Disabled(a.busy).OnClick(func() { a.epicAuthCode = ""; a.epicAuthOpen = true })
		nativeui.Button(c, "刷新认证状态").Height(30).FontSize(11).Disabled(a.busy).OnClick(func() {
			a.run("读取 Epic 认证状态", func(ctx context.Context) (func(), error) {
				status := a.svc.EpicAuthStatus()
				return func() { a.snapshot.EpicAuth = status }, nil
			})
		})
		nativeui.Box(c).Grow(1)
		nativeui.Button(c, "移除桥接登录").Height(26).FontSize(11).TextColor(c.Theme().Danger).Background(c.Theme().Surface).BorderWidth(0).Disabled(a.busy || a.snapshot.EpicAuth.State == "missing" || a.snapshot.EpicAuth.State == "").OnClick(func() { a.epicForgetOpen = true })
	})
}

func (a *app) epicAuthDialog(c *nativeui.Context) {
	nativeui.Modal(c.Key("epic-auth-dialog"), &a.epicAuthOpen, func() {
		nativeui.Text(c, "登录 Epic 独立认证").FontSize(20).Bold()
		nativeui.Text(c, "使用拥有游戏的 Epic 账号登录").FontSize(12).Width(dialogWidth(c))
		nativeui.Button(c, "打开 Epic 登录网页").Disabled(a.busy).OnClick(func() {
			// Only a fixed Epic URL is opened; user tokens never enter a shell command.
			if err := openEpicLogin(); err != nil {
				a.status = "Epic 登录失败：" + err.Error()
			}
		})
		nativeui.Text(c, "授权码（authorizationCode 或完整 JSON 响应）").FontSize(12).Width(dialogWidth(c))
		nativeui.TextInput(c, &a.epicAuthCode).Password().Label("Epic 授权码").Width(dialogWidth(c)).Disabled(a.busy)
		if strings.HasPrefix(a.status, "Epic 独立认证失败：") || strings.HasPrefix(a.status, "Epic 登录失败：") {
			nativeui.Text(c, a.status).FontSize(12).Width(dialogWidth(c)).TextColor(c.Theme().Danger)
		}
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "取消登录").Disabled(a.busy).OnClick(func() { a.epicAuthCode = ""; a.epicAuthOpen = false })
			nativeui.PrimaryButton(c, "确认 Epic 登录").Disabled(a.busy || strings.TrimSpace(a.epicAuthCode) == "").OnClick(func() {
				if a.busy || strings.TrimSpace(a.epicAuthCode) == "" {
					return
				}
				code := a.epicAuthCode
				a.epicAuthCode = ""
				a.run("Epic 独立认证", func(ctx context.Context) (func(), error) {
					err := a.svc.LoginEpic(ctx, code)
					code = ""
					status := a.svc.EpicAuthStatus()
					return func() {
						a.snapshot.EpicAuth = status
						if err == nil {
							a.epicAuthOpen = false
						}
					}, err
				})
			})
		})
	})
}

func (a *app) epicForgetDialog(c *nativeui.Context) {
	nativeui.Modal(c.Key("epic-forget-dialog"), &a.epicForgetOpen, func() {
		nativeui.Text(c, "移除桥接登录？").FontSize(20).Bold()
		nativeui.Text(c, "只删除本工具保存的独立认证，不注销官方 Epic、不删除游戏或快捷方式。之后认证直启需要重新登录。").Width(dialogWidth(c))
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "保留登录").Disabled(a.busy).OnClick(func() { a.epicForgetOpen = false })
			nativeui.Button(c, "确认移除登录").Disabled(a.busy).OnClick(func() {
				a.run("移除 Epic 独立认证", func(ctx context.Context) (func(), error) {
					err := a.svc.ForgetEpic(ctx)
					status := a.svc.EpicAuthStatus()
					return func() {
						a.snapshot.EpicAuth = status
						if err == nil {
							a.epicForgetOpen = false
						}
					}, err
				})
			})
		})
	})
}

// Kept separate from rendering, so previews/tests never open a browser.
func openEpicLogin() error { return system.Open(epicauth.LoginURL()) }
