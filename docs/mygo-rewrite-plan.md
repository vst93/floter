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

### P3-i orphan 接管与删除（已做）

- `extensions.PrepareAdopt` / `Adopt`：把 `extensions/` 下**仓库没有记录**的包目录正式 graft 进仓库
  ——复用安装的同一套校验、运行时解析与**权限门禁**（声明权限仍要用户批准），并拒绝清单 id 与目录名
  不一致的包（否则会被登记到错的名字下）。
- `DeleteOrphan`：删除孤儿目录；**拒绝**仓库已登记的 id（那个走 `Uninstall`，保持仓库与磁盘一致），
  也拒绝非法 id 与不存在的目录。
- 设置页 Integrations：孤儿行只给「接管 / 删除」两个操作（没有开关、检测、卸载——没有记录可操作），
  并说明孤儿是什么；接管走与安装相同的权限对话框。

### P4-n 终端尺寸写回与会话快照（已做）

- **尺寸写回**：终端是唯一可拖拽的表面，`Win.OnResize` 记下尺寸，**停止拖拽 600ms 后**写回
  `terminal_width`/`terminal_height`（一次拖拽只写一次盘；仍走 `settings` 的规范化，拖到小于最小
  尺寸会被夹回 640×360）。下次打开就是上次的尺寸。
- **会话快照**：`plugins/terminal` 的 `Snapshot()`（滚回 + 屏幕 + 光标 + 模式的转义序列）在
  **关闭会话与退出应用时**写入 `<config>/floter/terminal-snapshot`（临时文件 + fsync + rename，
  0600），下一次建会话时 `Feed` 回去——上次的滚回内容重新出现在新 shell 的提示符之上。
  文件不删除：下次关闭覆盖它，崩溃后恢复的也是用户最后看到的状态。
  （旧版的 `session_restore.rs` 是「工具会话 reattach」语义，Go 版终端是进程内会话，这里实现的是
  它能表达的等价物：快照恢复，已在代码注释与文档里写明差异。）
- 退出路径全覆盖：`mygo.App.OnQuit`（菜单的 Quit role 由框架处理）+ 应用自身的 quit 包装。

### P5 打磨：动效、无障碍、性能对照（已做）

- **动效**：结果行以 item id 为 key，出现时 120ms 淡入（不位移，避免列表跳动）；框架的 toast
  自带出现/消失动画与读屏播报，所以复制/删除/收藏的反馈不需要另写动效。窗口换壳是原生窗口
  尺寸变化，不做应用层过渡。
- **无障碍**：搜索结果列表与设置侧栏列表都带 Label；按钮/开关/滑块/输入框由框架给出角色与状态
  （框架的 a11y 树按需构建）；行内图标按钮（清空、删除、关闭）都带可读的 `Label`；反馈走框架
  toast（读屏会读）。旧版的 `aria-live` 区域由框架 toast 覆盖。
- **i18n 审计**：UI 包内没有裸的用户可见字面量（只有 `✕` 这类图形按钮，且都带无障碍标签），
  文案全部来自 `internal/i18n`。旧版 1700+ 键里的大部分属于已不存在的页面（插件 iframe 页、
  剪贴板页、会话页），不需要为「对齐键数」而搬。
- **性能对照**（Linux x86_64，Xvfb + llvmpipe 软件渲染，headless 基准）：
  - 一帧成本：启动器（500 应用 + 查询）**324 µs**、空查询 **67 µs**、设置 General **245 µs**、
    Integrations（20 集成 × 5 命令）**516 µs** —— 都在 60fps（16.6ms）预算的两个数量级以内；
  - 二进制：`go build` 20 MB、`-s -w` 14 MB（macOS 打包产物 16.0 MB app / 4.9 MB dmg，
    含 libghostty-vt 与 sqlite 原生库）；
  - 常驻内存：软件渲染下 floter **166 MB**，同一环境下 mygo 最小窗口应用 **137 MB** —— 即框架
    +llvmpipe 是主要部分，应用自身约 30 MB。官方「~44MB」应在 GPU 渲染路径与不同平台上测得，
    这里如实记录本机数字与测量条件（Xvfb + llvmpipe 会分配大块软件光栅缓冲）。
  - 基准以 `go test -bench .` 保留（`internal/launcher`、`internal/settingsui`），供回归对照。

### P6-c 发布链（已做，签名/公证留给维护者）

- **CI 三平台打包**：`.github/workflows/go.yml` 的 package job 改成矩阵——macOS（`darwin/universal`
  → .app + .dmg）、Linux（`linux/amd64` → 可执行文件 + .deb + 带 `install.sh` 的 .tar.gz）、
  Windows（`windows/amd64` → .exe，有 NSIS 时另有安装程序），各自上传产物。三平台都验证过本机
  可离线构建（Linux 6.5s / Windows 8.4s，Windows 会自动取 `ghostty-vt.dll`）。
- **安装器**：删除手写的 `scripts/install.sh`（它是 webview 时代下载 .dmg/.deb 的脚本）；
  现在 `mygo build` 在 tar.gz 里自带 `install.sh`（装到 `~/.local`、写桌面项、支持
  `--uninstall`），README 中英同步改写产物表。
- **更新检查**：About 页新增「检查更新 / 下载并安装」，走 `mygo.Updater`（签名校验 + 可选增量），
  检查与下载都在后台、进度回主线程；**没有更新源的构建如实显示「此构建没有更新源」**，不假装检查。
