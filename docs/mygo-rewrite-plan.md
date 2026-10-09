# floter × mygo 重构总计划（mygo-rewrite 分支）

> **交接优先看 [`docs/mygo-rewrite-status.md`](mygo-rewrite-status.md)**：当前进度、上手命令、目录导览、
> 未做清单与踩过的坑都在那里；本文是分轮决策与门槛的完整记录。

## 决策记录（2026-10-09）

- 用户指令：停止 v0.3.x 迭代；用 mygo（github.com/egoist/mygo）全量重构；主分支备份；新分支推进。
- 备份：`backup/main-pre-mygo` = `09fd4a3`（v0.3.14 收尾版），已 push。
- 工作分支：`mygo-rewrite`（本分支）。
- mygo 三卖点已核实：原生 GPU 自绘 UI（无 webview）、`plugins/glass`（GPU 着色器液态玻璃）、`plugins/terminal`（ghostty 渲染）。
- 文档：<https://mygo.egoist.dev/docs/ui>、`/docs/native`、`/docs/bindings`、仓库内 `docs/plugins/terminal.md`、`docs/plugins/glass.md`。
- 本机 Go 1.26.3（go.mod 声明 1.27+ 时工具链自动下载）。

## 架构映射（旧 → 新）

| 旧（tauri + React + Rust） | 新（mygo + Go） |
|---|---|
| tauri 窗口/WebviewWindow | `mygo.NewWindow`（native UI，无 webview） |
| React 三壳（launcher/settings/terminal 页） | `ui` 包视图函数（view = 状态的纯函数） |
| CSS backdrop-filter 玻璃 | `plugins/glass`（GPU shader；性能问题的根治） |
| 手写 canvas 终端渲染器 + Rust pty | `plugins/terminal`（ghostty 内核） |
| Rust `#[tauri::command]` ~90 个 | Go 服务方法（原生 UI 直接调用，无需 IPC 层） |
| settings JSON（config.rs 白名单） | 读取同一 JSON 文件、同一批 key（用户数据无缝迁移），Go 侧结构体 + normalize |
| 深链 floter://、全局快捷键、托盘、剪贴板历史 | mygo Desktop APIs（global shortcuts / tray / clipboard / deep links） |
| 独立插件窗口（detach） | 多窗口（原生） |
| extensions 目录/manifest/tool-lock | Go 移植（同一磁盘格式，兼容既有用户安装） |

## 插件参考（官方文档已读，2026-10-09：docs/plugins/glass.md + terminal.md）

**glass**（比 P0 探针结论丰富，P2 按此升级保真）：
- `glass.Glass{Style: Regular|Clear, Tint, Interactive}` —— Liquid Glass 本体；Regular 面积越大越 frost（macOS 行为）；Interactive = 按压生长动画（AppKit 27 同款）
- `glass.Blur{Radius, Mask}` —— **纯背景模糊（无 tint/rim/shadow）**，Radius=标准差（同 CSS blur()）；Mask=LinearGradient alpha 渐进模糊（同 CSS mask-image）→ **旧版 10/22/28px blur 的忠实对位物，frosted/regular/liquid 可逐档复刻，且可与 Glass 叠加**（"glass over it shows the blurred content through"）
- `glass.ScrollEdge{Soft|Hard}` —— macOS 26 滚动边缘：内容滚到栏下渐隐(Soft: 85%→0)或均匀磨砂(Hard: blur 6 + 饱和 1.25 + 82% 底) → 设置页/列表滚动条边缘的原生替代（旧 CSS 边缘效果退役）
- 性能：GPU = 每 pane 小几趟 pass，pane 随其下内容变化重绘；**CPU 路径慢（渐进模糊一条 1360×176=9ms@M5）→ 全程锁 GPU 渲染路径**；pane=圆角矩形、相邻不合体

**terminal**（P3 直接按此实现）：
- `terminal.New(Options{Command, Dir, Env, Font{Family,Size,Weight,Features,LineHeight,Thicken}, Theme, Cursor, OnTitle, OnExit})` + `terminal.View(c, term).Fill().AutoFocus()`
- 方法：`Send/Paste/Feed/Title/Dir/Size/Text/SetFont/SetTheme/Resize/Snapshot/Close`
- **`Snapshot`** = 滚回+屏幕+光标+模式的转义序列快照，喂给同尺寸新终端即还原 → 旧版「会话恢复/attach」语义可重建
- **`Conn`** = 无子进程的自定义连接（Feed 什么显示什么）→ 后台运行输出、远端会话可喂进终端视图
- libghostty-vt 原生库：`go run`/`go test` 首次自动下载到 `<cache>/mygo/natives/`（SHA-256 校验）；CLI 打包时嵌入 resources；`terminal.Load` 预加载可提前报缺库；`$MYGO_GHOSTTY_VT` 可指自编库
- 选中/复制粘贴语义内置（Ghostty 同款）

## 阶段（2026-10-09 修订：用户已在新项目验证过 mygo，砍验证环节，主体先行）

- **P0 地基（在飞）**：脚手架（native 模板）+ settings JSON 读取子集（theme/glass_step/language/opacity，保未知键）+ 启动器第一窗口 + glass 插件挂载。
- **P1 主体成型**：三壳全部立起来 —— 启动器（搜索框常聚焦 + 结果列表骨架 + 键盘 ↑↓/Enter/Esc）、设置页（页面路由 + General 页真控件 + settings **写回**仍保未知键）、终端页（plugins/terminal 一条会话）、主题 token 体系（light/dark 子集）、全局快捷键或托盘唤回。门槛含三窗截图。
- **P2 细节·启动器与设置**：真实搜索（应用扫描/计算器/表达式）、结果虚拟化、完整设置页、i18n en/zh 全量、**玻璃保真升级**（`glass.Blur{Radius}` 逐档复刻 10/22/28px，可与 Glass 叠加；ScrollEdge 替代旧滚动边缘效果；玻璃保真缺口在 P0 已登记）。
- **P3 细节·扩展**：extensions manifest/目录/安装 graft 的 Go 移植（磁盘格式兼容）。
- **P4 细节·系统集成**：托盘菜单、深链 floter://、自启动、剪贴板历史、独立插件窗口。
- **P5 打磨**：动效、无障碍、性能对照（官方数字：原生 UI ~44MB / ~7MB 二进制）。
- **P6 发布链**：CI Go 矩阵、打包（dmg/nsis/deb）、旧代码删除收尾、预发布通道。

## 进展

