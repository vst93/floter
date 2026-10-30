# Raycast 设计合约（本仓的落地版）

> ⚠️ **本文件已由 [`docs/liquid-glass.md`](liquid-glass.md) 升代（P12-a，2026-10-30）。**
> 本文**只保留行为契约与已落地条目**：§1 的语义 token、§2 半径梯、§3 高度链、
> §4 各表面规格（除光泽上限）、§5 accent 的四类用途与注脚、§6 的全部已落地条目。
>
> **被取代的两条**：
> 1. §1 末尾的「低光泽上限（透镜/光泽/卡片 sheen）全部 ≤ 0.04」——那是为「**无光泽**」
>    定的，而新总目标要「**有受光边**」，两者直接冲突。新上限见 `liquid-glass.md` §1
>    末尾的「旧值 → 新值」表（`LensRim` 0.04 → **0.09**，其余仍 ≤ 0.04）。
> 2. §1 的「分层靠面 + 线 + 字阶，不靠高光」——分层现在由 `liquid-glass.md` §2 的
>    三档 elevation（底 / 面 / 浮）承担：面差 + 一条受光边 + 浮层的两层外投影。
>
> 视觉方向的单一来源因此变成 **`docs/liquid-glass.md`**；本文是它的行为契约底座。
> 逐表面差距表与选题队列在 `liquid-glass.md` §6。
>
> 目标不是像素级复刻 Raycast，而是把「近黑底 + 1px 发丝线 + 低光泽 + 紧凑行 +
> 中性选中 + 右侧键帽提示」这套语言落到本仓的 token 与视图上。
>
> 视图只读 `theme.Tokens`，不写字面色值。
>
> 切片：P8-a = 合约 + token 层 + 启动器表面（已做）。P8-b = 设置页（已做）。
> P8-c = 终端 + 集成/弹层 + accent 预算勘误（已做），§6 给出逐条落地。
> P12-a = 升代到轻便液态玻璃：新合约 `liquid-glass.md` + 材质/层级 token 层（已做）。

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
| 控件静息面 | `Control` | `rgba(255,255,255,0.10)` | `rgba(0,0,0,0.07)` |
| 控件 hover / press | `ControlHover` / `ControlPress` | `0.14` / `0.05` | `0.105` / `0.032` |
| 行 hover 填充 | `RowHover` | `rgba(255,255,255,0.045)` | `rgba(0,0,0,0.04)` |
| 行选中填充 | `RowSelected` | `rgba(255,255,255,0.085)` | `rgba(0,0,0,0.06)` |
| 行按下填充 | `RowPressed` | `rgba(255,255,255,0.11)` | `rgba(0,0,0,0.08)` |
| 键帽底 / 键帽边 | `KeycapFace` / `KeycapEdge` | `rgba(255,255,255,0.075)` / `0.07` | `rgba(0,0,0,0.05)` / `0.06` |
| accent（参考的蓝） | `Theme.Accent` | `#8FB7FF` | `#064AA6` |
| accent 静默填充 | `AccentTintQuiet` | `rgba(143,183,255,0.085)` | `rgba(6,74,166,0.065)` |
| 卡片外阴影（环境 / 接触） | `ShadowAmbient` / `ShadowContact` | `rgba(0,0,0,0.62)` / `0.44` | `rgba(0,0,0,0.22)` / `0.12` |
| 卡片受光边 | `EdgeHighlight` | `rgba(255,255,255,0.20)` | `rgba(255,255,255,0.92)` |
| 光泽上限（**已被升代取代**） | `LensRim` ≤ 0.09，`LensSheen`/`LensEdge`/`SheenTop` ≤ 0.04 | 见 `liquid-glass.md` §1 | 同左 |

**P11-al（用户实测 F2「整体红色线条太扎眼」的根因）**：这两行曾是「品牌红」
`#F5544E`/`#E5484D`，而参考 `main` 的 accent 从头到尾是蓝
（`--accent: #8fb7ff` / `#064aa6`，`main:src/styles/base.css:675,942`）；base.css 里红只出现在
`--ansi-*` 与 `--text-warning`。accent 换回参考的蓝之后，**红在本仓只剩 `Warning` 一族**
（警告条、危险按钮、破坏性确认）。accent 家族其余档（`accent-edge .30/.36`、
`-edge-strong .72/.68`、`-halo .18/.24`、`-glow .08/.20`、`selection` `.13`/`.10`、`focus`）同轮对齐。

**两条不变量**（测试在 `internal/theme/theme_test.go`）：

1. `alpha(Hairline) == alpha(InputStroke)`，且严格大于 `HairlineSoft`：卡的边与
   分区线是同一条发丝的两种宽度，不是两种材质。
2. `RowHover < RowSelected < RowPressed`（按 alpha 严格递增），三档全部是
   **中性**（三通道相等），一个都不含 accent。

**P12-a 重定基线（本表有四行已升代）**：`Control`（`0.075/0.05` → `0.10/0.07`）、
`ControlHover`（`0.105/0.075` → `0.14/0.105`）、`ShadowAmbient`/`ShadowContact`
（深 `0.55/0.40` → `0.62/0.44`、浅 `0.18/0.10` → `0.22/0.12`）、`EdgeHighlight`
（深 `0.15` → `0.20`）。旧值到新值的完整理由与新上限在 `docs/liquid-glass.md` §1/§3。

