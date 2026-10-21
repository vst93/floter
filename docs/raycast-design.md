# Raycast 设计合约（本仓的落地版）

> 目标不是像素级复刻 Raycast，而是把「近黑底 + 1px 发丝线 + 低光泽 + 紧凑行 +
> 中性选中 + 右侧键帽提示」这套语言落到本仓的 token 与视图上。
>
> 本文件是本仓的**单一来源**：视图只读 `theme.Tokens`，不写字面色值。
>
> 切片：P8-a = 合约 + token 层 + 启动器表面（已做）。设置页、终端、集成/弹层
> 留给 P8-b / P8-c，§6 给出差距清单。

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

## §5 accent 的用途（全仓只允许三处）

1. **设置侧栏选中项的前导图标**（`internal/settingsui/settingsui.go:1332`）。
2. **主 CTA 填充**（`internal/settingsui/settingsui.go:3719` 的 `updateButton`）。
3. **焦点环**（`Focus` / `InputStrokeActive` / `AccentGlow`，见
   `internal/theme/theme.go`）。

其余一切——启动器选中行、页脚、分组头、空态、行 hover/按下、图标底——**不用
accent**。`AccentHalo` / `AccentGlow` 在启动器表面上一次都不画（Linux 聚焦卡的
keyline 属于卡片壳，不属于列表）。

反证留在测试里：`selface_test.go` 的选中行断言既等于 `RowSelected` 的合成值，
又证明「R−B 的差」与无彩色不足一个阈值（即不含 accent 的粉），
`raycast_test.go` 另有一条直接读三通道相等的 pin。

## §6 未落地页面的差距清单（P8-b / P8-c 直接引用）

### §6.1 设置页（P8-b）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/settingsui.go:1301` 侧栏选中行 | `RaisedQuiet`（不透明抬升面）+ `tokens.Lens` | Raycast 侧栏选中也是**中性**：`RowSelected` 叠在面板上，镜片去掉（或降到 `LensRim` 一档） |
| `internal/settingsui/settingsui.go:1292` 侧栏行圆角 | `RadiusSM`（8） | 保留 8；但行块左右缩进要对齐 §4.3 的 8u 语汇 |
| `internal/settingsui/settingsui.go:1332` 侧栏前导图标 | 选中时 `t.Accent` | **保留**（§5 第 1 条，这是 accent 的三处之一） |
| `internal/settingsui/rows.go:288` 分组卡 | `RadiusMD`(10) + `ControlEdge` + `Control` + `tokens.Lens` | 合约下面板内的块是 10 + 弱发丝线（`HairlineSoft`），镜片去掉；面用 `Control` |
| `internal/settingsui/rows.go:1188` `fadeRule`（横/竖分隔） | 渐变发丝（`lead 0.12 / solid 0.76`） | Raycast 的分隔是**一条实弱发丝**（`HairlineSoft`），渐隐属于旧语言；删除渐变，改画 1px |
| `internal/settingsui/pluginsheet.go:328` 字段槽 | `RadiusSM` + `ControlEdge` + `Field` + `FieldSlot` | 保留几何；面收敛到 `KeycapFace`/`Control` 一档（低光泽） |
| `internal/settingsui/pluginsheet.go:366` 弹层按钮 | `Radius(t.Space(3.5))`（跟随 `--u`） | 半径梯是 literal：`RadiusSM`(8)，不许 `Space()` 化 |
| `internal/settingsui/healthfaces.go:231` 权限药丸 | `AccentTintQuiet.Over(RaisedQuiet)` + `AccentEdge` inset | 合约只用中性：`Control` + `ControlEdge`；accent 留给 §5 的三处 |
| `internal/settingsui/settingsui.go:1994,2028` 状态 chip | `RaisedWarm` + `t.Warning` 环 | 预热面保留（系统语义色），但环降到 `ControlEdge`，字重不超过 600 |
| `internal/settingsui/settingsui.go:3719` 主 CTA | `Radius(999)` 胶囊 + `t.Accent` | **保留 accent**（§5 第 2 条）；胶囊改 `RadiusSM`(8)——Raycast 的 CTA 是圆角按钮，不是药丸 |
| `internal/settingsui/settingsui.go:3569` 录制器 | `RaisedQuiet` + `Border(t.Accent)` | 录制中才允许 accent 环；静息用 `Control` |

