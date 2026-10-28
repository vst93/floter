# Raycast 设计合约（本仓的落地版）

> 目标不是像素级复刻 Raycast，而是把「近黑底 + 1px 发丝线 + 低光泽 + 紧凑行 +
> 中性选中 + 右侧键帽提示」这套语言落到本仓的 token 与视图上。
>
> 本文件是本仓的**单一来源**：视图只读 `theme.Tokens`，不写字面色值。
>
> 切片：P8-a = 合约 + token 层 + 启动器表面（已做）。P8-b = 设置页（已做）。
> P8-c = 终端 + 集成/弹层 + accent 预算勘误（已做），§6 给出逐条落地。

---

## §1 颜色

两套外观，一套语义。表里的数值就是 `internal/theme/theme.go` 里
`paletteFor()` 落的值（`ForPreferences` 解析后）。

| 语义 | token | dark | light |
|---|---|---|---|
| 面板底（合成后） | `Theme.Background` | `#0E0E10` | `#F7F7F8` |
| 主文字 | `Theme.Text` / `TextStrong` | `#EDEDEF` | `#1B1B1D` |
| 次文字 | `TextSecondary` | `#9A9AA0` | `#6B6B70` |
| 中间档（占位/弱标签） | `Theme.TextMuted` | `#85858D` | `#808087` |
| 三级文字（键帽字/元数据） | `TextTertiary` | `#6E6E76` | `#8E8E94` |
| 发丝线（强：卡片的 1px 边） | `Hairline` = `InputStroke` | `rgba(255,255,255,0.09)` | `rgba(0,0,0,0.09)` |
| 发丝线（弱：行间/块内规则） | `HairlineSoft` | `rgba(255,255,255,0.055)` | `rgba(0,0,0,0.055)` |
| 控件自己的边 | `ControlEdge` | `rgba(255,255,255,0.07)` | `rgba(0,0,0,0.06)` |
| 控件静息面 | `Control` | `rgba(255,255,255,0.075)` | `rgba(0,0,0,0.05)` |
| 控件 hover / press | `ControlHover` / `ControlPress` | `0.105` / `0.05` | `0.075` / `0.032` |
| 行 hover 填充 | `RowHover` | `rgba(255,255,255,0.045)` | `rgba(0,0,0,0.04)` |
| 行选中填充 | `RowSelected` | `rgba(255,255,255,0.085)` | `rgba(0,0,0,0.06)` |
| 行按下填充 | `RowPressed` | `rgba(255,255,255,0.11)` | `rgba(0,0,0,0.08)` |
| 键帽底 / 键帽边 | `KeycapFace` / `KeycapEdge` | `rgba(255,255,255,0.075)` / `0.07` | `rgba(0,0,0,0.05)` / `0.06` |
| accent（品牌红） | `Theme.Accent` | `#F5544E` | `#E5484D` |
| accent 静默填充 | `AccentTintQuiet` | `rgba(245,84,78,0.14)` | `rgba(229,72,77,0.10)` |
| 卡片外阴影（环境 / 接触） | `ShadowAmbient` / `ShadowContact` | `rgba(0,0,0,0.55)` / `0.40` | `rgba(0,0,0,0.18)` / `0.10` |
| 卡片受光边 | `EdgeHighlight` | `rgba(255,255,255,0.15)` | `rgba(255,255,255,0.92)` |
| 低光泽上限（透镜/光泽/卡片 sheen） | `LensRim`/`LensSheen`/`LensEdge`/`SheenTop` | 全部 ≤ `0.04` | 全部 ≤ `0.04` |

**两条不变量**（测试在 `internal/theme/theme_test.go`）：

1. `alpha(Hairline) == alpha(InputStroke)`，且严格大于 `HairlineSoft`：卡的边与
   分区线是同一条发丝的两种宽度，不是两种材质。
2. `RowHover < RowSelected < RowPressed`（按 alpha 严格递增），三档全部是
   **中性**（三通道相等），一个都不含 accent。

低光泽上限是 Raycast 与「液态玻璃」的分界：旧的透镜/高光/彩色 rim 全部压到
0.04 以内，分层靠「面 + 线 + 字阶」，不靠高光。

**可读性不变量（P11-aa）**：用户要求更多对比度时（系统 `prefers-contrast: more`
→ `ui.Preferences.HighContrast` → `palette.contrast()`），**四档墨**在
**面板底**与**选中行填充**两个表面上都必须 ≥ 4.5:1（WCAG AA 正文档），且阶梯次序
（`Text` > `TextSecondary` > `TextMuted` > `TextTertiary`）在两个表面上都保持。
实测值（两个表面依次）：浅 `#1B1B1D` 16.06/14.2 · `#4E4E54` 7.72/6.75 · muted
5.68/5.36 · `#66666C` 5.33/4.66；深 `#EDEDEF` 16.49/14.4 · `#B0B0B8` 8.95/7.37 ·
muted 6.27/5.72 · `#8A8A92` 5.63/4.64。pin：
`internal/theme/theme_test.go` 的 `TestEveryInkStepClearsTheReadableContrast`。
**默认外观不承诺这一条**：三档安静墨同时过 4.5:1 在 `#F7F7F8` 上放不进一条 0.02
宽的亮度窗口（算术见 `docs/rounds/P11-aa-spec.md` 附注），属拍板项。

