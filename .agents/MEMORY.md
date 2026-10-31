# 项目记忆

## 首版范围与决策

- Windows amd64 客户端，MyGo 0.7.3 纯 Go 原生 UI；中文界面。按 design.md 实现深蓝灰工作台，主题位于 internal/ui/theme.go。Epic 专注导入，Steam 快捷方式统一承载启动与封面配置；保留既有认证、备份和写入安全规则。
- 只扫描 Epic 官方启动器本机已安装游戏，不接入账号游戏库管理，不复用 Epic 图片资源。独立认证只用于启动已安装游戏，不接管安装、更新或云存档。
- 客户端新导入、UpdateLaunch 和 PreviewLaunch 统一生成 --launch-mode authenticated 的 epic-bridge.exe 命令，所有游戏经独立认证桥接直启，导入前检查 Epic 登录；无启动方式/强制认证选择。所有权令牌仍按游戏清单要求获取。已有自定义命令保留；高级设置可重新生成桥接命令并显式保存，生成会替换草稿。桥接保留普通子进程与 Steam 环境，不启动官方 Epic，不接管游戏更新或云存档。
- 只修改最近使用的 Steam 账号，不提供账号切换。写入前要求用户手动退出 Steam，不自动关闭或重启。
- 全部非 Steam 快捷方式可查看、批量删除；删除确认框默认勾选清理对应 Steam 图片，可手动取消，不删除游戏本体或源图片。
- 重复游戏需确认接管。接管保留 AppID、原配置及图片；保存启动设置时才更新启动字段。
- 已导入/接管游戏可编辑启动程序、工作目录、完整参数和强制 Epic 选项。PreviewLaunch 只生成草稿；切换强制选项后先生成再添加 flag。SaveLaunch 比较编辑器原命令以拒绝外部变更，保留 AppID 及其他字段；自定义命令以 Steam 文件为准，重复导入不覆盖。
- 自动补图及手动在线/本地选图，仅针对导入或明确接管的 Epic 游戏；自动只补缺失图片。首版静态 PNG/JPEG，不提供裁剪。自动补图搜索共用 artwork.SearchQuery 的预填英文名称策略，优先唯一同名项，无同名项时尝试搜索首项；多个同名项拒绝自动选择，无结果错误包含搜索词。不读取编辑后的搜索框或当前选中来源。
- 封面配置统一位于 Steam 快捷方式详情：类型切换、搜索与本地图片工具栏，底部显示当前类型的小预览、状态和保存；列表不展示内部路径。候选图库按可用宽度排网格，独立滚动并占据剩余高度；竖版以 160px 最小卡宽计算列数，高度随列宽保持 2:3 图像比例，1280px 窗口四列，980×600 保持完整一行可见。仅保存更新对应 Steam 图片；自动补齐作用于三类缺图。三类选择在会话内保留，重新搜索、更换来源或打开其他游戏时重置。 候选卡片只有图片与选中边框，整块点击，编号仅用于可访问名称；分类、搜索与本地选图处于同一行。
- 图片搜索预填优先英文显示名；中文清单无独立英文标题字段时，从英文安装目录/EXE 提取可读搜索词、拆分驼峰，双方共同词干可去掉随机目录后缀和程序缩写（本机 Hogwarts Legacy、Bloodstained）。这只是可编辑的本地搜索提示，不当作官方译名，不使用哈希 AppName、不改游戏显示名；无可靠英文线索时保留原名。
- 封面有多个匹配游戏时使用下拉来源选择，避免多来源占用图库空间。候选最多 20 张、4 路并发加载缩略图（支持静态 WebP，缓存为 PNG）；点击候选只记录地址并高亮，保存时下载原图，失败保留选择。缩略图失败可直接选中后保存，完整图片与本地导入限 PNG/JPEG。仅清理快捷方式图片时匹配历史 Logo，不主动删除已有图片。 进入封面页自动搜索；搜索/候选加载使用独立可取消任务和局部 Spinner，不占用全局 busy，不读取可变服务；请求编号与游戏 ID 拒绝过期回写。当前图片后台读取使用捕获的服务快照与详情编号防止串页。

## 实现约束