- **留给维护者的一次性步骤**（文档已写明）：`mygo keygen` 生成密钥、`mygo.json` 填 `updates`
  （github + publicKey）、`macos.signingIdentity`/`macos.notarize` 签名与公证、CI 里注入
  `MYGO_UPDATER_PRIVATE_KEY`；在此之前构建不带更新源。

### P3-j 任务完成通知与权限审计（已做）

- **系统通知的文案与动作词表**：`i18n.Notifications`（安装/卸载/检测的成功与失败、后台命令完成），
  中英双语；规则与旧版一致——**只在面板隐藏时通知**（面板可见时结果就在屏幕上，再弹一条是重复），
  且没有窗口（测试/headless）时不调用框架通知（框架的通知需要 app）。
- **接线**：npm 安装、本地接管、卸载、检测四条后台路径在完成/失败时各发一条；后台命令沿用同一
  个通知入口（状态行 + 命令名）。
- **权限审计**：集成行新增「权限」按钮，展开后逐条列出权限名、说明与 **enforced / declared** 标注
  （`i18n.PermissionDescription`，中英）；默认折叠，行本身保持是一行。

### P3-k 列表协议（已做）

- **`extensions.ParseRows`**：把命令的 stdout 按列表协议解析（`[{id,title,subtitle,icon,group,kind,disabled,action}]`），
  规则是**全有或全无**——不是 JSON 数组、某一项缺 id/title、未知 icon、未知 kind、未知 action.type 或缺字段，
  整份输出退回文本（协议原话）；**空数组是「没有话说」的合法列表**，不是文本。
  行族、字形词表（link/file/folder/globe/star/clock/text/image/command）、action 三型
  （open/copy/insert）都按文档实现，并有正反两组单测。
- **启动器输出视图**：输出是列表时改画列表（可选中、↑↓ 走可运行行、跳过 status 行、分组名只印一次、
  status/disabled 行弱化且不可回车），回车按 action 执行：open 交给系统打开并收起、copy 写剪贴板、
  insert 把文本放回搜索框；列表与文本各有自己的按键提示行。文本形态不变（选中复制）。
- 未做：分页协议（`page.cursor/hasMore` 与滚动加载）、`⌘1`-`⌘0` 编号角标——这两项需要启动器的
  编号与分页机制（旧版由 `result-budget.ts` 统一分配），登记为后续。

### P2-i macOS 应用名与别名（已做）

- **`internal/plist`**：新包，读 Apple property list 的**两种编码**——XML 与 `bplist00`
  （macOS 实际写给 bundle 的那种）。只读，支持 dict/array/string/int/real/bool/date/data，
  损坏即错误（不返回半份数据）。单测用 Python `plistlib` 生成的同一份 Info.plist 两种编码做夹具
  （含中文名、嵌套数组、整数/实数/布尔）。
- **`internal/apps` 读 bundle 的 `Info.plist`**（此前只用文件夹名）：`CFBundleDisplayName`/
  `CFBundleName`、`CFBundleExecutable`、`CFBundleIdentifier`，以及 `en.lproj`/`Base.lproj` 与
  `zh-Hans.lproj` 等 `InfoPlist.strings`（plist 或 `"k" = "v";` 文本两种）。
- **名字归属按文字系统决定，而不是按读到的位置**（旧版的经验，逐条移植）：企业微信把中文写在
  Info.plist、英文在 `en.lproj`；Safari 反过来。`resolveBundleNames` 让**拉丁名可输入**（搜索用）、
  **本地化名作标题**、拉丁名作副标题；本地化名与拉丁名相同则不计。
- **别名**：bundle 名、文件夹名、`CFBundleName`、可执行名、identifier 的段（`com.apple.Safari`
  → `Safari`、`apple`），全部进启动器行的 `Search`，所以中英文名字都能搜到。
- 未做：应用图标（旧版从 `CFBundleIconFile` 取图并在行里画图标；Go 版行还没有图标位）。

### P2-j 应用图标（已做）

- **`internal/apps` 读图标**：macOS 从 `CFBundleIconFile`（缺省取 bundle 里唯一的 `.icns`）读
  `.icns`，`LargestPNGFromICNS` 取其中**最大的 PNG 条目**（`icp4`…`ic14`；老式原始位图类型跳过，
  宁可不显示也不显示错的）；Linux 解析 `.desktop` 的 `Icon=`（绝对路径直接用，名字去 hicolor /
  pixmaps 目录按尺寸找 PNG）。Windows 的 .lnk 图标在 shell 自己的数据库里，本构建不读。
- **懒读 + 缓存**：`apps.Icon` 只在行真的要画时才读文件（扫描 500 个应用不会读 500 个图标），
  解出的 `*ui.Bitmap` 在启动器里按路径缓存，重绘是查表。
- 行布局：有图标的行在标题前画 16DIP 图标（主题 `Space(4)`），其余行不变。

### P3-l 集成导入/导出（已做，范围如实说明）

- **文档格式**：旧版的 `version: 2` 文档（camelCase 字段名），本构建可读旧文档、旧构建也能读本构建的：
  `{version, exportedAt, extensions[{id, version, enabled, config, distributionSource,
  runtimeOwnership, fieldMetadata, manifest?, scriptContent?, providerDescriptor?}]}`。
- **什么会走**：身份/版本/启用状态/配置值；**包的要点**（manifest、脚本运行时的脚本、静态 provider 的
  descriptor），所以没有装过的机器也能装上；每个配置字段的分类记录（说明哪些被排除、为什么）。
