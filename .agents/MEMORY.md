# 项目记忆

## 首版范围与决策

- Windows amd64 客户端，MyGo 0.6.3 纯 Go 原生 UI；中文列表界面，无 Web 前端。按 tmp/stitch_game_to_steam_redesign 的 Midnight Gaming Fluent 原则改为统一深海蓝主题（internal/ui/theme.go），冰蓝主操作、200px 导航、状态页脚、列表选中边框与设置分组；设置保存固定在滚动区外。设计稿只提供视觉参考，业务以本项目为准，不引入稿中的账号切换、游戏大小或兼容性承诺。
- 只扫描 Epic 官方启动器本机已安装游戏，不接入账号游戏库管理，不复用 Epic 图片资源。独立认证只用于启动已安装游戏，不接管安装、更新或云存档。
- 启动策略：managed、非空 DLC 映射、bRequiresAuth、OwnershipToken（布尔值或字符串 true）或强制选项默认生成显式 --launch-mode authenticated 的桥接命令，取得有效凭据后普通子进程直启游戏、保留 Steam 环境，不启动官方 Epic；裸 EXE 不引入桥接。按用户决定移除官方 Epic 启动器选项和路径输入，官方启动需求直接回 Epic；强制选项命名为“强制开启 Epic 认证”。bCanRunOffline 不覆盖认证要求；本机霍格沃茨需认证与所有权令牌。新模式只影响新导入和按策略生成，已有自定义命令不自动覆盖。
- 只修改最近使用的 Steam 账号，不提供账号切换。写入前要求用户手动退出 Steam，不自动关闭或重启。
- 全部非 Steam 快捷方式可查看、批量删除；删除确认框默认勾选清理对应 Steam 图片，可手动取消，不删除游戏本体或源图片。
- 重复游戏需确认接管。接管保留 AppID、原配置及图片；保存启动设置时才更新启动字段。
- 已导入/接管游戏可编辑启动程序、工作目录、完整参数和强制 Epic 选项。PreviewLaunch 只生成草稿；切换强制选项后先生成再添加 flag。SaveLaunch 比较编辑器原命令以拒绝外部变更，保留 AppID 及其他字段；自定义命令以 Steam 文件为准，重复导入不覆盖。
- 自动补图及手动在线/本地选图，仅针对导入或明确接管的 Epic 游戏；自动只补缺失图片。首版静态 PNG/JPEG，不提供裁剪。自动补图搜索共用 artwork.SearchQuery 的预填英文名称策略，优先唯一同名项，无同名项时尝试搜索首项；多个同名项拒绝自动选择，无结果错误包含搜索词。不读取编辑后的搜索框或当前选中来源。
- 图片管理按 ui.md：上方三张当前图片卡片（竖版、横版、详情背景），未设置显示空框；下方候选按宽度自动换行（竖图卡片 200px、其他 260px），不固定三列。按用户要求删除 Logo 类型、下载接口和自动补齐处理，仅保留显式清理快捷方式时匹配历史 Logo 文件；不主动删除已有图片。搜索自动加载首个来源的当前类型，切换类型/来源自动加载，无“查看当前图片”和“浏览候选图片”；仅“保存”更新对应 Steam 图片及上方卡片。各类待保存选择在详情会话内保留，重新搜索/更换来源或打开游戏时重置。应用/自动补齐后重读全部当前图片，无新增持久化字段，不裁剪或缩放源图片。
- 图片搜索预填优先英文显示名；中文清单无独立英文标题字段时，从英文安装目录/EXE 提取可读搜索词、拆分驼峰，双方共同词干可去掉随机目录后缀和程序缩写（本机 Hogwarts Legacy、Bloodstained）。这只是可编辑的本地搜索提示，不当作官方译名，不使用哈希 AppName、不改游戏显示名；无可靠英文线索时保留原名。
- 封面来源用标明“游戏来源”的横向选择条，图片候选区独立占据剩余空间且至少 260px。候选最多 20 张、4 路并发加载缩略图（支持静态 WebP，缓存为 PNG）。按用户要求，点击候选只记录地址并高亮，不请求原图、不进入忙碌；仅“保存”时下载原图再导入，失败保留选择。缩略图失败不自动下载原图，可直接选中后保存。完整图片/本地导入仍限 PNG/JPEG；缓存写入失败不丢弃有效下载。

## 实现约束

