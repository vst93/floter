# floter × mygo 重构总计划（mygo-rewrite 分支）

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

### P2 还未做

- 结果虚拟化（目前 50 行上限）；插件命令与结果图标。
- 设置页除 General 外的真内容（Sessions / Shortcuts / Integrations / About 仍是占位）。
- i18n 仅新增了实际用到的键，未对齐旧 i18n.ts 全量表。
- surface-residency（保持上次页面）；唤回总回启动器。
- 无插件对位的终端项（滚轮行数、粗体模式、滚动条、选中即复制、安全粘贴）保持未接线。
- 终端窗口大小尚未在用户拖拽后写回 settings（下次开启仍是上次保存值）。

## 纪律（继承）

- 承包 runner：禁 commit/push，树留脏主线复核；门槛实跑；报告落 /tmp。
- 主线：复核 → commit → push（本分支）。
- 用户数据零迁移成本优先于代码优雅：settings/扩展/tool-lock 的磁盘格式不变。
- 性能基线记录在案：mygo 官方数字（原生 UI ~44MB 内存、~7MB 二进制）作为 P6 验收参照。
