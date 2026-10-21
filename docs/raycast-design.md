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

1. **设置侧栏选中项的前导图标**（`internal/settingsui/settingsui.go:1332`）。
2. **主 CTA 填充**（`internal/settingsui/settingsui.go:3719` 的 `updateButton`）。
3. **控件处于「已选 / 已开」状态时的状态色**：分段控件的选中段 keyline、开关的
   ON 态、收音机的选中项。这是**状态**，不是装饰——它回答的正是「哪个被选中了」。
4. **焦点环**（`Focus` / `InputStrokeActive` / `AccentGlow`，见
   `internal/theme/theme.go`）。

仍禁止：列表选中行、页脚、分组头、空态、行 hover/按下、图标底、弹层危险动作的
彩色环、终端状态灯的 halo、抽屉读数文字。

**判断标准一句话**：accent 只能回答「哪个被选中 / 哪个是开的」，不能回答「这块
面好看」。

> P8-c 勘误：本节在 P8-b 时写成「全仓只允许三处」，实测（General 页 1591 个
> accent 像素，分四带：侧栏图标、三组分段控件的选中段、开关 ON 态）发现条文写窄
> 了——Raycast 的分段控件选中项和开关 ON 态**本来就是** accent。这不是实现跑偏，
> 是合约漏了「控件状态」这一类。第 3 条即补上的那一类，并各有正面 pin 钉住
> （`TestTheChosenSegmentKeepsItsAccentKeyline`、`TestTheSwitchOnStateCarriesTheAccent`）。

其余一切——启动器选中行、页脚、分组头、空态、行 hover/按下、图标底——**不用
accent**。`AccentHalo` / `AccentGlow` 在启动器表面上一次都不画（Linux 聚焦卡的
keyline 属于卡片壳，不属于列表）；`AccentHalo` 在终端的状态灯上也于 P8-c 去掉
（§6.2 第 5 条）。

反证留在测试里：`selface_test.go` 的选中行断言既等于 `RowSelected` 的合成值，
又证明「R−B 的差」与无彩色不足一个阈值（即不含 accent 的粉），
`raycast_test.go` 另有一条直接读三通道相等的 pin。

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
| 状态灯（`terminalui.go:1298`） | `Shadow(0,0,0,Space(0.5), AccentHalo)` | **去掉 halo**，只留 1.75u 的 accent 点（§1 光泽上限 0.04，`AccentHalo` 是 0.2） | `已落地` — `terminalui.go:1301-1306` |
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

## §7 切片表

| 片 | 范围 | 状态 |
|---|---|---|
| P8-a | 合约文档 + token 层 + 启动器表面（卡片/搜索行/列表/分组头/页脚/空态/键帽） | **本轮完成** |
| P8-b | 设置页（侧栏/分组块/行/控件/弹层）+ 两个空态 | **本轮完成**（`⌘K` 动作面未做，见 §4.6） |
| P8-c | 终端表面（页签/边框/滚动边缘/抽屉）+ 集成页 + 通知/浮层 + §5 accent 预算勘误 | **本轮完成**（§6.2/§6.3 全部 `已落地`；§5 从「三处」改为四类，新增 9 条正面 pin） |