- internal/app 协调业务；epic 扫描清单；steam 处理账号、二进制 VDF 和图片命名；storage 管理状态与备份；system 封装 Windows API；artwork 处理 SteamGridDB。
- 整套原生 UI 位于 internal/ui（app、artwork、launch、auth 及对应测试），根目录 main.go 只调用 ui.Run 并处理启动错误。MyGo 控件包统一别名 nativeui；UI 预览和只读环境测试命令指向 ./internal/ui，相对预览路径仍以项目根目录为基准。
- 设置中的路径统一为 Windows 反斜杠和大写盘符。Epic 启动器优先从 com.epicgames.launcher 协议命令解析，再尝试安装记录和默认目录；仅检查候选文件，不递归扫描磁盘。自定义安装位置的 InstallLocation 可能缺失，协议记录仍有效。
- shortcuts.vdf 保留有序字段、未知字段和未操作条目。不可解析文件禁止覆盖。新 AppID 使用 CRC32(Exe+AppName) 设置最高位，已有 AppID 保持不变。
- 本工具删除已管理 Epic 游戏时，将原 AppID 和 localconfig.vdf 的 Apps 下有符号/无符号 ID 对应 Playtime 存入 state.json 的按账号/游戏隔离 playtimeHistory；重导入复用 ID、按最大值恢复累计分钟，不恢复易过期的 Playtime2wks，不覆盖其他 Steam 设置。文本 VDF 按字节位置修改，歧义/解析失败拒绝操作，源配置纳入删除事务备份。仅覆盖本工具删除，旧丢失时长无记录不可恢复；模拟重启/重导入、ID 冲突及数据保留测试通过，真实 Steam 显示待验收。
- UI 状态只在主线程修改，后台任务串行访问服务，用 Window.Update 回传。取消仅停止后续任务，不撤销成功操作。
- 设置与按账号隔离的管理关联位于 %AppData%/game-to-steam/state.json，图片缓存和备份也在此目录。API Key 使用 Windows 凭据管理器，禁止写入状态文件、日志或仓库。
- 网络设置为直连/系统代理/指定地址，旧配置默认系统代理。所有封面请求共享已保存配置；直连忽略 HTTP_PROXY/HTTPS_PROXY。Windows 系统代理用 WinHTTP 读取当前用户设置，包含协议映射、绕过列表、PAC 和 WPAD；自定义支持 HTTP/HTTPS/SOCKS5/SOCKS5H，不接受 URL 凭据。使用本机 WinHTTP 与临时 PAC 服务验证，无需修改系统代理。
- 修改前检查 Steam 进程、目标账号及文件快照，先备份全部目标，再以同目录临时文件和 MoveFileEx 替换。普通失败尝试恢复；断电/强制结束不保证跨文件原子性，须按备份清单恢复。
- 图片清理仅精确匹配目标 AppID 的 grid 文件名，不根据外部 icon 路径删除源文件。
- mise build 与 CI 直接调用 go tool mygo build -platform windows/amd64，不保留额外打包或重命名脚本。曾发生一次输出目录替换 Access denied，具体原因未确认，用户后续直接构建成功，不据此引入绕行；mygo.json buildCommand 先编译桥接到 build/bridge/，resources 将其复制到主程序旁，一并生成 NSIS Setup（Windows 首次自动下载 NSIS 到 LocalAppData/mygo）。dev 仍预构建桥接并通过 GAME_TO_STEAM_BRIDGE_PATH 指向 build/bridge/epic-bridge.exe；桥接保留控制台，无签名或自动更新。不要把资源源文件放在 MyGo 输出目录中。
- 图标原图 assets/icon.png；go generate ./assets 用 cmd/icon-gen（已有 x/image 缩放依赖）生成 16–256 九尺寸 PNG ICO 和根目录 icon_windows_amd64.syso（纯 Go COFF 图标资源），两项生成物随源码保存，普通 go build/CI 自动嵌入；窗口 SetIcon 使用嵌入 PNG，mygo.json 配置安装包图标。已从编译 EXE 提取确认图标。
- .github/workflows/release.yml 在 GitHub Release published 时构建标签，也可手动指定已有 Release 标签重跑。Windows 2025 runner 按 go.mod 安装 Go，测试/检查后用 MyGo 构建 NSIS，显示名统一 Game2Steam（mygo.json、窗口与导航）；主程序 Game2Steam.exe，保留 identifier io.daief.gametosteam 和既有 game-to-steam 数据/凭据目录。保留 MyGo 默认 Game2Steam Setup <version> amd64.exe 名称上传，由 GitHub 将空格规范为点，校验和清单使用最终 Release 名称；上传该 Setup、双 EXE 便携 ZIP 与 SHA256SUMS.txt，不创建 Release。标签支持 v1.2.3 / v1.2.3-beta.1（v 可省略），CI 写入 mygo.json version，UI 优先读取 MyGo 包版本。移除 installer/setup.iss；NSIS 用 io.daief.gametosteam 登记卸载，旧 Inno 安装应先卸载以避免双卸载项，数据/凭据保留。已本地实际生成 Setup、7-Zip 解包核对两个 EXE 的 SHA256，测试/Go/MyGo 检查和 actionlint 通过；未实际安装或触发 GitHub 发布。
- 主窗口默认 1280×900、最小 1100×800，按用户决定优先提供足够的图片浏览空间，不再适配 900×640。MyGo 恢复旧 StateKey 尺寸不按新最小值钳制，启动后显式确保内容区至少 1100×800；已用模拟接口下载 WebP 的真实原生窗口验证旧 900×640 状态恢复与图片显示。
- Steam 快捷方式页由 internal/ui/shortcuts.go 实现三栏：主菜单、300px 可搜索快捷列表、右侧启动配置/封面配置页签；勾选只用于批量删除，查看游戏与勾选独立。仅管理关联的 Epic 游戏可编辑，外部条目只读。启动草稿按 AppID 隔离保留，切游戏/页签不丢失，重置或成功保存后重建；保存仍以原快捷方式快照检测外部变更。所有页面共用 midnightTheme 和 Microsoft YaHei UI 中文字体，navigation.go 提供 200px 图标分组导航与底部账号卡，工作区使用 40px 顶部栏；设置按路径整行、认证与封面双栏、网络整行排列，底部右侧固定保存。Steam 页采用 studio.go 的设计稿布局：全高无外框三栏、炭灰面板、凹陷输入区、启动策略并排卡片；封面使用类型标签、当前类型的图片/路径横向卡片、候选图库及底部保存栏，三类图片仍分别保存。普通 Epic 独立详情继续显示三张当前图片卡片。仅进入封面页签时加载，不打开模态窗口。Epic 页原有独立详情/启动对话框继续保留。
- Epic 游戏页由 internal/ui/epic.go 独立实现：固定表头、紧凑对齐列表，全部/未导入/已导入计数筛选，名称与安装路径搜索，Ctrl+F 聚焦、Esc 清空。已导入必须同时存在管理关联和对应 Steam 快捷方式，外部删除后可重新导入；清单的 Managed 是认证策略字段，不能当导入状态。全选/表头复选仅影响当前筛选与搜索可见项，隐藏已选项保留且计入底部总数；批量操作栏仅选中时显示，绝对定位于列表视口底部，滚动内容末尾预留 96px 防遮挡；单项导入独立构造请求，不夹带批量选择，仍走重复接管确认。
- Go 要求 1.27.1+。mise.toml 提供 dev/build/test/check/preview 任务，使用本机 Go；项目环境设置 GOPROXY=https://goproxy.cn,direct，未修改全局 Go 设置。按用户要求，命令默认直接使用 `go`，不预设 `MISE_GO_VERSION`，不使用 go.exe 绝对路径；历史 shim 排障记录不能作为当前环境故障的依据。