- **P0 地基（完成，`6a6d65c`）**：settings 读取子集（保未知键）、glass 映射、启动器第一窗口、Xvfb 截图。
- **P1 主体（本轮完成）**：三壳立起，单窗口换壳按壳改尺寸（对齐旧版 surface-residency 模型）。
  - `internal/settings` 增 `Store`：读→改→写回同一 JSON，未知键逐层保留，无变化不写盘，监听器通知；
  - `internal/theme`：theme + ui_scale → `*ui.Theme`（light/dark 子集 + 半径 token）；
  - `internal/i18n`：en/zh 全量文案（结构化 Copy，缺字段编译期报错），启动器/设置/终端共用；
  - `internal/launcher`：真实搜索（词项全命中 + 前缀/词首/包含分级排序）、结果列表（虚拟化 `ui.List`）、↑↓ 选择 / Enter 执行 / Esc 先清空再隐藏、点击执行、常驻聚焦；内置命令=打开设置/打开终端/退出；
  - `internal/settingsui`：侧栏页面路由（General/Sessions/Shortcuts/Integrations/About）+ General 页真控件（theme/language/glass/main_opacity/terminal_opacity/ui_scale），全部经 `Store` 写回；其余页面注明后续迭代；
  - `internal/terminalui`：`plugins/terminal` 单会话页（标题栏 + 关闭），无会话时显示空态/错误态；
  - `internal/shell`：应用本体——单窗口三壳切换、按壳尺寸与可调大小（720×缩放+结果带 / 720×580 上限受屏幕 72% / 860×600 最小 640×360）、玻璃面板（`glassmap.Material`，off 档铺实底）、按平台内边距、全局快捷键唤回（读用户 `hotkey`，缺省 `Ctrl+Space`）；
  - 门槛：`gofmt`/`go vet` 净，`go test -count=1 ./...` 全绿（含 i18n 完整性、Store 往返保键、搜索排序/导航、三壳 `ui.Tester` 视图与 PNG 截图 `/tmp/floter-p1-shots/`）；真实启动一次（读取既有 settings.json：light/zh/glass off/hotkey Alt+Space）无 panic，快捷键注册无冲突。

### P1 登记的保真缺口（P2 处理）

- 玻璃仍是 P0 的两档映射（无 intensity）；`terminal_opacity` 已可设置但尚未作用于终端面板底色。
- 启动器只有内置命令；应用扫描、计算器、插件命令、结果图标在 P2。
- 设置页除 General 外为占位；终端外观（字号/字体/行距/调色板）与 `plugins/terminal` 的 Theme/Transparent 尚未接线。
- 换壳唤回总是回到启动器；旧版的 surface-residency（保持上次页面 N 秒）留待 P2/P4。

## P2 进展

### P2-a 玻璃保真（已核实，已改）

计划里「用 `glass.Blur{Radius}` 逐档复刻 10/22/28px」的前提经读源码证伪：

- `glass.Blur` / `glass.Glass` 的 backdrop 是 `internal/scene.BackdropOf`——**同一窗口内、元素之下已绘制的像素**，不是窗口后的桌面；
- 桌面模糊在 mygo 里是 `WindowOptions.Vibrancy`（macOS 是 behind-window 的 `NSVisualEffectView`），而 material 是**窗口级矩形**，与本项目圆角卡片（macOS 无 gutter，卡片＝窗口）会在四角露出未裁切的模糊，旧版 Windows 的同类问题有案可查（`8949418`）。

所以本轮的做法：

- `glassmap.Spec` 改为 `{Enabled, Style, HazeAlpha, TintAlpha}`：每档保留插件**能承载**的两轴——材质（Clear/Clear/Regular/Regular 的透镜阶梯）与雾图层 `dim`（0.6/0.5/0.2，旧 GLASS_STEP_TOKENS）；`main_opacity` 仍只作 tint alpha。
- 亮度/饱和度（130/170/200%）与 blur 10/22/28 无对位：前者插件无此旋钮，后者只能作用于窗口内内容（已在包注释与测试里登记为事实，不再假装可复刻）。
- shell 按「雾图层在下、玻璃在上」叠放（旧版 `--glass-step-dim` 合成于 `--glass-tint-alpha` 之下）。
- `glass.ScrollEdge` 用在它真正有效的地方：启动器结果列表在搜索行下滚动，行内容经 Soft 边缘渐隐（`PassThrough` 让指针仍可命中被覆盖的行）。

### P2-b 真实搜索（已做）

- `internal/calc`：递归下降的表达式求值（`+ - * / % ^`、括号、一元符号、小数、e 记数），带 `=` 前缀；`ErrNotAnExpression` 区分「在搜索」与「表达式写错」；`hasArithmetic` 要求同时含数字与运算符，所以搜 `1`、`1Password`、`C++` 不会弹计算器行；`Format` 给整体数与 10 位有效数字（`0.1+0.2 → 0.3`）。
- `internal/apps`：按平台扫描已安装应用——macOS 的 `.app`（不进入 bundle 内部，覆盖 `Utilities` 子目录）、Windows 的 `.lnk`、Linux 的 `.desktop`（解析 `[Desktop Entry]` 的 Name/Exec/隐藏标志，`Exec` 按 Desktop Entry 规范切分并去字段码）；`Open()` 有 Exec 就 spawn，否则走 `Shell.OpenPath`；`Roots()` 只返回真实存在的目录；按名排序去重，上限 500。
- 启动器：`Results()` = 计算器行（若查询是算式）+ 排名后的目录；空查询只显示内置命令（不糊一屏应用）；结果行上限 50（真正的虚拟化留给列表式结果，见下）；Enter 复制结果并弹「已复制」toast；点击应用行经 `Actions.OpenApp` 由 shell 启动并隐藏窗口。
- shell：启动时后台 `apps.Scan(apps.Roots())`，落地后 `win.Update` 刷新；`Copy` 走 `mygo.Clipboard`。

### P2-c 终端外观接线（已做）

- settings 接管终端外观字段并按 config.rs 规范化：字号 8–48、字体族（空白→monospace）、光标形状 beam|block|underline、闪烁开关、行距 0.5–2.0（非有限→1.2）、边距档、九档调色板、终端窗口尺寸 640×360–2560×1800；present key 覆盖默认、缺失键保留默认；这些键不再落在 extra。
- terminalui 从 store 构造 `terminal.Options`：Font（family/size/lineHeight）、Cursor/NoBlink、边距档落到视图内边距（1/3/6 DIP）、调色板按 terminal-appearance.ts 的固定 bg/fg/cursor/selection 映到 `terminal.Theme`（亮/暗底分别取插件 light/dark 基色），透明度走终端自身背景 alpha；`inherit` 继续跟随窗口。`Refresh()` 把字体与配色应用到运行中的会话（settings 监听里调用）。
- 终端窗口尺寸改读 settings。
- 插件无对位的项（滚轮行数、粗体模式、滚动条、选中即复制、安全粘贴）未接线，也不在 UI 里给出会骗人的开关。
- 门槛：新增 skip-by-default 的真会话端到端测试（`FLOTER_TERMINAL_TEST=1`）：加载 libghostty-vt、用映射后的参数起真 shell、等它打出提示符、再热应用一次外观。