**旧的低光泽上限已作废**：那句「旧 build 的透镜/高光/彩色 rim 全部压到 0.04 以内，
分层靠面 + 线 + 字阶，不靠高光」描述的是 P8 那一代的「无光泽」语言；新目标是
「**一条受光边**」，两者直接冲突，所以上限被 `docs/liquid-glass.md` §1 显式改写
（`LensRim` 0.04 → **0.09**，其余仍 ≤ 0.04，且 rim 必须严格亮于 sheen）。
分层也不再靠「线」：由 §2 的三档 elevation（底 / 面 / 浮）承担。

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
文字（`Theme.Text`）；占位符走 `Theme.TextMuted`。

**前导放大镜（P11-ab 已落地）**：Raycast 的放大镜在文本列**左侧**的独立槽里，本仓
在 P8-a 时**没画**（缺口记在这里），P11-ab 补上。落法不占字段行的 16u padding：
放大镜**占用结果行那 28u 的图标列 + 10u gap**（`Space(7)` 的方框、框内居中
`Space(4)` 的 `Glyph("search")`、右 `Space(2.5)` 的 margin），墨 `TextMuted`
（与占位符同档，不花 accent）。于是查询从 16u 挪到 **16 + 28 + 10 = 54u**——
**正好是结果行标题的列**，字段与列表第一次共用一条文字列
（`TestTheFieldAndTheRowsShareTheReferenceTextColumn` 的名字成为字面事实）。
**一个模式拥有字段时（scope mark）放大镜让位**：槽里只有一样东西，scope mark
保持自然宽度（它是一个词，裁了就什么都不指）。pin：
`internal/launcher/actionbarfooter_test.go` 的 `TestTheSearchRowLeadsWithANeutralMark`
（墨落在 16..44、44..53 的 gap 为空、墨无暖色）。

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
- 行内 padding `0 8u`，图标列 28u，**行自己的 gap 10u（`Space(2.5)`）**——这一个数同时
  管着图标列与文字列之间、文字列与行尾控件之间。
- **同一张卡里的两种列表只有一条文字列（P11-ad 定审）**：启动器列表与它自己的输出面板
  是同一张卡的两种列表（同一个窗口、同一个 8u 卡片内缩、同一个 28u 图标列、同一个 8u
  行内 padding），所以横向节奏必须取同一个数：**标题列 54u**。P11-ad 之前输出面板的行
  gap 是 8u，于是它的标题列是 52u——用户在列表与输出之间来回看时文字列无缘无故错 2u。
  输出面板的行 gap 已改成同一个 10u（`internal/launcher/output.go` 的 `Gap(t.Space(2.5))`），
  它的分组头 `Margin-left` 跟着 36u → 38u。pin：`internal/launcher/textcolumn_test.go`
  的 `TestTheTwoListsShareOneTextColumn`（断言两个表面的标题 x **相等**，不是字面数）。

### §4.4 分组头

字阶不动（沿用 body 步），weight 500 + `TextSecondary`，左缩进与行**文字**列对齐。
窗口为它 charge 的 band 不变（`headingPadding` 的 6u/4u）。

**分组头的位置只有一个答案（P11-ac 定审）**：分组头落在**本表面结果行标题的那一列**上，
而且这个位置必须从「行的标题列」这个**和**里推出来，不许另写两个手写数。落法是把行自己的
横向分量原样搬过来：行内 padding 8u（`rowBody` 的 `Padding(0, t.Space(2))`）+ 图标列 28u
（`iconPlate` 的 `Space(7)`）+ 该行自己的 gap（启动器列表 10u，输出面板 8u）= 文字的
`Margin-left`（启动器 38u / 输出面板 36u），于是两边分别落在 **54u** 与 **52u**——各自的
标题列。

P11-ac 之前这里是两个手写数（行左 padding 12u + `Margin-left` 42u），从来没有从行里推出来；
P8-a 把行块缩进 4u→8u、行内 padding 12u→8u 之后它们一个都没跟着动，于是**同一件元素在两个
表面上给了两个错答案**：启动器列表的分组头比它管辖的标题右挂 8u（实测 62u vs 54u），输出
面板的分组头比它自己的标题左挂 32u（20u vs 52u）。pin：`internal/launcher/headingcolumn_test.go`
的 `TestTheGroupHeadingSitsOnTheRowsTextColumn`（断言的是两个矩形的 **x 相等**，不是字面数，
所以以后再挪列两处会一起红）。

P11-ad 把「这个和」的最后一项也对齐了：输出面板的行 gap 从 8u 改成与启动器列表同一个 10u，
于是**两种列表的标题列都是 54u**，分组头的 `Margin-left` 跟着 36u → 38u（§4.3 同一条）。

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

**启动器里自己没做的一处**（不在 §6 表里，写在 §4 对应小节）：§4.6 的 `⌘K` 动作面
（chord 未绑定）。§4.2 的搜索行前导放大镜已由 **P11-ab** 补上（见 §6.8）。

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

