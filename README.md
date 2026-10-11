# game-to-steam

一个使用 MyGo 原生 Go UI 的 Windows 客户端，将本机 Epic 游戏添加到 Steam，并管理非 Steam 快捷方式。

## 运行

双击 `build/windows-amd64/game-to-steam.exe` 即可启动。

1. 打开客户端，自动扫描 Epic 官方启动器的已安装游戏，并识别最近使用的 Steam 账号。
2. 如果路径未找到，在“设置”中填写 Steam 安装目录、Epic 安装清单目录和 `EpicGamesLauncher.exe` 路径。
3. 在“Epic 游戏”中搜索、勾选游戏。需要时勾选“强制通过 Epic 启动”。
4. 从 Steam 菜单选择“退出”，再点击“导入所选”。完成后点击“打开 Steam”。
5. 在 Steam 库中启动新增的非 Steam 游戏。

默认读取 `%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests` 的 `.item` 安装清单，不登录 Epic，也不读取账号全部已购游戏。

启动方式参考 BoilR：`bIsManaged` 为真、`ExpectingDLCInstalled` 非空或用户强制时，使用 Epic 启动器及对应游戏 URI；否则直接运行游戏 EXE，并保留安装清单中的启动参数。自动判断不能覆盖所有游戏；直接启动失败时可改用 Epic。

## 已有条目、删除与封面

