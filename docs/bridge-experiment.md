# Epic 桥接与验收

桥接是独立程序，与主项目共用一个 Go module，无需 go.work。客户端已接入统一策略：识别需要 Epic 启动或勾选强制时生成桥接命令，裸 EXE 仍直启。桥接自身不修改 Steam 快捷方式、状态或游戏文件，不关闭 Epic 或游戏。

`mise run build` 同时构建两个 EXE，部署时放在同一目录；缺失桥接时客户端生成 Epic 型命令会报错，不回退。生成的命令包含 `--game`、当前 `--manifest-dir` 和 `--launcher`，保持客户端设置一致。已有快捷方式及接管的原命令不自动改写；在启动设置里按策略生成、确认草稿，退出 Steam 后保存即可迁移，AppID 和图片保留。也可手动保留原 Epic 启动器命令。

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

## 启动方式对照实验

本轮对照实验已撤销，代码恢复到桌面 Shell 父进程隔离并清理 Steam 环境变量的基线实现；客户端仍保留已提交的桥接接入，并未切换默认策略。基线会话结束可用，但 Steam Input 映射不生效，属于未完成方案。无法隔离时直接报错，不回退普通子进程启动；不继承桥接句柄、不关闭 Epic 或游戏、不输出环境变量值。

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

用户确认直接启动 Epic 时 Steam Input 布局修改生效；桌面父进程隔离且清理环境的基线映射不生效，但会话正常结束。保留桌面父进程隔离、继承 Steam 环境的对照版映射恢复；游戏退出后桥接结束、Epic 常驻时 Steam 仍显示游戏中，直到手动退出 Epic 才结束，Overlay 未测。普通父进程并清理环境的反向对照版则手柄任一按键均不生效，Overlay 仍无黑底遮罩，会话结束正常。两版对照代码均已撤销。当前尚未找到兼顾输入与会话结束的方案；后续可研究实际游戏与 Steam 关联的其他方式，默认直启 Epic/桥接可选仅为建议，尚未实施。辅助进程测试不等同真实游戏兼容性验收。

`mise run test` 覆盖启动超时、取消、入口交接、路径边界、已有游戏、Epic 常驻和隔离失败。可选原生测试启动的只有自身测试辅助进程，不启动真实 Epic/游戏：

```powershell
$env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST = "1"
go test -count=1 -run TestDesktopParentAndNativeSnapshot -v ./internal/bridge
Remove-Item Env:GAME_TO_STEAM_BRIDGE_NATIVE_TEST
```

实验只按游戏 EXE 名称与安装路径识别进程，不遍历整个游戏进程树；其他引导器、第三方启动器和不同名称的游戏进程需手动指定。轮询默认 500ms，持续不足此时间的进程可能漏检；入口与实际游戏间隔超过宽限期时需调整参数。没有启动桥接后自动关闭 Epic 的策略，没有云存档完成检测，也没有认证账号接入。