### §6.8 P11-ab · 搜索行补上前导放大镜：字段与结果行共用一条文字列（**已落地**）

`docs/raycast-design.md` §4.2 在 P8-a 时把这一条记成「未落地」：Raycast 的放大镜在
文本列左侧的独立槽里，本仓的字段行把 16u padding 全花在文本列上，插标记会把文本列
推右 22u。P11-ab 换了个落法——**标记不占 padding，占结果行那 28u 的图标列**。
只改墨与几何，不动高度链、键盘模型、文案。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/launcher/launcher.go:1266-1278`（field row，scope mark 之前） | 无元素，只有一段「本片不画前导标记」的注释 | 无 scope 时画 **28u 方框（`Space(7)`）+ 右 10u（`Space(2.5)`）**，框内居中 16u（`Space(4)`）的 `Glyph("search")`，墨 `t.TextMuted` | `已落地` — pin `TestTheSearchRowLeadsWithANeutralMark`（`internal/launcher/actionbarfooter_test.go`）：墨落在 16..44、44..53 的 gap 为空、暖像素 0 |
| 同一行，scope mark 分支 | 与字段并排，自然宽度 | 不变：**一个模式拥有字段时，槽里只有 scope mark**（放大镜让位） | `已落地` — `TestScopeMarker` 零改动仍绿 |
| 查询的起始列 | 16u（字段行的 padding 用尽） | **54u** = 16u + 28u 图标列 + 10u gap = 结果行标题的列 | `已落地` — 三条 V 重定基线（`internal/launcher` 一条、`internal/shell` 两条），见报告 §3 台账 |

**V 台账（三条，全部是「旧值 16u → 新值 54u」）**：
`TestTheFieldAndTheRowsShareTheReferenceTextColumn`（caret 16 → 54）、
`TestTheSearchColumnIsTheReferenceUnitsAtTheShippedStep`（查询 16u → 54u）、
`TestTheSearchPageFillsAWiderWindow`（同上，三个窗口宽度）。
另有一处**测试取样点**的修正（不是放宽断言）：
`TestThePlatformInsetIsTheReferenceUnitLadder` 原来在**卡片中缝**取「字段行的填充色」
再扫它的连续段，而占位符现在从 54u 起、正好盖住中缝那个像素——取样点改成卡内左缘
`left+2`（那里永远是行自己的填充）。断言的仍然是「填充段 == 卡片内容盒」这一条。

**图上实测（`/tmp/floter-P11-ab-searchrow-<light|dark>.png`，720×488）**：放大镜的
墨落在 `x=23..36, y=21..34`（95 个像素，暖像素 **0**）；浅色最深的墨
`(113,113,122)` = `TextMuted`、填充 `(250,250,251)`，深色 `(161,161,170)` 对填充
`(28,28,31)`——两套外观下都读得清，都不是 accent。y=28 行上第一个非填充像素是
**x=54**（文本框的光标），与结果行标题同列。

### §6.9 P11-ac · 列表的分组头落在它自己的文字列上（**已落地**）

用户 2026-10-28：「还是先优化当前的界面、质感、交互逻辑」。本轮取「质感」里最可量的一条：
**对齐**。分组头是每一屏列表都看得见的一行字，它必须落在它管辖的标题那一列。只改横向缩进，
纵向 band、字阶、weight、墨、键盘模型全部零改动。

| 位置 | 旧（实测） | 新（实测） | 落地 |
|---|---|---|---|
| `internal/launcher/launcher.go` 的 heading 分支 | 左 padding 12u + `Margin-left` 42u → 分组头 **62u**，它下面的标题 54u（右挂 8u） | 左 padding 8u + `Margin-left` 38u（= 图标列 28u + 行 gap 10u）→ **54u** = 合约 §3 的文本列 | `已落地` — pin `TestTheGroupHeadingSitsOnTheRowsTextColumn` |
| `internal/launcher/output.go` 的 group heading | 左 padding 12u、无图标列/gap 偏移 → 分组头 **20u**，它下面的标题 52u（左挂 32u） | 左 padding 8u + `Margin-left` 36u（= 图标列 28u + 该行 gap 8u）→ **52u** = 输出面板自己的标题列 | `已落地` — 同一条 pin 的后半 |

两处注释当时都写着「与行文字列对齐」（启动器侧甚至写着「sits on the same column a row's
title does」），而数字对不上——因为它们从没从行里推出来。pin 断言的是**两个矩形的 x 相等**，
不是某个字面数：以后再挪列，两个表面会一起红。

**图上实测**（`/tmp/floter-P11-ac-list-<light|dark>.png`、`/tmp/floter-P11-ac-output-<light|dark>.png`）：
启动器列表的分组头与它下面标题的墨都从 **x=55** 起（浅色）；输出面板的 `Group B` 与它下面的
`Gamma` 都从 **x=53** 起（浅色）。改前同一测量是 62/54 与 20/52。

### §6.10 P11-ad · 同一张卡里的两种列表只有一条文字列（**已落地**）

启动器列表与它自己的输出面板是**同一张卡里的两种列表**：同一个窗口、同一个 8u 卡片内缩、
同一个 28u 图标列、同一个 8u 行内 padding。P11-ad 之前横向分量里唯一不同的是**行自己的
gap**（列表 10u、面板 8u），于是标题列一个是 54u、一个是 52u——用户在列表与输出之间来回
看时文字列错 2u。改法：面板的行 gap 取列表的同一个 10u，它的分组头 `Margin-left` 从 36u
改成 38u（36 = 8 padding + 28 图标列 + 8 gap 的旧和）。字阶、weight、墨、band、行高、
图标列、keycap 槽、选中/悬停档一律未动。pin：`internal/launcher/textcolumn_test.go` 的
`TestTheTwoListsShareOneTextColumn`（断言两个表面的标题 x 相等、两个分组头的 x 相等）。

### §6.11 P11-ae · 终端标题栏的墨属于它浮着的那块面（**已落地**）

浮条（`titleBar`）读窗口的四档墨，浮着的面却是画布（`panelFill` = 调色板底色）。两者相反时
（浅色窗口 + `ink`/`dusk`/`forest`）窗口的次级墨 `#6B6B70` 在合成面上只有 **1.10:1**——
而同一块面上调色板自己的前景是 **4.80:1**，所以缺的是**面**不是墨（P11-ad 的选项 (c)
「墨随合成底明度选」实测不成立：合成面是一层中灰）。