### P2-d 设置页 General 成型（已做）

- General 分三组（外观 / 窗口行为 / 终端外观）用 `ui.Fieldset`；外观组：主题、语言、玻璃档、应用透明度、终端透明度；终端组：字号、字体、光标形状、闪烁、行距、边距、调色板（九项用下拉）。
- 选择项统一为 `i18n.Option{ID,Label}` 列表（主题/语言/玻璃/界面大小/光标/边距/调色板），控件与存储 id 一一对应；i18n 完整性测试扩展到切片逐项比对。
- 设置正文改为「提示固定、表单在 Soft 滚动边缘下滚动」，与启动器同一套 `glass.ScrollEdge`。
- 交互修正：值型控件必须查询 `.Changed()` 才会在同一帧拿到新值（复选框/开关尤其），已按框架约定修正。

### P2-e 设置页其余页面 + 启动壳（已做）

- **Sessions**：列出运行中的终端会话（标题 + 运行中 + 关闭按钮），无会话时给空态；会话由 shell 从 terminal 表面读（`terminalui.Label()` 给本地化标题）。
- **Shortcuts**：只列真实存在的全局快捷键（唤回收的键，来自 settings 的 `hotkey`），并说明更多快捷键随后续系统集成到来——不给尚未实现的假行。
- **About**：应用名与版本、框架版本（mygo）、链接协议 `floter://`、设置文件路径（可选中复制）、项目链接（可点击打开）。
- **Integrations** 仍为占位（扩展生态是 P3）。
- 启动壳：`FLOTER_OPEN=launcher|settings|terminal` 让打包后的冒烟测试/截图工具直接开在某个表面（`shell.ParseSurface`）；版本在 `go run` 下从 `mygo.json` 兜底读取，打包后走 bundle 版本。
- 门槛：真实启动三个表面各一次均无报错（terminal 会真的起 shell），`go test` 85 项全绿。

### P2-f 结果虚拟化（已做）

- 启动器结果改用 `ui.List`：只构建视口内的行（500 条应用只建约 6–10 行，测试用 `Texts()` 计数钉住），滚动与行高测量交给列表。
- 选择高亮仍由行自己画（accent 14% 淡染），不用列表自带的实心 accent 选中；键盘 ↑↓ 移动后用 `ListState.ScrollIntoView` 把所选行带入视野（仅在选择变化时滚动，避免与用户滚动打架）。
- 去掉 50 行上限：匹配与排序成本可忽略，展示交给虚拟化。

### P2-g 窗口行为：失焦隐藏 + 页面驻留（已做）

- settings 接管 `hide_on_blur`（默认 true；Hyprland 会话默认 false，对齐 hyprland.rs）与 `surface_residency_seconds`（默认 10s，0=关，`u32::MAX`=永不，>86400 截到一天），继续保留「缺失键→默认、present key→覆盖」。
- shell：`Hide()` 记录隐藏时刻；唤回（全局快捷键）时若当前不是启动器且驻留窗口仍成立，则**保持原页面**（设置/终端）而不是回启动器；`Open` 到非启动器表面时重置驻留计时。
- 失焦隐藏：`OnBlur` 在 `hide_on_blur` 为真时收起面板；显示后 400ms 内的失焦忽略（平台刚显示时的抖动），对齐旧版 `suppressBlurUntil`。
- General 页新增「窗口行为」组：失焦隐藏复选框 + 驻留时间下拉（Off/10/20/30/60/120 秒/永不；存储为自定义秒数时把它作为首个选项显示，控件不撒谎）。
- 门槛：驻留规则用注入时钟做单测（10s 内保留、11s 失效、重进重置、0/永不边界、启动器不驻留），设置解析/规范化单测，真实启动一次无报错。

### P2 还未做

- 结果虚拟化（目前 50 行上限）；插件命令与结果图标。
- 设置页除 General 外的真内容（Sessions / Shortcuts / Integrations / About 仍是占位）。
- i18n 仅新增了实际用到的键，未对齐旧 i18n.ts 全量表。
- surface-residency（保持上次页面）；唤回总回启动器。
- 无插件对位的终端项（滚轮行数、粗体模式、滚动条、选中即复制、安全粘贴）保持未接线。
- 终端窗口大小尚未在用户拖拽后写回 settings（下次开启仍是上次保存值）。

## P3 进展

### P3-a 扩展内核（已做）

- `internal/extensions`：按旧 Rust 的磁盘布局读同一批文件——`extensions/`、`extension-data/`、`extension-cache/`、权威状态 `extension-repository.json`（schemaVersion 1）、只读的 `tool-lock.json` / `extensions.lock.json`。
- **Manifest**：`floter.extension.json` 的类型化子集（schemaVersion/id/name/publisher/compatibility/distribution/runtime(系统或脚本)/provider/权限/lifecycle），协议超时缺省 5000/800ms。
- **Repository**：条目为「类型化字段 + 未知键原样保留」——`probeReport`、`runtimeIntegrity`、`signatureVerified`、审批记录、错误码等旧字段在 enable/disable 写回后一字不丢（有专门测试钉住）；写入是 temp+fsync+rename+目录 fsync 的原子替换。
- **Inventory**：仓库 × 包目录的联表；包缺失仍列出并带 `ManifestErr`；无仓库记录的包列为 orphan；`Running()`=enabled 且非 broken 且 manifest 可读。
- **运行时解析**：系统运行时（记录的可执行 → 搜索路径）、脚本运行时（解释器表：node/sh/pwsh+profile/python3.x/ruby/php；go/rust 是编译产物）、`joinWithin` 拒绝越出包的路径。
- **搜索路径**：进程 PATH（去重）+ macOS 登录 shell 的 `-ilc` PATH（5s 超时、哨兵解析、best-effort）+ Homebrew/`~/.local/bin` 基线，对齐 runtime_path.rs。
- **Provider 协议**：`describe --protocol 1.0`，带 manifest 环境与超时；`ProviderError` 带退出码与 stderr；`WaitDelay` 保证挂死的 provider 不会拖住应用；`capture` 归一化为 `pty`；static-descriptor 走包内描述文件（拒绝越界）。
- **Store**：一次 Refresh 完成「仓库 + 各包 manifest + 各启用 provider 的 describe」，读锁外执行；`Commands()` 摊平成启动器可用的 argv 条目；`SetEnabled` 写回仓库并刷新内存，随后后台重新 describe。
- **接线**：启动器把扩展命令并入搜索结果（名称/描述/别名/关键词/集成名都参与匹配），回车把命令交给终端表面运行（`terminalui.RunCommand` 用 `command + execution.argsPrefix` 组 argv，cwd 策略 home/current/inherit）；设置页 Integrations 列出集成（名称/发布者/版本/状态/错误 + 启用开关），orphan 只展示不可切换。

