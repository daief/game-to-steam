# Epic 桥接实验

这是独立验证程序，不接入客户端，不修改 Steam 快捷方式、状态或游戏文件，不关闭 Epic 或游戏。与主项目共用一个 Go module，无需 go.work。

## 构建与命令

```powershell
mise run bridge-build
.\build\windows-amd64\epic-bridge.exe --list
.\build\windows-amd64\epic-bridge.exe --game "上一步显示的完整游戏标识"
```

PowerShell 直接运行只能验证启动和进程识别；Steam 会话是否正确结束必须从 Steam 启动验证。

- `--launcher "C:\...\EpicGamesLauncher.exe"`：覆盖自动发现路径。
- `--watch-exe "Phoenix\Binaries\Win64\HogwartsLegacy.exe"`：只跟踪这个实际游戏 EXE；相对路径基于游戏安装目录。默认跟踪安装目录内与清单入口同名的所有 EXE，覆盖霍格沃茨入口与 Phoenix 游戏程序的交接。
- `--startup-timeout 5m`：等待游戏启动，默认五分钟；登录、更新耗时可调大。
- `--exit-grace 5s`：同名游戏进程全部消失后继续等待五秒，避免入口与游戏程序交接时过早退出。
- Ctrl+C 只结束桥接，不停止游戏或 Epic。Steam 的“停止”行为不由本实验保证。

诊断日志：`%AppData%\game-to-steam\bridge\logs\session-*.log`，同时输出到控制台。只记录游戏标识、路径、PID、父 PID 和生命周期，不读取或记录游戏命令行、认证令牌。日志包含个人路径，分享前请检查。

## 启动隔离

使用 GetShellWindow 定位当前会话的桌面进程，通过 Go 的 SysProcAttr.ParentProcess（Windows 的 PROC_THREAD_ATTRIBUTE_PARENT_PROCESS）创建 Epic 启动请求进程，使其以桌面进程而非桥接为父进程。移除继承的 Steam 环境变量，不继承桥接句柄，不使用 shell 命令字符串或管理员权限；无法隔离则报错，不回退到普通子进程启动。

父进程属性及继承行为见 [Microsoft 文档](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)。这只保证 Windows 层面的父进程选择，**不保证 Steam 的追踪策略或 Overlay/Steam Input 行为**。尤其 Epic 已被 Steam 启动过时，本实验不会调整现有进程的父进程，需要先完全退出该 Epic，再从桌面独立打开。

## 手动验收

在 Steam 手动添加一个临时非 Steam 快捷方式，目标为 `epic-bridge.exe`，工作目录为它所在目录，启动选项为 `--game "完整游戏标识"`。保留原游戏快捷方式；不要用客户端改写它。

分别验证：

1. Epic 完全退出，Steam 启动临时快捷方式 → Epic 启动游戏 → 退出游戏。
2. Epic 从桌面独立启动并已登录，Steam 启动临时快捷方式 → 游戏启动 → 退出游戏。

两种情况都要求游戏运行时 Steam 显示“游戏中”，游戏退出且宽限期结束后恢复“启动”，Epic 保持运行。记录 Overlay、Steam Input/手柄是否可用；进程隔离可能使其不再自动注入游戏，不能据此宣称等同直接从 Steam 启动。

若入口退出后实际游戏仍运行，日志应看到 Phoenix 路径进程；若实际 EXE 不同，显式设置 `--watch-exe` 重试。启动前已有匹配游戏进程会拒绝启动，避免误跟踪其他会话；候选进程读取失败会报错，不冒充已结束。

## 自动检查与限制

已完成本机霍格沃茨的用户验收：测试前 Epic 与游戏均退出，从 Steam 的独立桥接测试条目启动后，用户确认会话结束正常。Shift+Tab 能打开好友窗口和聊天，但未显示通常的半透明黑底遮罩，用户认为不影响使用。Epic 预先独立运行的场景及 Steam Input 仍待测试；不代表所有游戏或完整 Overlay 兼容性已验收。

`mise run test` 覆盖启动超时、取消、入口交接、路径边界、已有游戏、Epic 常驻和隔离失败。可选原生测试启动的只有自身测试辅助进程，不启动真实 Epic/游戏：

```powershell
$env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST = "1"
go test -count=1 -run TestDesktopParentAndNativeSnapshot -v ./internal/bridge
Remove-Item Env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST
```

实验只按游戏 EXE 名称与安装路径识别进程，不遍历整个游戏进程树；其他引导器、第三方启动器和不同名称的游戏进程需手动指定。轮询默认 500ms，持续不足此时间的进程可能漏检；入口与实际游戏间隔超过宽限期时需调整参数。没有启动桥接后自动关闭 Epic 的策略，没有云存档完成检测，也没有认证账号接入。