## §2 半径梯

literal px，**不随 `--ui-scale` 走**（`Token.Radius*` 在每一档界面大小里都是同一
组数）：

| token | 值 | 用在哪 |
|---|---|---|
| `RadiusXS` | 6 | 图标底/小 chip |
| `RadiusSM` | 8 | **行块、按钮、字段**（框架 `Theme.Radius` 也取这个） |
| `RadiusMD` | 10 | 面板内的块（分组卡、浮层） |
| `RadiusLG` | 12 | 窗口、整张卡 |
| `WindowRadius` | 12（三平台一律） | 启动器卡、设置卡、终端面板 |
| `KeycapRadius` | 5 | 键帽 |

`WindowRadius` 三平台统一 12 是刻意的：Windows 的 DWM 圆角已由
`shell.dressWindowsFrame`（`DWMWCP_DONOTROUND`）关掉，卡的角是屏上唯一的角，
所以三平台画同一张脸。

## §3 尺寸（高度链不动）

这一片**不改高**，只改墨与面。

- 窗口随内容定高、9/10 行预算、⌘N 槽表、行型常量单源：数值与公式一个字都没动
  （`internal/launcher/height.go`）。
- 行高 `rowUnits = 10`（40u），搜索行 `fieldRowUnits = 14`（56u），页脚
  `actionBarUnits = 11.25`（45u），状态行 `launcherStatusUnits = 7.5`（30u）：
  全部是改动前的值。
- 字号档位不动，只允许同档内改 weight / letter-spacing / 颜色。唯一例外是空态
  一句从 body 抬到 `Emphasis`（`docs` §4.7），因为旧值是 body 且 muted，读起来
  像一条错误提示。
- **水平方向的两个数**：
  - 文本列 = 卡内 54u（`8u 行块缩进 + 8u 行内 padding + 28u 图标列 + 10u gap`）。
    这是 `TestTheFieldAndTheRowsShareTheReferenceTextColumn` 钉住的不动点。
  - 行块绘制边 = 距卡内 8u（改动前 4u）。行内 padding 从 12u 降到 8u，所以
    **行块内缩 4u、文本列不动**。

## §4 表面规格

### §4.1 卡片

圆角 `WindowRadius`（12），四周 1px 强发丝线（`InputStroke`），外阴影
`ShadowAmbient` + `ShadowContact` 两笔（`0 18px 48px` / `0 1px 2px` 的等价
`Space()` 形式，见 `internal/shell/shell.go:4214-4215`）。平台的三笔 inset
rim 见 `internal/shell/cardedge.go`（Windows/Linux 各两条，macOS 不画）。

玻璃档位（`GlassStep`）与透明度设置语义**未改**：她只影响 frame 的 alpha、sheen
的 cap 与透镜的 `--glass-lens-scale`，不改变这一节的任何数值。

### §4.2 搜索行

高度不变（56u）。字段行是卡片的第一段（`FieldFill`），其下沿是**一条弱发丝线**
（`HairlineSoft`）：不再是「悬浮输入框 + 阴影」，而是卡里的一段。查询文字走主
文字（`Theme.Text`）落在 16u 列上；占位符走 `Theme.TextMuted`。

**未落地项**：Raycast 的放大镜在文本列**左侧**的独立槽里。本仓的字段行把 16u
padding 全花在文本列上（查询与行标题都从 16u 起），在行内插标记会把整条文本列
推右 22u——那是本片唯一不许动的水平度量。所以这一版**不画前导放大镜**，缺口记
在这里（见 §6 末尾）。

### §4.3 结果列表

- **行块缩进 8u**（左右镜像），行块圆角用 `RadiusSM`（8）：选中填充成为一块
  不顶到卡片边的圆角药丸。
- 行间不画线：靠明度与行块自身的圆角分。
- 三档状态（全部中性，无边框、无 ring、无 glow）：
  - hover `RowHover`
  - 选中 `RowSelected`，标题 weight 抬到 700（emphasis 在字重，不在颜色）
  - 按下 `RowPressed`，并 `Scale(0.986)` 沉进卡片（旧 build 的 `:active`）
- 选中行的**前导图标不换 accent**，只把图标墨色抬到主文字；accent 在列表里一次
  都不出现。
- 行内 padding `0 8u`，图标列 28u，标题与副标题间距 10u（`Space(2.5)`）。

### §4.4 分组头

字阶不动（沿用 body 步），weight 500 + `TextSecondary`，左缩进与行**文字**列对齐
（`Margin-left: Space(10.5)` = 42u，即 8u 行块 + 8u padding + 28u 图标列的起点-
行内偏移之和）。窗口为它 charge 的 band 不变（`headingPadding`）。

### §4.5 键帽

一行里出现的键是「可按的键」，不是一句带符号的文本：

