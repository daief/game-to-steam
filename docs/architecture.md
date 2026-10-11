# 面向 Go 新手的开发说明

## 分工

```text
main.go / *_ui.go               原生界面、选择状态、弹窗和后台任务
internal/app                   导入、接管、删除、启动策略和补图的协调
internal/model                 各模块共享的数据结构
internal/epic                  本地 .item 清单、启动方式和 Epic URI
internal/steam                 账号发现、VDF 编解码、AppID 和图片文件命名
internal/artwork               SteamGridDB HTTP、图片验证和缓存
internal/storage               JSON 状态、变更检查、备份和失败恢复
internal/system                Windows 注册表、进程、凭据和打开程序
```

Go 的 `package` 是一组在同一个目录中的代码。同一包的文件可以直接互相调用；其他包需要 `import`。`internal` 目录中的包只能被这个项目内部使用，适合不打算对外发布的实现。

程序只支持 Windows，`system_windows.go` 和文件替换实现使用 `//go:build windows` 标记。系统相关代码集中在这些文件，业务测试则使用临时文件和可替换的进程检查函数。

## 数据和错误

`struct` 类似其他语言中的记录或简单对象，例如 `model.Game` 保存游戏名称、安装路径和标识。大写开头的字段与函数可被其他包访问，小写则只在本包内可见。

```go
games, warnings, err := epic.Scan(manifestDir)
if err != nil {
    // 整个目录无法读取，需要停止扫描或向用户显示错误。
}
// warnings 是单个坏清单的提示；其他正常游戏仍然可用。
```

Go 通常通过返回值报告错误，而不是抛异常。多个返回值可以把成功数据与错误一起传回。`:=` 表示声明并赋值，编译器推导类型；后续重新赋值使用 `=`。

`func (s *Service) Import(...)` 是绑定到 `Service` 的方法。`*Service` 是指针，方法可以更新同一个服务实例中的状态。使用普通值返回 UI 快照，让界面不直接读取正在被后台修改的数据。

## 后台任务与界面

MyGo 每次需要更新界面时调用 `view`。这里读取状态和构造控件，不进行网络请求或扫描磁盘。按钮的 `OnClick` 回调在界面构造完成后执行，避免边遍历列表边改变列表。

`go func() { ... }()` 会启动一个 goroutine，也就是由 Go 管理的并发任务。项目的 `run` 方法一次只允许一个后台任务访问应用服务：

1. 在主线程捕获当前选项并设置忙碌状态。
2. 在 goroutine 中扫描、下载或修改文件。
3. 通过 `Window.Update` 回到主线程，更新 UI 快照与结果。

后台任务不直接改写界面正在读取的字段。`context.Context` 用于传递取消信号和 HTTP 超时；取消不会撤销已经成功写入的数据。

MyGo 的 `ui.Element` 只在当前构造阶段有效，不能放入持久状态或传给 goroutine。项目保存游戏数据和图片 `Bitmap`，不保存控件对象；`go tool mygo vet` 检查这种生命周期问题。

## Steam 文件处理

`loginusers.vdf` 是文本格式，用来识别最近使用的账号。`shortcuts.vdf` 是二进制格式，不能当 JSON 或普通文本写入。

解析器保存有序字段树以及未知字段的原始内容。修改快捷方式时只更新指定字段，其他字段和未操作条目保持原值。遇到无法理解的格式会报错，不创建空列表覆盖原文件。

新快捷方式的 AppID 是 `CRC32(Exe + AppName) | 0x80000000`。已有条目接管后保留 AppID，修改启动路径也不重新计算，避免已有图片和关联丢失。

启动设置通过 `PreviewLaunch` 生成策略草稿，通过 `SaveLaunch` 保存真实 Exe、StartDir、LaunchOptions 及强制选项。预览不写 Steam，可在 Steam 运行时使用；保存会比较打开编辑器时的原命令，拒绝覆盖外部变更。自定义命令以 Steam 文件为准，重复导入不会自动重写。

图片写入账号的 `config/grid`：`<AppID>p.png` 为竖版、`<AppID>.png` 为横版、`<AppID>_hero.png` 为背景、`<AppID>_logo.png` 为 Logo。删除图片使用精确文件名匹配，不使用前缀递归删除，也不根据条目的外部 `icon` 路径删除源文件。

## 写入与恢复

`storage.Snapshot` 保存文件是否存在及原始字节。写入前重新读取并比较，发现其他程序修改时停止操作。文件先写到同目录临时文件，刷新磁盘后使用 Windows `MoveFileEx` 替换。

一次操作涉及多个文件时先全部备份，再开始写入。普通写入失败会恢复已处理文件；这不是操作系统提供的跨文件事务，断电或强制结束进程仍可能需要按备份清单手动恢复。

测试重点是操作边界：未知字段保留、接管不改原配置、重复导入、账号歧义、图片清理范围、写入失败恢复以及模拟网络错误。原生 UI 测试用 `ui.NewTester` 模拟搜索、勾选和弹窗操作，无需打开真实窗口。