**真实数据验证**：直接读用户机器上的 `extension-repository.json`——9 个集成、8 个启用、21 条命令，manifest 与 static-descriptor 全部解析成功，脚本运行时（php/node）正确解析出解释器 + provider 脚本路径，仓库文件未被改写；随后真实启动一次应用无报错。

### P3-b 命令参数模式（已做）

- 搜索态：扩展命令行保持「回车直接运行（无参数）」，**Tab 展开为参数模式**（行尾显示 Tab 键帽提示）。
- 参数模式：输入框第一词固定为命令 id，其余为 argv；结果区列出该命令声明的参数（首名作标题、其余名与 enum 值参与匹配与补全），随当前词过滤。
- Tab 把所选参数补进命令行（替换正在输入的词或追加），Enter 运行（`program + 运行时 args + execution.argsPrefix + 用户参数`），Esc 退出模式；删掉命令 id 也会退出。
- `splitArgs` 支持引号与反斜杠转义（未闭合引号取到行尾，不吞掉半写参数），有单测。
- 门槛：Tester 驱动的模式测试（Tab 展开、过滤、别名匹配、补全、运行 argv、Esc/删除退出、点击直接运行），共 118 项测试。

### P3-c 配置注入（已做）

- 读旧版写入的宿主配置：`extension-data/<id>/config.json`（当前 envelope `{configVersion,secretGeneration,values,schema}` 与旧版裸 values map 都支持），密钥从 `config-secrets/<generation>.json` 或旧版 `config.secrets.json` 合并；口令占位 `[REDACTED]` 在没有密钥时被删掉而不是当成值；id 越界拒绝。
- 注入（对齐 config.rs）：schema 字段有值就按 `envVar` 或 `environmentMapping` 进环境变量，`argument` 进 argv（`true` 只给 flag、`false`/null 跳过、列表以逗号连接）；**口令只进环境变量，绝不进 argv**。
- 工具自管配置（`owner: "tool"`）生成一条可运行的 `Configuration` 命令（provider 的 `openCommand`）。
- 命令条目因此带上 `Env`（配置环境）与配置参数；终端运行时会应用它们。
- 门槛：配置读取（envelope/遗留/密钥代数/占位符/越界 id）、注入规则（含口令不入 argv）、tool 配置命令、以及 store 端到端（命令条目带 Env 与参数）单测全绿，共 123 项测试。

### P3-d 本地安装/更新/卸载（已做）

- `InstallLocal`：从本机包目录安装（distribution=local，用户机上全部集成都是这一类）——校验 id、**先解析运行时**（找不到解释器/可执行就不动盘）、staging 目录 → 备份旧包 → rename 上台 → 原子写仓库 → 删备份；任一步失败都保留旧安装可用。
- 条目生成：id/name/publisher/distribution=local/runtimeOwnership（system 或 script→bundled + runtimeRoot）/providerKind/enabled/versions/manifestPath/executablePath/时间戳；**更新时保留**审批记录、channel、pinned、configGeneration，broken 状态与错误码继续挂着，旧版本写进 previousVersion。
- 生命周期模板：`configurationTemplates` 在安装时复制进 `extension-data/<id>/`，已存在的不覆盖；源/目标都做越界校验。
- `Uninstall`：包先改名到 `.removing` → 写仓库 → 删目录 → （可选）删数据；仓库不认识 id 返回 `ErrNoIntegration`；`SetToolVersion` 记录 provider 报的版本；`ManifestDigest` 给审批用。
- 设置页 Integrations 每行加「卸载」按钮，shell 用原生确认对话框后执行并后台刷新（默认保留用户数据）。
- 门槛：安装/更新保留审计字段/卸载（含数据保留与删除）/digest/id 校验/越界包拒绝等单测；真实启动一次，仓库文件未被改写（14:04 未变）。

### P3-e npm 安装/更新（已做）

- **semver 子集**：`^ ~ >= <= > < =` 与精确版本、`*`、`||` 备选、空格交集；预发布排序按规范（数字标识符优先、release > prerelease）；**范围不匹配预发布**（除非约束自身带 `-` 或 dist-tag 指向），避免 `^1.0.0` 悄悄装上 `2.0.0-beta.1`。
- **registry 客户端**：`GET <registry>/<name>`（scope 的 `/` 转义），dist-tags（latest/beta）优先、其余按范围选最高版本；`dist.integrity` **SRI 校验**（sha512/sha256，多哈希任一命中，参数忽略），无 integrity 时回落到 `dist.shasum` 的 sha1 SRI。
- **tarball 解包**：gzip+tar，剥掉 npm 的一层包装目录；**拒绝绝对路径与任何逃逸**，符号链接/硬链接/设备文件一律跳过不跟随；文件权限带上属主可执行位（旧版 make_executable 语义）。
- **包入口**：`package.json` 的 `floter.manifest`（相对路径、拒绝越界），版本取 `package.json` 的 version；随后走与本地安装同一套 graft（staging/备份/rename/原子写仓库/模板）。
- **接线**：设置页 Integrations 顶部新增「npm 包名 + 版本（可空）+ 安装」一行；shell 后台安装并刷新（`Options.Registry` 可注入，测试指向 httptest）。
- 门槛：semver/SRI/解包（含逃逸与链接）/选择版本/registry httptest 端到端安装与拒绝用例，以及「设置按钮 → shell → registry → 入库 → 列表」全链路测试；共 153 项测试；另有 opt-in 真 registry 测试（拉真实 is-number：元数据→选择→下载→**真 SRI 校验**→解包），并确认文档示例包 `@vst93/floter-v` **尚未发布**（404）。

### P3-f 权限审批（已做）