规则一条：`inherit` / 未知 id / **空页**（没有会话，没有它的调色板）→ 窗口四档且
`Face.A == 0`，逐像素与改前一致；命名调色板 + 有会话 → 浮条穿画布自己的颜色，面 = 调色板
底色取「让调色板前景在窗口面之上过 4.5:1 的**最小** alpha」（`ink` 浅窗 0.625、`forest`
0.6875，深色窗口的前景本来就够，取 0），墨 = 调色板前景，quiet 两档 = 同一墨在**合成面**上
过 3.0:1 / 2.5:1 的最小不透明度。面**只在揭示时**画（静止时不覆盖第一行输出），顶角取
`panelCorner`，无渐变 / rim / sheen。pin：`internal/terminalui/barink_test.go` 的
`TestTheBarWearsTheColourOfTheCanvasItFloatsOn`（每套调色板 × 两外观：墨 ≥4.5、quiet 档是
最静、面是最薄）、`TestTheShippedBarKeepsTheWindowInk`、`TestTheBarPaintsItsOwnFaceOverAPalette`。

### §6.12 P11-af · 终端浮条不吃它下面的指针（**已落地**）

浮条是**浮在画布上**的一条 28u 全宽带，它以前整条 `.DragWindow()`。框架的命中表
（`ui/runtime.go:1208-1218`）把每个非 `PassThrough`、非 invisible 的元素都记进 `rt.hits`，
所以这条带把画布最上面 28u（第一/第二行输出）的指针事件全吃掉：点不进、拖不了选区，
终端的 select-to-copy 在这两行上失效——这是 F3「不要 overlay」在输入侧的后果。

规则一条：**浮条与它的 identity 行、状态点都是 `PassThrough`，窗口拖拽手柄收窄成会话
标题自己的字**（`.DragWindow()` 只在标题 `Text` 上）；揭示读**指针位置**
（`PointerPosition()`，它在 `PassThrough` 元素上仍算包含关系），不读命中，所以悬停仍能
把标题与按钮浮现出来。位置、28u 高、按钮槽位、抽屉几何、键盘模型零改动。
pin：`internal/terminalui/barreach_test.go` 的
`TestTheTerminalBarIsNotTheWindowsDragHandle`（源码契约：浮条/identity/状态点 PassThrough、
`DragWindow()` 只有一处且落在标题上、揭示读 `PointerPosition`）与
`TestTheBarRevealsUnderAPointerItDoesNotCatch`（指针停在标题字外的带子里——那里没有可命中
元素——仍然揭示，离开后折回）。

### §6.13 P11-ag · 空态落在列表那条带里（**已落地**）

§4.7 写的是「一句话，居中在 band 里」。那块 band 此前并不存在：字段行是浮在面板上的
绝对定位块（`launcher.fieldRow`，56u 不透明底），而空态是一条没有高度、没有内缩的
`Column`，所以 `Center()` 把句子摆在**卡片顶边**（实测 y=12–30，字段带占 0–56）——
整句被字段盖住，一个像素都没渲染：一个匹配不到任何东西的查询在界面上是**空白卡**。
普通页的空态还有 `HasText` 类 pin 兜着（它们读的是构建出来的文本树，不是像素，所以
一直是绿的）；插件模式（剪贴板为空、浏览器没配置、计算器没有历史）连列表和页脚都没有，
整页只剩这句看不见的话。

规则一条：**空态与列表读同一条带**——上边距 `listEdge`（字段行 + 呼吸 + 模式的 chips）、
`Grow(1)` 撑满面板剩下的高度、在带里居中。墨与字阶零改动（`Emphasis` + `TextStrong`，
无底板/图标/accent）。pin：`internal/launcher/emptystate_test.go` 的
`TestTheEmptyPageSaysSoUnderTheField`（句子矩形里的**墨量** > 0 + 中心落在带的中点 ±6px）；
`HasText` 类 pin 保留，它们钉的是文案，不钉可见性。

