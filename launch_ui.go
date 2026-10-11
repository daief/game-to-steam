package main

import (
	"context"
	"strings"

	"game-to-steam/internal/model"
	"github.com/egoist/mygo/ui"
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

func (a *app) launchDialog(c *ui.Context) {
	ui.Modal(c.Key("launch-dialog"), &a.launchOpen, func() {
		ui.Text(c, "启动设置："+a.launchOriginal.Name).FontSize(20).Bold()
		ui.Text(c, "编辑的是当前 Steam 快捷方式，AppID、名称、图片和标签保持原值。").FontSize(12)
		ui.Checkbox(c, &a.launchDraft.ForceEpic, "强制通过 Epic 启动（当前快捷方式）").Disabled(a.busy)
		ui.Button(c, "按策略生成命令").Disabled(a.busy).OnClick(func() {
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
		ui.Text(c, "生成会替换下面的命令草稿，请在生成后添加特殊 flag。取消强制后仍按 managed / DLC 自动判断。").FontSize(12).Width(680)
		ui.Text(c, "启动程序（路径含空格时用双引号包裹）")
		ui.TextInput(c, &a.launchDraft.Exe).Label("启动程序").Width(680).Disabled(a.busy)
		ui.Text(c, "工作目录")
		ui.TextInput(c, &a.launchDraft.StartDir).Label("工作目录").Width(680).Disabled(a.busy)
		ui.Text(c, "启动参数（可修改或添加 flag）")
		ui.TextInput(c, &a.launchDraft.Options).Label("启动参数").Width(680).Disabled(a.busy)
		ui.Text(c, "参数传给当前启动程序；使用 Epic 启动器时，游戏是否接收这些 flag 取决于启动器支持。").FontSize(12).Width(680)
		pending := a.launchGeneratedForce != a.launchDraft.ForceEpic
		if pending {
			ui.Text(c, "强制选项已改变，请先按策略生成命令，再调整参数。").FontSize(12)
		}
		ui.Text(c, "保存前请退出 Steam。仅修改草稿不会影响 Steam；保存失败时草稿会保留。").FontSize(12)
		ui.Text(c, a.status).FontSize(12).Width(680)
		ui.Row(c).Gap(12).Children(func() {
			ui.Button(c, "取消编辑").Disabled(a.busy).OnClick(func() { a.launchOpen = false })
			ui.PrimaryButton(c, "保存启动设置").Disabled(a.busy || pending || strings.TrimSpace(a.launchDraft.Exe) == "").OnClick(func() {
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