- internal/app 协调业务；epic 扫描清单；steam 处理账号、二进制 VDF 和图片命名；storage 管理状态与备份；system 封装 Windows API；artwork 处理 SteamGridDB。
- 整套原生 UI 位于 internal/ui（app、artwork、launch、auth 及对应测试），根目录 main.go 只调用 ui.Run 并处理启动错误。MyGo 控件包统一别名 nativeui；UI 预览和只读环境测试命令指向 ./internal/ui，相对预览路径仍以项目根目录为基准。
- 设置中的路径统一为 Windows 反斜杠和大写盘符。 目录输入最大 760px，Steam 与 Epic 清单各提供扫描/选择按钮：复用本机注册信息与默认目录，选择为原生文件夹选择器；校验现有目录后只填入设置草稿，取消或失败保留原值，保存后生效。代理输入最大 420px。Epic 启动器优先从 com.epicgames.launcher 协议命令解析，再尝试安装记录和默认目录；仅检查候选文件，不递归扫描磁盘。自定义安装位置的 InstallLocation 可能缺失，协议记录仍有效。
- shortcuts.vdf 保留有序字段、未知字段和未操作条目。不可解析文件禁止覆盖。新 AppID 使用 CRC32(Exe+AppName) 设置最高位，已有 AppID 保持不变。
- 本工具删除已管理 Epic 游戏时，将原 AppID 和 localconfig.vdf 的 Apps 下有符号/无符号 ID 对应 Playtime 存入 state.json 的按账号/游戏隔离 playtimeHistory；重导入复用 ID、按最大值恢复累计分钟，不恢复易过期的 Playtime2wks，不覆盖其他 Steam 设置。文本 VDF 按字节位置修改，歧义/解析失败拒绝操作，源配置纳入删除事务备份。仅覆盖本工具删除，旧丢失时长无记录不可恢复；模拟重启/重导入、ID 冲突及数据保留测试通过，真实 Steam 显示待验收。
- UI 状态只在主线程修改，后台任务串行访问服务，用 Window.Update 回传。取消仅停止后续任务，不撤销成功操作。
- 设置与按账号隔离的管理关联位于 %AppData%/Game2Steam/state.json，图片缓存和备份也在此目录。客户端与桥接共用 storage.DataDir，启动时迁移旧 game-to-steam 目录；同名业务文件冲突报错不覆盖，新目录已有窗口状态时旧状态保留为 window-state.legacy.json。API Key 使用 Windows 凭据管理器，禁止写入状态文件、日志或仓库。
- 网络设置为直连/系统代理/指定地址，旧配置默认系统代理。所有封面请求共享已保存配置；直连忽略 HTTP_PROXY/HTTPS_PROXY。Windows 系统代理用 WinHTTP 读取当前用户设置，包含协议映射、绕过列表、PAC 和 WPAD；自定义支持 HTTP/HTTPS/SOCKS5/SOCKS5H，不接受 URL 凭据。使用本机 WinHTTP 与临时 PAC 服务验证，无需修改系统代理。 自定义代理 UI 接受 IP:端口，默认 HTTP；旧带协议配置继续兼容，界面不显示协议解释文字。
- 修改前检查 Steam 进程、目标账号及文件快照，先备份全部目标，再以同目录临时文件和 MoveFileEx 替换。普通失败尝试恢复；断电/强制结束不保证跨文件原子性，须按备份清单恢复。 全局设置保存也在写入前检查 Steam 进程；保存失败保留编辑草稿，顶部仅在确认 Steam 运行时显示“Steam 运行时无法保存，请退出后再保存”。
- 图片清理仅精确匹配目标 AppID 的 grid 文件名，不根据外部 icon 路径删除源文件。
- mise build 与 CI 直接调用 go tool mygo build -platform windows/amd64，不保留额外打包或重命名脚本。曾发生一次输出目录替换 Access denied，具体原因未确认，用户后续直接构建成功，不据此引入绕行；mygo.json buildCommand 先编译桥接到 build/bridge/，resources 将其复制到主程序旁，一并生成 NSIS Setup（Windows 首次自动下载 NSIS 到 LocalAppData/mygo）。dev 仍预构建桥接并通过 GAME_TO_STEAM_BRIDGE_PATH 指向 build/bridge/epic-bridge.exe；桥接保留控制台，无签名或自动更新。不要把资源源文件放在 MyGo 输出目录中。
- 图标原图 assets/icon.png；go generate ./assets 用 cmd/icon-gen（已有 x/image 缩放依赖）生成 16–256 九尺寸 PNG ICO 和根目录 icon_windows_amd64.syso（纯 Go COFF 图标资源），两项生成物随源码保存，普通 go build/CI 自动嵌入；窗口 SetIcon 使用嵌入 PNG，mygo.json 配置安装包图标。已从编译 EXE 提取确认图标。
- .github/workflows/release.yml 在 GitHub Release published 时构建标签，也可手动指定已有 Release 标签重跑。Windows 2025 runner 按 go.mod 安装 Go，测试/检查后用 MyGo 构建 NSIS，显示名统一 Game2Steam（mygo.json、窗口与导航）；主程序 Game2Steam.exe，保留 identifier io.daief.gametosteam 和Game2Steam 数据目录及既有 game-to-steam 凭据标识。保留 MyGo 默认 Game2Steam Setup <version> amd64.exe 名称上传，由 GitHub 将空格规范为点，校验和清单使用最终 Release 名称；上传该 Setup、双 EXE 便携 ZIP 与 SHA256SUMS.txt，不创建 Release。标签支持 v1.2.3 / v1.2.3-beta.1（v 可省略），CI 写入 mygo.json version，UI 优先读取 MyGo 包版本。移除 installer/setup.iss；NSIS 用 io.daief.gametosteam 登记卸载，旧 Inno 安装应先卸载以避免双卸载项，数据/凭据保留。已本地实际生成 Setup、7-Zip 解包核对两个 EXE 的 SHA256，测试/Go/MyGo 检查和 actionlint 通过；未实际安装或触发 GitHub 发布。
- 主窗口默认 980×600、最小内容区 980×600，恢复旧尺寸后显式钳制。160px 左侧导航始终展开，仅启动程序问号提供 hover 说明；Steam 列表在 >=1280 时宽 300px，否则 280px，所有支持尺寸保持列表与详情并排。主工作区占满可用宽度，无固定右侧留白；滚动区 MinHeight(0)，底部保存固定。 左下角 Steam 账号卡片内含小号按钮：运行时 danger 色退出/普通打开，未运行时启动；每秒后台调用共享 SteamRunning 检测，通过 Window.Update 发布，窗口关闭取消监控。退出仅发送 steam.exe -shutdown 请求，实际进程结束后再变更按钮；检测失败禁用操作。
- Steam 快捷方式页由 internal/ui/shortcuts.go 实现，游戏名与启动/封面页签同一行。仅管理关联的 Epic 游戏可编辑，外部条目只读且可删除。勾选与查看独立，启动配置采用占满详情宽度的左对齐无外框表单，默认只显示游戏参数，路径、重新生成桥接命令与完整参数放入可展开高级设置，启动程序旁问号说明免 Epic 启动器直启，移除重复命令预览；桥接附加参数使用 Windows 命令行分解/组合保留内部选项，未编辑时保持原始命令字节，裸 EXE 参数按原文保留。启动草稿按 AppID 保留，跨页签、游戏和尺寸切换不丢失；重置或成功保存后重建，保存继续核对原命令快照；启动程序、工作目录或参数有变动时，在保存左侧显示“未保存”，还原或保存成功后隐藏。封面仅在进入对应页签时加载；设置窄屏单列、宽屏双列，表单滚动且保存固定。操作结果与警告使用可展开详情，避免常驻结果面板压缩内容。 游戏卡片空白、图标与标题均可点击选中，复选框独立控制批量删除。数量为普通文字、刷新使用 SVG；列表不显示启动 EXE，优先读取 Steam Icon 与管理关联的真实游戏 EXE 图标；本地位图/Windows 嵌入图标在后台读取，缺失时显示名称首字。设置的 Epic 与 SteamGridDB 双列分组等高且高度随内容；移除登录与登录/刷新处于同一行。
- Epic 页占满可用宽度，搜索与计数筛选、刷新同一行。由 internal/ui/epic.go 实现，搜索、计数筛选、刷新与单一表头全选入口；列表只保留名称/目录、状态及导入/查看。已导入需同时存在管理关联与 Steam 快捷方式；查看进入 Steam 对应条目。单项和批量共用 import.go 确认，统一使用独立认证桥接，本地登录不可用时引导登录。确认后继续逐项接管，不自动覆盖已有命令。隐藏选择保留，批量栏位于滚动区外。
- Go 要求 1.27.3+。mise.toml 提供 dev/build/test/check/preview 任务，使用本机 Go；项目环境设置 GOPROXY=https://goproxy.cn,direct，未修改全局 Go 设置。按用户要求，命令默认直接使用 `go`，不预设 `MISE_GO_VERSION`，不使用 go.exe 绝对路径；历史 shim 排障记录不能作为当前环境故障的依据。