- **什么绝不走**：密钥。字段名像凭据（secret/password/token/key/credential…）或值就是 `[REDACTED]`
  占位符 → 分类为 `secret` 且排除；绝对路径/`~`/Windows 盘符 → `device_path` 排除；版本约束单独分类。
  导出读的是**未合并密钥的原始配置**，导入写回时保留占位符与密钥代数，所以密钥始终留在 secrets 文件里
  （测试钉住：本机密钥不丢、不写进 values 文件）。
- **导入语义**：已安装的集成 → 只改启用状态与文档里的配置值（本机密钥与本机包保持）；未安装且文档带包
  → 用文档的 manifest/脚本/descriptor 走**与安装同一套** `PrepareLocal`（权限仍要用户批准，回调可注入）；
  未安装且文档不带包（旧构建可能的形态）→ 跳过并说明原因。每条都有 succeeded/failed/skipped 报告。
- **UI**：Integrations 页「导出 / 导入」两个按钮 + 一行结果；文件选择走 `mygo.Dialog`（测试注入）。
- **权限审批一次汇总**：先把所有待装包 stage 好、收集权限，再问一次；拒绝则一个都不装（staging 目录
  全部清掉），逐条记为 skipped。与旧版一致。
- **未做**：旧文档的 `package` 字段（旧构建写的包归档）本构建既不写也不读，但**原样保留**以便往返。

### P2-k 结果编号快捷键（已做）

- 启动器每帧给**能运行的前十个结果**编号：1–9 与 0（第十个），按当前视口重新分配（滚动即重编号），
  行尾显示编号角标；`⌘/Ctrl + 数字` 直接运行该行（旧版的 `select_result` 家族）。
- 只编号 `Run != nil` 的行（信息行不占号），与旧版「编号纯按序分配、没有保留位」一致。

### P4-o 文件拖放（已做）

- **`internal/drops`**：把桌面递过来的路径**规范化**——`~` 展开、相对路径按 drop 解析所在目录（不是
  进程 cwd）、不存在的丢掉；产出 `{path,name,directory,isDirectory}`。另外 `ShellQuote`/`CDCommand`
  按 POSIX（单引号 + `'\''`）与 Windows（双引号、内嵌引号丢弃）两种规则。
- **启动器 files 模式**（触发词 `files`/`file`/`drops`/`drop`/`文件`）：**每个文件三行安全动作**——
  「打开」（交给系统打开器）、「在此打开终端」（目录进目录、文件进其所在目录）、「复制路径」；
  可输入词过滤。**落地时什么都不执行**（旧版的原话：「结果只被提供，绝不被运行」），动作只在回车时发生。
- **终端**：`terminalui.RunShellIn(dir)` 以该目录起一个新会话（Go 版是进程内会话，直接给
  `Options.Dir`；旧版是往 PTY 里打 `cd ...`，两者语义相同）。
- 与旧版的差异：旧版把动作放在「动作条」（ActionBar）上、一行文件切换三个动作；Go 版没有动作条，
  改成每个文件三行——同样的三个选择，写在行本来就在的地方。

### P4-p 应用快捷键表（已做）

- **`shortcuts` 设置块**（类型化）：七个动作（toggle_window / new_command / open_external_terminal /
  copy_selection / paste / open_settings / select_result）各有平台默认值（`CmdOrCtrl` 语义：
  macOS 是 Cmd、别处是 Ctrl；非 macOS 的复制/粘贴用 `Ctrl+Shift+C/V`，以免抢走终端的 Ctrl+C）；
  存了不可解析的绑定就回落到默认（不会让动作没有键）。
- **`shortcuts.Parse`/`Match`**：把归一化的加速键**反解**成 `ui.Modifiers` + `ui.Key`，视图才能直接与
  键盘事件比对（不必维护第二张表）；无 Shift 的绑定接受大写字母，有 Shift 的必须按 Shift。
- **接线**：`select_result` 的数字键由该绑定推导（`SelectResultDigit`），改绑定即整套数字跟着走；
  `new_command` 清空输入、`open_external_terminal` 打开系统终端窗口在启动器内生效；`open_settings`
  同时是应用菜单的加速键；`copy_selection`/`paste` 由终端插件自己处理；`toggle_window` 是系统持有的
  唤回键（写 `hotkey` + `shortcuts.toggle_window`）。
- 设置页 Shortcuts 新增「应用按键」组：每个动作一行（名字 + 当前绑定 + 录制按钮，录制只改该动作）。

### P4-q 旧启动计数的导入（已做）

- 旧版把启动次数放在 `settings.json` 的 `launch_counts` 里；Go 版用 `usage.json`（次数 + 最近时间）。
  `usage.Store.Seed` 在启动时把**本构建没见过**的 id 按其次数补进来（本构建自己的记录优先、不覆盖），
  所以「最常使用」的列表不会因为换实现而消失；有变化才写盘。

### P2-l Windows 图标（已做）

- `.lnk` 的图标在 shell 自己的数据库里，唯一的 PNG 途径是问 shell：`icons_windows.go` 用
  PowerShell 的 `[System.Drawing.Icon]::ExtractAssociatedIcon` 提取，缓存到
  `<用户缓存>/floter/icons/<hash>.png`（hash = 快捷方式路径 + mtime，所以换了目标就会重新提取）。
