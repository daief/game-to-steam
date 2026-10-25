# Epic 桥接与验收

## 默认认证直启

客户端设置中提供 Epic 登录、更换账号、本地认证状态刷新和移除登录，与 CLI 共用 DPAPI 会话文件，之前 CLI 登录无需重复登录。需要认证的游戏统一走认证桥接，裸 EXE 直启；“强制开启 Epic 认证”用于覆盖自动判断。官方启动器选项及路径输入已移除，旧配置 epicLaunchMode 被忽略。

新增纯 Go 的独立认证模式，不安装或调用 Legendary，不导入 Epic 客户端保存的刷新令牌、不修改其登录配置。仅复用现有 `.item` 清单和安装文件；官方客户端继续负责安装、更新等日常管理。2026-10-18 用户完成独立登录，并确认《赤痕：夜之仪式》和《霍格沃茨之遗》可启动，Steam Input 自定义映射、Overlay 和退出游戏后的 Steam 状态均正常；霍格沃茨覆盖需要所有权令牌的游戏。跨有效期刷新及 Epic 独立常驻时的共存仍待验收，不代表所有 Epic 游戏均可支持。

先运行 `mise run build`，在交互式 PowerShell 中首次登录：

```powershell
.\build\bridge\epic-bridge.exe --auth-login
```

浏览器直接打开 Epic 官方登录页，可复用默认浏览器当前登录。确认是拥有游戏的账号，完成登录后，把响应里的 `authorizationCode` 值（或单行完整 JSON）粘贴到桥接终端，按 Enter。输入不回显，不要把授权码放进命令行、截图或发给其他人。需要验证码、条款确认时在 Epic 网页处理；失败后重新运行登录，不自动读取客户端凭据。

```powershell
.\build\bridge\epic-bridge.exe --auth-status
.\build\bridge\epic-bridge.exe --list
.\build\bridge\epic-bridge.exe --launch-mode authenticated --game "上一步列出的完整标识"
```

`--auth-status` 只读取本地状态，不验证服务器会话；`--auth-forget` 只删除桥接本地认证，不调用 Epic 注销接口。会话保存在 `%AppData%\Game2Steam\bridge\auth\session.dpapi`，用当前 Windows 用户 DPAPI 加密并限制目录 ACL；刷新操作有跨进程锁，轮换凭据先原子保存再获取启动凭据。请求沿用客户端保存的网络设置（缺省系统代理），不会记录完整认证响应、令牌或启动命令。

客户端直接导入即可使用认证模式，无需手动修改参数。旧条目及自定义命令不自动改写，可在启动设置里按策略生成，退出 Steam 后保存，AppID 和图片保留。需要官方启动器时直接回 Epic 客户端启动。

认证模式按清单要求获取所有权令牌，再临近启动时取得一次性交换码；游戏以桥接普通子进程启动，保留 Steam 环境。默认按 Windows 用户语言启动，可用 `--locale zh-CN` 覆盖。保留清单原启动参数，游戏附加参数放在 `--` 后，例如 `-- -windowed`；所有用户参数均拒绝覆盖认证字段。失败直接停止，不回退打开 Epic；不需要有效的官方启动器路径。

所有权令牌位于本次随机受限 `launch-ovt-*` 子目录，正常完成会话或创建游戏前失败时清理。取消、超时、追踪失败时游戏可能仍在运行，因此保留文件；确认游戏退出后可手动删除该次 `launch-ovt-*` 目录，不要删除整个认证目录。没有跨次自动清理，也不记录令牌内容。

验收分两组：先完全退出 Epic 和游戏，从 Steam 认证启动，检查游戏可进入、Steam Input 自定义映射、完整 Overlay 和退出游戏后 Steam 会话结束（且 Epic 没有被启动）；再测试 Epic 从桌面独立运行的场景，确认它保持运行且官方登录可继续使用。重复启动应无需再次登录。自动测试仅使用模拟 API、临时 DPAPI 数据和自身辅助进程，不能替代以上验收。

**不提供云存档同步**，也不保证官方客户端记录此次游戏时长或运行状态。来回使用两种启动方式前注意本地/云端存档冲突，不要在 Epic 同步、更新或运行同一游戏时启动实验模式。首次测试前建议备份存档。本实验不绕过游戏所有权验证，也不处理第三方启动器、额外 DRM 或反作弊的特殊要求。

