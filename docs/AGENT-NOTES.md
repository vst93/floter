# AGENT NOTES — floter 迭代上下文

给在本仓库工作的 AI agent 看的项目级备忘：产品方向、磁盘格式约定、门槛与纪律。
构建/运行命令见 [`mygo-rewrite-status.md`](mygo-rewrite-status.md)，分轮决策记录见
[`mygo-rewrite-plan.md`](mygo-rewrite-plan.md)。

## 这是什么

floter = 一个常驻的浮动面板，一个快捷键唤出，面板里三件事：**启动器**（搜应用 / 文件 /
算式 / 扩展命令）、**终端**（ghostty 内核）、**设置**。用
[mygo](https://mygo.egoist.dev)（原生 GPU 自绘 UI，无 webview）+ Go 实现。

仓库里**只有 Go**：`cmd/floter` 入口，`internal/*` 各功能包。历史上曾用
Tauri + Rust + React 实现（见 `docs/archive/` 与 git 历史），那套代码已删除。

## 插件体系方向（2026-08-23 用户确认，仍然有效）

1. **去 NPM 强依赖**：NPM 分发不作为核心路径；围绕它建的信任栈优先级下降。
   （npm 安装能力保留，用于自建 registry / 私有包，但产品叙事不依赖它。）
2. **发现优先，用户介入越少越好**：PATH 扫描 + 约定位置 manifest + 本地连接都应能自动
   识别注入工具；不强制一套规则适配所有工具。
3. **v-tools 平权**：`extensions/v-tools` 只是"推荐工具"的参考包，和其他扩展走同一套代码路径。
4. **管理面板尽量轻**：安装/连接、开关、卸载；不做商店式 Discover。
5. **权限=诚实披露**：声明权限 + 明确告知非沙箱（`environment` / `process-spawn` 两条宿主
   真的拦，其余只披露，UI 上如实标注），不假装能强制执行。

## 磁盘格式：零迁移是硬约束

用户数据文件**格式不变**，换实现不能让用户重配。任何动这些文件的重构，先看这条：

| 文件 | 位置（`<config dir>` = `os.UserConfigDir()`） | 谁写 |
|---|---|---|
| `floter/settings.json` | 应用与终端全部设置 | 设置页 / 快捷键录制 |
| `floter/extension-repository.json` | 扩展权威状态（未知键原样保留） | 安装 / 启停 |
| `floter/extensions/<id>/` | 扩展包目录 | graft 安装 |
| `floter/extension-data/<id>/config.json` | 扩展配置（含密钥代数） | 配置注入 |
| `floter/clipboard-history/index.json` + `images/` | 剪贴板历史 | 剪贴板监听 |
| `floter/usage.json` | 启动次数（最近使用） | 每次启动应用/命令 |

**未知键逐层保留**是这套约定的实现方式：读进来 → 改自己认识的键 → 原样写回。
新增设置项时不要"重写整个文件"，也不要删掉不认识的值。

**首次运行迁移**（`internal/settings/migrate.go`、`windowstate.go`）：`settings.json`
与旧版同路径，所以常规情况下 `Load` 已经继承；迁移只覆盖不同路径/格式的遗留物——
旧版的 `settings.json.backup` 恢复副本、Tauri identifier 目录
（`com.v.floter` 等）、pre-glass-step 文件的单值玻璃强度拆分，以及旧版
`plugin-window-geometry.json` → 框架 `window-state.json`。迁移幂等（`migrated-from-old.json`
即标记）、不覆盖已有数据、迁移前备份，返回 `MigrationReport` 供无头测试断言。

## 轮次档案：spec 与报告进仓库，不进 /tmp

每一轮的**规格**与**验收报告**是项目文档，不是临时产物。`/tmp` 会被清，攒在里面的决策等于丢掉。

约定：

| 文件 | 位置 | 内容 |
|---|---|---|
| 规格 | `docs/rounds/<轮号>-spec.md` | 动手前写：意图、逐条落地项（file:line）、零改动红线、门槛与基线数字、mutation 计划 |
| 报告 | `docs/rounds/<轮号>-report.md` | 收尾写：V/B/S 台账、门槛实测数字、mutation 的原始报错串、仍待 |
| 残骸 | `docs/rounds/<轮号>-orphan.diff` | 仅当一轮被打断、主线清树时留档 |

- 一轮一个轮号（`P9-x`、`P9-y`…）；同一轮被打断后重派沿用轮号，用 `-retry` / `-orphan` 后缀区分，
  别另起轮号把历史撕开。
- 这些文件随本轮的 `git add -A && git commit` 一起进仓库。
- **报告必须增量写**（每完成一步就落盘）——攒到最后一刻，一旦被打断，整轮的账就没了。
- 下一轮开头读的是 `docs/rounds/` 里**最新那份 report**，不是 /tmp、也不是记忆。

已归档：`P9-x`（含第一次被打断的 orphan diff 与两份 spec）、`P9-z`。

## 门槛（每次改动都要过）

```sh
gofmt -l cmd internal     # 必须为空
go vet ./...
go test -count=1 ./...    # 全绿；平台相关断言必须在任何平台都成立
GOOS=linux go build ./... && GOOS=windows go build ./...
go tool mygo build        # macOS 打包产物（可选，改动打包相关时跑）
```

**平台无关性**：CI 跑 ubuntu / macos / windows 三平台矩阵。任何平台相关的断言要么
按 `runtime.GOOS` 门控，要么把平台表写成 `(goos, home, ...)` 的**纯函数**，让每个平台的
布局在任何宿主上都能被断言（`internal/browser` 的 `chromiumBrowsers` 就是这么做的——
旧实现曾因"只在 macOS 编译"的路径表把 Edge 的目录写错而没人发现）。

**测试不碰真实系统资源**：剪贴板、登录项、窗口、原生库（libghostty-vt / sqlite）都要能注入
（`shell.Options` 里的 `WriteClipboard` / `RegisterShortcut` / `OpenAtLogin` / `OpenPinned` /
`ConfirmPermissions` / `Registry` / `NewTerminal` / `Paths`）。需要真资源的测试用环境变量
opt-in（`FLOTER_TERMINAL_TEST=1`、`FLOTER_SQLITE_TEST=1`、`FLOTER_REGISTRY_TEST=1`）。

## 反馈通道与三态

- **唯一反馈面是宿主 toast**（`launcher` 的 `toast` 字段 + 窗口底部提示）。任何表面都只发
  消息、不画自己的提示条；时长与视觉只有一份来源。
- **三态**（空 / 加载 / 错误）适用于任何会拉数据的表面：空态说"没有东西 + 为什么/怎么办"；
  加载态是**行内** spinner，有旧内容就不换掉它；错误态给"重试（重试真可能成功时才有）+ 关闭"，
  同一错误重复渲染要复用已画节点，否则重绘会丢焦点。

## 纪律

- 每轮改动自带测试；先跑门槛再提交。
- 报告里"已做 / 未做"要如实分开写，未做的不要写成半成品。
- 用户可见文案走 `internal/i18n`（en/zh 结构体，缺字段编译期报错），不要散落字面量。

## 2026-10-11 · P7（mygo v0.4.0、单位、插件表单、五个没接线的动作）

- **升级到 mygo v0.4.0**（新增 `ui.WebView`/`Window.NewWebView`、`OnPaste`、`Dismissed`、
  `InsetShadow`；本应用不需要 WebView，插件是 CLI 工具不是网页）。
- **不要忘了 `SetMenu(nil)`**：框架在从未调用 `SetMenu` 时会装一份默认应用菜单，Linux/Windows 上
  它画在窗口*内部*（浮动面板顶上一条 File/Edit/View）。非 macOS 显式清掉。
- **没有 backdrop 的平台不要画 haze**：`c.Vibrancy()` 为 false 时（Linux、Windows 11 之前），
  面纱不是"盖住模糊"而只是"加不透明"，半屏面纱会把玻璃变成实心板。
- **单位**：本仓库的 `Space(n)` = 旧版 4u；正文 `t.FontSize` 必须是 `11 * ui-scale`（框架默认
  13/14pt 大了四分之一，会让每行都比原型高）。`internal/theme.BodyUnits` 是那个 11。
- **启动器窗口高度**：`66 + rows × 42`（u）——输入行 56u + 呼吸 4u + 面板上内边距 4u + 尾 2u。
  `internal/launcher/height.go` 的常量与 shell 画的卡片内边距必须同时改，否则卡片比窗口高/矮。
- **`Shortcut()` 会消费按键**：`if a.pending != "" && c.Shortcut(...)` 而不是反过来，否则一个
  总是被问的 Escape 会把后面所有 Escape 处理都吃掉（踩过一次）。
- **审计 action 接线**：曾经有 5 个 `launcher.Actions` 字段在 shell 里是 nil（电源、打开路径、
  在目录里开终端、外部终端、安装会话），功能静默失效。现在有反射测试钉住"shell 必须回答每个
  action"，其他包也可以照做。
- **mygo 的 `Tester.Texts()` 只包含这一帧构建过的元素**：视图里的早退 `return` 会把它后面的所有
  元素都跳过（输入行曾经因此整行消失）。

## 2026-10-11 · 追加：ANSI、尺寸账本、选项重命名

- **输出里的 ANSI**：`internal/ansi` 把 CLI 的 SGR 读成 span，非 SGR 转义丢弃；输出视图用
  `theme.ANSI(appearance)` 的 16 色画，复制走 `ansi.Plain`。
- **画出来的必须等于记账的**：行高要用 `c.MeasureText` 实测（框架行盒 = 字体 ascent+descent+gap，
  14pt → 22px，不是 1.4×）；新增的 `TestChargedHeightHoldsTheList` 把每个状态渲染在记账高度的窗口里，
  任何一行出界即失败。加新带（chips/status/sheet/…）时先加进这个测试。
- **`shell.Options.ConfirmPower` 改名 `ConfirmDialog`**：电源确认已经是面板内的行，这个 seam 只服务
  设置页的"清空剪贴板历史"。

## 2026-10-11 · 追加：脚本工具、召唤归位、终端多会话

- **终端会话模型**：`terminalui.Session`（Term/Title/Resident/ExitCode/Created）+ `App.Sessions`/`Active`。
  取当前会话用 `a.Current()`，终端用 `a.term()`。运行命令/在目录开终端/安装会话都**开新会话**，
  不再顶掉旧的；`Close()` 只关当前，`CloseAll()` 在退出时用。
- **测试里的假终端**：`&terminal.Terminal{}` 只当"存在"的占位，**不能 Close**（内部 vt 为 nil 会 panic）。
  要测关闭路径就自己拼 `Sessions`（`Term: nil`），或设 `FLOTER_TERMINAL_TEST=1` 用真库。
- **本地工具可以是脚本**：`internal/scriptlang`（八语言 + 模板 + 状态投影）、`extensions.ScriptRuntimeFor`
  （从不失败）、表单的 kind 选择器；草稿字段走 `customRequest` 一个映射。
- **召唤会重新归位**：`Show()` 前后各 `placeWindow()` 一次（macOS 丢弃未映射窗口的几何设置）；
  屏幕用窗口中心点判定（框架没有"焦点屏幕"查询）。
- **终端链接**：`Options.OpenLink` / `OnNotify` 已接线（`openTerminalLink` / `notifyFromProgram`）；
  `Options.OpenURL` 也是可注入的 opener（测试用）。

## 2026-10-11 · 追加：行内控件、注册字号、mygo v0.4.1

- **行内控件**：历史行的星标（点击收藏不运行行）、编号徽标 = 存储绑定 + 数字、设置头带的退出/关闭。
- **字号注册**：行的标题/副标题、设置的标签/次行、分组标题/提示、页头/副标题，全部按旧版的阶梯
  （body 11u、emphasis 12u、title 13u、display 17u；本仓库 `Space(n)` = 4u）。
- **mygo v0.4.1**：新增 `NewFlyout`（Tray/Parent 锚定、Placement、Popover、Focusable）与
  `ListState.RoundRows`。Flyout 用于"必须伸出窗口边缘的弹出层"——本应用的弹出层（select、chips 齿轮、
  插件表单）都在窗口内，mygo 会把窗口内的弹出层夹回窗口，所以暂不需要；分离输出/钉住窗口必须
  **活得比面板久**，Flyout 由 parent 拥有、不可用。
- **仍被框架挡住的细节**：`⌃⌫` 清空命令历史（编辑器先吃 Backspace/Delete）；会话的 cwd 行
  （插件不报告会话 cwd）。~~行内的 source 槽~~、~~插件列表的分页页脚~~ 都已完成（分页页脚见 P7-s，
  source 槽见下面一节）。

## 2026-10-13 · 对照原版补齐：行的 source 槽（右侧固定槽）

- **发现**：旧版 `row-content.ts` 的 `resultRowContent` 给每行算出两个可选字符串——`subtitle`
  与**右侧固定槽** `source`（`.launcher-result__source`：`flex: 0 1 auto; max-width: 30%`、
  `text-align: right`、tertiary，选中升一档）。`source` 对 `command` 行是**贡献该命令的集成名**，
  对 install 行是**包管理器**（`displayName`）。本构建此前把这两个事实塞进了 `Detail`
  （`commandItems` 的 `detail = description || IntegrationName`），install 行则把管理器放进了
  `Shortcut` 槽——同一格既当键位又当出处。于是有描述的命令行**看不到**它来自哪个集成。
- **修法**：`launcher.Item.Source`；`commandItems` 的 `Detail` 只放 description、`Source` 放
  `IntegrationName`（有描述时两者并存，而不是一个顶替另一个）；`toolInstallItems` 的
  `Source = tools.ManagerName(...)`（`brew` → `Homebrew`，管理器的 proper noun），`Shortcut` 留空；
  命令模式的补全行同样带 `Source`。
- **右端成组**：行的右端按旧版顺序：**警告点、`␣` 键帽、source、收藏星、行控件、编号、键位**。
  之前警告点与 `␣` 键帽画在标题行**内**（标题文字之后、行中间），现在它们是文本列的**兄弟**，
  因此窄窗口的省略号先吃掉标题而不是它们——这正是「出处」最需要可见的时候。
- **测试**：`TestCommandRowCarriesItsIntegrationAsItsSource`、`TestCommandModeRowsCarryTheSource`、
  `TestInstallRowNamesTheManagerAsItsSource`、`TestManagerName`、
  `TestTheSourceWordSitsAtTheRowsRightEdge`（source 在标题右侧、编号徽标左侧）、
  `TestTheRowsMarksSitAtTheRightEdge`（`␣` 在标题与副标题的右外侧）。
- **文本寄存器单行**：标题与副标题现在 `SingleLine()`（旧版 `.launcher-result__title` /
  `__subtitle` 的 `white-space: nowrap; text-overflow: ellipsis`）。此前是普通文本，超长时会
  **换行**——画出的行比窗口记账的高。source 槽把文本列变窄，所以这一步与它同做；
  `TestLongRowTextIsCutNotWrapped` 钉住超长标题/副标题仍是一行。

## 2026-10-11 · 追加：scope 标记（模型改造完成）

- **插件模式的状态模型现在是旧版的**：进入模式消耗触发词并清空字段（`enterClipboardWord` 等设
  `modeWord` + `Query=""`），模式内搜索把字段当 needle（`clipboardQuery`/`browserQuery`/`calculatorQuery`/
  `filesQuery` 直接返回 `a.Query`，`commandArgs`/`commandTokens` 不再剥首词），`syncCommandMode` 变成
  no-op（模式只由用户离开），字段行左侧画 scope 标记（`scopeMark` + `glyphClipboard`）。
- **离开语义**：Esc 走 `leave*`（**保留** needle 为普通查询，R31）；**运行**路径（`runClipboard`/`runMode`/
  browser 运行）在离开后**清空** `a.Query`（旧版自己运行后 setQuery("")）。
- 注意：`syncTypedMode` 的 files 分支必须走 `enterFiles()`（直接设标志不会消耗触发词）。
- 测试：直接操作状态的旧测试要按新语义改（needle 即字段文本；`"clip "` 变 `""`）。

## 2026-10-11 · 追加：Linux 生命周期（用户报告的"启动应用后 floter 自动退出"）

两个机制，都已修复：

1. **`.desktop` Exec 的子应用共享 floter 的进程组与会话**——`apps.Open` 用裸
   `exec.Command`（stdio detached 就完了）。信号（SIGHUP/SIGINT/SIGTERM）发给进程组时
   两边都收到。现在 `detachExec` 走 `Setsid`（Linux）/ `CREATE_NEW_PROCESS_GROUP |
   DETACHED_PROCESS`（Windows），后台 `cmd.Wait()` 收尸——`internal/spawn` 自己的
   detach，重述而不引用（避免 apps→spawn 的依赖方向反转）。
2. **mygo 只处理 SIGINT/SIGTERM，SIGHUP 走默认行为 = 死**。从终端启动的面板在那个
   会话里：面板隐藏后 shell 回到提示符，**关闭终端 = SIGHUP 发给整个会话**。
   `shell.Start` 里 `signal.Notify(SIGHUP)` + 忽略（真退出仍走 tray/行/关窗的 quit 监听）。
   已用 Xvfb 验证：SIGHUP 后存活、SIGTERM 后优雅退出。
- **注意**：`syscall.SysProcAttr{Setsid}` 是 unix-only，跨平台要走 build-tagged 的
  detach（`internal/apps/detach_unix.go` / `detach_windows.go`）。

## 2026-10-12 · 对照原版补齐（本轮）

按用户要求全面对照 Tauri 原版（`backup/main-pre-mygo`）排查剩余偏离，修了四处**用户可感知**的
行为差异（都在 `internal/launcher` / `internal/terminalui`）：

- **计算器模式不再把字段当搜索词**：原版 R50 明确「字段是待算算式，不是历史搜索 needle」，
  用算式过滤会在用户正写新算式时把历史全藏起来。历史只受 chip（全部/收藏）过滤。
  顺带修正行寄存器：历史行副标题是**时间**（今天 `HH:MM` / 昨天 / 日期），结果在**右端** trailing
  ——此前结果被画了两遍（副标题 + trailing）。答案行也回到 R50 寄存器（算式为标题、答案在右）。
  新增 `i18n.Launcher.CalculatorYesterday`。
- **浏览器触发词决定进入的档位**：原版 `parseBrowserMode` 里 `bookmarks/bookmark/书签 ` 进书签档、
  `history/hist/历史/历史记录 ` 进历史档、`browser/浏览器 ` 进全部档；本构建此前一律进 `all`，
  且缺 `hist/历史/历史记录`。
- **URL scheme 大小写不敏感**：原版 `/^(?:https?|ftp):\/\//i`，本构建漏了 `i`，粘 `HTTPS://` 会当命令跑。
- **应用行不再显示另一语言的同名**：原版 R14 把「企业微信 / WeCom」「QQ音乐 / QQMusic」这种
  副标题删了（「还是很丑，清爽一点」），只在搜索里保留；本构建此前把它当 Detail 显示（行也更高）。
  现在标题取本地化名、Detail 空，两个名字 + 别名 + 拼音首字母都可搜。
- **终端栏内容随 hover/focus 显现**：原版 `.terminal-bar__identity` 与 `.terminal-panel__actions`
  都是 hover/`:focus-within` 才显；本构建此前只藏了标题，按钮常驻，遮住终端首行。现在整组按钮
  也 `Opacity(0).PassThrough()`（隐藏时不吃点击），`bar.FocusWithin()` 负责键盘到达。

## 2026-10-12 · 追加：mygo v0.4.5（Draggable，本应用不用）

- **版本**：`go.mod` 升到 `v0.4.5`（#244 新增 `Element.Draggable` / `DragOffset` / `ui.Nearest`：
  指针拖动元素并抛出，Bounds / Snap / 弹簧收尾；其余差异都是为它铺的内部改动）。
- **为什么不用**：floter 没有「列表项排序」；设置里没有靠拖动定位的元素；唯一的值拖动控件
  （设置页 / 插件表单的滑块）是框架自带的 `ui.Slider`，窗口移动是原生 `DragWindow()` 拖动区。
  历史行的删除是**选中后二次确认**（控件只在选中态出现，防误触），再加一套「横向拖走删除」是
  第二套删除语义、且拖拽与「点行即运行」的手势重叠——按纪律不为凑 API 改产品行为。
- 以后真做「可重排列表」或面板内 bottom-sheet 时，再回来接 `Draggable`（`internal/` 里目前没有
  任何 `ui.Point` / `AnimatePose` / 自定义拖动手势）。

## 2026-10-12 · 追加：窗口定位回归 + 背景真透明

用户报告两件事：「窗口定位也被修坏了」「应用整体有不透明的底，配置了透明度没效果」。

### 1 · 窗口定位：把 mygo 的 DIP 当成 Tauri 的物理像素，除以 scale 两次

- **引入点**：`de5816c feat(go): the panel is placed in the display's own work area`
  （`internal/shell/shell.go` 的 `placeWindow` + `placement`，两者同一次提交引入）。
- **根因**：`placeWindow` 先把 `display.WorkArea` 各分量 `/ scale`，再调
  `placement(..., scale, ...)`，而 `placement` 内部又 `/ scale` 一次——
  **同一个 scale 除了两遍**。这段除法是从旧版 Tauri 抄来的：Tauri 的
  `monitor.work_area()` 是**物理像素**（`src-tauri/src/lib.rs` 的 `default_position`：
  “A monitor's reported position is physical … while `set_position` converts … back”），
  所以旧版除以一次是对的。但 mygo 的 `Display.Bounds` / `WorkArea` 本来就是 **DIP**
  （`internal/platform/platform.go`: “Rect is a rectangle in screen coordinates (DIPs)”；
  mygo 自己的居中 `window_state.go` 的 `initState` / `platform.Centered` 都不除 scale）。
  于是 1× 屏看不出，2× 屏上工作区被缩成一半，720 宽的面板被 clamp 到左上角。
- **修法**：`placeWindow` 直接把 DIP 工作区交给 `placement`；`placement` 去掉 `scale`
  参数与除法。`TestPlacementIgnoresTheDisplayScale` 钉住「2560 DIP 的工作区把 720 宽的
  面板居中在 920」，`TestPlacementIsCentredAndClamped` 的 2× 档同步改成 DIP 值。

### 2 · 背景真透明：Windows 上 `VibrancyUnderWindow` 落到不透明的 Mica

- **根因**：窗口无条件用 `mygo.VibrancyUnderWindow`（`internal/shell/shell.go` 的
  `Start()`）。mygo 的 Windows 后端里 `backdropFor` 只显式认 `mica` / `tabbed` /
  `acrylic` 等，**其余一律 fall through 到 `backdropMica`**
  （`$(go env GOMODCACHE)/github.com/egoist/mygo@v0.4.5/internal/windows/window.go`
  的 `backdropFor` 最后一行 `return backdropMica`）。Mica 是**不透明的壁纸着色**，不是
  模糊；窗口于是有一层不透明的底，`main_opacity` 只改场景内 tint 的 alpha，用户看到
  「拖了没反应」。macOS 的 `under-window` 是半透明 NSVisualEffectView（material 21），
  Linux 无材质（tint 的 alpha 就是全部玻璃），所以只有 Windows 中招。
- **修法**：新增纯函数 `vibrancyFor(goos)`：Windows → `mygo.VibrancyAcrylic`
  （会真正模糊身后的窗口，对应 `backdropAcrylic`），macOS → `VibrancyUnderWindow`，
  Linux → `VibrancyNone`。`TestVibrancyMaterialIsTranslucent` 钉住这三个配置值；
  `TestResolveCarriesMainOpacityIntoTheFill` 钉住「设置页写的 `main_opacity` 确实到达
  渲染层（`Resolve(...).Glass.TintAlpha`），且滑块越低 fill 越薄、默认不是不透明」。

## 2026-10-13 · 对照原版补齐：声明式输入的行内校验真的接线了

- **发现**：文档 P7-o 声称「选择型缺选项、布尔型缺旗标都会在该行下面印一条警告行」，但
  `extensions.ParamRowIssues`、`settingsui.issueUnder`、`i18n.Settings.ParamNeedsOptions`/
  `ParamNeedsFlag` 三样都**没有调用点**（`ParamRowIssues` 的注释与代码还自相矛盾：注释说空 id
  不产生 issue，代码却写了 `"id"`——没人用才没暴露）。表单照旧把笔误弹回后端。
- **接线**：`toolParamsEditor` 用 `editorParams()`（与编辑器逐行对应、空 id 保留）算
  `ParamRowIssues`，把 `ParamRowIssue` 映射成文案，用 `issueUnder` 印在**导致它的那一行下面**；
  `localToolSection` 的保存按钮在任何 issue 下**不提交**。`declaredParams` 现在复用
  `editorParams` 再丢空 id 行，两者不会分歧。
- **规则对齐后端 `ValidateParams`**：非法 id / 重复 id / 选项型无选项 / 布尔型无旗标 / 非单 token
  旗标；重复只怪后出现的行，空 id 行不算声明。文案用旧版原文（`ParamInvalidID` 等）。
- **注意**：`issueUnder` 是**文本**不是 `a.row`，不参与卡片行计数器（不画分隔线）。
- **测试**：`TestParamRowIssues`、`TestConnectFormNamesAnUnfinishedInputAndRefusesToSave`。

## 2026-10-13 · 第二次报告：透明度仍失效 + 模式切换重置窗口位置

用户报告两件事：「透明度还是有问题，底部还是非透明的，无论怎么调节应用透明度都不起作用」
「在设置页面拖动了应用的位置，返回搜索框时，位置被重置到屏幕中央了」。

### 1 · 透明度：上一轮修的是**材质**，而挡住滑块的**是内容面**

- **上一轮（`7caa60b`）做了什么**：把 `Start()` 的 `VibrancyUnderWindow` 换成
  `vibrancyFor(runtime.GOOS)`（Windows → Acrylic、macOS → under-window、Linux → None），
  并加 `TestResolveCarriesMainOpacityIntoTheFill` 钉住 `Resolve(...).Glass.TintAlpha`。
- **为什么没生效**：`main_opacity` 真正移动的是**帧的 tint**（`glassmap.Frame`，
  `internal/shell/shell.go:2652` 的 `face.Background(fill)`），但帧上面还压着一层
  **内容面**：`internal/theme/theme.go:246` 把 `Sunken` 算成固定带
  `0.62 + 0.18 * main_opacity`（10% → 0.638，100% → 0.80），
  `internal/launcher/launcher.go:693` 和 `internal/settingsui/settingsui.go:555/563`
  用它铺满结果列表 / 设置正文。合成 alpha = `1-(1-pane)(1-tint)`，
  于是整块背景在滑块两端只从 ~0.86 动到 1.0（light）/~0.83 到 1.0（dark），用户读作
  「拖了没反应」。**窗口材质在这层 fill 的后面**，改 Mica→Acrylic 动不了这个数字——
  上一轮的测试只钉了被遮住的 `TintAlpha`，所以它绿着而用户看不到变化。
- **原版怎么做**：`backup/main-pre-mygo` 的 `src/glass-material.ts` 把 `main_opacity`
  定为「帧的唯一 alpha 真值」：`--glass-frame-alpha = min(--glass-solid-top, max(floor, opacity))`，
  `--glass-tint-alpha = 1-(1-dim*(1-op))*(1-frame_alpha)`（`src/styles/base.css:417-426`）。
  内容面 `--glass-content-alpha` 是固定带（`base.css:461`），而浮动面板
  `--glass-float-alpha = var(--glass-tint-alpha) + 0.1`（`base.css:471`）——即「比帧厚一档、
  但仍跟着滑块走」。原版把透明度的可见性押在**帧**上（launcher 还另有 0.74 的地板）。
- **修法**：`glassmap` 补上原版的 `--glass-solid-top: 0.98` 上限（`SpecFor`），并新增
  `Spec.ContentAlpha()`（内容面就是帧自己的一层，骑在滑块上）和 `Spec.FloatAlpha()`
  （`tint + 0.1`，原版的浮动公式）。`theme.For` 用它们替换固定的 `Sunken` 带与错误的
  `Float`（`theme.go:250-252`）。这样应用自己的背景才是滑块移动的东西；
  实测 dark 0.76→1.0、light 0.81→1.0。
- **测试**：`internal/shell/transparency_test.go` 渲染 launcher 两端并读内容面 alpha
  （旧代码在 light 档 0.863→0.996 会失败）；`glassmap.TestContentAndFloatRideTheSlider`；
  `theme.TestSunkenFollowsTheOpacity` 改成钉新语义；`glassmap` 的 100% 档改成钉 0.98。

### 2 · 位置重置：`Open()` → `Show()` 每次 reveal 都 `placeWindow()`

- **触发点**：设置页的关闭 / ⌘W / Escape 走 `settingsui.Actions.Close`
  （`internal/shell/shell.go:529`）→ `a.Open(SurfaceLauncher)`；`Open` 末尾无条件
  `a.Show()`（`shell.go:2228`），而 `Show()` 无条件 `a.placeWindow()`
  （旧 `shell.go:2452`）。于是「回到搜索框」= 一次 reveal = 一次居中。
- **原版怎么做**：`backup/main-pre-mygo` 的 `show_input`（`src-tauri/src/lib.rs:1654`）
  只在 `!preserve_anchor`（`preserve_anchor = state.window_visible`）时调
  `move_to_default_position`——窗口已经可见时，模式切换**不**重新定位；
  只有从隐藏被召唤（`reveal_saved_mode`，`lib.rs:1129`）才归位到当前显示器。
- **修法**：`Show()` 先读 `a.Win.IsVisible()`，只有**从隐藏到显示**才
  `placeWindow()`（前后各一次，macOS 未 map 的窗口会丢几何）；可见窗口的 reveal
  只 show/focus。决策抽成纯函数 `placementForReveal(wasVisible)`。
- **测试**：`shell.TestPlacementOnlyOnAFreshReveal`。

## 2026-10-13 · 对照原版补齐：字段自己的两个键（⌘⌫ 删除历史行、⌃⇧⌫ 清空命令历史）

- **发现**：旧版 `useLauncherActions` 里有两个手势是**搜索字段自己**回答的——`CmdOrCtrl+Backspace`
  （⌘⌫ / Ctrl+⌫）装填并确认选中历史行的两步删除，`Ctrl+Shift+Backspace` 清空输入过的命令历史。
  本构建把前者记成「框架限制」（能力只留在行尾 ✕ 控件上），后者**完全没有实现**（命令历史没有
  清空的方式）。
- **框架的按键顺序**（`ui/input.go` 的 `keyDownPhysical`）：
  1. 聚焦元素的 `HandleInput`（`!rt.claimed(k)` 时）；
  2. **编辑器**的 `wants`（**没有** `claimed` 守卫）；
  3. 快捷键注册（`c.Shortcut` / `Element.Shortcut`）。
  所以 `c.Shortcut(Cmd, KeyBackspace)` 永远拿不到键：`editor.wants` 对带修饰键的 Backspace 一律
  返回 true，而且它排在快捷键之前。**`Element.HandleInput` 挂在字段上**是唯一先于编辑器的缝
  （返回 false 就原样落到编辑器，指针事件也照旧）。
- **落地**：新文件 `internal/launcher/fieldkeys.go`；字段加 `.HandleInput(a.fieldKeys)`。
  装填从「行号」改成「行的 item id」（`armedDelete string`），列表在装填期间移位只会重新装填；
  任何其他键解除装填，Escape 先回答它且不离开模式；离开模式也清装填；**离开启动器表面
  （`shell.Open`）与窗口失焦（`OnBlur`）也清装填**（旧版 `window blur` 的规则）；删除后的选中
  回到旧版 `selectionAfterRemoval` 的「邻居接管选中，不跳顶」（此前是跳回第一行）。
- **测试的坑**：`tt.Type("x")` 只发文本、不发 keydown，`HandleInput` 看不到——测按键规则要用
  `tt.TypeKey(0, ui.KeyX, "x")`。剪贴板行的 `Item.ID` 是 `"clip:"+entry.ID`，别拿 `entry.ID` 比。

## 2026-10-14 · 第三次透明度报告 + Super 键交互

### 1 · 透明底：内容面把帧 tint 又画了一遍

- **前两次为何无效**：第一次只改窗口材质（在场景下面），第二次把内容面改成「骑滑块」却把它的
  alpha 设成**等于帧的 tint**，于是卡片 + 内容面叠成 `1-(1-t)²`，再叠固定 sheen——底部最低
  仍是 0.76（dark）/0.82（light），滑块只把底部推 0.76→1.0，用户读作「不透明、拖了没反应」。
- **修法**：内容面只带 `frame + 0.06` 凹槽的**过帧 alpha**（`glassmap.ContentAlpha` 用
  `overFrame`），sheen 按帧 tint 缩放（`theme.scaleAlpha`），终端 `inherit` 主题改
  `Transparent: true` 让卡片 tint 透出。详见 `docs/mygo-rewrite-status.md` 的 P7-w。
- **纪律**：透明度的验收是**无头生产帧的像素 alpha**（`shell/transparency_test.go`），
  不是内部 token；改前/改后表在 status 文档里（底部幅度 dark 0.239→0.430、light 0.180→0.378）。
- **平台限制**：Windows 的 mygo surface 只在 Windows 11 22H2（build ≥22621）+ DirectComposition
  下开 `WS_EX_NOREDIRECTIONBITMAP`；否则 `PresentPixels` 走 `SetDIBitsToDevice`（忽略 alpha），
  窗口不透明。这是框架侧的限制，应用侧无法绕过。

### 2 · Super 键：松手高度不回退

- **根因**：`internal/launcher/height.go` 的 `HoldRows` 带一行 `Hysteresis`，而持有行恰好只
  加/减 1 行 → 松手时 `target < current-1` 为假 → 高度保留。
- **原版**：`src/App.tsx` 直接 `launcherListUnits(displayedResults)`，`resolveLauncherRows` 是
  死代码；R66 注释写明「窗口精确跟随内容」。且 `src/launcher/terminal-row.ts` 的
  `bareTerminalPlacement` 对**非空 query 返回 null**——「有 query 时不出现」是原版语义，保留。
- **修法**：`HoldRows` 精确跟随内容；测试见 `shell/heldmodifier_test.go`。

## 2026-10-13 · mygo v0.4.6：逐条评估与采用

- **版本**：`go.mod` 已是 `v0.4.6`（无需升级），门槛基线先跑绿。八条 release notes 逐条读
  源码（`$(go env GOMODCACHE)/github.com/egoist/mygo@v0.4.6/`）与 v0.4.5 对 diff 后：

  | release notes | 本项目用点 | 结论 |
  | --- | --- | --- |
  | #253 WebView 背景可改 | 无 webview：窗口是 `Content: ui.View(...)` 的原生 UI，`Window.SetBackgroundColor` v0.4.5 就有 | **不用**（无 web view；窗口 Transparent+Vibrancy，填底色反而毁透明） |
  | #254 终端 `Options.OnScroll` 接管滚轮 | 终端是本机 PTY，scrollback 在插件内 | **不用**（该钩子给「历史在对端」的 `Conn`，本机无对端；插件不暴露自定义滚动步长的接口） |
  | #252 进场元素非动画轴跟随布局 | 启动器行/反馈条/终端便签的 `Enter` | **自动生效**（`ui/transition.go` 的 `follow`，无需接线） |
  | #248 文本布局缓存有界工作集 | 启动器列表、终端文本 | **自动生效**（`internal/text/layout.go`） |
  | #257 仅在视图显示焦点时因焦点变化重绘 | 隐藏/唤出频繁的面板 | **自动生效**（`ui/input.go` 的 `focusChanged`，省 GPU 驱动内存） |
  | #260 窗口状态存进应用自选的 store | 独立插件窗/钉住文本窗的 `StateKey` | **采用**（见下） |
  | #249 从按钮内部拖动 | 本应用无 Draggable 列表（v0.4.5 已判「不用」） | **不用**（结论不变：没有可重排列表/拖动定位控件） |
  | #258/#259 裁剪遮罩按 run 光栅化 + 跨帧保留 | 圆角卡片、滚动裁剪 | **自动生效**（`internal/raster/clipmask.go`，多核） |

- **采用 #260 `App.SetWindowStateStore`**：新增 `settings.WindowStore`（`windowstore.go`），
  把框架的窗口几何存进 **settings.json** 的 carried key `window_states`——与其余设置同一文件、
  同一个原子写与备份回退。`window-state.json`（迁移产物）作为**一次性种子**：首次加载时若
  settings.json 还没有该 key，就从它读入，升级到本构建不会丢已记住的独立窗位置。
  迁移代码不再自带一份 `windowState` 结构，直接用 `mygo.WindowState`。
- **测试**：`settings.TestWindowStoreRoundTripsThroughTheSettingsFile`（settings.json 里确有
  `window_states` 且不吞掉其它设置）、`TestWindowStoreRoundTripsInMemory`、
  `TestWindowStoreSeedsFromTheMigratedFile`、`TestWindowStoreWithoutStates`。
- **纪律**：其余条目是框架内部改动，升级即得，不为了「用上 API」去改产品行为。

## 2026-10-14 · 第四次透明度报告 + super+n 复刻原版

用户报告：「floter 的开玻璃透明底问题还是没修好。同时现在列表页的 super+n 快捷键逻辑也没有
复刻原版，现在是固定打到前几项上」。

### 1 · 透明底：帧的 sheen 是玻璃开启时多出来的那一层

- **前几轮为何无效**：`7caa60b` 改窗口材质、`622efae`/`5dee787` 改内容面、`20f82fc` 改
  `SpecFor.TintAlpha`——都在改**帧的 tint**，但 `shell.View` 在 tint 之上还画了一层
  **sheen 渐变**（`ui.Box(...).Absolute().LinearGradient(SheenTop, SheenBottom)`），
  而这层只在效果档非零（`theme.For` 的 `sheen = TintAlpha * LensScale`，`off` 为 0）。
  它自带 alpha，composite 到 tint 上就变厚：light 47% 的列表带 off 0.564 → regular
  0.678（+0.114）→ liquid 0.746（+0.182）。用户读作「开玻璃透明底失效」。
- **原版怎么做**：`main:src/styles/launcher.css:119` 的卡片背景是
  `linear-gradient(180deg, var(--sheen-top), var(--sheen-bottom)), var(--launcher-tint)`
  ——sheen 和 tint 是**同一个 background 的两个层**，原版的 `--sheen-*` 是**固定 token**
  （dark 0.046/0.01、light 0.66/0.22），档位不改它；档位到像素的唯一 rung 是
  `--glass-step-blur`（窗口材质）和 `--glass-lens-scale`（控件透镜）。所以原版里
  `off` 与效果档的差别不在帧的 sheen 上。
- **修法**：新增 `theme.Tokens.FaceFill(alpha)`，返回帧的 tint alpha——
  `1-(1-base)(1-sheen) == alpha` 反解出 base，即 sheen 重多少就把 tint 减多少，
  于是 sheen 这层不再增加帧的 alpha。sheen 自身也被 `capAlpha` 夹在帧 alpha 以内
  （极薄的帧上 sheen 比帧还重时，顶端不会比滑块更厚）。`shell.View` 用
  `FaceFill(spec.TintAlpha)` 作卡片底色，sheen 层保留（档位仍让亮面变色）。
  线性渐变的中间段会略微**低于**滑块（light 47% liquid 最低 -0.021），但**永不更高**。
- **像素实测**（`shell/glass_test.go` 的 `TestTheFrameAlphaIsTheSlidersOnEveryStop`，
  无头生产帧，band y=90..120 / 200..260 / 350..400）：改前 light 47% regular 顶带
  +0.114、liquid +0.182；改后 dark 全档 ±0.003 内、light 47% regular -0.006..
  +0.001、liquid -0.021..+0.004（都是更薄，不是更厚）。
- **测试**：新表测试钉住「效果档不得比 off 更厚」；`TestTheGlassStopsAreVisiblyDifferent`
  仍钉亮面颜色随档位上升（dark 21.4→28.3、light 125.3→133.2）。

### 2 · super+n：档位编号跟着滚动视口，不是列表前几项

- **原版（`main:src/launcher/result-budget.ts:856` 的 `resultShortcutSlots`）**：
  `1`-`9` + `0` 发给**滚动视口内**的前九个可运行行（`start=floor(visible.start)`、
  `end=ceil(visible.end)`，视口外的行 `null`），`resultIndexForSlot` 用同一个表把键映射到行
  ——「what you see is what you select」，滚动会重新编号。R72 的闸门：列表**不超过 10 行**时
  不查视口，按顺序整表编号（能放下的列表不可滚动，查视口只添风险）。
- **本构建原来**：`launcher.assignNumbers` 对整表 `results` 的前十个可运行行编号，所以数字
  钉在列表头部，滚动不动——正是用户说的「固定打到前几项」。
- **修法**：`assignNumbers` 超过 `maxResultSlots`(=10) 时读 `a.List.Visible()`，只给视口内的行
  编号；`off` 的短表仍整表编号（R72）。视口是上一帧的，和原版按键处理器读上一次渲染的
  slot 表一致（滚动后下一帧重编号）。
- **测试**：`launcher/resultnumbers_test.go` 的 `TestResultShortcutsFollowTheScrollViewport`
  （30 行，滚前 `numbers[0]==1`，滚后 `numbers[vf]==1`、`numbers[0]` 消失、所有编号行都在
  视口内、modifier+1 跑的是可见行而非 App 00）与 `TestAShortListIsNumberedWhole`。