- 源码/目标路径作为**单引号参数**拼进脚本（引号按 PowerShell 规则双写），路径里有引号也逃不出去。
- 单测：脚本引用规则在任何平台都能跑；真提取测试在 Windows 上跑（用 PowerShell 自己建一个 .lnk），
  非 Windows 编译不到。
- 三个平台因此都有图标：macOS `.icns` 最大 PNG 条目、Linux `Icon=`、Windows shell 提取。

### P4-r Safari 书签（已做）

- Safari 的书签在 `~/Library/Safari/Bookmarks.plist`（二进制 plist）。`internal/plist` 已经能读，
  于是补上 `readSafariBookmarks`：按 Safari 自己的节点类型递归——`WebBookmarkTypeList` 与
  `WebBookmarkTypeProxy`（Safari 自己的文件夹，用户的书签栏就在这里）都下钻，`WebBookmarkTypeLeaf`
  取 `URLString` 与 `URIDictionary.title`（缺省回落到 `Title`）。
- Safari 的 profile 现在**历史或书签存在任一**就列出（此前只有历史文件存在时才有 profile），
  书签文件缺失时只读历史。
- 夹具是 Python `plistlib` 生成的**真二进制 plist**（含嵌套文件夹、proxy 文件夹、根层书签），
  另有 XML 夹具覆盖「没有 URL 的叶子跳过」「没有标题的叶子用 URL」。
- 这是旧版**没有**的能力（旧版只读 Chromium JSON 与 Firefox places.sqlite），属补齐而非对齐。

### P4-s 工具安装目录与推荐工具接管（已做）

- **`internal/tools`**：旧版 `tool_catalog.rs` 的移植——12 个工具（flameshot / yt-dlp / jq / fd /
  ripgrep / fzf / bat / eza / tldr / httpie / gh / lazygit）的名称、中文关键词、探测名与**按平台的
  安装配方**，以及 8 个包管理器（brew/winget/pacman/apt/dnf/npm/cargo/pipx）的探测名与命令语法。
  规则照旧：**探测是 stat，不是 spawn**（不声称版本）、配方宁缺勿猜（比如 lazygit 没有 apt 配方、
  Windows 的 eza 走 winget）、`fdfind`/`batcat` 这类发行版改名也算命中。
- **启动器的安装行**：查询命中某个**尚未安装**的工具时给一行「安装 X」，Detail 是**为该机器选出的**
  命令（优先用本机已有的包管理器，没有就用该平台的第一条配方）；**回车只复制命令**，绝不执行——
  旧版的原话是「命令只由用户在自己的 shell 里输入」，那里才有代理、镜像与环境变量。
  已安装的工具不给安装行：PATH 命令扫描会找到可执行文件本身。
- **推荐工具接管**：仓库里的参考包移到 `internal/extensions/recommended/v-tools` 并 `go:embed`，
  Integrations 页列出**尚未安装**的推荐工具，一键「接管」= 落盘 → 走与其他本地工具**完全相同**的
  安装管线（权限照旧审批、失败照旧通知）；全部装好后该区消失。
- 未做：旧版的多来源工具清点（dpkg/rpm/pacman/flatpak/snap/nix/brew/注册表/scoop/choco/winget/WSL）
  ——那套主要用于「已发现工具 → 接管」的列表；Go 侧 PATH 命令扫描 + 本地/npm 安装 + 推荐工具接管
  覆盖了同一目标的主要路径。

### P4-t 快捷键收尾（已做）

- **恢复默认**：Shortcuts 页新增「恢复默认按键」——清空 `shortcuts` 映射（各动作回落到平台默认）
  并把 `hotkey` 写回 `Ctrl+Space`，随后**重新注册**系统唤回键（旧的先释放）。注册被系统拒绝时保留
  原来的键并记日志：因为一次被拒的改动而让快捷键失效，比不改更糟。
- **录制期间释放全局键**：录制时（内置唤回键或自定义快捷键）把**所有全局快捷键交给系统释放**，
  录制结束后再注册回来——否则按下「当前唤回键」会真的唤回窗口而不是被录下来。设置页在每帧末尾把
  录制状态同步给 shell（只发一次变化），所以捕获、Esc 取消、离开页面三条路径都覆盖。
- **顺带修一个真 bug**：自定义快捷键的录制器没有让出 Escape（只有内置录制器让了），于是按 Esc 取消
  录制时会把设置面板关掉。现在两个录制器都让出 Escape。

### P4-u 电源动作与图片缩略图（已做）

- **重启/关机**：查询**正好等于**这些词（`restart`/`reboot`/`重启`、`shutdown`/`shut down`/
  `power off`/`关机`…）时给出对应行，并**先于其它匹配**占位——用户打 `restart` 指的是机器，不是名字里
  带 restart 的应用。动作是 shell 的：**先弹原生确认**（平台自己的话 + 「所有程序会被关闭，不会替你
  保存」），确认后才执行平台命令（macOS 走 System Events 的 AppleScript、Windows `shutdown /r|/s /t 0`、
  Linux `systemctl reboot|poweroff`）；确认与执行都可注入，测试不碰真实系统。
- **剪贴板图片缩略图**：图片条目的行显示 32×24 缩略图（图片条目只靠「[image] + 时间」很难认），
  首次绘制时解码、按文件路径缓存——一百张图片的历史也只解码屏幕上那几张。

### P3-m 集成配置表单（已做）