- 一个键 = 一块 `20u × 20u` 的板（`MinWidth/Height` = `Space(5)`），圆角
  `KeycapRadius`（5），底 `KeycapFace`、边 1px `KeycapEdge`。
- 标号 `Caption` 步、weight 500、`TextTertiary`，单行居中。
- 组合键 = 每键一块，键间 `Space(0.5)`（2u），**不画 `+`**。
- 平台拼写由 `shortcuts.Badge` 决定（macOS `⌘⇧1` 逐个字形切；其它平台按 `+`
  切）；`Ctrl` 这类词比 20u 宽，板沿自己的 padding 长出去，**高度永不变化**。
- 行尾的键槽（`Ctrl+1`）与页脚自己的 hint（`Ctrl+↩`）是**同一语汇**：同底、同边、
  同圆角、同高。没有第二套键帽形状。

### §4.6 页脚动作条

- 页脚是卡片的一个 **block**，不是第二张卡：顶部一条弱发丝线
  （`HairlineSoft`），**没有**独立抬升面、没有旁边投影、没有渐变分隔、不花
  accent。
- 页脚整行的选中态与结果行同一档（`RowSelected`），因为 Enter 在这一行就是
  「运行它」。
- 右侧 = `⌘K` / `Ctrl+K` 键帽（§4.5 的同一块板），是「动作面」的图例。
  **这个 chord 在本 build 里没有绑定**：键盘模型不动，动作面属于 P8-b。
- 页脚高度 `actionBarUnits`（45u）不变；页脚 band 钉在窗口底边，不随列表滚动。

### §4.7 空态

一句话，居中在 band 里：`Emphasis` 步 + `TextStrong`，单行，**无底板、无图标、
无 accent**。旧值（body + muted）读起来像一条错误提示。

### §4.8 动效

- 颜色过渡：`rowTransition` = 90ms ease-out（Colors only）。按帧可预测，无弹跳。
- 行的进入：120ms 透明度（`Enter`），位置由布局决定，**不用整卡 pose
  transform**（mygo 的渲染红线：整棵子树会掉进逐像素路径）。
- 选中即时，不做缩放/弹跳；按下的 `0.986` 是唯一的几何变化。

## §5 accent 的用途（四类，判据一句话）

1. **设置侧栏选中项的前导图标**（`internal/settingsui/settingsui.go:1328`）。
2. **主 CTA 填充**（`internal/settingsui/settingsui.go:3739` 的 `updateButton`）。
3. **控件处于「已选 / 已开」状态时的状态色**：分段控件的选中段 keyline、开关的
   ON 态、收音机的选中项。这是**状态**，不是装饰——它回答的正是「哪个被选中了」。
4. **控件级焦点环**（框架的 `ui.Theme.Focus`，即 `FocusRing`，以及输入框 caret）。
   **P11-z 定审（用户实测 F2「整体红色线条太扎眼」）**：本条只到「控件」为止——
   一个控件在若干兄弟里被键盘选中的那一圈。**窗口/卡片级的整圈描边不属此列**：
   `InputStrokeActive`（启动器整张卡的 1px 边）与 `AccentGlow`（Linux 聚焦卡的
   inset 环）回答的是「这个窗口有焦点」，而窗口在用时**永远**有焦点，于是那圈红
   是常量装饰。两者已改/删：`InputStrokeActive` 现在是同一根发丝线的亮一档
   （浅 0.09→0.16 黑，深 0.09→0.16 白），`AccentGlow` 标 `Deprecated`。

仍禁止：列表选中行、页脚、分组头、空态、行 hover/按下、图标底、弹层危险动作的
彩色环、终端状态灯（已去掉 halo，P8-e 再去掉色相——只靠呼吸动）、抽屉读数文字、
**整个窗口/卡片外圈的描边**（P11-z）。

**判断标准一句话**：accent 只能回答「哪个被选中 / 哪个是开的」，不能回答「这块
面好看」。

> **预算只约束色相，不约束明度**：本条约束的是**墨与面的色相**；中性档之间的
> 差异（`TextStrong`/`TextSecondary`/`TextTertiary`、`RowHover`/`RowSelected`/
> `RowPressed`）不属本预算，靠明度分层是**鼓励**的做法。一句「这条不要 accent」
> 的正确改法通常是**降一档明度**（`Accent` → `TextStrong`）而不是把所有差异
> 抹平；两条中性断言反过来也要能分出上下（P8-d 的 pin 就看这个）。

> P8-c 勘误：本节在 P8-b 时写成「全仓只允许三处」，实测（General 页 1591 个
> accent 像素，分四带：侧栏图标、三组分段控件的选中段、开关 ON 态）发现条文写窄
> 了——Raycast 的分段控件选中项和开关 ON 态**本来就是** accent。这不是实现跑偏，
> 是合约漏了「控件状态」这一类。第 3 条即补上的那一类，并各有正面 pin 钉住
> （`TestTheChosenSegmentKeepsItsAccentKeyline`、`TestTheSwitchOnStateCarriesTheAccent`）。

