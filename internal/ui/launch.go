package ui

import (
	"context"
	"strings"

	"game-to-steam/internal/model"
	nativeui "github.com/egoist/mygo/ui"
)

func (a *app) openLaunch(id string) {
	m, ok := a.snapshot.Managed[id]
	if !ok {
		return
	}
	for _, sc := range a.snapshot.Shortcuts {
		if sc.AppID == m.AppID {
			a.launchID = id
			a.launchOriginal = sc
			a.launchDraft = model.LaunchSettings{ForceEpic: m.Game.ForceEpic, Exe: sc.Exe, StartDir: sc.StartDir, Options: sc.Options}
			a.launchGeneratedForce = m.Game.ForceEpic
			a.launchOpen = true
			return
		}
	}
	a.status = "快捷方式已不存在，请刷新"
}

func (a *app) launchDialog(c *nativeui.Context) {
	nativeui.Modal(c.Key("launch-dialog"), &a.launchOpen, func() {
		nativeui.Text(c, "启动设置："+a.launchOriginal.Name).FontSize(20).Bold()
		nativeui.Text(c, "编辑的是当前 Steam 快捷方式，AppID、名称、图片和标签保持原值。").FontSize(12)
		nativeui.Checkbox(c, &a.launchDraft.ForceEpic, "强制开启 Epic 认证（当前快捷方式）").Disabled(a.busy)
		nativeui.Button(c, "按策略生成命令").Disabled(a.busy).OnClick(func() {
			id, force := a.launchID, a.launchDraft.ForceEpic
			a.run("生成启动命令草稿", func(ctx context.Context) (func(), error) {
				draft, err := a.svc.PreviewLaunch(id, force)
				return func() {
					if err == nil {
						a.launchDraft = draft
						a.launchGeneratedForce = force
					}
				}, err
			})
		})
		nativeui.Text(c, "生成会替换草稿。需要认证或勾选强制时通过桥接启动，否则直启 EXE；取消强制仍按清单判断。").FontSize(12).Width(680)
		nativeui.Text(c, "启动程序（路径含空格时用双引号包裹）")
		nativeui.TextInput(c, &a.launchDraft.Exe).Label("启动程序").Width(680).Disabled(a.busy)
		nativeui.Text(c, "工作目录")
		nativeui.TextInput(c, &a.launchDraft.StartDir).Label("工作目录").Width(680).Disabled(a.busy)
		nativeui.Text(c, "启动参数（可修改或添加 flag）")
		nativeui.TextInput(c, &a.launchDraft.Options).Label("启动参数").Width(680).Disabled(a.busy)
		nativeui.Text(c, "认证桥接支持 --watch-exe、--startup-timeout、--exit-grace；游戏 flag 放在 -- 之后，例如 -- -windowed。裸 EXE 参数直接传给游戏。").FontSize(12).Width(680)
		nativeui.Text(c, "桥接需先在设置中登录 Epic，不启动官方客户端，也不提供云存档同步。").FontSize(12).Width(680)
		pending := a.launchGeneratedForce != a.launchDraft.ForceEpic
		if pending {
			nativeui.Text(c, "强制选项已改变，请先按策略生成命令，再调整参数。").FontSize(12)
		}
		nativeui.Text(c, "保存前请退出 Steam。仅修改草稿不会影响 Steam；保存失败时草稿会保留。").FontSize(12)
		nativeui.Text(c, a.status).FontSize(12).Width(680)
		nativeui.Row(c).Gap(12).Children(func() {
			nativeui.Button(c, "取消编辑").Disabled(a.busy).OnClick(func() { a.launchOpen = false })
			nativeui.PrimaryButton(c, "保存启动设置").Disabled(a.busy || pending || strings.TrimSpace(a.launchDraft.Exe) == "").OnClick(func() {
				id, draft, original := a.launchID, a.launchDraft, a.launchOriginal
				a.run("保存启动设置", func(ctx context.Context) (func(), error) {
					err := a.svc.SaveLaunch(id, draft, original)
					if err != nil {
						return nil, err
					}
					snap := a.svc.Refresh()
					return func() { a.accept(snap); a.launchOpen = false }, nil
				})
			})
		})
	})
}
