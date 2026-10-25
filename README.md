# Game2Steam

一个使用 MyGo 原生 Go UI 的 Windows 客户端，将本机 Epic 游戏添加到 Steam，并管理非 Steam 快捷方式。

## 运行

双击 `build/windows-amd64/Game2Steam.exe` 即可启动。发布或移动客户端时，将同目录的 `epic-bridge.exe` 一起保留。

1. 打开客户端，自动扫描 Epic 官方启动器的已安装游戏，并识别最近使用的 Steam 账号。
2. 如果路径未找到，在“设置”中填写 Steam 安装目录和 Epic 安装清单目录。默认使用“独立认证直启”；在“Epic 独立认证”点击登录，打开 Epic 网页并把返回的授权码粘贴到客户端。已通过桥接 CLI 登录的账号可直接复用。
3. 在“Epic 游戏”中搜索、勾选游戏。需要时勾选“强制开启 Epic 认证”。
4. 从 Steam 菜单选择“退出”，再点击“导入所选”。完成后点击“打开 Steam”。
5. 在 Steam 库中启动新增的非 Steam 游戏。

默认读取 `%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests` 的 `.item` 安装清单，不读取账号全部已购游戏。独立认证只用于启动已安装游戏，不读取或改写官方客户端的登录信息。

启动方式：`bIsManaged` 为真、`ExpectingDLCInstalled` 非空、`bRequiresAuth` 为真、`OwnershipToken` 为布尔值或字符串 `true`，或用户强制时，默认由桥接取得有效的 Epic 交换码及必要的所有权令牌，直接启动游戏，不启动官方 Epic 客户端。游戏作为普通子进程保留 Steam 环境，桥接跟踪游戏退出；否则直接运行裸游戏 EXE，不引入桥接，保留安装清单参数。允许离线运行不代表可以绕过认证。

本工具不提供官方 Epic 启动器模式，需要它时请直接从 Epic 客户端启动。认证失败或桥接缺失会明确报错，不静默回退；可导入后再登录，但启动认证游戏前必须完成登录。已有快捷方式和自定义命令不自动改写。

会话保存在当前 Windows 用户 DPAPI 加密文件中，启动时自动刷新，失效时在设置中重新登录。“移除桥接登录”只删除本工具的本地凭据，不注销官方 Epic。认证使用已保存的代理设置。**不提供云存档同步**，也不保证官方客户端记录游戏时长；切换启动方式前注意存档冲突，首次测试建议备份存档。官方客户端继续负责安装和更新。

## 已有条目、删除与封面