## 验证与后续

- design.md 界面重构通过 CGO_ENABLED=0 全量测试、Go/MyGo 静态检查与客户端构建；980×600、1100×800、1600×900 软件预览及卡片选中/复选框独立、搜索非阻塞/过期结果、设置等高、工具栏同行、参数保留、图标 ICO 读取回归通过。预览位于 build/redesign；真实网络与业务写入仍需用户验收。
- 核心服务、模拟 HTTP、原生 UI 交互测试及 Go/MyGo 静态检查通过；测试不修改真实 Steam 库或凭据。
- MyGo 固定 v0.7.3，工具与 UI 共用模块；传递依赖 x/text v0.43.0、x/net v0.59.0（MyGo contentrules 使用 publicsuffix）。从 v0.6.3 升级至 v0.7.3，官方 migrate-ui 执行后无源码差异，现有控件调用兼容；CGO_ENABLED=0 全量测试、mygo vet（含 Go vet）、客户端与桥接构建通过。本次未验收真实窗口或安装包。v0.5 起构造控件时已写入绑定值，当前帧 Disabled 不能撤销上一帧允许的输入；局部变量的 OnChange 绑定已回归。实际业务由 run 的 busy 检查串行保护，导入/删除准备、接管推进及授权码提交在修改待处理状态前额外检查 busy。无动画测试时钟兼容修改需求。
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