## 验证与后续

- 核心服务、模拟 HTTP、原生 UI 交互测试及 Go/MyGo 静态检查通过；测试不修改真实 Steam 库或凭据。
- MyGo 固定 v0.6.3，工具与 UI 共用模块；传递依赖 x/text v0.43.0、x/net v0.59.0（MyGo contentrules 使用 publicsuffix）。从 v0.5.5 升级后官方 migrate-ui 无待迁移项，无需修改现有控件调用；全量测试、Go/MyGo 静态检查、客户端与桥接构建通过，并检查设置和窄窗口封面软件预览。v0.5 起构造控件时已写入绑定值，当前帧 Disabled 不能撤销上一帧允许的输入；局部变量的 OnChange 绑定已回归。实际业务由 run 的 busy 检查串行保护，导入/删除准备、接管推进及授权码提交在修改待处理状态前额外检查 busy。无动画测试时钟兼容修改需求。
- v0.5.5 已通过 mise test/check/build、四张软件渲染预览检查及真实 Windows 窗口的页面切换、搜索、滚轮、密码属性、登录取消和正常关闭验证；未进行真实登录、导入、删除或启动游戏。
- 本机只读发现 1 个已安装 Epic 游戏，最近账号当时无非 Steam 条目。已检查示例 UI 渲染和实际 EXE 主窗口创建/正常关闭。
- 按用户要求已移除 smoke 脚本及任务，不再维护隐藏窗口启动检查。
- 桥接已接入导入、PreviewLaunch 与 UpdateLaunch：cmd/epic-bridge 与 internal/bridge 共用 Go module，使用客户端同目录的 epic-bridge.exe，缺失报错、不静默回退。新生成认证命令只包含游戏标识、清单目录与模式，不要求启动器路径或写入凭据；客户端不提供官方启动器模式。CLI 默认 launcher 仅为兼容旧快捷方式，旧隔离实现不再是客户端默认；重复候选识别桥接标识，接管和重复导入保留原命令。
- 桥接默认跟踪安装目录内与清单同名的 EXE，覆盖霍格沃茨入口与 Phoenix 交接；支持 --watch-exe、五分钟启动等待与五秒退出宽限，不关闭 Epic/游戏。认证模式游戏附加参数放在 -- 后，拒绝覆盖认证字段；直启保留清单原参数。日志在 bridge/logs，不记录凭据或完整游戏命令。
- 用户授权后已备份并新增真实 Steam 的独立霍格沃茨桥接测试快捷方式，保留原条目；用户确认测试成功，测试前相关程序已退出。Shift+Tab 可打开好友窗口和聊天，但没有通常的半透明黑底遮罩；用户认为不影响使用，只确认 Overlay 部分可用。用户后续反馈手柄可用，但 Steam Input 自定义按键映射疑似未生效：保留为待办，不阻塞后续接入，不视为 Steam Input 兼容。Epic 已预先独立运行的场景仍待验证。用户此前也确认在线封面下载正常；有效 API Key 的真实请求由用户验收。
- 不自行提交或推送；源码和记忆随用户后续正常 Git 流程处理。
- Steam 新写出的 loginusers.vdf 可能只有小写 timestamp、没有 MostRecent；只读字段查询已兼容大小写，仍按唯一最近时间选择，歧义拒绝，不绕过账号核对。
- 本次完整集成后按用户明确要求，已删除当前账号的霍格沃茨和赤痕两条历史非 Steam 快捷方式及管理关联，并永久清空本工具 backups 的 21 组备份（含删除前备份）；保留游戏文件、封面、网络设置和 Epic DPAPI 登录，供用户重新导入。勿恢复旧测试条目或备份；后续正常操作仍会生成新备份。
- 旧启动器模式 Steam Input 对照：直接启动 Epic 映射生效；桌面父进程隔离+清理环境的基线映射不生效、会话可结束；桌面父进程隔离+继承环境则映射正常，但游戏/桥接退出后必须手动退出 Epic 才结束 Steam 会话，Overlay 未测；普通父进程+清理环境则所有手柄按键失效、Overlay 无黑底遮罩、会话结束正常。两版对照代码均已撤销并保存 WIP，不能作为兼容方案；后续认证直启的验证与此区分。
- 独立认证已接入客户端设置：登录/更换账号、本地状态刷新、二次确认移除登录；Password 授权码输入取消/提交后清空，错误不保留秘密。不依赖 Legendary，只参考协议；用户明确选择浏览器独立会话、不读取官方客户端持久凭据，继续用官方客户端管理游戏。与 CLI 共用 bridge/auth/session.dpapi，已有 CLI 登录可复用。DPAPI、受限 ACL、跨进程锁、原子轮换存储；请求复用已保存网络设置。旧 epicLaunchMode 字段被忽略并在后续保存时移除，统一认证桥接；不自动改写已有条目。
- 认证直启拒绝原参数/附加参数的认证字段冲突，复用路径边界、已有游戏拒绝、入口交接和宽限跟踪。所有权令牌放本次随机受限目录；正常完成或创建进程前失败清理，取消/超时/追踪失败且已启动游戏时保留。无云存档同步，不保证官方时长统计。模拟 API、DPAPI、参数/生命周期、UI/默认模式/切换保留测试及 Go/MyGo 检查和构建通过，已检查软件渲染登录/设置预览（非真实窗口人工验收）。用户确认赤痕和霍格沃茨认证直启、Steam Input 映射、Overlay、退出后的 Steam 状态正常；霍格沃茨覆盖所有权令牌游戏。跨有效期刷新和 Epic 独立常驻共存仍待用户验收。