- 同一路径、Epic 游戏 URI 或同名条目会触发重复候选提示。核对候选的路径与 AppID 后选择接管或跳过。同名可能是不同游戏，不会自动接管。
- 接管保留已有 AppID、名称、参数、标签和图片。之后保存“启动设置”才会改写启动配置；已有 AppID 保持不变。
- 已导入或接管的游戏可从 Epic 列表、Steam 列表或详情页打开“启动设置”。弹窗读取当前 Steam 启动程序、工作目录和完整启动参数，支持添加特殊 flag。
- 修改“强制通过 Epic 启动”后先点击“按策略生成命令”，再调整参数并保存。生成只修改草稿，会替换手动命令；未修改启动策略时可直接编辑现有参数。取消强制会恢复 managed/DLC 自动判断，不保证一定改为直接启动。
- 参数传给当前启动程序：通过 Epic 启动时先传给 Epic 启动器，游戏是否收到这些 flag 取决于启动器支持。保存前需退出 Steam；AppID、名称、图片、标签及未知字段保留，重复导入不会覆盖手动命令。打开编辑器后发生的外部命令修改会阻止保存。
- “Steam 快捷方式”列出当前账号的全部非 Steam 游戏，支持搜索与批量删除。删除不会卸载游戏。
- 删除确认框的“同时清理图片”默认勾选，可手动取消。清理仅删除 Steam `grid` 目录中与所选 AppID 精确对应的自定义图片，不删除游戏目录、外部图标或本地选图的源文件。
- 仅已导入或明确接管的 Epic 游戏提供“封面与启动”管理。未安装但仍有管理关联的条目，也可从 Steam 列表进入封面管理。
- 在设置中填写 [SteamGridDB API Key](https://www.steamgriddb.com/profile/preferences/api)，新导入游戏会自动补齐缺失的横竖封面、背景和 Logo；已有图片保留。接管时不自动补图。
- 自动匹配仅接受唯一同名结果，匹配不到时在详情页修改搜索词并选择游戏。每次浏览展示最多 20 张候选，前 6 张加载缩略图；点击预览后再应用。
- 支持选择本地 PNG/JPEG；无 API Key 或网络错误不影响快捷方式导入。首版使用静态图片，不复用 Epic 图片缓存，不做图片裁剪。
- 扫描、搜索和下载在后台执行。可取消后续任务，已经完成的导入或图片应用会保留；当前文件事务会先完成。

## 网络设置

“设置 → 网络”提供三种模式，保存后用于游戏图片搜索、缩略图和原图下载：

- **直连**：不使用应用代理，也不读取 HTTP_PROXY / HTTPS_PROXY 环境变量。
- **系统代理**：默认模式，读取 Windows 当前用户的手动代理及绕过列表，也支持 PAC 自动配置和 WPAD 自动发现。系统未配置代理时直连。
- **指定代理地址**：例如 `http://127.0.0.1:7890` 或 `socks5://127.0.0.1:1080`，也支持 HTTPS / SOCKS5H 地址；暂不支持在地址中填写用户名和密码。

网络设置失败时会显示具体错误。指定地址格式不正确时不能保存；自动代理解析失败会报错，不默默改为直连。Windows 自动发现没有找到 WPAD 服务时使用其手动代理设置。

## 开发与构建

安装 Go **1.27.1 或以上**，项目已固定 MyGo 0.4.1 及依赖版本。

已安装 mise 时，项目根目录提供以下任务（首次使用新配置执行 `mise trust`）：

```powershell
mise run dev      # 开发窗口，保存 Go 文件后自动重建
mise run build    # build/windows-amd64/game-to-steam.exe
mise run test     # 核心服务及原生 UI 测试
mise run check    # Go 与 MyGo 静态检查
mise run preview  # build/ui-preview.png，示例数据的界面预览
```

`mise.toml` 的 `[env]` 为这些任务设置 `GOPROXY=https://goproxy.cn,direct`，使用本机已安装的 Go，不自动安装新工具。逗号表示仅在代理返回 404/410 时尝试直接从源码仓库获取；连接超时不会触发 `direct` 回退。项目配置不修改 `go env -w` 的全局设置。

不使用 mise 时，可直接执行下面的原始命令。

已安装 mise 时，项目根目录提供以下任务（首次使用新配置执行 `mise trust`）：

```powershell
mise run dev      # 开发窗口，保存 Go 文件后自动重建
mise run build    # build/windows-amd64/game-to-steam.exe
mise run test     # 核心服务及原生 UI 测试
mise run check    # Go 与 MyGo 静态检查
mise run preview  # build/ui-preview.png，示例数据的界面预览
```

`mise.toml` 的 `[env]` 为这些任务设置 `GOPROXY=https://goproxy.cn,direct`，使用本机已安装的 Go，不自动安装新工具。逗号表示仅在代理返回 404/410 时尝试直接从源码仓库获取；连接超时不会触发 `direct` 回退。项目配置不修改 `go env -w` 的全局设置。

不使用 mise 时，可直接执行下面的原始命令。

```powershell
go mod download
go tool mygo dev
```

不需要前端工具链。验证与生成便携 EXE：

```powershell
go test ./...
go vet ./...
go tool mygo vet
go build -mod=readonly -trimpath -ldflags="-s -w -H=windowsgui" -o build/windows-amd64/game-to-steam.exe .
```

`mise run build` 直接调用 `go build`，并设置 `GOOS=windows`、`GOARCH=amd64`，生成不带控制台窗口的便携 EXE，不下载 NSIS 或生成安装包。上面的原始命令适用于 Windows amd64 开发环境。`mygo.json` 用于 MyGo 开发工具；`go tool mygo build` 则会运行框架完整打包流程，可能下载安装包工具。

国内网络连接默认代理超时时，可仅在当前 PowerShell 会话设置 `$env:GOPROXY='https://goproxy.cn,direct'`，然后重新下载依赖。

若希望为所有 Go 项目持久设置，可执行 `go env -w GOPROXY=https://goproxy.cn,direct`；使用 `go env -u GOPROXY` 撤销。通过 `go env GOPROXY` 查看当前生效值，当前进程的环境变量优先于持久设置。

若希望为所有 Go 项目持久设置，可执行 `go env -w GOPROXY=https://goproxy.cn,direct`；使用 `go env -u GOPROXY` 撤销。通过 `go env GOPROXY` 查看当前生效值，当前进程的环境变量优先于持久设置。

可选验证：只读检查真实 Epic/Steam 配置，不导入、删除或启动游戏：

```powershell
$env:GAME_TO_STEAM_LIVE_READONLY='1'
go test -run TestInstalledEnvironmentReadOnly -v .
Remove-Item Env:GAME_TO_STEAM_LIVE_READONLY
```

导出示例界面预览（使用原生 UI 渲染器和示例数据）：

```powershell
$env:GAME_TO_STEAM_PREVIEW_DIR='build'
go test -run TestPreviewArtifact -v .
Remove-Item Env:GAME_TO_STEAM_PREVIEW_DIR
```

## 数据与恢复

应用设置和游戏关联保存在 `%AppData%\game-to-steam\state.json`，按 Steam 账号隔离。图片缓存位于同目录的 `cache`。API Key 单独保存在 Windows 凭据管理器的 `game-to-steam/SteamGridDB` 项中，不写入 JSON、日志或仓库。

Steam 写操作前检查 `steam.exe` 已退出并再次确认目标账号。无法解析 Steam 文件、账号不明确、检测到外部修改或权限错误时停止写入。不会自动退出 Steam，也不会操作其他账号。

每次修改前，受影响文件会备份到 `backups/<时间>-<随机标识>/`。`manifest.json` 记录原路径及对应的 `000.bin` 等原始内容；`Exists: false` 表示该文件原先不存在。写入失败会尝试恢复当前事务，错误信息包含备份路径。

需要手动恢复时，先退出客户端和 Steam，再按备份清单将对应 `.bin` 复制回原路径；原先不存在的文件按清单移除。恢复是针对该次操作的文件快照，可能撤销备份之后对同一文件的其他修改。备份可能包含本地账号信息和路径，请勿公开上传。

## Go 入门与验证范围

代码结构和关键 Go 写法见 [开发说明](docs/architecture.md)。测试使用临时目录和模拟 HTTP 服务，不修改真实 Steam 库或凭据。

当前已通过核心服务、原生 UI 交互及静态检查，验证本机安装清单和 Steam 账号的只读发现，并完成 EXE 原生窗口启动与关闭检查。真实 Steam 导入后启动游戏、以及有效 API Key 下的在线选图，需要在实际使用环境中验收。

参考：[BoilR](https://github.com/PhilipK/BoilR)、[MyGo 原生 UI](https://mygo.egoist.dev/docs/ui)、[SteamGridDB API](https://www.steamgriddb.com/api/v2)。本项目按需求独立实现 Go 服务，没有引入 BoilR Rust 源码。