- **`extensions.SaveConfiguration`**：按 schema 校验后写盘，密钥的走向与旧版一致——
  password 字段进一个**新生成的密钥文件**（0600、原子写、旧代清理），values 文件写 `[REDACTED]`
  占位符，存储的 schema 也一并落盘（password 的默认值打码）；校验规则照搬：未知键拒绝、必填缺省
  用默认值补、类型必须匹配、select/multiSelect 只收枚举值、number 有范围、text 有长度。
  之前 Go 侧只能**读**配置用于注入，用户没有办法改一个集成的配置——这是扩展平台的主要 UX 缺口。
- **设置页表单**：集成行新增「配置」按钮展开表单，按类型渲染控件
  （text/password/path→输入框，password 遮盖；boolean→复选；select→下拉；multiSelect→逐项勾选；
  number→数字输入），必填与说明都在；草稿在展开期间存活（重绘不丢）；错误显示在表单上。
- shell 把 provider 描述里的 schema 与本机已存的值接给表单，保存走 `SaveConfiguration` 并刷新集成。
- 端到端测试：带 schema 的 fixture → 表单 → 保存 → values 文件无密钥、密钥文件可读回、注入环境正确。

### P2-m 操作条（已做）

- 旧版 R10 的「操作条」：输入框里是 URL、文件路径还是命令，启动器直接给对应的行——
  URL →「在浏览器打开」；路径（存在与否都行）→「在文件管理器打开」+「在此打开终端」+「复制路径」；
  其它 →「在终端运行」。
- **Enter 落点的规则照搬**（`shouldDefaultToActionBar`）：shell 命令只在「会是回车运行的东西」时
  才给行——没有可运行结果、查询带空白或 shell 语法、或是那批命令词（git/npm/cargo/…）之一；
  否则目录的匹配行保留字段（不能被一条跑不了的发现行抢走 Enter）。
- 电源候选照旧版的 `run_first`：Linux 是 `systemctl reboot|poweroff` 然后 `reboot|poweroff`
  （非 systemd 的 init 走 SysV 二进制）；第一个**成功启动**的算数，之后的失败不再重试别的。
- 与旧版的差异：旧版把操作条画成字段下的一条、Enter 默认落在它上面（⌘⏎ 是「在 shell 运行」）；
  Go 版把它作为**结果列表的第一行**，Enter/编号/指针都一样能到——同一个默认落点，少一套键盘规则。
  「无结果」的空态因此被 shell 行取代（旧版在 resultCount==0 时也是这样落 Enter）。

### P2-n 内置命令的双语与拼音搜索（已做，应用名的拼音仍受阻）

- 内置命令行（设置/终端/浏览器/剪贴板/计算器/重启/关机/退出）现在带**完整搜索词表**：
  中英文措辞 + 中文名的**拼音首字母**（旧版在 `SYSTEM_COMMANDS` 里手写的那些，含 重 的两读
  `cq`/`zq`）。搜索词进 `Item.Search`，与别名同一条匹配路径。
- 新增 19 组用例：中文词、拼音首字母、英文措辞，全部命中对应的命令行。
- **应用名的拼音仍受阻**：`compute_initials` 用的是 `pinyin` 表（2 万+字），本机无网络、模块缓存里
  没有拼音数据；嵌入一个没法验证的表，错误的映射比没有功能更糟。受阻步骤记录在
  `docs/mygo-rewrite-status.md`：生成表 + 用已知词表校验后接入 `apps.computeInitials`。
- **子序列匹配**（`vsc` → Visual Studio Code，旧版 `scoreNormalized` 的第四档）同属此列：它只对
  应用名有意义（内置命令是精确词），和应用名拼音一起做。

### P4-v connect / register 深链（已做）

- 旧版 README 的「一条链接接入工具」：`floter://connect?manifest=/path/tool.json` 与
  `floter://register?cmd=rg`。Go 版此前只路由 settings/terminal/search。
- **connect**：manifest 路径（支持 `~`，相对路径按 home 解析）→ **同一个安装审查**：先准备、
  声明了权限就弹原生审批，批准才落盘；拒绝则什么都不装。manifest 可以是文件（包取其所在目录）
  或目录本身。
- **register**：工具已在 PATH 上、不需要 manifest——按旧版的语义「高亮集成页然后停住」，Go 版
  没有 Detected 列表，如实做法是**只把集成页带上来**，不高亮、不安装。
- 差异：旧版的审查是一个专门的 review surface（链接绝不安装，用户再按一次），Go 版用权限对话框
  完成同一次审查——同一道门，少一个界面。

### P3-n 生命周期探针（已做）

- 旧版 `probe_executor.rs` 的移植：manifest `lifecycle.probes` 的执行与 `probeReport` 的持久化。
  规则照搬——**探针参数是工具自己的参数**（与 provider 协议前缀无关：runtime 前缀 + probe.args），
  空声明不执行任何东西（v1 manifest 也是）；`stat` 级检测、stdout/stderr 捕获并封顶、
  每条探针自己的超时（缺省 10s）、required 失败 → `unhealthy` + 集成标记 broken
  （`lastErrorCode=probe_failed` + detail），optional 失败 → `degraded`（仍可用），
  全过 → `healthy`；空探针集证明不了什么，也不会抹掉早前的失败（`ErrNoProbes`）。
- 集成页「检查」现在**先跑探针再跑 diagnose**：探针是工具自己的健康检查，其状态（healthy/degraded/
  unhealthy）显示在行上，broken 的集成不再贡献命令（CommandEntries 已按 `Running()` 过滤）。