- 权限模型对齐 FEP-5/manifest.rs：七个 id、schema 顺序、未知 id 丢弃；**两条 host 自己决策**（`environment`、`process-spawn`）标 enforced，其余是 disclosed（不是沙箱，这一点如实展示）。
- `RequiresApproval(previous, manifest, digest)`：批准绑定**清单字节的 sha256**；清单变了（新增权限或内容变化）要重新批准，未变则沿用旧批准。
- `Prepare* / Commit` 两阶段安装：`PrepareLocal`/`PrepareRegistry` 完成下载与校验并给出「需要批准什么」，`Commit(approved)` 才真正 graft；未批准时返回 `ErrPermissionApprovalRequired` 且**什么都不落盘**（下载目录也清掉）。`InstallLocal`/`InstallFromRegistry` 是「无待批权限时才成功」的便捷封装。
- 记录写入 `approvedPermissions` / `approvedAt` / `approvedManifestDigest`（旧字段，写回时本来就会被保留）。
- shell：npm 安装走 prepare → 需要时弹原生对话框（逐条列出权限并标注 enforced/disclosed）→ commit；`ConfirmPermissions` 可注入，测试两个分支（拒绝 → 不落地；同意 → 落地且记录批准）。
- 设置页集成行新增权限行（本地化名称 + enforced 标注）。
- 门槛：权限模型与审批规则单测、需要批准/拒绝/同意的安装流程测试、设置页展示测试；共 160 项测试。

### P3-g complete / diagnose 协议（已做）

- **complete**：`<program> <argsPrefix> complete --protocol 1.0`，请求 JSON 走 stdin（`{command,tokens,cwd}`），按 manifest 的 `completeTimeoutMs` 超时，解析 `{completions:[{label,kind,detail}]}`；provider 不实现（非零退出）、超时或答案非法都是**可降级的错误**（命令模式退回静态补全），且不会被挂死的 provider 拖住。
- **diagnose**：`diagnose --protocol 1.0`（5s），解析 `{status,checks:[{id,status,message}]}`。
- **静态补全**（对齐 catalog.rs）：正在输入的词前缀匹配参数名；若前一个 token 是取值参数，则按 kind 给出 enum 候选、路径/目录候选（目录带尾分隔符）或交由 provider（kind=command 时静态为空）；`NeedsDynamicCompletion` 只在确实是 command 型参数时才请求动态补全。
- **命令模式接线**：token 变化才请求一次（带 token key 防止过期结果回填），动态结果优先合并、去重后接静态结果；Tab 补全、Enter 运行不变。
- **设置页**：每个集成行加「检测」按钮，结果记在行上（status/问题消息，失败用 Danger 色）。
- 门槛：静态补全/合并/动态判定单测、假 provider 的 complete/diagnose/超时/非零退出/坏 JSON 测试、命令模式的动态合并测试、shell 的 complete 与 diagnose 端到端测试；共 167 项测试。

### P3 还未做

- 安装/更新/卸载（graft、事务、journal、回滚）、导入导出、npm 下载与校验。
- 命令参数模式（进入某条命令后输入参数、静态/动态补全）、`complete`/`diagnose` 操作。
- 配置注入（host-owned schema → 环境变量/文件、config generation、模板）。
- 权限审批 UI 与诊断抽屉、orphan 的接管/删除。

## P4 进展

### P4-a 系统集成（已做）

- **单实例**：`RequestSingleInstanceLock`，第二次启动把参数（含深链）交给运行中的实例并退出；`OnSecondInstance` 唤起窗口。
- **深链**：`floter://settings` / `floter://terminal` / `floter://search?q=…`（也接受 `floter://?q=…`）路由到对应表面并预填搜索；未知/非法 URL 回落启动器且不动搜索词。包内含 `urlSchemes: ["floter"]`，运行期再 `RegisterURLScheme`（打包后由 CLI 写进 Info.plist/桌面项/注册表；`go run` 下 macOS 无 bundle，会记一条日志而非失败）。
- **自启动**：settings 接管 `launch_at_startup`（默认 false），启动时按设置同步登录项，设置页改动即时同步；登录项读写通过可注入的 `OpenAtLogin`/`SetOpenAtLogin`，测试不碰机器状态（`go run` 无 bundle 时只记日志）。
- **托盘**：菜单栏/通知区图标（内嵌 32×32 应用图标）+ 菜单（设置、终端、退出），点击切换窗口显示。
- 门槛：深链路由与登录项同步单测、共 126 项测试、真实启动一次（`go run` 下两条「需要 bundle」的日志属预期，其余无报错）。

### P4-b 剪贴板历史（已做）

- `internal/clipboard`：读写同一份 `clipboard-history/index.json`（text/image/files 三种 kind、`image_file`/`paths`/`hash`/`created_at`/`favorite`），未知键原样保留；文本以 sha256 去重（重复复制把旧条目提到最前而不是新增），保留策略与旧版一致——**收藏永不淘汰**、非收藏只留最新 `max_items`（默认 300，10–500 截断）、超过 30 天的非收藏丢弃；写入是 temp+fsync+rename 原子替换（0600）；索引缺失/损坏按空历史处理而不是崩掉。
- 查询：按词全命中（大小写不敏感，文本与文件路径都搜），结果按时间倒序。
- 开关与容量读 settings 的 `clipboard_history_enabled` / `clipboard_history_max_items`（本包不拥有这两个键，走 extra），设置变更即时应用到 store 与监听器。
- 启动器新增内置行「剪贴板历史」，回车进入剪贴板模式（字段首词 `clipboard`），输入过滤条目，回车复制并收起窗口；`Esc` 或删掉首词退出模式。
- shell 每 600ms 轮询 `mygo.Clipboard.ReadText()` 增量入库（空白跳过、与上一次相同跳过、**应用自己写剪贴板的内容不回灌**），有新增就重绘。
- 门槛：读取真实索引（含未知键与三种 kind）、去重/容量/保留期/收藏豁免、搜索与增删清空、模式测试；共 143 项测试；真机启动后历史从 303 增至 304（捕获了当时剪贴板内容）。

## P6 进展

### P6-a 打包与 CI（已做）

- go.mod 增加 `tool github.com/egoist/mygo/cmd/mygo`，项目用 `go tool mygo build` 打包（与原生模板一致，不依赖 npm/bun）。
- **真机验证**：`go tool mygo build` 产出 `dist/darwin-arm64/floter.app`（14.3 MB）与 `floter 0.3.14.dmg`（4.3 MB）：
  - 二进制 12.9 MB，`Contents/Resources/libghostty-vt.dylib` 由 CLI 自动嵌入（应用 import 了 terminal 插件）；
  - Info.plist 带 `CFBundleURLTypes`（`floter`，来自 mygo.json 的 `urlSchemes`）、`CFBundleIconFile=AppIcon.icns`（由 `resources/icon.png` 生成）；
  - 用临时 HOME 直接跑 bundle 内二进制：**不再有「需要 bundle」的两条日志**（深链注册与登录项都可用），且把原生库缓存移走后终端仍能启动 → 用的是 bundle 内的 `libghostty-vt.dylib`。
