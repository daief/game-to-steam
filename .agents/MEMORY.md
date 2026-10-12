# 项目记忆

## 首版范围与决策

- Windows amd64 客户端，MyGo 0.4.1 纯 Go 原生 UI；中文列表界面，跟随系统主题，无 Web 前端。
- 只扫描 Epic 官方启动器本机已安装游戏，不接入 Epic 账号游戏库；不复用 Epic 图片资源。
- 启动策略：managed、非空 DLC 映射、bRequiresAuth、OwnershipToken（布尔值或字符串 true）或强制选项统一走桥接，再调用 Epic URI；裸 EXE 不引入桥接。bCanRunOffline 不覆盖认证要求；本机霍格沃茨清单 managed=false 但需认证与所有权令牌，不能仅沿用 BoilR 的 managed/DLC 判断。直启保留 LaunchCommand，更新策略前重新解析清单以支持安装移动；不自动覆盖已导入的自定义命令。
- 只修改最近使用的 Steam 账号，不提供账号切换。写入前要求用户手动退出 Steam，不自动关闭或重启。
- 全部非 Steam 快捷方式可查看、批量删除；删除确认框默认勾选清理对应 Steam 图片，可手动取消，不删除游戏本体或源图片。
- 重复游戏需确认接管。接管保留 AppID、原配置及图片；保存启动设置时才更新启动字段。
- 已导入/接管游戏可编辑启动程序、工作目录、完整参数和强制 Epic 选项。PreviewLaunch 只生成草稿；切换强制选项后先生成再添加 flag。SaveLaunch 比较编辑器原命令以拒绝外部变更，保留 AppID 及其他字段；自定义命令以 Steam 文件为准，重复导入不覆盖。
- 自动补图及手动在线/本地选图，仅针对导入或明确接管的 Epic 游戏；自动只补缺失图片。首版静态 PNG/JPEG，不提供裁剪。

## 实现约束

- internal/app 协调业务；epic 扫描清单；steam 处理账号、二进制 VDF 和图片命名；storage 管理状态与备份；system 封装 Windows API；artwork 处理 SteamGridDB。
- 设置中的路径统一为 Windows 反斜杠和大写盘符。Epic 启动器优先从 com.epicgames.launcher 协议命令解析，再尝试安装记录和默认目录；仅检查候选文件，不递归扫描磁盘。自定义安装位置的 InstallLocation 可能缺失，协议记录仍有效。
- shortcuts.vdf 保留有序字段、未知字段和未操作条目。不可解析文件禁止覆盖。新 AppID 使用 CRC32(Exe+AppName) 设置最高位，已有 AppID 保持不变。
- UI 状态只在主线程修改，后台任务串行访问服务，用 Window.Update 回传。取消仅停止后续任务，不撤销成功操作。
- 设置与按账号隔离的管理关联位于 %AppData%/game-to-steam/state.json，图片缓存和备份也在此目录。API Key 使用 Windows 凭据管理器，禁止写入状态文件、日志或仓库。
- 网络设置为直连/系统代理/指定地址，旧配置默认系统代理。所有封面请求共享已保存配置；直连忽略 HTTP_PROXY/HTTPS_PROXY。Windows 系统代理用 WinHTTP 读取当前用户设置，包含协议映射、绕过列表、PAC 和 WPAD；自定义支持 HTTP/HTTPS/SOCKS5/SOCKS5H，不接受 URL 凭据。使用本机 WinHTTP 与临时 PAC 服务验证，无需修改系统代理。
- 修改前检查 Steam 进程、目标账号及文件快照，先备份全部目标，再以同目录临时文件和 MoveFileEx 替换。普通失败尝试恢复；断电/强制结束不保证跨文件原子性，须按备份清单恢复。
- 图片清理仅精确匹配目标 AppID 的 grid 文件名，不根据外部 icon 路径删除源文件。
- mise build 先运行 bridge-build，再直接 go build 客户端，任务环境指定 windows/amd64；两个 EXE 同目录发布，桥接保留诊断控制台。dev 同样预构建桥接，通过 GAME_TO_STEAM_BRIDGE_PATH 指定其路径。无 build.ps1，避免完整 MyGo 打包自动下载 NSIS；首版无安装包、签名或自动更新。
- Go 要求 1.27.1+。mise.toml 提供 dev/build/test/check/preview 任务，使用本机 Go；项目环境设置 GOPROXY=https://goproxy.cn,direct，未修改全局 Go 设置。

## 验证与后续

- 核心服务、模拟 HTTP、原生 UI 交互测试及 Go/MyGo 静态检查通过；测试不修改真实 Steam 库或凭据。
- 本机只读发现 1 个已安装 Epic 游戏，最近账号当时无非 Steam 条目。已检查示例 UI 渲染和实际 EXE 主窗口创建/正常关闭。
- 按用户要求已移除 smoke 脚本及任务，不再维护隐藏窗口启动检查。
- 桥接已接入导入、PreviewLaunch 与 UpdateLaunch：cmd/epic-bridge 与 internal/bridge 共用 Go module，生产使用客户端同目录的 epic-bridge.exe，缺失时报错、不静默回退；直启不依赖桥接。Windows 命令转义生成 --game、--manifest-dir、--launcher，桥接参数不转发任意游戏 flag；重复候选识别桥接游戏标识，接管和重复导入保留原命令，用户按策略生成并保存才迁移。本轮对照代码已按用户要求撤销，恢复桌面 Shell 父进程隔离+清理 Steam 环境变量的已提交基线，默认策略未更改。原生辅助进程测试已验证父 PID、路径、创建时间与环境清理；不记录环境变量值。
- 桥接默认跟踪游戏安装目录内与清单同名的 EXE（覆盖霍格沃茨入口与 Phoenix 程序交接），可用 --watch-exe 指定实际路径；启动等待五分钟、退出宽限五秒，不关闭 Epic/游戏、不读取认证参数。日志在 AppData/game-to-steam/bridge/logs；详见 docs/bridge-experiment.md。
- 用户授权后已备份并新增真实 Steam 的独立霍格沃茨桥接测试快捷方式，保留原条目；用户确认测试成功，测试前相关程序已退出。Shift+Tab 可打开好友窗口和聊天，但没有通常的半透明黑底遮罩；用户认为不影响使用，只确认 Overlay 部分可用。用户后续反馈手柄可用，但 Steam Input 自定义按键映射疑似未生效：保留为待办，不阻塞后续接入，不视为 Steam Input 兼容。Epic 已预先独立运行的场景仍待验证。用户此前也确认在线封面下载正常；有效 API Key 的真实请求由用户验收。
- 不自行提交或推送；源码和记忆随用户后续正常 Git 流程处理。
- Steam Input 对照：直接启动 Epic 映射生效；桌面父进程隔离+清理环境的基线映射不生效、会话可结束；桌面父进程隔离+继承环境则映射正常，但游戏/桥接退出后必须手动退出 Epic 才结束 Steam 会话，Overlay 未测；普通父进程+清理环境则所有手柄按键失效、Overlay 无黑底遮罩、会话结束正常。两版对照代码均已撤销；尚无兼顾输入与会话结束的解法。后续需研究实际游戏关联方式；默认直启 Epic/桥接可选仅是建议，尚未实施。用户要求保存 WIP 后暂停，勿将现有桥接视为完成兼容性验收。