其余一切——启动器选中行、页脚、分组头、空态、行 hover/按下、图标底——**不用
accent**。`AccentHalo` / `AccentGlow` 在启动器表面上一次都不画（Linux 聚焦卡的
keyline 属于卡片壳，不属于列表）；`AccentHalo` 在终端的状态灯上也于 P8-c 去掉
（§6.2 第 5 条），P8-e 再去掉它的**色相**——那个点现在穿 `TextTertiary`，存活只由呼吸动说（§6.2 的「状态灯」行 / §6.5 第 1 条）。

反证留在测试里：`selface_test.go` 的选中行断言既等于 `RowSelected` 的合成值，
又证明「R−B 的差」与无彩色不足一个阈值（即不含 accent 的粉），
`raycast_test.go` 另有一条直接读三通道相等的 pin。

### 注脚：边界四条（P8-d 定审）

§5 的四类只写了「控件状态」。以下四条一直卡在边缘（既像状态又像标记），P8-d
逐条定审并把理由写在这里，避免下一轮又把它们当成越界重新改一遍。**两条保留 / 一条
改中性 / 一条保留**：

1. **权限 ● 标记（集成页权限列表）→ 保留 accent**
   （`internal/settingsui/settingsui.go` 的 `permissionTier`）。它就是「已强制」的
   另一种画法：实心/空心即 ON/OFF，属第 3 条的状态色，只是把「开关」换成了
   「●/○」两个字形。P8-d 另有一条守卫 pin（`TestThePermissionDotAndTheRegisterChipKeepTheAccent`）
   防止一刀切把 accent 全删掉。
2. **更新进度条填充 → 保留 accent**
   （`internal/settingsui/settingsui.go:3785`，`updateRow` 里的 `update-banner__progress-fill`）。
   它不是装饰，而是**主 CTA 的派生物**：同一个下载动作的进度，属第 2 条主 CTA
   填充那条预算的延伸。进度条没有第二套中性视觉语汇可用，换成灰档会丢「这条
   正在动」这个唯一的含义。
3. **会话可恢复红点（5u 徽标）→ 改中性**（`internal/settingsui/rows.go` 的
   `sessionIconButton`）。它说的是「这条**可以恢复**」——一个动作的**机会**，
   既不是「这条被选中/被打开」，也不属于主 CTA。P8-d 改为 `TextSecondary`；
   召唤力交给**位置**（按钮自己的右上角）与按钮本身的 hover 档，不用色相。
4. **`packageChip` 的 register 行前导图标 → 保留**
   （`internal/settingsui/rows.go` 的 `packageChip(…, chosen)`）。它就是第 1 条的
   同一个处理（选中项的前导图标），与设置侧栏的同名位置同构；P8-c 已落地，
   P8-d 补同一守卫 pin 的后半（`TestThePermissionDotAndTheRegisterChipKeepTheAccent`）。

判这四条的方法是同一个：问它回答的是「哪个被选中 / 哪个是开的 / 这是主 CTA 的
一部分」还是「这块面好看 / 这条重要 / 这是活的」。前者合法，后者越界。

## §6 差距清单（P8-b / P8-c 已全部落地）

### §6.1 设置页（P8-b，**已落地**）

| 位置 | 改成的 | 落地 |
|---|---|---|
| 侧栏选中行（`internal/settingsui/settingsui.go:1302`） | 中性 `RowSelected`，镜片去掉；圆角保留 8 | `已落地` — `settingsui.go:1286`（`sidebarRow`） |
| 侧栏行圆角/缩进（`settingsui.go:1290`） | 保留 `RadiusSM`(8)；行块缩进沿用侧栏自己的 12u 列内 padding（§4.3 的 8u 语汇在 148u 的固定列里已由列 padding 承担，本片不改水平度量） | `已落地` — 度量零改动，见报告「判不准项」第 1 条 |
| 侧栏前导图标（`settingsui.go:1328`） | **保留** `t.Accent`（§5 第 1 条） | `已落地`（未改，反证 pin：`TestTheSidebarChosenGlyphKeepsTheAccent`） |
| 分组卡（`internal/settingsui/rows.go:288`） | 面 `Control`、边弱发丝 `HairlineSoft`、半径 10、无镜片 | `已落地` — `rows.go:288-292`（`cardGeometry` 的卡片框） |
| 分隔线（`internal/settingsui/rows.go:1193`） | 渐变发丝 → **一条实弱发丝** `HairlineSoft`（1px，全宽同色） | `已落地` — `rows.go:1193`（`hairlineRule`，替换原 `fadeRule`）；调用点 `settingsui.go:1194,1228` |
| 弹层字段槽（`internal/settingsui/pluginsheet.go:327`） | 几何保留（28u / `RadiusSM` / `ControlEdge` / 8u），面收敛到 `Control` 一档，去掉 `FieldSlot` 凹痕叠层 | `已落地` — `pluginsheet.go:327` |
| 弹层按钮半径（`pluginsheet.go:363`） | `Radius(t.Space(3.5))` → `RadiusSM`(8)（literal，不随 `--ui-scale`） | `已落地` — `pluginsheet.go:368` |
| 权限药丸（`internal/settingsui/healthfaces.go:212`） | 两级都中性：`Control` + `ControlEdge`；accent inset 去掉 | `已落地` — `healthfaces.go:212` |
| 状态 chip（`settingsui.go:1983`、`healthfaces.go:47`） | 预热面保留，环降到 `ControlEdge`，字重 620 → **600**；`healthy` 的 `RaisedQuiet`+`AccentEdge` → `Control`+`ControlEdge` | `已落地` — `healthfaces.go:47,78`、`settingsui.go:1983,2014`（自加第 12 条） |
| 主 CTA（`settingsui.go:3714`） | **保留 accent 填充**（§5 第 2 条），胶囊 → `RadiusSM`(8) | `已落地` — `settingsui.go:3717-3718`（`updateButton` 的 `Radius`/`Background`） |
| 录制器（`settingsui.go:3556`、`custom.go:76`） | 静息 `Control`；只有录制中才 accent 环；静息 hover 不再把边抬成 accent | `已落地` — `settingsui.go:3567-3569`（armed）、`settingsui.go:3627-3639`（静息）、`custom.go:76-79`（custom armed） |
| 空态（`internal/settingsui/empty.go:30`） | §4.7：主句 emphasis + `TextStrong`，图标 `TextSecondary`，无底板无 accent | `已落地` — `empty.go:30`（`customempty` / `integrationsempty` 两处都走这一个原语） |