- **应用图标**：托盘/任务栏图标跟随 `app_icon` 设置（dark 默认 / light），两套 32×32 图标内嵌，设置变更即时生效；托盘菜单、窗口图标同步；bundle 图标用深色版。
- **CI**：新增 `.github/workflows/go.yml`——三平台（ubuntu/macos/windows）矩阵跑 gofmt/vet/build/test，macOS 任务再跑 `go tool mygo build` 并上传 .app/.dmg 产物。
- 遗留：`mygo build` 会因仓库里仍有旧 `package.json` 而生成空的 `src/mygo.ts`（已加入 .gitignore），P6-b 删除旧前端后自然消失。

### P4-c 应用菜单（已做）

- `InstallMenu()`：标准 macOS 应用菜单（设置 `Cmd+,`、隐藏/隐藏其他/全部显示、退出）+ **Edit 菜单的角色**（撤销/重做/剪切/复制/粘贴/全选——原生文本框与终端插件都依赖它）+ View 菜单（显示 floter / 终端 `Cmd+Shift+T` / 全屏）+ Window 菜单。
- 模板是纯函数（`menuTemplate`），单测检查结构与回调；平台专属角色（macOS-only）由框架过滤，一套模板跨平台。
- 门槛：新增模板测试，共 157 项测试；真机启动无报错（`go run` 下无 bundle 的两条日志属预期）。

### P4-d 钉住到独立窗口（已做）

- `shell.PinText(title, text)`：开一个**普通窗口**（可移动/缩放/关闭，`StateKey: pinned`）显示快照文本，可选中复制；关闭只忘记这个 pin；`OpenPinned` 可注入便于测试。
- 终端表面标题栏新增「钉住输出」（无会话时不显示），把当前会话文本交给 `PinText`；`Text` 可注入便于测试。
- 启动器剪贴板模式：**Tab 把选中条目的完整文本钉成窗口**（Enter 仍是复制并收起），与搜索态的 Tab=展开参数模式形成一致的心智模型。
- 顺带修掉一个真实交互缺陷：进入命令/剪贴板模式或深链预填时，输入框光标停在旧位置（打字会插进命令词中间导致模式退出）；现在进入模式/设置查询后把光标移到末尾（`SetTextSelection`），并有测试钉住。
- 门槛：终端 pin 控件（有/无会话）、剪贴板 Tab 钉住、光标末尾化；共 170 项测试。

### P4-e 剪贴板：图片与文件（已做）

- `clipboard.AddImage`：PNG 落盘到 `images/<id>.png`（0600，与旧版同目录同命名），条目记录 `image_file/width/height/hash`；sha256 去重（同图再次复制提到最前）；**容量/保留期淘汰时连带删除图片文件**；`ImagePath` 给回读路径。
- `clipboard.AddFiles`：文件列表条目（`paths` + 其 hash），去重、按路径可搜。
- 监听器改为「文本每轮廉价比对；格式列表变化时才尝试图片→文件」，避免每 600ms 读大图；`formatKey/containsFormat/imageSize` 为纯函数并单测。
- 回写：`Actions.CopyClip` 按 kind 还原（文本/文件列表/图片字节），并通过**可注入的 `WriteClipboard`** 落到系统剪贴板；应用自己写入的内容不会被监听器再次记录（`selfCopied`）。
- 门槛：图片/文件存储（含淘汰删文件）、回写 payload 三态、纯函数；共 174 项测试；**真机验证**：把一张 40×24 PNG 放进系统剪贴板，启动应用后历史 22→23 张图片，条目宽高与文件名均正确。

### P4-f 浏览器插件内核（已做）

- `internal/browser`：**只读**读取各浏览器自己的文件——Chrome 家族（Chrome/Chromium/Brave/Edge/Arc）的 `History`(SQLite) 与 `Bookmarks`(JSON)、Safari 的 `History.db`、Firefox 的 `places.sqlite`；SQLite 走 mygo 的 `plugins/sqlite`（purego 加载预编译库，无 cgo）。每个 profile 独立失败：锁住/无权限/schema 不认识只跳过它自己。
- 时间戳按各家纪元转换（Chrome 1601 微秒、Safari 2001 秒、Firefox 1970 微秒），缺失时为零值（书签无日期）。
- 查询：先取各 profile 最新 N 行，再在 Go 里做多词全命中（标题+URL，大小写不敏感），按访问时间倒序、书签按标题；`Options{Days,Limit}` 支持「只搜最近 N 天」（书签不受限）与结果上限。
- 设置接线：读 `browser_plugin.enabled`（默认开）与 `history_days`（默认 30），上限 50 条。
- 启动器新增内置行「浏览器历史」→ 浏览器模式（字段词 `browser`）：**Enter 用默认浏览器打开**该页并收起窗口，**Tab 复制 URL**；结果异步搜索（生 sqlite 读取不阻塞 UI，按查询去重、过期答案丢弃）。
- 门槛：profile 发现、Chrome 书签树、时间戳换算、匹配/排序/限制、缺失与坏文件；另有 opt-in 测试用 sqlite 插件**真建一个 Chrome 形状的 History 库并读回**（本机通过）；共 181 项测试。真机上 Safari 的 History.db 因缺少「完全磁盘访问」而不可读，包内如实跳过（打包应用可由用户授权）。
- 打包：应用现在同时内嵌 `libghostty-vt.dylib` 与 `libmygo-sqlite3.dylib`（app 15.9 MB / dmg 4.9 MB）。

### P4-g 空查询的「最近使用」（已做）

- `internal/usage`：`<config 根>/usage.json` 记录「启动次数 + 最后启动时间」，按 id（应用路径 / 扩展命令 id）计数；缺失或损坏的文件按空历史处理，写入是 temp+rename 原子替换；`Top(limit, keep)` 过滤掉已卸载的项、按次数（并列时按最近）排序。
- 启动器：`SetRecent(paths, show)` + `ShowRecent`；空查询在**内置命令之后**列出最多 5 个最近应用（已卸载的自动剔除，顺序即 usage 顺序）。
- shell：应用启动成功后记一次使用并刷新列表；扫描完应用后也刷新一次；`show_recent_in_launcher`（默认开）决定是否展示。
- 门槛：usage 存储（计数/排序/并列/过滤/缺失/损坏/无路径）、启动器空状态顺序与开关、shell 的设置与使用文件联动；共 185 项测试。