- 同一路径、Epic 游戏 URI、桥接游戏标识或同名条目会触发重复候选提示。核对候选的路径与 AppID 后选择接管或跳过。同名可能是不同游戏，不会自动接管。
- 接管保留已有 AppID、名称、参数、标签和图片。之后保存“启动设置”才会改写启动配置；已有 AppID 保持不变。
- 已导入或接管的游戏可从 Epic 列表、Steam 列表或详情页打开“启动设置”。弹窗读取当前 Steam 启动程序、工作目录和完整启动参数，支持添加特殊 flag。
- 修改“强制开启 Epic 认证”后先点击“按策略生成命令”，再调整参数并保存。勾选后即使清单认为可裸 EXE 启动，也强制认证桥接；取消后恢复自动判断，游戏本身需要认证时仍走桥接。生成只替换草稿，未改策略时可直接编辑现有命令。
- 裸 EXE 参数直接传给游戏；认证桥接参数包括 `--watch-exe`、`--startup-timeout`、`--exit-grace`，游戏附加参数放在 `--` 后，例如 `-- -windowed`，不得覆盖认证字段。保存前需退出 Steam；AppID、名称、图片、标签及未知字段保留，重复导入不会覆盖手动命令。打开编辑器后发生的外部命令修改会阻止保存。
- “Steam 快捷方式”列出当前账号的全部非 Steam 游戏，支持搜索与批量删除。删除不会卸载游戏。
- 通过本工具删除已导入或接管的 Epic 游戏时，会按 Steam 账号保存原 AppID 及 `localconfig.vdf` 中的累计时长（`Playtime`）；重新导入同一 Epic 游戏时恢复，不覆盖 Steam 已有的更高时长。不恢复已过期的“两周内时长”。记录保存在本工具的 `state.json`，不适用于直接在 Steam 中删除的条目；此前已丢失且无记录的时长无法自动找回。已通过模拟删除/重新导入测试，真实 Steam 显示仍待验证。
- 删除确认框的“同时清理图片”默认勾选，可手动取消。清理仅删除 Steam `grid` 目录中与所选 AppID 精确对应的自定义图片，不删除游戏目录、外部图标或本地选图的源文件。
- 仅已导入或明确接管的 Epic 游戏提供“封面与启动”管理。未安装但仍有管理关联的条目，也可从 Steam 列表进入封面管理。
- 在设置中填写 [SteamGridDB API Key](https://www.steamgriddb.com/profile/preferences/api)，新导入游戏会自动补齐缺失的横竖封面、背景和 Logo；已有图片保留。接管时不自动补图。
- 自动补图使用与界面预填一致的英文名称搜索（无法提取时使用原名），优先唯一同名结果，没有同名结果时尝试首个结果；多个同名结果或无搜索结果时提示手动选择。仅补缺失的三类图片，保留已有图片。
- 支持选择本地 PNG/JPEG；无 API Key 或网络错误不影响快捷方式导入。首版使用静态图片，不复用 Epic 图片缓存，不做图片裁剪。
- 扫描、搜索和下载在后台执行。可取消后续任务，已经完成的导入或图片应用会保留；当前文件事务会先完成。

## 网络设置

“设置 → 网络”提供三种模式，保存后用于游戏图片搜索、缩略图和原图下载：

- **直连**：不使用应用代理，也不读取 HTTP_PROXY / HTTPS_PROXY 环境变量。
- **系统代理**：默认模式，读取 Windows 当前用户的手动代理及绕过列表，也支持 PAC 自动配置和 WPAD 自动发现。系统未配置代理时直连。
- **指定代理地址**：例如 `http://127.0.0.1:7890` 或 `socks5://127.0.0.1:1080`，也支持 HTTPS / SOCKS5H 地址；暂不支持在地址中填写用户名和密码。

网络设置失败时会显示具体错误。指定地址格式不正确时不能保存；自动代理解析失败会报错，不默默改为直连。Windows 自动发现没有找到 WPAD 服务时使用其手动代理设置。

## Epic 桥接

`mise run build` 同时生成客户端与桥接 EXE；`mise run bridge-build` 可单独构建桥接。它隔离 Epic 的启动进程并跟踪游戏退出，不关闭 Epic；已在霍格沃茨测试中确认 Steam 会话能结束。新导入和按策略生成命令已接入，已有快捷方式不自动迁移。桥接下 Overlay 显示有差异，Steam Input 自定义映射疑似未生效（待办），不能视为完整兼容；见[桥接说明](docs/bridge-experiment.md)。

## 开发与构建

安装 Go **1.27.1 或以上**，项目已固定 MyGo 0.4.1 及依赖版本。

已安装 mise 时，项目根目录提供以下任务（首次使用新配置执行 `mise trust`）：

```powershell
mise run dev      # 开发窗口，保存 Go 文件后自动重建
mise run build    # Windows EXE、桥接与 NSIS Setup 安装包
mise run test     # 核心服务及原生 UI 测试
mise run check    # Go 与 MyGo 静态检查
mise run preview  # build/ui-preview.png，示例数据的界面预览
```

应用图标原图为 `assets/icon.png`；更换后执行 `go generate ./assets`，重新生成多尺寸 `assets/icon.ico` 和根目录的 `icon_windows_amd64.syso`，将三者一同纳入 Git。普通 `go build` 和发布构建会自动嵌入资源，窗口及安装包也使用该图标。


`mise.toml` 的 `[env]` 为这些任务设置 `GOPROXY=https://goproxy.cn,direct`，使用本机已安装的 Go，不自动安装新工具。逗号表示仅在代理返回 404/410 时尝试直接从源码仓库获取；连接超时不会触发 `direct` 回退。项目配置不修改 `go env -w` 的全局设置。

不使用 mise 时，可直接执行下面的原始命令。

已安装 mise 时，项目根目录提供以下任务（首次使用新配置执行 `mise trust`）：

```powershell
mise run dev      # 开发窗口，保存 Go 文件后自动重建
mise run build    # Windows EXE、桥接与 NSIS Setup 安装包
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
go build -mod=readonly -trimpath -ldflags="-s -w -H=windowsgui" -o build/windows-amd64/Game2Steam.exe .
```

`mise run build` 直接调用 `go tool mygo build -platform windows/amd64`，由 `mygo.json` 的 buildCommand 预构建桥接到 `build/bridge/`，再作为资源打包到客户端旁，并生成 NSIS Setup。客户端不带控制台，桥接保留诊断控制台。上面的原始 `go build` 仅构建客户端，需另行构建桥接。`mise run dev` 通过环境变量指定 `build/bridge/epic-bridge.exe`；生产客户端默认使用自身目录中的桥接。

国内网络连接默认代理超时时，可仅在当前 PowerShell 会话设置 `$env:GOPROXY='https://goproxy.cn,direct'`，然后重新下载依赖。

若希望为所有 Go 项目持久设置，可执行 `go env -w GOPROXY=https://goproxy.cn,direct`；使用 `go env -u GOPROXY` 撤销。通过 `go env GOPROXY` 查看当前生效值，当前进程的环境变量优先于持久设置。

若希望为所有 Go 项目持久设置，可执行 `go env -w GOPROXY=https://goproxy.cn,direct`；使用 `go env -u GOPROXY` 撤销。通过 `go env GOPROXY` 查看当前生效值，当前进程的环境变量优先于持久设置。

可选验证：只读检查真实 Epic/Steam 配置，不导入、删除或启动游戏：

```powershell
$env:GAME_TO_STEAM_LIVE_READONLY='1'
go test -run TestInstalledEnvironmentReadOnly -v ./internal/ui
Remove-Item Env:GAME_TO_STEAM_LIVE_READONLY
```

导出示例界面预览（使用原生 UI 渲染器和示例数据）：

```powershell
$env:GAME_TO_STEAM_PREVIEW_DIR='build'
go test -run TestPreviewArtifact -v ./internal/ui
Remove-Item Env:GAME_TO_STEAM_PREVIEW_DIR
```

## 数据与恢复

应用设置和游戏关联保存在 `%AppData%\Game2Steam\state.json`，按 Steam 账号隔离。图片缓存位于同目录的 `cache`。启动时自动从旧 `%AppData%\game-to-steam` 迁移；同名业务文件冲突时停止迁移并报错，避免覆盖。已有窗口状态优先保留，旧窗口状态另存为 `window-state.legacy.json`。API Key 单独保存在 Windows 凭据管理器的 `game-to-steam/SteamGridDB` 项中，不写入 JSON、日志或仓库。

Steam 写操作前检查 `steam.exe` 已退出并再次确认目标账号。无法解析 Steam 文件、账号不明确、检测到外部修改或权限错误时停止写入。不会自动退出 Steam，也不会操作其他账号。

每次修改前，受影响文件会备份到 `backups/<时间>-<随机标识>/`。`manifest.json` 记录原路径及对应的 `000.bin` 等原始内容；`Exists: false` 表示该文件原先不存在。写入失败会尝试恢复当前事务，错误信息包含备份路径。

需要手动恢复时，先退出客户端和 Steam，再按备份清单将对应 `.bin` 复制回原路径；原先不存在的文件按清单移除。恢复是针对该次操作的文件快照，可能撤销备份之后对同一文件的其他修改。备份可能包含本地账号信息和路径，请勿公开上传。

## Go 入门与验证范围

代码结构和关键 Go 写法见 [开发说明](docs/architecture.md)。测试使用临时目录和模拟 HTTP 服务，不修改真实 Steam 库或凭据。

当前已通过核心服务、原生 UI 交互及静态检查，验证本机安装清单和 Steam 账号的只读发现，并完成 EXE 原生窗口启动与关闭检查。真实 Steam 导入后启动游戏、以及有效 API Key 下的在线选图，需要在实际使用环境中验收。

参考：[BoilR](https://github.com/PhilipK/BoilR)、[MyGo 原生 UI](https://mygo.egoist.dev/docs/ui)、[SteamGridDB API](https://www.steamgriddb.com/api/v2)。本项目按需求独立实现 Go 服务，没有引入 BoilR Rust 源码。


## Release builds

Publish a GitHub Release with a version tag such as `v1.2.3` or `v1.2.3-beta.1` to run the **Release Windows** workflow. It builds the exact tagged commit, runs tests and static checks, and uploads these assets to that release:

- `Game2Steam.Setup.1.2.3.amd64.exe`: English NSIS installer generated by MyGo with an installation-directory page and an optional desktop shortcut.
- `Game2Steam-1.2.3-windows-amd64.zip`: portable application and Epic bridge in the same directory.
- `SHA256SUMS.txt`: SHA-256 checksums for both downloads.

The installer defaults to `%LocalAppData%\Programs\Game2Steam` and installs for the current user without administrator privileges. Both executables are installed together. Uninstalling removes the installed program and its shortcut; application settings, credentials, and Steam files are preserved. The application itself keeps its existing Chinese interface.

To rebuild assets for an existing release, run **Release Windows** manually and enter its tag. This replaces assets with the same filenames and does not create a new release. The workflow uses the repository's built-in `GITHUB_TOKEN`; no extra secret is required. The workflow must be included in the tagged commit.

For a local installer build, run:

```powershell
mise build
# Or, on Windows:
go tool mygo build -platform windows/amd64
```

MyGo first compiles the Epic bridge into `build/bridge/`, then bundles it beside the desktop executable through `mygo.json` resources. The setup is written to `build/windows-amd64/Game2Steam Setup <version> amd64.exe`. On Windows, MyGo downloads and caches NSIS on first use if it is not installed. Local builds use the version in `mygo.json`; CI replaces it with the release tag, and the UI reads MyGo's packaged version. The original MyGo filename is uploaded unchanged; GitHub normalizes its spaces to periods. SHA256SUMS.txt uses the resulting release filenames.

The new NSIS installer uses `io.daief.gametosteam` as its uninstall registration identity. Existing installations made with the old Inno Setup installer should be uninstalled before installing this version to avoid duplicate uninstall entries. Application data and credentials are retained. Future MyGo packages must keep the same identifier.