### §6.2 终端（P8-c）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go:864` 面板 | `Radius(tokens.WindowRadius)` + `panelFill` | 保留（面板**是**窗口，§2）；`panelFill` 的档位要按 §1 的低光泽上限复核 |
| `internal/terminalui/terminalui.go:926` 会话挂载 | `Radius(RadiusLG)`(12) + 1px `edge` | 终端画布在面板里，应用 `RadiusMD`(10)（面板内的块）；12 只给「是窗口」的面 |
| `internal/terminalui/terminalui.go:1104` 反馈条 | `RadiusMD` + `ControlEdge` + `Float` | 保留几何；`Float` 的低光泽要复核 |
| `internal/terminalui/terminalui.go:1150` 警告条 | `RadiusMD` + `Border(tokens.AccentEdge)` | accent 环在这一处不属于 §5 的三处：改 `t.Warning.Alpha(...)` 或 `ControlEdge` |
| `internal/terminalui/terminalui.go:1298` 状态灯 | `Shadow(0,0,0,Space(0.5), AccentHalo)` | `--elev-halo` 是旧语言的辉光；合约要**去掉 halo**，只留一个 1.75u 的 accent 点（§1 低光泽） |
| `internal/terminalui/terminalui.go:1198` 标题条 | hover 时 `Shadow(0, Space(1.5), Space(4), 0, rgba(0,0,0,0.18))` | 保留（它是「可拖拽手柄」的唯一提示）；但让它读 `ShadowContact` 而不是字面 rgba |
| `internal/terminalui/drawer.go:265` 读数 | `FontSize(Body)` + `Accent` + 600 | accent 在这里越界（§5）；改 `TextStrong` + tnum |
| `internal/terminalui/drawer.go:45` 抽屉面 | 浮层（`Float` + `RadiusMD`） | 保留；抽屉的边缘按 §4.1 的弱发丝线 |

### §6.3 集成页 / 弹层（P8-c）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/settingsui.go:1994` 状态 chip | `RadiusSM` + `RaisedWarm` + `t.Warning` inset | 同上表 §6.1 |
| `internal/settingsui/settingsui.go:2373` 行面 | `RaisedQuiet` + `AccentEdge` + `Accent` bar | 中性：`RowSelected` / `RowHover`，accent 只留前导图标 |
| `internal/settingsui/pluginsheet.go:360` 弹层动作 | `Radius(Space(3.5))` + `danger` 用 `t.Warning.Alpha(0.45)` | 半径改 `RadiusSM`；danger 的环保留（语义色），但不要额外 accent |
| `internal/settingsui/pluginsheet.go` 弹层面 | `Float` + `RadiusMD` | 保留；确认低光泽（无彩色 rim） |

**启动器里自己没做的两处**（不在 §6 表里，写在 §4 对应小节）：
§4.2 的搜索行前导放大镜、§4.6 的 `⌘K` 动作面（chord 未绑定）。

## §7 切片表

| 片 | 范围 | 状态 |
|---|---|---|
| P8-a | 合约文档 + token 层 + 启动器表面（卡片/搜索行/列表/分组头/页脚/空态/键帽） | **本轮完成** |
| P8-b | 设置页（侧栏/分组块/行/控件/弹层）+ 两个空态 + `⌘K` 动作面 | 排队 |
| P8-c | 终端表面（页签/边框/滚动边缘/抽屉）+ 集成页 + 通知/浮层 | 排队 |