### §6 末尾 · P8-b 自加条目（第 12 条）

§6.1 没列、但同属设置页的旧 glass 语言残留，本片一并收敛：

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| 健康 chip 的 `healthy` 面与环（`healthfaces.go:78`） | `RaisedQuiet` 不透面 + `AccentEdge` accent 环 | `Control` + `ControlEdge`（面上只留一档中性洗） | `已落地` |
| 健康 chip 的透镜（`healthfaces.go:47`） | 在发丝环上再叠 `--elev-0`（`tokens.Lens`） | 只有那一圈 `ControlEdge`；4px 高的药丸不要内侧高光 | `已落地` |
| `stateMark` / `runtimeUnavailableBadge` 的 broken chip（`settingsui.go:1983,2014`） | `t.Warning` 1px 环 | `ControlEdge`（预热面保留，环中性），字重 ≤600 | `已落地` |
| 分组卡的分隔线（`rows.go:1188` 旧 `fadeRule`） | 三段渐变，颜色取**强**发丝 | 一条 `HairlineSoft` 实线 | `已落地`（并入 §6.1 第 4 条） |
| 录制器静息 hover（`settingsui.go:3638`） | hover 把边抬到 `t.Accent` | hover 只抬面（`ControlHover`）与字（`TextStrong`），不抬色相 | `已落地` |
| 录制器的 armed 面（`settingsui.go:3569`、`custom.go:78`） | `RaisedQuiet` 不透面 | `Control`（与静息同一档面）+ accent 环 | `已落地` |

### §6.2 终端（P8-c，**已落地**）

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| 面板（`internal/terminalui/terminalui.go:866`） | `Radius(WindowRadius)` + `panelFill` | 保留（面板**是**窗口，§2），并抽成 `panelCorner`；`panelFill` 只有一层面（调色板底色 × `CanvasAlpha`），无 rim/sheen/lens，故无光泽要压 | `已落地` — `terminalui.go:866`（`panelCorner`），复核见报告 §① 第 1 条 |
| 会话挂载（`terminalui.go:932`） | `Radius(RadiusLG)`(12) + 1px `edge` | **`RadiusMD`(10)**（面板内的块；12 只给「是窗口」的面），抽成 `mountCorner` | `已落地` — `terminalui.go:931-932`（`mountCorner`）+ `terminalui.go:1359-1368`（两个 helper） |
| 反馈条（`terminalui.go:1110`） | `RadiusMD` + `ControlEdge` + `Float` | 几何保留；`Float` 只是一层不透面（`TintAlpha + floatStep`，无 rim/sheen），低光泽复核通过 | `已落地` — `terminalui.go:1110`，复核见报告 §① 第 3 条 |
| 警告条（`terminalui.go:1161`） | `Border(tokens.AccentEdge)` | **`Border(tokens.ControlEdge)`** —— accent 环不属于 §5 的任何一处；语义由 warning 墨 + 15u 警示字形说 | `已落地` — `terminalui.go:1161` |
| 状态灯（`terminalui.go:1298`） | `Shadow(0,0,0,Space(0.5), AccentHalo)` + `t.Accent` 的点 | **去掉 halo**，只留 1.75u 的点（§1 光泽上限 0.04，`AccentHalo` 是 0.2）；P8-e 再把点的**填充**从 `t.Accent` 改 `tokens.TextTertiary`，呼吸动效保留 | `已落地` — P8-c：`terminalui.go:1301-1306`；P8-e：`terminalui.go:1309-1315`（见 §6.5 第 1 条） |
| 标题条（`terminalui.go:1216`） | hover `Shadow(…, rgba(0,0,0,0.18))` | 读 **`tokens.ShadowContact`**（深 0.40 / 浅 0.10），不再写字面 rgba | `已落地` — `terminalui.go:1222` |
| 抽屉读数（`internal/terminalui/drawer.go:276`） | `FontSize(Body)` + `Theme.Accent` + 600 | **`TextStrong`** + tnum（register 与权重不动：body / mono / tnum / 600） | `已落地` — `drawer.go:274-277` |
| 抽屉面（`drawer.go:59`） | 浮层几何 + `BorderColor(ControlEdge)` 的上边线 | 几何保留（`Control` 面 + 16/12u padding + 10u gap）；上边线按 §4.1 降为**弱发丝** `HairlineSoft` | `已落地` — `drawer.go:59` |