### P4-h 系统命令搜索（已做）

- `apps.CommandDirs()`（进程 PATH 拆分）+ `apps.ScanCommands(dirs)`：逐目录列出可执行**普通文件**，同名以先出现的目录为准（与 shell 解析一致），跳过点文件与 `.dylib/.so/.a/.o/.h/...` 等非命令，Windows 只认 `.exe/.cmd/.bat/.com`；上限 800 条、按名排序。
- 启动器：`Tools` + `ShowTools`；仅当 **`show_commands_in_search`（默认关，与旧版一致）** 为真时，非空查询把命令并入结果；运行命令交给终端表面（`RunInTerminal` → 终端会话）。
- shell：启动时按设置后台扫描 PATH；设置为真才展示。
- 门槛：目录扫描（首目录优先/权限/点文件/产物文件/缺失目录）、dirs 拆分、启动器（默认不展示、开启后可搜可运行、关闭后消失）、shell（默认关、开启后扫描到达启动器并交接终端）；共 189 项测试。

### P4-i 快捷键录制（已做）

- `internal/shortcuts`：旧拼写（`Cmd+Comma`、`Option+Space`、`CommandOrControl`）→ 框架接受的拼写（`Cmd+,`、`Alt+Space`、`CmdOrCtrl`）互转，修饰键顺序固定、幂等、未知名拒绝；`FromKey` 把键盘事件转成同一拼写（要求 Shift 之外至少一个修饰键，避免抢走普通输入）。
- 设置页 Shortcuts 页：当前加速键 + 「录制」按钮；录制时捕获下一个组合键（**录制期间 Escape 归录制器**，不关面板），Esc 取消，无修饰键的按键被忽略。
- shell：重新注册全局快捷键——系统拒绝（被别的应用占用）时保留旧的且不落盘；成功则写回 `hotkey` 与 `shortcuts.toggle_window`，下次启动沿用。
- 门槛：拼写转换表与拒绝用例、`FromKey` 覆盖各键组、Tester 驱动的录制器交互、shell 的注册/拒绝/持久化路径；共 194 项测试。

### P6-b 删除旧实现 + 纯 mygo 树（已做）

- **删除**：`src/`（React）、`src-tauri/`（Rust）、`tests/`（旧 node 套件）、`index.html`、
  `vite.config.ts`、`tsconfig*.json`、`package.json`/`package-lock.json`/`pnpm-workspace.yaml`、
  `node_modules/`、`public/`、`packaging/arch`（Tauri 时代的 deb 重打包）、
  `scripts/verify-tray-identity.sh`（读 `tauri.conf.json`）、旧 CI
  （`pull-request.yml`/`release.yml`/`windows-smoke.yml`）、`IMPLEMENTATION_REPORT.md`、
  `docs/phase-reports/`、`docs/plugin-system-audit.md`、`docs/screenshots/`、根目录的旧任务便签。
- **保留**：`cmd/` + `internal/`（唯一实现）、`mygo.json`（CLI 的构建输入）、
  `resources/icon.png`（bundle 图标）、`extensions/v-tools`（参考扩展包）、
  `docs/extensions/`（FEP 协议规范，Go 侧实现的依据）、`CHANGELOG.md`（发布史）。
- `docs/DEVELOPMENT_PLAN.md` 与 `docs/tool-binding-design.md` 移入 `docs/archive/` 并加历史标注
  （它们描述的是重构前的实现，代码引用已失效）。
- 注释里对已删文件的引用（`config.rs`、`store.rs`、`src/i18n.ts` 等）全部改写为描述行为本身，
  不再指向不存在的路径。
- README（中英）重写：原生 UI、Go 工具链、三表面、扩展与数据兼容说明；去掉 webview 时代的
  安装/EGL/更新器章节。
- `docs/AGENT-NOTES.md` 重写为面向 Go 树的备忘：产品方向、**磁盘格式零迁移清单**、
  门槛命令、平台无关性纪律、反馈通道与三态。

### P6-c 三平台测试修复（已做）

CI 是三平台矩阵，但两处断言只在作者机器上成立，Linux 上直接失败：

- `internal/browser` 的 profile 发现表**按宿主平台**返回，测试却只写 macOS 布局 → 把表写成
  `(goos, home, localAppData)` 的纯函数（`chromiumBrowsers`/`firefoxRoot`/`joinFor`），
  并补齐 Windows（`%LOCALAPPDATA%` 下的 `User Data`）与 Linux 布局；测试改为**在任何宿主上断言
  三个平台**。顺带按旧实现补齐：Edge 四个渠道（Beta/Dev/Canary）、Arc、profile 排序
  （Default → Profile N 按数字 → 其它按名）、`Local State` 里的 profile 显示名、
  自定义 base 目录（可以是浏览器 base 也可以是单个 profile）。
- `internal/extensions` 的 npm 安装测试依赖机器上装了 `v`（fixture manifest 的运行时）→
  运行期解析走可注入的 `lookTool`/`findInterpreter`，测试用 fixture 可执行文件，不再看机器 PATH。

### P4-j 命令开关与命令别名（已做）

- **`plugin_command_switches`（命令级开关）**：`internal/settings` 新增类型化读写（缺省即关闭、
  空 id 丢弃、写回保留其它扩展的条目）；启动器只列出**开关打开**的命令——旧版的开关门禁的是
  「插件模式」通路，Go 侧命令行本身既是模式入口（Tab 进参数模式），所以门禁落在行上；
  设置页 Integrations 每个集成下给出**每条命令的开关**，并标注「运行时不可用」（运行时不解析
  的命令仍可开关，只是运行会失败）。开关改动即时重发命令表给启动器，不需要 provider 往返。
- **`command_aliases`（命令别名）**：类型化读写 + `ResolveCommandAliases`（按命令名升序，先到
  先得、大小写不敏感去重、空别名忽略）；启动器里别名与命令名走**同一条打分阶梯**（别名命中
  不降级），别名只参与匹配不参与显示；清空别名即删除条目。
- 顺带把三个启动器开关（`show_commands_in_search` / `show_recent_in_launcher` /
  `show_menubar_icon`）与 `last_settings_page`、`launch_counts` 收进类型化访问器，
  `show_menubar_icon` 真正接线（托盘随设置安装/销毁，窗口未就绪时不碰托盘）。

### P2-h 设置页 Plugins（已做）

- 新页 **Plugins**（内置插件各自的设置）：浏览器插件卡片（开关、目标浏览器、额外 profile
  目录、历史范围、排序、搜索范围、调试端口与开关）与剪贴板卡片（开关、容量）。