### §6.14 P11-ah · 滚轮的距离归滚轮，选中的视野归键盘（**已落地**）

§4.3 的列表是滚轮表面。一格滚轮（120u）该走三行，实测只走一行且是瞬跳。

规则一条：**「把选中的行滚进视野」是键盘的规则，不是指针的**。指针进入某行就接管选中
（R21）——但那一行**就在指针底下**，`near` 语义下本就不会移动内容；发出去的
`ScrollIntoView` 唯一的作用是让框架认为「应用接管了偏移」，从而丢弃在途的滚轮滑行
（`mygo` 的 `glideWheels` 见 `ListState.scrolled` 即退出）。所以指针给的选择不滚，
键盘步进 / 查询变更后的默认选中 / 历史删除后的邻居接管照旧滚。
pin：`internal/launcher/wheelscroll_test.go` 的 `TestTheWheelScrollsTheListUnderThePointer`
（四格 120u 后列表必须前进 ≥ 8 行，改前 4 行）与 `TestTheKeyboardStillScrollsItsRowIntoView`。

### §6.15 P11-ai · 指针不动就不算「进入」（**已落地**）

R21 的入口边（§4.3，`onPointerEnter`）在本仓曾被读成「指针底下那一行变了」。行变有两种
原因：指针动了，或者**内容动了**——而「把选中滚进视野」正是键盘的规则（§6.14），所以键盘
每跨过窗口末行，静止指针底下的行就换一次人，选中被拉回去。

规则一条：**一个指针位置算「用户给的」，当且仅当指针自己动过**（坐标变化 / 进出面板）；
**滚轮是例外，它本来就是指针手势**（§6.14 的 pin 要求滚过之后 `Selected == hoveredRow`，
不动）。app 替键盘滚的那一滚由 reveal 自报家门（`App.appScrolled`），列表静下来才复位。
窗口出现在光标下不补发 `mouseenter`，所以「开机即选指针那一行」也一并去掉（选中仍是第一行）。
pin：`internal/launcher/pointeridle_test.go` 的 `TestTheKeyboardWalksTheListUnderARestingPointer`
（修前 15 步走 3 行 → 修后 15 行）、`TestThePointersWordCountsAgainOnceItMovesAfterAScroll`
（门不是把指针关掉）、`TestATypedQueryKeepsTheChoiceOffTheIdlePointersRow`。

### §6.16 P11-aj · 一条面上没有两段字压在同一块像素上（**已落地**）

这条不变量看着像「像素细节」，其实是 P11-ag 的同一族：那句话一直在文本树里，只是被
字段块整块盖住——`Tester.Texts` / `HasText` 对**被盖住的**字与**看得见的**字给同一个
答案，所以一层层绝对定位的带子（字段行浮在列表上、页脚钉在面板底、设置页的分组标题
靠 `Gap` 保着）出错时，没有任何断言会红。

判据三条，两个表面同一份：

1. **整窗宽的节点不是一段字**：面板/列表自己的无障碍标签（`ResultsLabel`）按构造包含
   每一段字，不参与比较；
2. **被另一个盒完全包含的盒是一块面上一段字**（按钮的字在按钮里），不算压字；
3. 剩下任意两盒在**两个轴上都重叠 > 1 DIP** 才算压字；任意盒越出窗口边框算「从来没
   被看见」。

pin：`internal/launcher/textframe_test.go` 的 `TestTheLauncherPaintsNoTextOverText`
（空页 / 有匹配 / 零匹配 × 两外观，夹具含一个会被省略号截断的超长应用名与行尾键帽）、
`internal/settingsui/textframe_test.go` 的 `TestTheSettingsPagesPaintNoTextOverText`
（五页 × 两外观 × `860×900` 与 `720×560`）。本轮实测三表面（启动器 / 设置页 / 终端页）
零压字、零出框，产品墨与面零改动。

### §6.17 P11-bd · 浮在画布上的东西要整块让出指针（`PassThrough` 不沿子树传播）（**已落地**）

参考给终端页上三样「自己不接动作」的浮层都写了 `pointer-events: none`：
`.terminal-resident`（`main:src/styles/terminal.css:303`）、`.terminal-feedback`（`:414`）、
以及未揭示时的 `.terminal-panel__actions`（`:501-505`，注释原话 “While hidden the buttons
must not catch clicks meant for the terminal underneath”）。本仓三处都漏在同一个地方。