### §6.3 集成页 / 弹层（P8-c，**已落地**）

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| 状态 chip（`internal/settingsui/settingsui.go:1994`） | `RadiusSM` + `RaisedWarm` + `t.Warning` inset | P8-b 已改成 `ControlEdge` 环，本片复核并**补正面 pin**（`TestTheStatusChipRingIsTheControlEdge` 已在 P8-b 落地，P8-c 复核像素侧仍绿） | `已落地`（复核） — `settingsui.go:1989-1993`、`healthfaces.go:47,78` |
| 行面（`settingsui.go:2376`，原 2368） | `RaisedQuiet` + `AccentEdge` + `Accent` bar | **`RowSelected`** 一档，无环、无前导规则条；accent 只留前导图标（`packageChip(…, chosen)`） | `已落地` — `settingsui.go:2372-2379`（`integrationRowFace`）、`settingsui.go:2412`（`packageChip` 调用）、`rows.go:1044-1069`（`packageChip`） |
| 弹层动作（`internal/settingsui/pluginsheet.go:368`） | `Radius(Space(3.5))` + `danger` 用 `t.Warning.Alpha(0.45)` | P8-b 已把半径改 `RadiusSM`；danger 的**语义色环保留**（`t.Warning` 语义，不是 accent），本片确认无额外 accent | `已落地`（复核） — `pluginsheet.go:367-377` |
| 弹层面（`pluginsheet.go`） | `Float` + `RadiusMD` | 保留；确认无彩色 rim（`Float` 是一次 `floatFill.Alpha`，无 rim/sheen 叠层） | `已落地`（复核） — 见报告 §① 第 4 条 |

### §6 末尾 · P8-c 自加条目（第 13 条）

§6.2/§6.3 没列、但同一族语言、本片一并收敛的：

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `fieldFace` 的字段材质（`internal/settingsui/textinput.go:76`） | `tokens.Field` + `FieldSlot`（浅色下 0.85 白边） | `tokens.Control` + `ControlEdge` 一档（与弹层 `sheetFieldSlot` 同面） | `已落地` — `textinput.go:64-77` |
| 多选列表箱（`internal/settingsui/config.go:830`） | `tokens.Field` + `FieldSlot` | `tokens.Control` 一档（同一个应用里字段只有一种材质） | `已落地` — `config.go:826-840` |
| `theme.Tokens.FieldSlot`（`internal/theme/theme.go:1019`） | 0.85 白边的凹痕叠层 | 标记 `Deprecated`，**保留函数体**（删除留给下一次清理轮；已 grep 确认无其它调用点） | `已落地` — `theme.go:1004-1031` |

**启动器里自己没做的两处**（不在 §6 表里，写在 §4 对应小节）：
§4.2 的搜索行前导放大镜、§4.6 的 `⌘K` 动作面（chord 未绑定）。

### §6.4 P8-d · accent 花名册收口（**已落地**）

不新增功能、不改交互、不动几何：只把「文字/标记用 accent」的越界处改成中性墨，
并把 §5 的判据第一次落到全仓（§5 注脚四条即本片的定审）。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `statusInk` 的 `ok`/`healthy`（`internal/settingsui/settingsui.go`） | `t.Accent` | `tokens.TextStrong`；`error`/`broken` **保留** `t.Warning`（语义色不是 accent） | `已落地` — pin `TestStatusInkIsNeutralWhenHealthyAndWarningWhenNot` |
| `stateMark` 的启用分支（「运行中」字，同上） | `t.Accent` | `tokens.TextStrong`（与 `default` 分支的 `TextSecondary` 差一档明度） | `已落地` — pin `TestTheRunningStateMarkIsTheNeutralStrongRung` |
| `rowSegmentRole`（`rows.go`） | `rowSegmentAccent` → `t.Accent` | 更名为 `rowSegmentStrong` → `tokens.TextStrong`（会话 meta 的「运行中」） | `已落地` — pin `TestTheSessionRunningSegmentIsNeutral` |
| `actionWithGlyph`（`rows.go`，reset 行图标 + 文字） | `t.Accent` | 静息 `tokens.TextSecondary`，hover `t.Text`（只抬明度） | `已落地` — pin `TestTheResetActionIsNeutralAtRestAndOnHover` |
| `cardRow` 的 hover 前导规则条（`rows.go`） | hover 时 `bar = t.Accent` | **删掉**该 bar（hover 只抬面 `SurfaceHover`）；这是 §5 明写的禁止项「行 hover/按下」 | `已落地` — pin `TestTheCardRowHoverCarriesNoAccentPixel` |
| `transferReport` 导入成功提示（`transfer.go`） | `t.Accent` | `tokens.TextStrong`（成功是结果语义，不是选中） | `已落地` — pin `TestTheImportSummaryIsNeutral` |
| `uninstallActionButton` 危险确认按钮 hover 边（`uninstall.go`） | `tokens.AccentEdge`（且 hover 把 warning 墨改成 `t.Text`） | hover 边 `ControlEdge`、面 `ControlHover`，**墨仍是 `t.Warning`**：危险语义留在墨与文案上 | `已落地` — pin `TestTheDangerPillHoverKeepsTheNeutralEdgeAndTheWarningInk` |
| `uninstallPartRow` 勾选卡 hover 边（同上） | `tokens.AccentEdge` | `ControlEdge` | `已落地` — 同一条 pin 的后半 |
| `extensionActionFace` hover 边（`rows.go`） | `tokens.AccentEdge` | `ControlEdge` | `已落地` — 同一条 pin |
| `barButton` hover 边（`settingsui.go`，P8-d 自加第 14 条：同一个 `.extensions-action-button` hover 规则的最后一处尾巴） | `tokens.AccentEdge` | `ControlEdge` | `已落地` — 同一条 pin |