- 目标浏览器的候选来自实际发现（`Actions.BrowserTargets`），没发现就只给「自动」——
  控件不撒谎；「运行时不可用」等状态如实标注。
- `last_settings_page` 接线：切页写回，进设置面恢复上次页面（未知值落 General）。

### P4-k 自定义快捷键（已做）

- **`custom_shortcuts`**：类型化读写 + 规范化（空 key/空 action 视为草稿不入库；同一按键去重，
  先到先得，比较时把 `CmdOrCtrl` 按**平台**解析成真实修饰键——macOS 上它等于 Cmd，别处等于
  Ctrl，所以「哪些键算重复」与系统实际行为一致）。动作词表与旧版一致：
  `plugin:<id>`（可见地打开插件模式）、`action:<id>`（应用自身动作）、其余当作**静默命令行**。
- **注册**：随设置变更重注册（先释放再注册），被系统占用或与唤回键冲突的键**如实报告**在页面上，
  不假装绑定成功；`Options.RegisterShortcut`/`UnregisterShortcut` 可注入，测试不碰真实全局键。
- **动作**：`plugin:clipboard`/`plugin:browser` 打开启动器对应模式；`action:toggle_window`、
  `new_command`、`open_settings`、`open_external_terminal`（打开系统终端窗口——Go 版终端是进程内
  会话，所以这是**新开**一个 shell，不是旧版的会话移交，已在代码里写明）；其余走 `spawn.Command`
  （Unix 用 `$SHELL -c`，Windows 用 `cmd /C`，脱离会话、无窗口）。
- **新 `internal/spawn`**：全应用唯一的「启动一个不等它的程序」入口（浏览器打开、静默命令、终端
  模拟器），平台差异（Unix `setsid` / Windows `DETACHED_PROCESS`）只写一次。
- 设置页 Shortcuts 新增「自定义快捷键」区：已绑定的行（键位 + 动作下拉 + 移除）、新建行
  （录制按键 + 动作下拉，动作选「命令行…」时出现文本框 + 添加）、失败提示行。

### P4-l 计算器插件（已做）

- **`internal/calculator`**：计算历史的存储与策略，磁盘格式与旧版一致
  （`<config>/floter/calculator-history/index.json`，条目 `{id, expression, result, created_at,
  favorite}` + 未知键原样保留，临时文件 + fsync + rename 原子写，缺失/损坏按空历史）。
  重算同一算式**提升到最前**而不是叠一条（收藏标记跟着走）；**收藏豁免容量与时间两轴**；
  淘汰在写入时做（新增/收藏切换/删除/设置变更都在同一临界区里裁剪）。
- **`calculator_plugin` 设置块**：类型化读写 + 规范化（容量 10–500、保留 0/1/7/30 天、
  复制模式 full|result），未知子键保留。
- **启动器计算器模式**：触发词 `calc` / `calculator` / `计算器` / `计算` / `=`（后跟空格即进入，
  与旧版同一规则；字段内容同时是待算算式与历史搜索词）。新算式优先给出答案行，回车求值 →
  入库 → 按复制模式复制并收起；历史行回车复制、`Tab` 在「全部/收藏」间切换、`⌘/Ctrl+D` 加星、
  `Esc` 退出。内置命令新增「计算器历史」行。
- **顺带**：内置模式现在也认**输入触发词直接进入**（`clip `/`browser `/`calc `），
  触发词表按旧版补齐（含中文词与 `=`），离开模式仍是「首词不再是触发词」。
- 设置页 Plugins 新增计算器卡片（容量、保留时长、回车复制内容）。

### P4-m 历史行的收藏与删除（已做）

- 剪贴板模式：`⌘/Ctrl+D` 加星（旧版键位），行尾显示 `★`；历史行（剪贴板与计算器）在**选中行**
  上显示删除控件（✕ + 无障碍标签），点击即删并给一行反馈，模式不退出。
- **为什么删除不是 `⌘⌫`**：mygo 的文本框把带修饰键的 Backspace/Delete 判为自己的编辑手势
  （`editor.wants`），窗口级快捷键收不到——实测确认。旧版在 DOM 里自己拦这个键，Go 版做不到，
  于是把删除做成行上的控件（更可发现，也不与文本编辑抢键），并在代码注释里写明原因。
- 计算器模式同一套：`⌘/Ctrl+D` 加星 + 行上删除。
- 旧版剪贴板模式的六档 kind 过滤（全部/收藏/文本/图片/链接/文件，Tab 循环）**未接线**：Go 版
  Tab 已定为「钉住」（见 P2/P4-d），两种语义只能留一个；收藏/删除已可用，kind 过滤登记为未做。

### P3-h 后台运行与输出回看（capture 通道，已做）

- **路由**：manifest 的 `output`（`terminal` | `background`，缺省 terminal）决定命令输出去哪；
  `CommandEntry.Route` 带上它。旧版这个字段一直存在，Go 侧此前忽略（capture 被归一化成 pty）。
- **`extensions.RunCaptured`**：无 shell 直接 spawn（argv 与终端路由同源）、stdin 为 null、
  带上配置环境与 cwd、`RUN_TIMEOUT` 300s（可被调用方 ctx 收紧）、`WaitDelay` 兜住不听话的子进程、
  每条流 1 MiB 上限并在超限时置 `Truncated`（旧版不截断却带着这个字段，这里让它说实话）；
  **非零退出是答案不是错误**（`ExitCode` + `Success=false`），超时/起不来才是错误。
- **启动器的输出视图**：状态行（完成/超时/退出码 + 耗时 + 是否截断）+ 可滚动、可选中的等宽文本
  （字体跟随终端字体设置），`Enter` 复制、`Esc` 关闭——对应旧版的 PluginTextView（最小三行、
  上限十行高度、内部滚动）的语义。
- **完成提示**：面板隐藏时后台运行完成会发一条系统通知（`mygo.NewNotification`），面板可见时
  不重复通知（旧版「系统通知只用于后台完成，绝不做屏幕上 toast 的第二份」）；通知文案走 i18n，
  测试可注入 `Notify`。
- 未做：旧版的**列表协议**（stdout 是 JSON 数组时渲染成带编号/分页的列表）——目前一律按文本展示。

## 纪律（继承）

- 承包 runner：禁 commit/push，树留脏主线复核；门槛实跑；报告落 /tmp。
- 主线：复核 → commit → push（本分支）。
- 用户数据零迁移成本优先于代码优雅：settings/扩展/tool-lock 的磁盘格式不变。
- 性能基线记录在案：mygo 官方数字（原生 UI ~44MB 内存、~7MB 二进制）作为 P6 验收参照。