- 探针报告持久化在仓库条目的 `probeReport` 字段（旧版的字段名），未知键测试改为用其它键做未知样例，
  并新增「类型化的报告在写回后保留」的断言（旧构建未知形状的报告解析成空报告后依然保留）。

### P3-o lifecycle.launch 的接线（已做）

- manifest `lifecycle.launch` 是「集成整体怎么跑」的声明：program + 前置参数 + cwd 策略 +
  终端环境。此前 Go 版在 manifest 里有模型但从未接线——每个命令仍按自己的 execution 跑。
- 接线规则照旧版 `declared_execution`：**runtime binding 解析出的可执行在前**
  （解释器 + 脚本，或工具本身）；`program: "self"`（schema 默认）就是它；声明的程序替换它时，
  解释器参数一并替换；launch 的前置参数在其后，然后才是命令自己的 argsPrefix；
  cwd 策略解析（home / toolData（集成自己的数据目录）/ fixed:<绝对路径> / 其余继承）；
  终端环境 `TERM=floter-256color`、`COLORTERM=truecolor`、`TERM_PROGRAM=floter`。

### P3-p 健康状态的行上显示（已做）

- 仓库条目里持久化的 `probeReport.status`（healthy / degraded / unhealthy）现在显示在集成行上：
  「健康状态: 健康 / 部分功能可用 / 无法工作」。它是**跨运行**的——探针是仓库数据，列表从仓库读，
  所以不用等下一次检查也能看到上次的结果；「检查」会刷新它（先探针后 diagnose）。
- broken 的集成贡献的命令数为零（此前已有），这里再加断言钉住。

### P2-o 命令行的结构化解析（已做）

- 旧版 `parseCommandLine` 的移植：**结构化**解析字段文本——token（引号/转义/Windows 反斜杠路径的
  规则与旧版一致）、**前导 NAME=value 赋值**解码进环境（不再被 shell 重新解释）、以及
  shell 语法（未加引号的 `| & ; < > ( )`，只有真 shell 能跑）。
- 操作条的 shell 行改用它：argv 是解析出的命令与参数（引号不会拆开一个路径）、赋值作为环境
  传给终端会话（`RunInTerminalWithEnv`，终端以这些变量起会话）、只有赋值没有命令的行不运行。
- 与旧版的差异：POSIX 语法下反斜杠会转义下一个字符（所以 Windows 路径丢分隔符——旧版因此在
  Windows 上跑单独的 "windows" 语法，路径用反斜杠原样保留），双引号内的反斜杠保留。

### P2-p 安装行的终端交接（已做，修正 P4-s 的决定）

- P4-s 把安装行做成了「复制命令」——对照旧版发现那是**错的**：旧版 R68 的规则是「打开一个
  **裸** 终端会话（无命令），把命令**打进去**」，这样 R62 的「会话是命令启动的」判定保持 false，
  shell 是用户自己的、**安装结束后会话仍是交互式的**。
- `terminalui.RunShellWithCommand`：新开裸会话（`Command=nil`），等 shell 打出第一屏（提示符，
  5s 截止）后用 `Send` 把命令打进去——安装跑在用户自己的 shell 里（代理/镜像/环境都在），
  floter 不等待、不执行。
- `OpenInstallSession` 可注入，测试不碰真实终端。

### P2-q 查询历史（已做）

- 旧版 R7 的 shell 历史：用户在终端跑过的命令行（操作条的 shell 行）进历史——去重、提升、上限 20。
  ↑/↓ 的规则照旧版：**有结果时**箭头是结果导航；**无结果、无操作条时**箭头落到历史，且 ↑ 的第一下
  保留正在打的草稿（`draftBeforeHistory`），↓ 走到现在时带回来。
- 历史遍历的决策是纯函数式驱动测试的（不走字段编辑器）：编辑器的内部缓冲是框架按窗口保留的，
  测试走字段会跟它的状态打架——所以断言打在 walker 的决定上。

### P2-r 应用名的拼音首字母（已做，解除 P2-n 的受阻项）

- **`internal/pinyin`**：GB2312 一级常用字的拼音首字母表（约 2200 字，按最常见读音分组），
  `Initial(char)` 与 `Initials(name)`（ASCII 字母数字原样、CJK 换首字母、其余丢弃——分隔符不算，
  所以连打的查询能跨过它们）。多音字只列一个读音：initials 按**子序列**匹配名字，多音字通过名字里
  的其它字可达，两个读音都认反而会搅乱同名的其它应用。
- **`apps.computeInitials`**：名字与本地化名字折进同一个键（旧版 `compute_initials` 的语义），
  三个平台的扫描都填 `App.Initials`；启动器的应用行把它放进搜索词——**中文名从拉丁键盘可达**
  （`wyyyy` → 网易云音乐、`qywx` → 企业微信）。
- 表是离线写成的（GB2312 一级常用字集），并用已知词表验证（网易云/企业微信/微信/文件管理器/腾讯视频/
  百度网盘/Visual Studio Code…），多音字（乐 lè/yuè、重 chóng/zhòng）按常用读音归组并在测试里钉住。
- **子序列匹配仍未做**：initials 进了搜索词，但 `Match` 的阶梯还没有旧版 `scoreNormalized` 的第四档
  （subsequence），留作下一轮（它对内置命令的拼音也有效）。

### P2-s 子序列匹配（已做）