**根因一条**：`mygo` 的 `PassThrough()` **不沿子树传播**。框架的命中表逐个元素记
（`ui/runtime.go:1208-1218`），只排除 `flagInvisible`/`flagInert`（同文件 `:1196`）——
**不看透明度**，也不看父元素。而一句 `ui.Text` 自己就是可选中面（`flagSelectable`），
所以「把外层标成透明」在这三处都静默地漏掉了字。CSS 的 `pointer-events: none` 是继承的，
这就是移植时最容易被抹平的一处差异。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/terminalui/terminalui.go` 的 `feedbackNote` | 行没有任何 `PassThrough`（参考 `:414` 有） | 行 + `ui.Icon` + `ui.Text` 各一次 | `已落地` — pin `TestTheNoticeTakesNoPointer` |
| 同文件的 `residentNote`（P11-bc 标的） | 只有**行**是 `PassThrough`，两句 `Text` 仍是命中目标 | 两句 `Text` 各一次 | `已落地` — pin `TestTheResidentNoteTakesNoPointer` |
| 同文件的 `barButton` | 签名没有「揭示」这一档，只靠 `actions.Opacity(0).PassThrough()`（管不到子元素） | 新增 `live bool`：未揭示时按钮与字形各 `PassThrough()`、且不挂 tooltip；`Focusable()` 保留（参考的 `:focus-within` 揭示靠它） | `已落地` — pin `TestTheHiddenBarButtonsTakeNoClick` |

判据一条：**浮在画布上、自己不接动作的东西，指针要穿过去落到画布上**——画布是这一页唯一
响应指针的东西（聚焦、选区、滚轮回滚）。三条 pin 的观测因此是同一个：点在那个位置之后，
画布有没有拿到键盘。源码契约 `TestTheCanvasOverlaysGoQuietWhenHidden` 钉住根因本身：
这两个便条的 `PassThrough()` 数量必须 ≥ 1 + 它们画的 `Text`/`Icon` 数量，只标外层必红。

**零像素改动**（本轮是命中标志轮）：修前/修后同一帧导出 PNG 的 md5 全等
（`/tmp/floter-P11-bd-notes-<light|dark>.png` 对 `/tmp/before/`，见报告 §5）。

### §6.18 P11-bf · 卡行「下面那块」只有一个横向列（**已落地**）

`cardRow` 的 `below` 槽画的是「这一行底下展开的东西」：权限审计、配置表单、批准记录的
权限块。它曾是行的**兄弟**，直接排在卡自己的列里，于是从**卡的边**起排；而行的内容盒在
`rule(1) + Padding(vertical, inset-1)` 之后，正好是 `rowInset`（设置卡 14u、集成卡 8u）。
同一个问题于是有了三个答案：两块自己补了一遍 inset，其余一处没补。

**判据一条**：**卡的内容列只有一个产生者——`cardRow`**。`below` 槽里的一切从行的内容盒
左沿起排（`Padding(0, inset)`，垂直 0，y 与行高不变）；块自己不许再问 `rowInset`。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/settingsui/rows.go` 的 `cardRow` `below` 槽 | 每个回调当兄弟直排（无 Padding/Margin） | `ui.Column(c).FillWidth().Padding(0, inset)` 包住整个槽 | `已落地` — pin `TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn` / `TestTheConfigFormSitsOnTheCardsTextColumn` |
| 同包 `permissionAudit` | 自补 `Padding(0, inset, t.Space(1), inset)` | `Padding(0, 0, t.Space(1), 0)` | `已落地` — 源码契约 `TestNoBelowBlockReappliesTheCardsInset` |
| `internal/settingsui/config.go` 的 `configFields` | 自补 `Padding(0, inset, 0, inset)` | 不再补（像素逐字节不变） | 同上 |

实测（720×900，批准记录块）：caption `Floter enforces` 192 → **200**（== 行标签 200）、
药丸文字 201 → **209**、stale 横幅句子 204 → **212**。整帧差异包围盒
x=188…702、y=692…824（只有那一块）；`configFields` 的帧**零像素变化**——它原本算出来的
位置与新的逐字节相同，正是「同一件事两个答案」的证据。

### §6.19 P11-bg · 终端的失败提示也挂成画布顶边的一条带（**已落地**）

F3 的判据只看「它是不是一块压在内容上的浮面」，不看它是常驻还是转瞬。P11-bc 把
「进程已退出」的常驻便条改成了画布底边的带；动作失败时那条 4.5 秒的提示
（`internal/terminalui/terminalui.go` 的 `feedbackNote`）当时**故意留着**，于是同一页
有两种说法：底边是带，顶边还是 `Float` 面 + `ElevThrow` 投影 + `LensRim` 受光边 +
`RadiusMD` 圆角 + 160ms 入场位移的浮卡，离面板左上各 12u、自然宽。

**判据一条**：**终端页上贴在画布边缘的状态都是带子，没有第二档**。带子 = 整宽、贴边、
无圆角、无投影、无受光边、无入场动画，面用 `Control`，收口是朝画布内侧的一条
`HairlineSoft`（底边的带是上边一条，顶边的带是底边一条）。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `feedbackNote` 的锚 | `Absolute().Top(Space(3)).Left(Space(3))`（12u 浮卡） | `Absolute().Top(barHeight(t)).Left(0).Right(0)`（挂在标题条下沿、整宽） | `已落地` — pin `TestTheFailureNoteIsADockedStrip` / `TestTheFailureNoteLetsThePointerThrough` |
| 同处的四件套 | `Radius(RadiusMD)` / `Border(1, ControlEdge)` / `ElevThrow` / `InsetShadow` / `Transition` | 面 `tokens.Control` + `BorderWidth(0,0,1,0)` `HairlineSoft`，四件套全删 | 同上（源码契约：四件套与 `Radius(`/`Transition(` 一律不许回来） |
| 「标题条高度」 | `titleBar` 的 `Height`、`body` 的 `MinHeight`、这条带的 `Top` 三处各写 `t.Space(7)` | `barHeight(t)` 一个答案 | 同处 |