**判不准四条 → 定审结果**（理由见 §5 注脚）：权限 ● **保留**、更新进度条**保留**、
会话可恢复红点**改中性**、`packageChip` 前导图标**保留**。

### §6.5 P8-e · accent 收口最后三处（**已落地**）

P8-d 报告 §⑥ 把全仓 accent 收敛到 25 个代码位点后，还剩三条**确实越界、但不在
P8-d 授权范围内**的位点。三者同一族（都不是「哪个被选中 / 哪个是开的」），一轮收完，
§5 的规则才第一次真正做到全仓一致。只改墨，不动几何/交互/文案。

| # | 位置 | 旧 | 新 | 落地 |
|---|---|---|---|---|
| 1 | 终端状态灯（`internal/terminalui/terminalui.go` 的 `statusDot`） | 活着的会话 = `t.Accent` 的点（带呼吸） | **`tokens.TextTertiary`**，呼吸保留——存活由「动」说，不由色相说。exited 态的 0.7 灰点不动 | `已落地` — `terminalui.go:1309-1315`；pin `TestTheStatusDotIsNeutralAndStillBreathes`（正面：通道差 ≤ 12；反面：不是 accent；呼吸：两帧墨量差 ≥ 500）、`TestTheExitedStatusDotStaysAGreyDot` |
| 2 | 成功 toast（`internal/shell/notices.go` 的 `toastTone`） | `success → Theme.Accent` | **`tokens.TextStrong`**（比未标记 toast 的 `TextSecondary` 高一档明度）；成功语义靠 ✓ 字形与文案。warning/error 的语义色原样保留 | `已落地` — `notices.go:168`；pin `TestToastTonesAndMarksFollowTheirKind`（V 翻面） |
| 3 | 输出面板选中行（`internal/launcher/output.go` 的 `outputRow`） | `tokens.AccentTintQuiet` | **`tokens.RowSelected`**（与启动器列表 P8-a 的选中行同一档） | `已落地` — `output.go:547`；pin `TestTheOutputSelectedRowIsTheNeutralChosenRung`（两套外观，正面 + 反证 + 行内零 accent 像素） |

**随之收口的 token**：`theme.Tokens.AccentTintQuiet` 在本次改动后**零生产调用点**
（只剩字段定义与 `theme_test.go` 的值 pin），按 P8-c 对 `FieldSlot` 的处理标
`Deprecated` 并**保留函数体/字段**，删除留给下一次清理轮。

**新增守卫**（§2，同 `TestTheTerminalViewsPaintFromTokensNotLiterals` 一族）：

1. `TestTheTerminalAndShellViewsSpendNoAccent` —— `internal/terminalui/terminalui.go`
   与 `internal/shell/notices.go` 的生产行不得再出现 `Accent`（注释除外）。
2. `TestTheStatusDotIsNeutralAndStillBreathes` + `TestTheExitedStatusDotStaysAGreyDot`
   —— 正面（无彩度、呼吸在、exited 仍是灰点）与反面（不是 accent）成对。
3. `TestTheOutputSelectedRowIsTheNeutralChosenRung` —— 面 == `RowSelected` over 面板底，
   且行内无 accent 像素。

**为什么 success 用 `TextStrong` 而不是 `TextSecondary`**：未标记 toast 已经穿
`TextSecondary`（CSS 的默认 `color`）。若成功也用它，「确认」与「普通信息」就只剩
一个 ✓ 字形可分；§5 注脚允许并**鼓励**用中性档之间的明度分层，而 P8-d 已把同族的
导入成功提示（`transferReport`）定在 `TextStrong`，两者同语汇。

### §6.6 P11-z · 用户实测 F2 + F3：红线只回答语义、终端画布不再套第二圈框（**已落地**）