- `candidateScore` 的第四档：**词首子序列**——查询的字符按序出现在候选里，起点是候选的开头或
  词边界（空格/标点/CJK 之后）。`vsc` → Visual Studio Code、`wyyyy` → 网易云音乐（拼音 initials
  本身就是子序列）；比「包含」低一档。
- **词首约束**是必要的：无约束的子序列会让每个多词查询拖进一堆名字里恰好共享字母的行
  （`wcom` 会命中 "Shut down"）。匹配到 `Search` 文本时也按这条规则。
- 测试：拼音 initials（`qywx` → 企业微信，来自 `App.Initials`）、别名、以及「子序列可能拖进
  共享字母的行，但应用在其中」的如实断言。

### P2-t 应用扫描的缓存（已做）

- 旧版 `check_applications` 的性能语义：launcher 打开时**立即**用上一次的扫描结果，只有磁盘上的
  来源（根目录与前两级的条目）变了才重新扫描；签名检查限频 30 秒（突发 reveal 最多一批 stat）。
- `apps.CachedScan` / `StoreScan` / `SourceSignature`：签名是 FNV-1a（排序去重的路径 + 每条路径的
  长度与 mtime），与旧版 `paths_signature` 相同；缓存在进程内（旧版另有磁盘持久化缓存，这里没有——
  磁盘缓存的好处是冷启动更快，进程内缓存在同一运行里已经足够）。
- shell 的 `scanApps`：先问缓存，up-to-date 就直接交给 launcher；否则后台扫描并入库。

### P2-u 剪贴板历史的「清空历史」（已做）

- Plugins 页的剪贴板卡片新增「清空历史」：**确认后**移除所有未收藏条目（收藏豁免，`Clear` 的既有
  语义），确认可注入；原生对话框走与电源动作同一个 confirm 辅助。

### P4-w 终端空态的控制（已做）

- 旧版 R60 的终端空态：页面可以在没有会话时被打开（`floter://` 的 show-terminal 路径、✕ 关闭会话），
  而空画布「既没有出去的路，也没有进去的路」。空态现在是**标题 + 说明 + 两个控件**——
  「新建空白会话」与「回到搜索」；spawn 失败的错误替换说明文字。
- 沿用旧版的时序决定：空态只在页面真正没有内容时出现（有会话或保留帧时立即消失）。

### P4-x 终端标题栏的旧控件（已做）

- 旧版终端头部的两个控制回到标题行：「新命令」（回启动器、清空输入）与「在终端打开」
  （交给系统自己的终端窗口）。旧版只在有会话时给「在终端打开」（移交需要会话）；
  Go 版的 open-external 是**开一个新窗口**（进程内会话无法移交），所以有会话与否都显示——
  这个差异写在 Actions 里。
- 「在终端打开」的 `OpenExternal` 与自定义快捷键/应用菜单共用同一个入口（`OpenExternalTerminal`）。

### P3-p 组件化卸载（已做）

- 旧版的组件化卸载对话框：**程序总是移除**，三个数据类别（宿主配置 / 工具数据 / 生成的产物）是
  用户的勾选；三个都选 = 移除整个数据目录（旧版 `remove_entire_data` 的语义）。
  `extensions.UninstallComponentized` 走与普通卸载同一个 rename-aside-then-delete 的程序移除，
  数据类别按自己的路径移除；拒绝非法 id 与未知集成。
- 集成行的「卸载…」按钮展开对话框（标题 + 说明 + 三个勾选 + 卸载/取消），shell 的确认对话框照旧。
- i18n：en/zh 的对话框标题/说明/类别名/提示全部就位。

### P3-q 能力探测（已做）

- 旧版 `capability_probe.rs` 的移植：version/help 两个结构性探针 + manifest 声明的自定义探针。
  每条探针给工具传一组参数，退出码与 stdout 对照预期（expectedExitCode 缺省 0、expectedOutput
  是子串）；stdout 封顶 64KB、每条 5s 超时。
- 报告聚合：version 探针通过时取第一行非空输出为版本号（否则 `unknown`）、通过的探针 id 进
  `SupportedFeatures`、失败的进 `Limitations`（带参数与原因）。
- 集成页「检查」的流程现在是：探针（健康状况+能力）→ diagnose（工具自检）。

### P2-v .strings 的 UTF-16 与无引号键（已做）

- 旧版 `localized_name_from_strings_text` 的移植：`.strings` 文本的编码可能是 UTF-16（Xcode 为
  中文/日文名写出），带 BOM 检测（LE/BE）解码；键的**两种拼法**都读——Xcode 写引号形式，但格式
  允许无引号，手写的文件用那种形式（企业微信的英文名 `CFBundleDisplayName = "WeCom";` 就是无引号键）。
  块注释剥离避免注释里的键名被误读为条目。转义（`\"`、`\\`、`\\UXXXX`）解码。
- P2-r 的 `.strings` 读取此前只支持引号键的简单逐行格式——企业微信的英文名会因此读不到。

### P2-w 子序列搜索（已做）

- `candidateScore` 加了一档 `isSubsequence`：候选的字符按序出现（中间夹任何字符）即命中，
  评 3 分——低于包含（2），高于无匹配。`vsc` 找到 Visual Studio Code，`wyyyy` 通过拼音首字母
  找到网易云音乐（首字母正是名字的子序列）。
- `Match` 的逐词 AND 逻辑保持不变：多词查询的每个词都各自走梯子，子序列只在最深层兜底。