**没动的**：15u 警示字形、body 步 560、`t.Warning` 墨、7u/10u 内边距、4.5s 时钟、
`AnimationFrame`、逐层的 `PassThrough()`（参考 `terminal.css:414` 的 `pointer-events: none`）。
这条带是 `Absolute()`，**画布高度与 PTY 行数零改动**。实测（860×600，浅色）修前那块面
在 `x=2` 这一列铺出一圈影子 `(251,251,251)→(248,248,248)→(244,244,244)`（y=14/20/30），
修后同一列是画布自己的 `(255,255,255)` 与带面的 `(242,242,242)`；`(429,30/56/60)` 修前是
画布的 `(255,255,255)`、修后是带面的 `(242,242,242)`——整宽贴边。

### §6.20 P11-cp · 一次拖拽一次落盘：抽屉与设置页共用一扇 180ms 窗（**已落地**）

用户报的「流畅性」在输入侧已由 P11-co / P11-cl / P11-cn 收口（一帧一份设置、一帧一遍
目录），本轮补的是**指针侧**的最后一处：终端外观抽屉的字号 / 行高滑轴**每帧写一次文件**。

| 位置 | 旧（实测） | 新（实测） | 落地 |
|---|---|---|---|
| `internal/terminalui/drawer.go` 的字号滑轴（`:150`）与行高滑轴（`:177`） | `a.set(...)` → `Store.Update` → **每次都写盘**；22 帧的一次拖拽 **17 次落盘** | `a.preview(...)` → `Store.Preview`（值实时、不写盘）+ 180ms 窗到点 `Commit` 一次 | `已落地` — `internal/terminalui/debounce.go`；pin `TestADrawerDragWritesTheFileOnce`（**两个半边**：`Version()` 涨 ≥3 且 `Writes()` 恰好 +1） |
| 同文件 `set`（`:39`） | `_ = a.Store.Update(mutate)`：写失败**静默吞掉** | 成功即关窗（那次写已把 preview 的值落盘，`Commit` 是空操作）；失败 → `ShowFeedback(SaveFailed)` | `已落地` — 同处 |
| `internal/shell/shell.go` 的三个出口（`Open` / `Hide` / `quit`） | 只 flush 设置页 | 各补 `a.Terminal.Flush()`：隐藏的窗口不再画帧，`deadline` 的唤醒永远不到，那次拖拽会**静默丢写** | `已落地` — pin `TestLeavingTheTerminalSurfaceWritesTheDraggedValue`（**修前红**）、`TestHidingTheWindowWritesTheDraggedValue` |

**为什么这不是行为改动**：参考实现本来就对这两个滑轴 debounce
（`main:src/hooks/useSettings.ts:444-500` 的 `changeFontSize` /
`changeTerminalLineHeight`，抽屉接的正是它们，`App.tsx:3925-3926`），而其余七行是
离散选择、参考也立即写（`changeGeneralSetting`）。值在拖拽中**始终实时**
（`Preview` 与 `Update` 同一条 normalize 链，都推版本并通知监听者），变的只是落盘次数。

**零像素**：两套外观的 900×620 dump 在干净 HEAD worktree 与本轮**md5 相同**；
`TestAPreviewAndAWritePaintTheSameFrame` 把这条主张钉成 pin（带控制臂：未改值的帧
必须与它们不同，否则两臂画的是出厂帧）。

### §6.21 P11-cq · 外壳这一帧也只拷一份设置（**已落地**）

P11-co 给 `launcher` / `terminalui` / `settingsui` 三张表面都装了「一帧一份设置」的 memo，
**漏了外壳自己**。用户报的「流畅性」在帧里还剩这一处：`Store.Snapshot` 是整份深拷贝，
而外壳每读一次就拷一次。

| 位置 | 旧（实测） | 新（实测） | 落地 |
|---|---|---|---|
| `internal/shell/shell.go:4234` `View` 的 `s` | 每帧一份深拷贝 | `a.settings()`（`App.settingsMemo`） | `已落地` — `shell.go:244` 的字段 + `:4244` 的方法 |
| `internal/shell/notices.go:331-332` `toastViewport` | 每帧**两份**（tokens 与文案各一） | 闭包外一次 `a.settings()`，两处读它 | `已落地` |
| `internal/shell/shell.go:4178/4189/4197` `appShortcuts` | 每次判定一份 | 函数开头一次 `snapshot := a.settings()` | `已落地` — 只影响设置表面（终端/启动器走不到那两支） |
| `internal/shell/shell.go:1647-1648` `integrationList` | 每帧**两份**（命令开关与别名各一） | 一次 `stored := a.settings()` | `已落地` |
| `internal/terminalui/terminalui.go:416/427` `LabelOf` / `NoteOf` | **每条会话两份**；16 会话（`maxSessions` 上限）= 32 份 | 读本表面已有的 `a.settings()`（P11-co 装的同一把尺子） | `已落地` — pin `TestASessionsFrameTakesNoSettingsCopy` 等五条 |
| `internal/terminalui/terminalui.go:693` `Label` | 同一族但 `deadcode` 报它无调用点 | **只改读法、不删**（删它属于死代码清理那一轮） | `已落地`（读法一致） |