用户实测原话：「整体红色线条太扎眼」「终端页面外圈那圈类玻璃边偏离『轻』的质感」。
只改墨与面，不动几何/交互/文案。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/theme/theme.go` 的 `strokeActive`（浅/深） | `rgba(229,72,77,.55)` / `rgba(245,84,78,.55)` | `rgba(0,0,0,.16)` / `rgba(255,255,255,.16)`（静息 `stroke` 0.09 的亮一档） | `已落地` — pin `TestTheFocusedCardEdgeIsNeutral`（两套外观：通道差 ≤ 8，且与静息档不同） |
| `internal/shell/cardedge.go` 的 Linux 聚焦卡 | 多画一圈 `inset 0 0 0 1px AccentGlow` | 删掉（焦点只由 `EdgeHighlightActive` 与 `SheenAtFocus` 的亮面说） | `已落地` — pin `TestTheFocusedCardDrawsNoAccentRing`；`TestTheCardEdgeBrightensWithTheKeyboard` V 重定基线（3 条 → 2 条） |
| `internal/terminalui/terminalui.go` 的画布 | `Border(1, edge)`（聚焦时 `InputStrokeActive`），面板(12) 里再套一圈(10) 的框 | 删掉，只留 `Radius(mountCorner)` 的填充：面板是这一页唯一的框 | `已落地` — pin `TestTheTerminalCanvasDrawsNoSecondFrame`（页面最外一列像素 == 画布填充） |

**旧账（本轮查到的）**：`theme.For` 从未给 `Tokens.AccentGlow` 赋过值（构造块只有
`AccentEdge`/`AccentEdgeStrong`/`AccentHalo` 三行），所以 Linux 那圈 inset 环一直
用零值颜色画，**从来没真的画出来过**；旧 pin 拿 `tokens.AccentGlow`（零值）去比，
自然永远绿。本轮删掉那条路径，字段按 P8-e 对 `AccentTintQuiet` 的先例标
`Deprecated`、保留不删。

**留下的唯一一条红线**：搜索框的文本光标（框架 `ui/editor_layout.go` 用
`Theme.Accent` 画 caret）。它不是描边，也不是「这块面好看」——它回答「文字从这里
进去」，且只有 1px 宽。要不要也改中性属拍板项（`.pending-bugs.md`）。

### §6.7 P11-aa · 高对比度要抬「字」，不只抬「线」（**已落地**）

用户实测 F2（「整体红色线条太扎眼」）的验收标准里写着「保持浅色背景下的**可读性与
对比度达标**」。P11-z 量的是描边像素，**没有量过一个字**；本轮补上这半边。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/theme/theme.go` 的 `palette.contrast()`（浅） | 只动 `stroke`/`controlEdge`/`textMuted`；`TextSecondary` `#6B6B70` 4.95:1、`TextTertiary` `#8E8E94` **3.04:1** | `#4E4E54` 7.72:1、`#66666C` 5.33:1（选中行填充上 6.75 / 4.66） | `已落地` — pin `TestEveryInkStepClearsTheReadableContrast` |
| 同上（深） | `#9A9AA0` 6.89:1、`#6E6E76` **3.82:1** | `#B0B0B8` 8.95:1、`#8A8A92` 5.63:1（填充上 7.37 / 4.64） | `已落地` — 同一条 pin；`TestHighContrastSwapsTheStrokeAndPlaceholderTokens` V 重定基线（旧「只动三条」→ 新「只动五条」） |

**默认外观一个字节都没改**（`paletteFor` 未动），所以普通用户的界面像素级不变。
剩下的缺口是**默认**浅色安静档的 3.04:1，它是设计取舍（见 §1 可读性不变量与
`docs/rounds/P11-aa-spec.md` 附注），已进 `.pending-bugs.md` 的需拍板表。

## §7 切片表

| 片 | 范围 | 状态 |
|---|---|---|
| P8-a | 合约文档 + token 层 + 启动器表面（卡片/搜索行/列表/分组头/页脚/空态/键帽） | **本轮完成** |
| P8-b | 设置页（侧栏/分组块/行/控件/弹层）+ 两个空态 | **本轮完成**（`⌘K` 动作面未做，见 §4.6） |
| P8-c | 终端表面（页签/边框/滚动边缘/抽屉）+ 集成页 + 通知/浮层 + §5 accent 预算勘误 | **本轮完成**（§6.2/§6.3 全部 `已落地`；§5 从「三处」改为四类，新增 9 条正面 pin） |
| P8-d | accent 花名册收口：越界 9 处转中性 + 判不准 4 条定审 + §5 注脚 + 视觉 pin 处置 | **本轮完成**（§6.4 全部 `已落地`；§5 补「只约束色相、鼓励明度分层」一句与边界四条注脚；新增 10 条 pin + 2 张×2 视觉证据） |
| P8-e | accent 收口最后三处：终端状态灯去色相、成功 toast 转中性、输出面板选中行改 `RowSelected` | **本轮完成**（§6.5 全部 `已落地`；§5/§6.2 同步；`AccentTintQuiet` 标 `Deprecated`；新增 4 条 pin + 3 张×2 视觉证据） |