### P4-y 命令别名编辑器（已做）

- 设置 Integrations 页的每命令别名输入框（旧版 `ExtensionsPanel` 的 `extension-command-alias`）：
  `ui.TextInput` 挂在每个命令行下方，提交时经 `shell.setCommandAlias` 写入
  `settings.command_aliases`（键为命令名，清空即删除），随后重交启动器命令列表并重扫 PATH 工具。
- 冲突标记：列表填充时同时读原始 map 与 `ResolveCommandAliases` 的生效 map——字段保留用户
  输入（map 从不重写），但另一命令先占了别名时行内标注"别名已被其他命令占用"。先到先得、
  按命令名字典序，与设置层的既有语义一致。
- 顺带清掉了 `integrationList` 里遗留的一条调试日志。

### P1-z 启动器窗口高度跟随内容（已做）

- `internal/launcher/height.go`：窗口高度 = 卡片自己的带（字段行 Space(7)、列表上缘 Space(2)、
  每行 Space(1.5)×2 + 行高×1.4、卡片内边距 Space(2.5)×2）+ 平台内缩 + 不得超过屏幕帽。
  `Geometry` 收主题的 Font/Spacing 与每行的行数（View 里逐行测出：有副标题 2 行、没有 1 行），
  所以画的卡和收的窗口是同一个决定——旧版 R58 的"窗口=内容"不变式。
- `RowCount` 十行帽（超出的滚动）、`HoldRows` 收缩滞回一整行（1↔2 边界不抖）。
- 接线：View 逐帧把 `RowLines`/`Font`/`Spacing`/`HeldRows` 交回 shell（`Actions.ResizeTo`），
  `resizeLauncher` 记目标，`walkLauncherHeight` 在 View 里用 `Animate`（150ms EaseOut）逐帧
  `SetSize`——旧版 R68 的边缘行走，mygo 的 Animate 替我们续帧；高度没变就不碰平台。
- `targetSize`/`resize()` 用同一几何：召唤落点就是列表所在带，而不是固定板。

### P3-r `--help` 推断（已做）

- `internal/extensions/helpargs.go`：旧版 `help_args.rs` 的移植——`DeriveArguments`（clap/argparse/
  Go flag/cobra 四种风格的选项行 + 紧凑旗标摘要行 `I/O: -pipe (auto) · -file <path>`）、
  `DeriveSubcommands`（v 风格插件行 + cobra `Available Commands:` 段）、`stripANSI`（CSI/OSC/双字节
  转义）、`(aliases: …)` 组提取、字形装饰剥离、严格命令名 charset、`usage:`/URL/段落头/元行剔除。
- `ProbeDerive`：连接时一次 `--help` 探测根参数，发现是子命令列表时对每个子命令（至多 12 个）
  再探测自己的 `--help`（空了再试 `-h`）。尽力而为契约：任何失败静默降级，绝不阻塞连接。
  修了一个潜在 bug：`PackageDir()` 不存在时 `cmd.Dir` 会让 exec 直接失败（chdir before fork），
  现在只有目录真存在才设置（生命周期探测同样受益）。
- `SemverFromVersionOutput`：从 `--version` 输出提取第一个真 semver（容忍 `v` 前缀、噪声词、
  两段补齐、预发布保留）；垃圾进空出。

### P1-aa 指针交互的验证（已做）

- 鼠标点击与滚轮此前未被测试钉住。两个新测试：`TestClickingThroughTheScrollEdge`（滚动边缘
  条是 `PassThrough`，盖住的行点击仍然生效）与 `TestScrollingTheResultList`（滚轮事件移动列表）。
- 真实窗口的交互验证受限于 Xvfb 无窗口管理器（窗口无法 map/聚焦，事件到不了）——headless
  tester 与真实窗口走同一条 `ui.List`/`row.Clicked()` 路径，测试即证明。

### P1-ab 设置面板的指针路径（已做）

- `TestClickingASidebarRowSwitchesThePage`（侧栏点击换页）与 `TestScrollingTheSettingsBody`
  （设置主体滚轮滚动，`ScrollState.Y`/`MaxY` 钉住）——设置面板的点击与滚动同启动器一起有测试了。

### P1-ac 毛玻璃雾层的 PassThrough（已做，修「点不动」）

- **根因**：shell 卡片的雾层（haze veil）构建在 face 之后（paint order 在行上面），Absolute
  全卡覆盖，且没带 `PassThrough`——指针的 hit chain 命中雾层（祖先链只有 card→root），而普通
  box 自己没有动作。行全都**看得见**（Find 读的是标签），但点击永远到不了——用户报的
  「连基本的鼠标点击都没法进行」就是它。
- 修复：雾层是颜色不是控件，`PassThrough()` 让指针穿透——与启动器自己 scroll edge 的同一规则。
- `TestHazeVeilLetsThePointerThrough`（启动器行穿透雾层生效）与
  `TestSettingsRowsAnswerThroughTheVeil`（设置页同卡片）钉住整条路径。

## 纪律（继承）

- 承包 runner：禁 commit/push，树留脏主线复核；门槛实跑；报告落 /tmp。
- 主线：复核 → commit → push（本分支）。
- 用户数据零迁移成本优先于代码优雅：settings/扩展/tool-lock 的磁盘格式不变。
- 性能基线记录在案：mygo 官方数字（原生 UI ~44MB 内存、~7MB 二进制）作为 P6 验收参照。