**实测**：六张表面静止帧 **全部 0 份**深拷贝（修前 sessions 36 / integrations 40）；
16 会话帧 13096 → **11368 B/op**、238 → **202 allocs/op**；闭包单独 6538 ns/op。
**零像素**：四张 dump 修前修后逐字节相同，且深/浅两两不同。
**尺子读 `Store.Snapshots()`，不读 `Memo.Builds()`** —— 后者在有人绕过 memo 时仍然绿。

## §7 切片表
| 片 | 范围 | 状态 |
|---|---|---|
| P8-a | 合约文档 + token 层 + 启动器表面（卡片/搜索行/列表/分组头/页脚/空态/键帽） | **本轮完成** |
| P8-b | 设置页（侧栏/分组块/行/控件/弹层）+ 两个空态 | **本轮完成**（`⌘K` 动作面未做，见 §4.6） |
| P8-c | 终端表面（页签/边框/滚动边缘/抽屉）+ 集成页 + 通知/浮层 + §5 accent 预算勘误 | **本轮完成**（§6.2/§6.3 全部 `已落地`；§5 从「三处」改为四类，新增 9 条正面 pin） |
| P8-d | accent 花名册收口：越界 9 处转中性 + 判不准 4 条定审 + §5 注脚 + 视觉 pin 处置 | **本轮完成**（§6.4 全部 `已落地`；§5 补「只约束色相、鼓励明度分层」一句与边界四条注脚；新增 10 条 pin + 2 张×2 视觉证据） |
| P11-cp | 抽屉滑轴的 180ms 窗（与设置页同一扇）+ 三个出口的 flush | **本轮完成**（§6.20；17 次落盘 → 1，零像素；新增 6 条 pin + 4 条变异） |
| P11-cq | 外壳这一帧也只拷一份设置（会话行不再按行数拷） | **本轮完成**（§6.21；六张表面 0 份，零像素；新增 5 条 pin + 4 条变异） |
| P11-cr | 死代码清账：`deadcode` 量出的 24 个零调用点函数（18 生产 + 6 测试 helper） | **本轮完成**（§6.22；25 行 → 1 行，剩下那条是假阳性；零像素 6 对；新增 2 条 pin + 5 条变异） |
| P8-e | accent 收口最后三处：终端状态灯去色相、成功 toast 转中性、输出面板选中行改 `RowSelected` | **本轮完成**（§6.5 全部 `已落地`；§5/§6.2 同步；`AccentTintQuiet` 标 `Deprecated`；新增 4 条 pin + 3 张×2 视觉证据） |

### §6.22 P11-cr · 死代码清账：`deadcode` 量出来的 24 个零调用点函数（**已落地**）

不是设计切片，是**账**。`deadcode -test ./...` 在 P11-cr 开工时量出 25 行，
逐条读实现后分三类（转发壳 / 被合并掉的旧渲染路径 / 从没接上的入口），删掉 24 个
（18 生产 + 6 测试 helper），复跑降到 **1 行**——剩下那条是假阳性，写进了账本底部。

| 位置 | 旧 | 新 | 落地 |
|---|---|---|---|
| `internal/inventory/sources.go` `LookPath` 等 5 条转发壳 | 一行转发底层实现 | 删（调用方直接读底层那个） | `已落地` — 连带删 `inventory` 的 `pathlist` import |
| `internal/settingsui` 的 `text` / `freshnessDetail` | 被 `configform.go` / `freshnessSection` 接手的旧渲染路径 | 删 | `已落地` — **连带发现**：`i18n.Copy.FreshnessProbeSource` 的唯一读者是它，`TestNoDeadCopy` 因此变红，该文案一并删（§3.1） |
| `internal/terminalui/terminalui.go` `App.Label` | P11-cq 记「只改读法、不删」 | 删（外壳读的是 `LabelOf(index)`） | `已落地` — P11-cq 的账在此结清 |
| `internal/shell/chrome_other.go` `dressWindowsFrame` | `deadcode` 报它不可达 | **不删**（`!windows` 桩，windows 那支在调） | `已落地` — 反面 pin `TestTheDeadCodeFalsePositiveStaysOut` 守它 |

**pin**：`internal/ci/deadcode_contract_test.go` 两条 —— 账本里每个符号不许再长出 `func`
定义，且账本条目数必须恰好 24（加/删都红）。**为什么是源码扫描不是跑工具**：
`deadcode` 是 `go run …@version`（要联网），且 `GOOS=windows` 下自己 `exec format error`。
**零像素**：三张表面 × 两外观 = 6 对 dump 修前修后**逐字节相同**。
**变异**：5 条实跑变红并还原（含 M2/M3 账本放宽与收窄两个方向）。