协议依据：[Epic 认证参数说明](https://dev.epicgames.com/docs/web-api-ref/authentication)、[Legendary API 协议实现](https://github.com/legendary-gl/legendary/blob/master/legendary/api/egs.py)。后者仅作协议参考，本项目无其运行时依赖；启动器接口及应用标识不是稳定的第三方公开契约，未来可能失效。

## 旧启动器隔离实验（历史参考）

以下仅记录历史 launcher CLI 隔离实验，不是客户端可选模式。CLI 默认 launcher 仅用于旧命令兼容，客户端生成显式 authenticated 参数；客户端不再提供官方启动器模式。桥接不修改 Steam 文件，不关闭 Epic 或游戏。

`mise run build` 构建两个 EXE，放在同一目录。认证型生成不依赖 Epic 启动器，缺失桥接或游戏入口会报错；已有命令仍须用户显式生成并保存才迁移。下方旧启动器实验中，`--launcher` 可指定官方启动器路径。

## 构建与命令

```powershell
mise run bridge-build
.\build\bridge\epic-bridge.exe --list
.\build\bridge\epic-bridge.exe --game "上一步显示的完整游戏标识"
```

PowerShell 直接运行只能验证启动和进程识别；Steam 会话是否正确结束必须从 Steam 启动验证。

- `--launcher "C:\...\EpicGamesLauncher.exe"`：覆盖自动发现路径。
- `--watch-exe "Phoenix\Binaries\Win64\HogwartsLegacy.exe"`：只跟踪这个实际游戏 EXE；相对路径基于游戏安装目录。默认跟踪安装目录内与清单入口同名的所有 EXE，覆盖霍格沃茨入口与 Phoenix 游戏程序的交接。
- `--startup-timeout 5m`：等待游戏启动，默认五分钟；登录、更新耗时可调大。
- `--exit-grace 5s`：同名游戏进程全部消失后继续等待五秒，避免入口与游戏程序交接时过早退出。
- Ctrl+C 只结束桥接，不停止游戏或 Epic。Steam 的“停止”行为不由本实验保证。

诊断日志：`%AppData%\Game2Steam\bridge\logs\session-*.log`，同时输出到控制台。只记录游戏标识、路径、PID、父 PID 和生命周期，不读取或记录游戏命令行、认证令牌。日志包含个人路径，分享前请检查。

## 启动方式对照实验

旧对照实验已撤销，launcher CLI 保留桌面 Shell 父进程隔离和清理 Steam 环境的基线；其输入兼容性有问题，不作为客户端默认或可选路径。客户端统一认证直启或裸 EXE；官方启动需求由用户回 Epic 客户端处理。

早期实验使用桌面 Shell 父进程隔离（[Microsoft 文档](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)），但不能同时保证 Steam Input 与会话结束。当前版同样**不保证 Steam 的追踪策略或 Overlay/Steam Input 行为**。本实验不会改变已运行 Epic 的父进程或环境；做对照前应完全退出 Epic 与游戏，以避免复用旧进程影响结果。

## 手动验收

可通过客户端新导入游戏或在已有游戏的启动设置中按策略生成、保存，也可在 Steam 手动添加临时非 Steam 快捷方式，目标为 `epic-bridge.exe`，工作目录为它所在目录，启动选项为 `--game "完整游戏标识"`。手动验收时可保留原快捷方式用于对照。

分别验证：

1. Epic 完全退出，Steam 启动临时快捷方式 → Epic 启动游戏 → 退出游戏。
2. Epic 从桌面独立启动并已登录，Steam 启动临时快捷方式 → 游戏启动 → 退出游戏。

两种情况都要求游戏运行时 Steam 显示“游戏中”，游戏退出且宽限期结束后恢复“启动”，Epic 保持运行。记录 Overlay、Steam Input/手柄是否可用；进程隔离可能使其不再自动注入游戏，不能据此宣称等同直接从 Steam 启动。

若入口退出后实际游戏仍运行，日志应看到 Phoenix 路径进程；若实际 EXE 不同，显式设置 `--watch-exe` 重试。启动前已有匹配游戏进程会拒绝启动，避免误跟踪其他会话；候选进程读取失败会报错，不冒充已结束。

## 自动检查与限制

已完成本机霍格沃茨的用户验收：测试前 Epic 与游戏均退出，从 Steam 的独立桥接测试条目启动后，用户确认会话结束正常。Shift+Tab 能打开好友窗口和聊天，但未显示通常的半透明黑底遮罩，用户认为不影响使用。后续手柄测试中，手柄能正常使用，但 Steam Input 自定义按键映射疑似未生效；不代表所有游戏或完整 Overlay/Steam Input 兼容性已验收。Epic 预先独立运行的场景仍待测试。

历史对照结论：直启 Epic 映射生效；桌面父进程隔离且清理环境时映射失效、会话正常结束；保留环境则映射恢复，但需退出 Epic 才结束会话；普通父进程并清理环境时所有手柄按键失效、Overlay 无黑底遮罩，会话正常结束。两版对照代码均已撤销。后续认证直启已经通过赤痕和霍格沃茨的用户验证，不使用上述隔离方法。

`mise run test` 覆盖启动超时、取消、入口交接、路径边界、已有游戏、Epic 常驻和隔离失败。可选原生测试启动的只有自身测试辅助进程，不启动真实 Epic/游戏：

```powershell
$env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST = "1"
go test -count=1 -run TestDesktopParentAndNativeSnapshot -v ./internal/bridge
Remove-Item Env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST
```

实验只按游戏 EXE 名称与安装路径识别进程，不遍历整个游戏进程树；其他引导器、第三方启动器和不同名称的游戏进程需手动指定。轮询默认 500ms，持续不足此时间的进程可能漏检；入口与实际游戏间隔超过宽限期时需调整参数。没有启动桥接后自动关闭 Epic 的策略，没有云存档完成检测，也没有认证账号接入。
