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
		nativeui.Text(c, "凭据由 Windows DPAPI 加密保存，启动时刷新会话。").FontSize(11).TextColor(c.Theme().TextMuted).WidthPercent(100)
	})
	nativeui.Text(c, "游戏更新仍由官方客户端管理，不提供云存档同步。").FontSize(12).WidthPercent(100)
	nativeui.Row(c).Gap(8).Wrap().Children(func() {
		nativeui.PrimaryButton(c, "登录 / 更换 Epic 账号").Height(30).FontSize(11).Disabled(a.busy).OnClick(func() { a.epicAuthCode = ""; a.epicAuthOpen = true })
		nativeui.Button(c, "刷新认证状态").Height(30).FontSize(11).Disabled(a.busy).OnClick(func() {
			a.run("读取 Epic 认证状态", func(ctx context.Context) (func(), error) {
				status := a.svc.EpicAuthStatus()
				return func() { a.snapshot.EpicAuth = status }, nil
			})
		})
	})
	nativeui.Row(c).WidthPercent(100).Justify(nativeui.End).Children(func() {
		nativeui.Button(c, "移除桥接登录").Height(26).FontSize(11).TextColor(c.Theme().Danger).Background(c.Theme().Surface).BorderWidth(0).Disabled(a.busy || a.snapshot.EpicAuth.State == "missing" || a.snapshot.EpicAuth.State == "").OnClick(func() { a.epicForgetOpen = true })
	})
}

func (a *app) epicAuthDialog(c *nativeui.Context) {
	nativeui.Modal(c.Key("epic-auth-dialog"), &a.epicAuthOpen, func() {
		nativeui.Text(c, "登录 Epic 独立认证").FontSize(20).Bold()
		nativeui.Text(c, "1. 在浏览器登录拥有游戏的 Epic 账号，可复用浏览器现有登录。").FontSize(12).Width(680)
		nativeui.Button(c, "打开 Epic 登录网页").Disabled(a.busy).OnClick(func() {
			// Only a fixed Epic URL is opened; user tokens never enter a shell command.
			if err := openEpicLogin(); err != nil {
				a.status = err.Error()
			}
		})
		nativeui.Text(c, "2. 粘贴网页返回的 authorizationCode 或完整 JSON 响应，再确认登录。").FontSize(12).Width(680)
		nativeui.TextInput(c, &a.epicAuthCode).Password().Label("Epic 授权码").Width(680).Disabled(a.busy)
		nativeui.Text(c, "授权码不写入日志或 Steam；登录信息由当前 Windows 用户加密保存。不影响官方客户端的本地配置。").FontSize(12).Width(680)
		nativeui.Text(c, "使用已保存的网络设置；需要调整代理时先取消并保存设置。").FontSize(12).Width(680)
		nativeui.Text(c, a.status).FontSize(12).Width(680)
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
		nativeui.Text(c, "只删除本工具保存的独立认证，不注销官方 Epic、不删除游戏或快捷方式。之后认证直启需要重新登录。").Width(600)
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
