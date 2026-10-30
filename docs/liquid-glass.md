# 轻便液态玻璃合约（Liquid Glass, Light）

> **本文件是 `docs/raycast-design.md` 的升代。** 那一份钉的是「近黑底 + 1px 发丝线 +
> **无光泽** + 靠线分层」，是 P8 那一代的语言；它把光泽上限压到 `alpha ≤ 0.04`，
> 那条线是为「没有受光边」定的。用户 2026-10-30 换的总目标是**轻便液态玻璃**
> （半透明面 + 一层受光边 + 柔和外投影）+ **空间感**（明确的 elevation 阶梯，
> 靠面差 / 受光边 / 阴影分层，而不是靠画框）。
>
> 两件事直接冲突：**「无光泽」与「有受光边」不能同时成立**。所以这份文件显式地把
> 那条上限**重新定基线**（§1 末尾有 旧值→新值 表），并把 raycast 合约降级为
> 「行为契约 + 已落地条目」——它 §1~§5 的**语义 token 与行为契约全部继续有效**，
> 只有光泽上限与分层手段被本文件取代。
>
> 单一来源仍然是 `internal/theme/theme.go`：视图只读 `theme.Tokens`，不写字面色值。
>
> 切片：**P12-a = 合约 + 材质/层级 token 层升代（已做）**。表面切片逐轮推进，
> §6 的差距表就是选题队列；空表才算这一支做完。

---

## §0 本仓能画什么（自绘的物理边界）

本仓是 Go 自绘（mygo），**没有 `backdrop-filter`，也不许假装有**。等价物只有四件：

| 手段 | 本仓的实现 | 允许用在哪 |
|---|---|---|
| **面差** | 同一色相的三级 alpha（底 / 面 / 浮） | 全部三层，差值必须一眼看出（§1） |
| **受光边** | 1px 顶边更亮（`LensRim` 组件级、`EdgeHighlight` 浮层级） | 只给「面」与「浮」，底不画 |
| **阴影** | 环境影 + 接触影两层（`ShadowAmbient` / `ShadowContact`，`theme.ElevThrow`） | **只给浮层**，面板内元素不用 |
| **半透明** | 真透明度只给**窗口层**（`glassmap` 的 `main_opacity` / `glass_step` 链） | 只有窗口层；组件层不许自造透明 |

**硬禁令**（这一支的「反 AI 套路」）：

1. 不许拿模糊贴图 / 高斯近似 / 预渲染阴影图**冒充玻璃**。自绘没有 blur，就没有 blur。
2. **glass 不许叠 glass**：一个浮层内部不许再开一个浮层；浮层里的一切是「面」档。
3. **不许用渐变白边模拟高光**（用户原话里点名的「类玻璃边」）。受光边是一条 1px
   的实色线，不是 gradient。
4. 组件层不许自造半透明（否则叠起来必糊）；透明度只有窗口层那条链。

---

## §1 材质三档

一个应用里只有三种面。**同一色相**，只差 alpha；差值必须能在 1x 屏上一眼分开。

| 档 | token / 来源 | dark | light | 用在哪 |
|---|---|---|---|---|
| **底** ground | 窗框 tint（`glassmap.Spec.TintAlpha`，即 `main_opacity` 滑块）+ 卡的 sheen（`SheenTop`/`SheenBottom` ≤ 0.04） | 滑块（默认 47%） | 滑块 | 卡片/面板整体、设置页内容页（`Sunken` = 底 + 0.09） |
| **面** face | `Control` | `rgba(255,255,255,0.10)` | `rgba(0,0,0,0.07)` | 分组卡、字段、控件静息、行块底 |
| 面·指针抬起 | `ControlHover` | `rgba(255,255,255,0.14)` | `rgba(0,0,0,0.105)` | hover 抬一档 |
| 面·指针按下 | `ControlPress` | `rgba(255,255,255,0.05)` | `rgba(0,0,0,0.032)` | 按下沉到静息**之下** |
| **浮** float | `Float` = 窗框 tint + `glassmap.floatStep`（0.10），色 `#18191D` / `#FCFCFE` | 滑块+0.10 | 滑块+0.10 | 抽屉、弹层、toast、便条 |

**为什么「面」要抬到 0.10 / 0.07**：旧的 0.075 / 0.05 压在卡片自己的 sheen 上，
实测在默认 47% 的框上只差 3~4 个亮度级——用户读到的就是「所有东西挤在同一个 z 平面上」。
抬到 0.10 / 0.07 之后「面」与「底」的差在 1x 屏上是 8~11 级，一眼能分；而
`ControlHover` 与 `Control` 之间仍然只有一档（0.04 / 0.035），指针反馈不会变成跳变。

### 光泽上限的重定基线（旧值 → 新值）

旧的 `docs/raycast-design.md` §1 最后一行写着「低光泽上限（透镜/光泽/卡片 sheen）
`LensRim`/`LensSheen`/`LensEdge`/`SheenTop` 全部 ≤ 0.04」。**那条上限作废**，
换成「一条受光边 + 一片不反光的面」：

| token | 旧值（≤0.04 时代） | 新值 | 理由 |
|---|---|---|---|
| `LensRim`（组件顶受光边） | `0.04` | **`0.09`** | 这是**唯一**被允许亮起来的一条线：控件顶边 1px。旧值 0.04 在 1x 屏上画不出「受光」，只画得出一条噪点。 |
| `LensSheen`（组件宽面光泽） | `0.04` | **`0.04`（不动）** | 面**不许反光**。轻便玻璃的反光是「一条线」，不是「一片渐变」。 |
| `LensEdge`（组件底影） | `0.04` | **深 `0.20` / 浅 `0.06`** | 受光边要有对侧：顶亮一分，底暗一分，控件才站起来。**P13-n 重定基线**：深色的面是 39，0.06 只降 **2 级**（等于没画）→ 0.20 降 **8 级**；浅色的面是 230，0.06 已经降 **14 级**，而抬它会把 accent 面洗成类玻璃边（§1 禁令 3），所以浅色**不动**。旧值 → 新值：深 `0.06 → 0.20`，浅 `0.06` 不动。 |
| `TrackRim`（凹槽底受光边） | `0.09`（借 `LensRim`） | **浅 `0.70` / 深 `0.09`** | 凹槽的底边和面的顶边**不是同一条线**：浅色档的凹槽面上压着它自己的发丝线（`TrackEdge` 黑 0.08），0.09 的白叠上去只剩 **217 < 槽面 232**——受光边赢不了它下面那条环，凹槽读成「两条暗线的框」。**P13-o 重定基线**：浅色 `0.09 → 0.70`（实测底内线 221 → 242，−11 → **+10**，仍在地面 247 之下）；深色两者同值 0.09，**不动**（实测 +33，本来就亮）。 |
| `SheenTop` / `SheenBottom`（卡面） | `0.04` / `0.01` | **不动** | 窗口层的大面仍然是最轻的那一层；它靠透明度和外投影分层，不靠光泽。 |
| `SheenTopActive` / `SheenBottomActive` | `0.05` / `0.012` | **不动** | 聚焦档同上。 |

**新上限一句话**：`LensRim ≤ 0.09`，`LensEdge ≤ 0.20（深）/ 0.06（浅）`，`TrackRim = 0.70（浅）/ 0.09（深）`，其余光泽 token `≤ 0.04`；且
`alpha(LensRim) > alpha(LensSheen)` 必须严格成立——**受光边比面亮**，这是这一支
唯一的材质不变量。`TrackRim` 是唯一的例外，理由只有一个：它画在**凹槽**上，不是画在面上；
浅色档的凹槽面本身已经是 232，一条 0.09 的白线在那里连自己下面那条环都盖不住。
pin 在 `internal/launcher/raycast_test.go`（`TestTheControlRimIsTheOneLitLine`）与
`internal/settingsui/groovepair_test.go`（凹槽的一对线，两套外观三个凹槽）。

---

## §2 elevation 阶梯（三档，判据一句话）

空间感来自**层数少而差值明确**，不是来自更多层。

| 档 | 面 | 边 | 受光边 | 外投影 | 判据 |
|---|---|---|---|---|---|
| **E0 底** | 窗框 tint + sheen | 卡自己的 1px `InputStroke` | 无 | 窗口自己那一笔（Linux/Windows） | 它**是**窗口/面板本身 |
| **E1 面** | `Control` | 1px `ControlEdge` | `LensRim` 1px 顶边 | **无** | 它长在底上，压不出影子 |
| **E2 浮** | `Float` | 1px `InputStroke` | `EdgeHighlight` 顶边 | `ShadowAmbient` + `ShadowContact` 两层 | 它压在其他内容上，且可关闭 |

**三条判据**：

1. **一个表面只有一个 E2**。抽屉开着的时候，抽屉里的一切是 E1；弹层开着的时候，
   弹层里的一切是 E1。同屏两个 E2 = 玻璃叠玻璃，禁止。
2. **E1 不许有外投影**。`Shadow(` 出现在面板内元素上就是越界（`internal/ci/elevation_contract_test.go`
   已经守着「投影必须走 `ElevThrow`/`ElevContact`」这条，本轮补的是「E1 根本不画影」）。
3. **E2 必须两层影**。只有环境影 = 浮层「贴」在底上；只有接触影 = 浮层「悬」在空里。
   两层同时在场才读得出「浮起来一张纸」。

---

## §3 受光边与阴影的数值

| token | dark | light | 用在哪 |
|---|---|---|---|
| `LensRim` | `rgba(255,255,255,0.09)` | `rgba(255,255,255,0.09)` | E1 的 1px 顶边（`theme.Tokens.Lens`） |
| `LensEdge`（对侧暗线） | `rgba(0,0,0,0.20)` | `rgba(0,0,0,0.06)` | E1 的 1px 底边（P13-n 重定基线，见下） |
| `TrackRim`（凹槽底受光边） | `rgba(255,255,255,0.09)` | `rgba(255,255,255,0.70)` | 凹槽（开关轨道 / 分段槽）的 1px **底边**（`theme.Tokens.LensInset`，P13-o 重定基线，见下） |
| `EdgeHighlight`（静息） | `rgba(255,255,255,0.20)` | `rgba(255,255,255,0.92)` | E2 的 1px 顶边、平台卡边 |
| `EdgeHighlightActive`（聚焦） | `0.24` | `0.96` | 同一根线亮一档（`cardedge.go`） |
| `ShadowAmbient`（环境影） | `rgba(0,0,0,0.62)` | `rgba(0,0,0,0.22)` | E2 宽投影 |
| `ShadowContact`（接触影） | `rgba(0,0,0,0.44)` | `rgba(0,0,0,0.12)` | E2 紧贴边 |

旧值 → 新值：`EdgeHighlight` 深 `0.15 → 0.20`（浅 `0.92` 不动，已经近白）；
`ShadowAmbient` 深 `0.55 → 0.62`、浅 `0.18 → 0.22`；
`ShadowContact` 深 `0.40 → 0.44`、浅 `0.10 → 0.12`。
两者**必须**保持 `Ambient > Contact`（宽的那笔更淡才散得开，而这里 ambient 的 alpha
更大是因为它铺得宽、像素上更稀——这是 base.css 的原始关系，不动）。

**浅色的受光边为什么是 0.92 而不是对称的 0.20**：浅色底上一条 0.20 的白线**看不见**
（白压白）。浅色的受光边只能是近白（0.92），它靠对侧的 `EdgeShadow` 才成立。
这不是两套规则，是同一条规则在两种底上的唯一解。

**凹槽的底边是另一条线（P13-o）**：`LensInset` 的底边不是 `LensRim` 而是 `TrackRim`。
原因不是「凹槽更重要」，而是它下面**多压了一条线**：凹槽自己那条发丝（`TrackEdge`）
先把它压暗 19 级，受光边随后叠上去，0.09 只剩 +2 —— 净 **−15**。
实测（真设置页地面）：浅色槽面 232 ｜ 顶内线 184（−48）｜ 底内线 **221（−11）** → 改后 **242（+10）**；
深色槽面 36 ｜ 顶内线 29（−7）｜ 底内线 69（+33）**不动**。
两条规则合起来只有一句话：**受光边必须比它脚下的那一行更亮，且不许比它被切进去的那张纸更亮**
（浅色凹槽的底内线 242 < 地面 247）。

---

## §4 圆角与间距刻度（**不改**，只重申）

`docs/raycast-design.md` §2/§3 的度量全部继续有效，这一支**一个字都不改**：

| 项 | 值 |
|---|---|
| 圆角梯 | `RadiusXS` 6 / `RadiusSM` 8 / `RadiusMD` 10 / `RadiusLG` 12 / `WindowRadius` 12 / `KeycapRadius` 5 |
| 行块缩进 | 距卡内 8u（左右镜像） |
| 行内 padding | 8u；图标列 28u；行 gap 10u |
| 文字列 | **54u**（8 + 8 + 28 + 10），字段与列表共用 |
| 行高 | `rowUnits` 10（40u）；搜索行 14（56u）；页脚 11.25（45u）；状态行 7.5（30u） |

**为什么重申**：这一支只改墨与面，不动高度链、不动水平度量。视觉轮的红线是
「行为契约零改动」——高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线。

---

## §5 状态与动效

沿用 raycast §4.8，并补浮层这一档：

| 场景 | 时长 / 曲线 | 备注 |
|---|---|---|
| 颜色过渡（行、控件） | 90ms ease-out，只动颜色 | 按帧可预测 |
| 行的进入 | 120ms 透明度 | 位置由布局决定 |
| 按下 | `Scale(0.986)` | 唯一的几何变化 |
| **E2 浮层入场** | 160ms，`translateY(8u→0)` + `scale(0.98→1)` + 透明度 | 材质「落下来」，不是「淡进来」 |
| 窗口入场（`shell-enter`） | 6px lift + scale + 透明度，一次缓动 | 参考的 `base.css:1244-1256` |

**不动效的红线**：不做弹跳（没有手势动量就没有 bounce），不做整卡 pose transform
（mygo 的渲染红线：整棵子树会掉进逐像素路径）。

---

## §6 逐表面差距表（**选题队列**）

状态取值：`未开始` / `已落地` / `需拍板` / `故意不做（附理由）`。**空表才算这一支做完。**

### §6.0 材质 / 层级 token 层（P12-a，**已落地**）

E1 的落地函数 `theme.Tokens.LitFace`（顶 `LensRim` + 底 `LensEdge`，无投影）随 P12-b 加入。

| # | 位置 | 现状（旧） | 应改成（新） | 状态 |
|---|---|---|---|---|
| 0-1 | `internal/theme/theme.go` `palette.control` | 深 0.075 / 浅 0.05：与底只差 3~4 级 | 深 **0.10** / 浅 **0.07** | `已落地` |
| 0-2 | 同处 `controlHover` | 深 0.105 / 浅 0.075 | 深 **0.14** / 浅 **0.105** | `已落地` |
| 0-3 | 同处 `lensRim`（唯一受光边） | 0.04，1x 屏画不出 | **0.09** | `已落地` |
| 0-4 | 同处 `lensEdge` | 0.04 | **0.06**（受光边的对侧） | `已落地` |
| 0-5 | 同处 `edgeHighlight` | 深 0.15 | 深 **0.20**（浅 0.92 不动） | `已落地` |
| 0-6 | 同处 `shadowAmbient` / `shadowContact` | 深 0.55/0.40、浅 0.18/0.10 | 深 **0.62/0.44**、浅 **0.22/0.12** | `已落地` |
| 0-7 | 光泽上限条文 | 全部 ≤ 0.04（「无光泽」时代） | `LensRim ≤ 0.09`，其余 ≤ 0.04，且 rim > sheen | `已落地` |
| 0-8 | **新增**（P13-n）｜`lensEdge` 深色档 | `rgba(0,0,0,0.06)`：深色的面是 39，只降 **2 级** —— 「底暗一分」是名义上的，E1 只有顶亮一半 | **`rgba(0,0,0,0.20)`**：实测底内线 37 → 31（−2 → **−8**；面 65 上是 −4 → −13）。浅色档**不动**（0.06 已降 14 级，抬它会把 accent 面的顶行洗成 55% 的白 = §1 禁令 3 的「类玻璃边」，实测浅色 rim 0.55 会让 `TestThePrimaryCTAIsAnAccentButtonAtRadiusSM` 找不到强调色顶行） | `已落地` |
| 0-9 | **新增**（P13-o）｜凹槽的底受光边（新 token `TrackRim`，`LensInset` 用它） | 借 `LensRim` 0.09：浅色档槽面 232 上先被 `TrackEdge`（黑 0.08）压到 213，再叠 0.09 的白只剩 217 —— **比槽面暗 15 级**，凹槽读成「两条暗线的框」 | **浅 `rgba(255,255,255,0.70)` / 深 `0.09`**：实测浅色底内线 221 → **242**（−11 → **+10**，仍在地面 247 之下），深色 69（+33）不动。深色档两者同值，因为那里两条线都是白的 | `已落地` |

### §6.1 启动器（launcher）— **已落地（P12-b）**

**落地的形状是 `theme.Tokens.LitFace`**（`internal/theme/theme.go`）：顶边 1px `LensRim` +
底边 1px `LensEdge`，不画投影。它不是 `Lens`——`Lens` 的第四层是四边 `ControlEdge` 环，
正是这一支要退掉的「画框分层」。**为什么必须有底对侧**：浅色档的面是近白，0.09 的白线压在
它上面只抬 2 级（实测 221→227）；让块真正站起来的是对侧那条 `LensEdge`（实测 220→203，
17 级）。这是 §3 的「顶亮一分、底暗一分」落到组件上。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 1-1 | 卡片（`shell/cardedge.go` 的 card） | E0 只有 tint + 1px 边；Linux/Windows 有影，macOS 没有 | **保持 E0**；macOS 由窗口服务器投影，不补 | `已落地` |
| 1-2 | 结果行选中块（`launcher.go` `rowBody`） | `RowSelected` 0.085/0.06，一块平涂，无受光边 | E1：`tokens.LitFace(row)` —— 实测顶边 `(600,94)` 61→88（深）/ 221→227（浅），底边 `(600,129)` 220→203（浅） | `已落地` |
| 1-3 | 页脚动作条的选中块（`launcher.go` `rowBody` 的 `isBar` 臂） | 同上，一块平涂 | E1：同一个 `tokens.LitFace`（**页脚 band 自己仍不填面**，它还是卡片的一个 block） | `已落地` |
| 1-4 | 键帽（`keycap.go` `keycap`） | `KeycapFace` + `KeycapEdge`，无顶边 | E1：`tokens.LitFace(key)` | `已落地` |
| 1-5 | 捕获输出页选中行（`output.go` `outputRow`） | `RowSelected`，与列表同档但无受光边 | 与 1-2 同一条规则、同一个函数 | `已落地` |
| 1-6 | 空态（`launcher.go` 的空页） | 一句居中，无底板 | **保持 E0**，不加面 | `已落地` |
| 1-7 | 反馈行（`launcher.go` `feedbackRow`） | 顶边 `LensRim` **只在警告档**画，静息档是一块平涂 | E1：`tokens.LitFace(feedback)` 无条件画；警告档多出来的只是 keyline 与两层影 | `已落地` |
| 1-8 | **新增**｜28u 图标板（`glyphs.go` `iconPlate`，启动器所有页 + 捕获输出页共用） | `tokens.Lens(plate)`：四层，第四层 `ControlEdge` 环**画在最后**，把第一条 `LensRim` 压在下面——浅色档顶边实测 **212 < 面 221**（一条暗线），深色档 97（0.09 白 + 0.07 白叠在同一行） | E1：`tokens.LitFace(plate)` —— 浅色顶 224 / 底 208，深色顶 82 / 底 61，两侧回到面本身（P13-h） | `已落地` |
| 1-9 | **新增**｜两处悬停小格（`launcher.go` `rowControl` 20u / `fieldSettingsButton` 28u 的 `Hovered()` 臂） | `tokens.Lens(…)`：同一个四边环，在 20u/28u 的格子上读成「带框的按钮」 | E1：`tokens.LitFace(…)`，与 P13-d 的终端页悬停按钮同一语汇（P13-h） | `已落地` |

pin：`internal/launcher/litedge_test.go` 四条（选中块 / 键帽 / 反馈带两态 / 输出页选中行），
每条都在深/浅两套外观上跑。出图 `/tmp/floter-P12-b-{list,note,warning}-{dark,light}-{before,after}.png`。

**1-8 / 1-9 是 P13-h 补的两行（「细节打磨」队列，不是新表面）**：`Lens` 的第四层
（四边 `ControlEdge` 环）是**最后画上去的**，于是它压在第一条 `LensRim` 之上——浅色档
那块板的「顶边」实际是 0.06 的黑线（比面暗 9 级），深色档是 0.09 白 + 0.07 白叠在同一行
（比面亮 32 级）。也就是说 P12-a 把 `LensRim` 抬到 0.09 之后，这块板**一次都没真正亮过**。
退成 `LitFace` 之后它才第一次是 E1。pin：`internal/launcher/plateface_test.go` 三条
（板的一对面 / 字段行悬停格 / 20u 悬停格），每条在深/浅两套外观上跑，并量「冠线是 E1 的
`LensRim` 而不是 E2 的 `EdgeHighlight`」（把两条候选各自源叠加在同一个面上当参照，
pin 里没有一处写死渲染器算出来的像素值）。census 的两条 `.launcher-result__icon` /
`.launcher-action-bar__icon` 按 P13-d 的既有机制翻成 `dropped`。

### §6.2 设置页（settingsui）— **已落地（P12-c）**

**落地的形状**：四个面走 `theme.Tokens.LitFace`（顶 `LensRim` + 底 `LensEdge`，无影），
两张 `Float` 面走 E2 的 `EdgeHighlight` 冠线。**为什么浮层要换线**：那张面板的面是
`Float`、带 `ElevThrow` 两层影、还能关掉——§2 三条判据全中，它就是 E2；而 E1 的
`LensRim` 是「贴着的块」的边。此前它戴的是组件级那条线，于是「浮起来的纸」与
「贴着的块」是同一个边，这正是「空间感不够」的算术来源。

**2-6 的例外**：主 CTA 胶囊本来就戴 `LensRim`（`Lens` 的第一层），本轮只加 pin 钉住
「是 E1 的线、不是 E2 的」。它的对侧暗线**没有**补：accent 面上一条黑线会被色相吃掉，
要不要补需拍板（`.pending-bugs.md`）。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 2-1 | 内容页 `Sunken` | 底 + `contentStep` 0.09 | 保持 E0（它是底，不是面） | `已落地` |
| 2-2 | 侧栏选中页 `sidebarRow` | `RowSelected` 一块平涂 | E1：`tokens.LitFace(row)` —— 实测深 `(130,71)` 37→51（+14）、浅 `(130,99)` 244→235（底暗 9 级） | `已落地` |
| 2-3 | 分组卡 `cardGeometry` | `Control` + `HairlineSoft`，无顶边 | E1：`tokens.LitFace(card)`，外圈仍是弱发丝（实测深 `(x,178)` 55→61） | `已落地` |
| 2-4 | 字段（`fieldFace` / `sheetFieldSlot`） | `Control` + `ControlEdge` | E1：同一个 `tokens.LitFace`，两处不另起数值 | `已落地` |
| 2-5 | 开关 / 分段控件 | `LensInset` 凹槽 | **P13-o 改的是这条的底边线**：`LensEdge` 已随 P12-a 到 0.06（顶边，深色档 P13-n 后是 0.20，顶内线随之加深 −8 级，方向正确：凹槽的顶边本来就该更暗）；**底边**从借来的 `LensRim` 0.09 换成 `TrackRim`（浅 0.70 / 深 0.09），浅色档的「洞里积光」才第一次存在（实测 221 → 242）。反面 pin 守住「凹槽不许有冠线」 | `已落地` |
| 2-6 | 主 CTA 胶囊 | accent 填充 + `AccentEdgeStrong`，已有 `LensRim` | 复核通过 + pin：戴的是 E1 冠线、不是 E2 的；对侧暗线**未补**（accent 面上读不出，需拍板） | `已落地` |
| 2-7 | 卸载对话框 | `Float` + `ElevThrow` + **E1 的** `LensRim` | E2：冠线换 `EdgeHighlight`（实测深 `(x,21)` 101→108） | `已落地` |
| 2-8 | 保存告警（`savealert`） | `Float` 面，无冠线 | E2 冠线 `EdgeHighlight`（实测深 `(x,21)` 24→51）；**两层影故意不加**：它随内容滚，宽投影会被视口裁成硬边，做成真浮层需拍板 | `已落地` |
| 2-9 | 状态 chip / 权限药丸 | `Control` + `ControlEdge` | **故意不做**：4px 高的药丸加一条线就成噪点 | `故意不做`（见理由） |

pin：`internal/settingsui/liteedge_test.go` 五条（侧栏 / 分组卡 / 字段两处 / 凹槽反面 /
两张浮层），每条在深/浅两套外观上跑。**冠线怎么量**：`liteMeasure` 把同一个面、同一个
底色、两条候选线（`LensRim` 与 `EdgeHighlight`）各画一遍当参照，再要求被量到的线离它
「应该」那条更近——pin 里没有一处写死渲染器算出来的像素值。

出图 `/tmp/floter-P12-c-{general,dialog,banner}-{dark,light}-{before,after}.png`。

#### §6.2b 控件面：十处 `Lens` 退成 E1（P13-i，**已落地**）

**根因**：`Lens` 的四层是「rim → sheen → edge → **四边 `ControlEdge` 环**」，而环画在
最后、且落在**同一个像素行**上（mygo 的 `insetShadow` 把 hole 从 padding edge 算起：
`spread=1` 的环占 padding 第一行，`y=1,spread=0` 的 rim 也是那一行）。于是 P12-a 抬到
0.09 的受光边被压在下面——**这一页的控件至今一次都没亮过**，顶内线一直是
`ControlEdge` 的暗线。P13-h 在启动器的 28u 图标板上抓到同一个根因，这一轮是同一族在
设置页上的十处。十处控件**都各自带着 `Border(1, ControlEdge)`**（两处悬停小格是 hover
时才加），所以环与边框本来就是同一条边的第二次上色：退掉它**不改几何**（外框还在，
边缘仍是「边框 + 内线」两像素），只改内线那一像素的墨。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-1 | `rows.go` `extensionActionFace`（`.extensions-action-button` 胶囊） | `Border` + `Lens`：内圈是第二条 `ControlEdge`，顶边是暗线 | `LitFace`：顶 `LensRim` + 底 `LensEdge`，无环（实测浅色侧列 115 → 面 120） | `已落地` |
| 9-2 | `rows.go` `extensionIconButton` 的 hover 臂（30u） | 同上 | `LitFace` | `已落地` |
| 9-3 | `parameditor.go` `paramIconButton` 的 hover 臂（28u） | 同上 | `LitFace` | `已落地` |
| 9-4 | `settingsui.go` `headerButton` 的 hover 臂（页头两个 28u） | 静息透明，hover 抬面 + `Lens` | `LitFace` | `已落地` |
| 9-5 | `settingsui.go` `barButton`（丢弃条胶囊） | `Border` + `Lens` | `LitFace` | `已落地` |
| 9-6 | `uninstall.go` 卸载对话框动作胶囊 | `Border` + `Lens` | `LitFace` | `已落地` |
| 9-7/8/9 | `uninstall.go` `uninstallPartRow` 的 fixed / hover / default 三臂 | `Border` + `Lens` | `LitFace`（三臂同一对线） | `已落地` |
| 9-10 | `savealert.go` 保存告警的 ✕（24u，E2 横幅里） | hover 抬面 + `Lens` | `LitFace` | `已落地` |
| 9-11 | `control/control.go` `SegmentedIndex` 的选中段（`.settings-option--active`） | `Lens` + **紧跟其后的** `AccentEdge` 四边环 | **透镜本身：故意不做**：它自己的身份线是那条 accent 环，四边都在、把 rim 与 edge 全盖住；换成 `LitFace` 画出的是立刻被遮住的两条线（像素上不可见），而 §2 也不许一个面说两句话。它是设置页与终端抽屉共用的控件（跨表面）。**填充另案**：这一格的填充原本借 `ControlHover`，在浅色档把「抬起」画成「槽里再挖深一层」——P13-r 落地，见 §6.2b-5 | `故意不做`（透镜）/ `已落地`（填充，§6.2b-5） |
| 9-12 | `shell/notices.go` toast 的动作 chip（`.app-toast__action`） | `Control` 面 + `Lens`，**没有自己的边框**——环就是它唯一的边界 | E1：`tokens.LitFace`（**不补边框**：面差就是它的边界）——P13-j 落地，实测见 §6.6 第 6-3 行 | `已落地` |

#### §6.2b-2 漏在 `Lens` 账本外的两处控件面（P13-l，**已落地**）

上面九行是「`Lens` 调用点」的账；这一节是同一族的**另一面**——从来没有调过 `Lens`、
所以 `glasscensus_test.go` 的账本照不到，但它们停在了 P12-c 第 2-4 行点名的那个状态：
「一块被画框圈住的平涂」（`Control` + 一圈 `ControlEdge`，**没有内线**）。两处都是**面**，
不是槽（槽是 `LensInset` 的凹槽，不许混）。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-13 | `settingsui.go` `shortcutRecorder`（82u × 27u 录制钮，快捷键页 + 自定义工具表单共用） | `Control` + `Border(1, ControlEdge)` + `FocusRing`，**无内线**：顶内线就是边框本身，浅色档顶 125 / 侧 116 / 底 105（面 120） | E1：`tokens.LitFace(button)`，三个 rung 共用（hover / press 只换面）；`FocusRing` 不动。实测浅色顶 **131** / 侧 **119（= 面）** / 底 **112**；深色顶 **151** / 侧 **141** / 底 **133**（面 141） | `已落地` |
| 9-14 | `settingsui.go` `noticeBand` 的**非危险臂**（`disableConfirmation`：关掉一个扩展时出现） | 同一个组件的两条臂里只有 `danger` 那条说了话（一圈 `InsetShadow(0,0,0,1, Warning)`）；非危险臂只剩发丝，比它坐着的 E1 分组卡还平（浅色档顶 = 底 = 侧 = 面 119） | E1：`else { tokens.LitFace(band) }`。实测浅色顶 **131** / 底 **112**；深色顶 **151** / 底 **133** | `已落地` |
| 9-14b | 同处的 `danger` 臂 | `Control` + 警告环 | **故意不加** `LitFace`：`InsetShadow(0,0,0,1,…)` 的环占 padding 第一行，与 `LitFace` 的 `y=1` 受光边是同一行——加了会把环顶洗淡 9% 白、底压暗 6%（实测浅色 `156,31,24` → 顶 `165,51,45` / 底 `147,29,23`，深色 `234,171,160` → `236,179,169` / `220,161,151`），一圈同色的环就不再成立。一个面只说一句话（同 9-11、§6.4 4-4） | `故意不做`（见理由） |

#### §6.2b-3 多行写法漏在单行 grep 框外的三块（P13-m，**已落地**）

P13-i 与 P13-l 的排查都是**单行**匹配：

```
grep -n "tokens.Control)" internal/settingsui/*.go | grep "Border(1"
```

下面三处的 `Border(...)` 与 `Background(tokens.Control)` **不在同一行**（一个在前一行、
一个在后一行），所以两轮都照不到它们——不是「判过之后决定不做」，是**从来没进过视野**。
三块都停在 P12-c 第 2-4 行点名的那个状态：「一块被画框圈住的平涂」。**外圈线一律不动**
（两处是 `Hairline`、一处是 `ControlEdge`）：受光边是**内线**，画框是外线（§1）。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-15 | `config.go` `configMultiselectBox`（66u 多选字段箱） | `Border(1, ControlEdge)` + `Background(Control)`，无内线——而它自己的注释写着「与 `fieldFace`/`sheetFieldSlot` 同面」，那两个字段自 P12-c 起就是 E1 | `tokens.LitFace(box)`。实测浅色顶 **119 → 125**、底 **119 → 115**、两侧 = 面 119；深色顶 **141 → 146**、底 **141 → 137** | `已落地` |
| 9-16 | `settingsui.go` `commandPlate`（扩展页命令列表的每一条命令块） | `Background(Control)` + `Border(1, Hairline)`，无内线：与 §6.2 第 2-3 行的分组卡同档，卡有那一对线它没有 | `tokens.LitFace(plate)`（放在 `Border` 之后、`Opacity(0.62)` 之前：透明度是整块的状态，不参与材质）。实测数字与 9-15 同（同一块地面） | `已落地` |
| 9-17 | `textinput.go` `aliasField`（面板字段族） | `Background(Control)` + `Border(1, Hairline)`，无内线：它是面板里唯一没有那一对线的**能打字的槽** | `tokens.LitFace(...)`。它原来的注释说「它是 inline command-chip，不是 settings slot」——那说的是**边框**的选择（`Hairline` 而不是 `ControlEdge`），不是「不要受光边」的理由 | `已落地` |

pin：`internal/settingsui/multilineface_test.go` 三条（每块一条，深+浅两套外观，走
`lensAssertE1` 的四条判据：顶亮 / 底暗 / 两侧 = 面 / 块外无影），加一条 opt-in dump。
出图 `/tmp/floter-P13-m-faces-{light,dark}.png`：六行「同一个画框 before/after」+ 三块真组件。

pin：`internal/settingsui/bandface_test.go` 三条——录制钮与非危险带各一条像素 pin（深+浅，
走 `lensAssertE1` 的四条判据），加一条**反面** pin 守 `danger` 臂的警告环（量顶内行的
暖度 `r−b ≥ 25`，警告墨在两套外观上都是暖的，而 `LensRim` 的白/黑与面本身都是 `r ≈ b`）。
出图 `/tmp/floter-P13-l-faces-{light,dark}.png`。

pin：`internal/settingsui/lensface_test.go` 四条——胶囊 / 30u 悬停图标钮 / 卸载组件行卡片
三条像素 pin（每条深+浅两套外观，量「顶内线亮于面」「底内线暗于面」「两侧列 = 面」
「块外仍是地」四条判据），加一条结构 pin（这一页的源码里不许再出现四层透镜的调用）。
V 2（`lens_test.go` 的玻璃档涨幅、`uninstallmetrics_test.go` 的按下丢影）/ S 1（材质从宽
sheen 变成 1px 一对线）/ B 0。

出图 `/tmp/floter-P13-i-pill-after-{light,dark}.png`（同一张图上并排画「`Lens` 之前」与
「`LitFace` 之后」的同一枚胶囊）。

#### §6.2b-4 材质 census，不是函数名 census——最后三块（P13-p，**已落地**）

前三轮的排查都是 grep：P13-i 数 `tokens.Lens(`，P13-m 数 `Border(1,` 与 `tokens.Control)`
在不在**同一行**。下面三块面对这两种查询都不可见——它们从来没有 `Lens` 可数，面还写在
变量里（`plane`），或者两条线分两行写。**根因不是这三处，是 census 的查询方式**：
数「某个函数的调用」只能找到「已经这么写的」，找不到「本该这么写却没写的」。
所以本轮除了补三块，还把 census 换成按**函数体**扫材质
（`internal/settingsui/facecensus_test.go`，`go/ast` 取每个 `FuncDecl` 的源码文本）：
一个函数里只要同时出现「`Control`/`ControlHover`/`ControlPress`/`Field`/`RaisedWarm`/
`plane` 的面」与「`Border(1` 或 `InsetShadow(` 的边」，就必须出现 `LitFace` / `.Lens(` /
`LensInset(` / 直接的 `InsetShadow(0,1,0,0,…)` 之一，否则必须在 `faceCensusAllow`
里写明理由；允许表**反向**也钉着——某条开始戴一对线而表没删，同样红（表不许变成
历史垃圾场）。多行写法、变量面、跨语句的材质，从此都躲不过去。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-18 | `rows.go` `sessionIconButton` 的 `Hovered()` 臂（Sessions 页 28u 方格） | `Border(1, ControlEdge)` + `Background(ControlHover)`，**没有一对线**：它从来没有 `Lens` 调用，所以三轮 grep 都照不到 | E1：`tokens.LitFace(button)`。实测浅色顶 **221 → 224**、底 **221 → 208**（面 221）；深色顶 **48 → 67**、底 **48 → 38**（面 48）。三个同胞（30u 的 `extensionIconButton`、28u 的 `paramIconButton`、页头 28u 的 `headerButton`）自 P13-i 起就是这一对 | `已落地` |
| 9-19 | `savealert.go` `shortcutRejectedAlert`（Shortcuts 页的拒绝条） | `Background(Control)` 与 `Border(1, ControlEdge)` **分两行写**，单行 grep 照不到 | E1：`tokens.LitFace(ui.Box(…))`。实测浅色顶 **230 → 232**、底 **230 → 216**；深色顶 **39 → 58**、底 **39 → 31**。与 `noticeBand` 的非危险臂（9-14）同一个形状、同一档；**不是 E2**（不浮、无影），所以不碰 `EdgeHighlight` | `已落地` |
| 9-20 | `pluginsheet.go` `sheetActionButton` 的**中性**臂（弹层 28u 胶囊） | 面写在变量 `plane` 里，`Border(1, edge)` 也是变量——两种 grep 都数不到 | E1：`if plane.A > 0 { tokens.LitFace(button) }`。实测浅色顶 **230 → 232**、底 **230 → 216**；深色顶 **39 → 58**、底 **39 → 31** | `已落地` |
| 9-20b | 同处的 `danger` 臂 | 面是 `ui.Transparent`，身份线是 45% 的警告发丝 | **故意不加**：在一块透明的面上画一条 0.09 的白线就是 §1 禁令 3 的「类玻璃边」，而且它没有面可以「站起来」。指针把面点亮（`plane = ControlHover`）之后它才戴——那时它真的是一块玻璃。反面 pin 量「上下两条内行都不许离开地面 ±2 级」 | `故意不做`（见理由） |

pin：`internal/settingsui/litpair_census_test.go` 四条——三块面各一条像素 pin（深+浅，量
「顶内行是受光边 / 底内行比面暗 ≥9 级」）+ 一条反面 pin（`danger` 臂的上下内行都贴着地面）。
外加 `facecensus_test.go` 的材质 census 与它的允许表（11 条，每条一句话写清是 §2 的哪条
理由：4px 药丸 / 26~30u 方格 / 14u 勾选框是噪点，警告内环与 accent 环占住那一行）。
**V 0 / B 0 / S 0**（没有既有 pin 需要改断言：三处此前都没有 pin）。
出图 `/tmp/floter-P13-p-{sessionbutton,refusalband,sheetpill}-{light,dark}-{before,after}.png`：
差异的 bbox 恰好是控件自己的盒子（按钮 24×24、两条带子整宽），控件之外一个像素没动。

#### §6.2b-5 凹槽里的那一格：浅色档的「抬起」方向反了（P13-r，**已落地**）

§6.2b 9-11 判的是**透镜**（`Lens` ↔ `LitFace`），这一轮判的是同一格的**填充**——
两件事，因为环盖住线、盖不住面。

**根因**：选中格借 `tokens.ControlHover`，而这个 token 在两种外观上指向**相反的 z 方向**。
浅色档的「抬起」是黑洗（`0,0,0,0.105`），压在凹槽面 216 上得到 **193**——比它自己的槽
还暗 23 级；深色档的「抬起」是白洗（`255,255,255,0.14`），压在 78 上得到 **103**。
于是同一枚控件在深色档说「这是抬起来的」，在浅色档说「这是挖下去的」，而代码注释与
§2 的 E1 都写着 raised pane。「凹槽里更深的一格」正是用户点名的 macOS 10.x 读法
（「有点偏 macOS 旧版本」）；现代 macOS/iOS 的分段滑块是**纸色抬起**。

**落地**：新 token `theme.Tokens.Thumb`。浅色档 = 纸自己的颜色（`rgba(255,255,255,0.85)`，
在卡面的凹槽 216 上复合到 **249**，页面是 247 ±2 —— 抬起的面就是那张纸穿过了凹槽）；
深色档 = 原来那档白洗（`0.14`，**外观一个字节没动**：深色没有更亮的纸可借，页面是 39，
所以两档不同值不是不一致，是同一条规则在两种底上的唯一解）。`Lens` 与 `AccentEdge` 环
**不动**（9-11 的理由照旧）。

实测（真 `SegmentedIndex` 画在设置页的卡面 `Control` 上）：

| 外观 | 页面 | 卡面 | 槽面 | 选中格（旧 → 新） | 抬高量 | accent 环（蓝−红） |
|---|---|---|---|---|---|---|
| 浅色 | 247 | 230 | 216 | **193 → 249** | −23 → **+33** | 58（面 0） |
| 深色 | 39 | 61 | 78 | 103（不动） | +25 | 31（面 4） |

pin：`internal/control/thumb_test.go` 三条——①两套外观下选中格比槽面亮 ≥10 级；
②浅色档选中格落在纸色 ±3 且高于卡面；③accent 环相对选中格面仍然「是一条蓝线」
（蓝−红 差 ≥25；浅色档用 R 通道量会在深色档误判：深色环的底行 R 只高 6 级，但它仍是蓝的）。
**V 0 / B 0 / S 0**（选中格的填充此前没有任何像素 pin；深色档的渲染前后逐字节相同——
`/tmp/floter-P13-r-segment-{before,after}-dark.png` 的 diff 是 **0 像素**）。
出图 `/tmp/floter-P13-r-segment-{before,after}-{light,dark}.png`：浅色档的差异 bbox 是
`(150,29)-(269,55)`，**恰好是选中那一格自己的盒子**（中间段 148.8~271.2），控件之外
一个像素没动。

#### §6.2b-6 禁用面：面还在，光熄了（P13-s，**已落地**）

上一轮（P13-r）判的是凹槽里那一格的**填充**；这一轮判的是**状态反馈**的另一半——禁用 /
忙碌面。根因不在设置页的调用点，而在**两个淡相乘**：

- 框架的画笔给 `Disabled` 的子树乘 **0.5**（`ui/paint.go:257-259`，`p.opacity *= 0.5`）；
- 产品在外面又叠了一道 `.Opacity(0.4)`（`busyActionButton` / `configActions` 的 Restore
  defaults / `configOpenToolRow` 的 Open configuration）。

有效透明度 = **0.2**。实测（真设置页地面，`extensionGlyphActionButton` 的禁用臂）：

| 外观 | 地面 | 静息面 | 静息顶 / 底内线 | 禁用面 | 禁用顶 / 底内线 |
|---|---|---|---|---|---|
| 浅色 | 247 | 230（−17） | 232（+2）/ 216（−14） | **243（−4）** | 243（= 面）/ 240（−3） |
| 深色 | 15 | 39（+25） | 58（+19）/ 31（−8） | **19（+4）** | 19（= 面）/ 18（−1） |

面台阶只剩 4 级、底对侧只剩 1~3 级：E1 的一对线双双掉进噪声，禁用胶囊不是「一块不能碰
的面」，是一块**几乎不存在的面**——这正是「平 / 糊 / 没有空间感」在状态反馈这一侧的算术来源。

**落地**：只留一道淡（框架那一遭），材质换成 `theme.Tokens.DimFace`——**底对侧还在，受光边
熄了**。受光边是「这块面可以被碰」的记号（§3），禁用面不戴它；面本身继续用 `Control`，
在 0.5 之下正好是半个台阶。**不新增 token**：候选 `ControlDim`（`Control` 的一半）实测被
框架的 0.5 再乘一次，落到浅色 243（−4），与改前逐像素相同——这一档框架已经给了。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-21 | `theme.Tokens.DimFace`（新材质） | 不存在；禁用面沿用 `LitFace`（顶 `LensRim` + 底 `LensEdge`） | 只画底对侧 `LensEdge`，**不画受光边**；面由调用方给 `Control` | `已落地` |
| 9-22 | `settingsui/rows.go` `extensionActionFace` 的 `disabled` 臂 | `disabled` 只影响 hover / press，材质仍 `LitFace` | 面 `Control` + `tokens.DimFace(button)`。实测浅色面 **238（−9）** / 顶 = 面 / 底 **231（−7）**；深色 **27（+12）** / 顶 = 面 / 底 **24（−3）** | `已落地` |
| 9-23 | `rows.go` `busyActionButton`（在飞的胶囊） | `Disabled(true).Opacity(0.4)` | `Disabled(true)`（淡交给框架那一道 0.5）。墨对比 43 → **106** 级 | `已落地` |
| 9-24 | `rows.go` `extensionGlyphActionButton` | 无 `disabled` 参数，内部恒传 `false` | 加 `disabled bool` 透传，两个调用点（`configActions` 的 Restore defaults、`configOpenToolRow`）不再自己叠 `Opacity(0.4)` | `已落地` |
| 9-25 | accent 主按钮（`extensionPrimaryButton` / `updateButton`）的禁用臂 | `.Opacity(0.4)` / `.Opacity(0.6)` | **故意不做**：accent 面的禁用要淡的是**色相**（fill 本身），不是「面还在」的问题——那是另一条规则，另起一轮 | `故意不做`（见理由） |
| 9-26 | `sessionKillConfirm`（警告色的 raised 胶囊） | `.Opacity(0.4)` | **故意不做**：它戴的是 `RaisedWarm` + `ShadowContact` + `LensRim` 的暖色抬升面，与中性 E1 不同族；同一个 `DimFace` 会把它的警告环洗淡 | `故意不做`（见理由） |

pin：`internal/settingsui/dimface_test.go` 三条——①禁用胶囊还有面台阶（≥6 级）、顶内线**不亮**
（受光边熄灯）、底内线还是对侧（浅 ≥5 / 深 ≥2 级），并且台阶不许大到读成静息面（≤15 级）；
②在飞的胶囊（`busyActionButton`）同一套判据；③墨只淡了一道（对比 60~160 级，实测 106；
双重淡时是 43，静息是 202）。每条都在深/浅两套外观上跑。**V 0 / B 0 / S 0**。
出图 `/tmp/floter-P13-s-dimface-{light,dark}-{before,after}.png`：差异 bbox
`(24,63)-(295,128)` **只覆盖第二、三枚胶囊**（两枚禁用面），第一枚静息胶囊一个像素没动。

#### §6.2b-6b 锁定的槽与开关：一道淡，不是三道（P13-t，**已落地**）

9-21~9-26 判的是**胶囊**（raised pane）。同一族在**槽类控件**上还没量过：扩展配置表单
的锁定态（`configLocked`，行上任何操作在飞时整张表单只读）此前在三处画了三个深度——

| 控件 | 有效淡度 | 来源 |
|---|---|---|
| 字段 / 开关 / 下拉 | **0.3** | 框架的 0.5 × 产品自己的 `.Opacity(0.6)` |
| 列表框（multiselect） | **0.6** | 只有产品那一档，`Disabled` 忘了挂 |
| 框里的勾选框 | **0.25** | 0.6 × 自己的 `Disabled` 0.5 |

实测（浅色地面 247 / 深色 14）：字段面 **242（−5）/ 22（+7）**、开关轨道 **195 / 67**、
列表框面 **236（−11）/ 29**、选项文字 **160（−76）**。0.3 把字段的 E1 一对面**双双抹平**
（顶内线 = 面 = 242、底内线 = 面）——与 9-22 量到胶囊只剩 4 级台阶是同一个算术。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 9-27 | `settingsui/config.go` `lockedControl`（新 helper） | 五个臂各写一遍 `.Disabled(true).Opacity(0.6).Cursor(...)` | 一个 helper：`Disabled(true)` + 默认光标，**不带 `Opacity`**。实测字段面 **238（−9）/ 27（+12）**、顶内线 239、底对侧 231（−7）、墨对比 65 → **101** | `已落地` |
| 9-28 | `configMultiselectBox`（列表框） | 档位在**调用方**，且只有 `.Opacity(0.6)`、没有 `Disabled` | 签名收 `locked bool`，档位落在**控件根**上：箱面、选项文字、勾选框同乘 0.5。实测箱面 **238（−9）/ 27**、选项文字 160 → **177（−61）** | `已落地` |
| 9-29 | `configOptionRow` 的勾选框 | `locked` 时再挂一次 `Disabled(true)`（0.5 × 0.5 = 0.25） | 删掉：祖先已 disabled，`IsDisabled` 走祖先链（`ui/element.go:1349-1356`）；`locked` 只留两条点击守卫 | `已落地` |

**规矩（写进 §1 的材质分档）**：禁用的**档位属于控件根**，不属于每一块面——一个 flag 挂
在根上，整棵子树同乘框架的 0.5。禁用的**槽**与禁用的**面**是两回事：面靠 `DimFace` 熄掉
受光边，槽（字段 / 开关 / 列表框）没有受光边可熄，它靠「整块退一半、内线各留一半」表达
「还在 z 上、现在不能碰」。滚轮不受 `Disabled` 影响（`ui/input.go:515-536`），锁定的长
选项列表仍然读得到。

pin：`internal/settingsui/lockedform_test.go` 四条——①字段、②开关、③列表框的每一个读数
（面 / 受光边 / 底对侧 / 墨）都必须正好是「静息与地面各一半」的混合；④字段与列表框
（同一材质）锁定后必须落在**同一个像素**（改前 242 vs 236）。**V 0 / B 0 / S 0**。
出图 `/tmp/floter-P13-t-lockedform-{light,dark}-{before,after}.png`：浅色锁定字段
`(160,66)` 由 `(242,242,243)` → `(238,238,239)`，地面 `(247,247,248)` 不动。

### §6.3 终端页（terminalui）— **已落地（P13-d）**

**落地的形状**：抽屉的缝与空态键帽走 E1 的 `theme.Tokens.LitFace`（顶 `LensRim` +
底对侧暗线，无影）；悬停的图标按钮从四层控件透镜退回 `LitFace`（**退掉的第四层就是
`ControlEdge` 的环**，§2 的「画框分层」）；悬停的标题条补上 §2 判据 3 要求的**第二层
影**。**为什么这一页此前「没有空间感」**：它只有**两档**在场——底（面板 + 画布，两者
都是 E0）与一条四面环的控件透镜；中间那档**面**（E1）一次都没画过，而唯一像「浮」的
东西又只压了一道**紧贴边的重影**（几何是宽影、墨是紧贴边墨）。三处改完之后，
「底 / 面 / 浮」三档第一次同时出现在这一页。

**3-2 的改判（`未开始` → `故意不做`）**：原行要求给画布补 `LensRim` 顶边。**不做**，
两条理由：①画布**是**这一页的**底**（终端内容面），不是浮在底上的块——§2 判据里的
「块」指的是长在底上的那一层；②P11-z 刚按用户实测 F3 退掉的就是这一页的**第二层框**
（`Border(1, edge)`，聚焦时还是 accent 红），给画布补一条受光边等于在 F3 的同一处把
第二层分界加回来，只是换了颜色——那正是用户读到的「类玻璃边」。

**3-3 没有用 E2 的线**：原行写的是 `EdgeHighlight`。抽屉是**长在面板里的一个块**
（画布的兄弟、占布局高度、把画布压小，**不压在内容上**）→ §2 判据给的是 **E1**，
所以戴 `LensRim`。E2 的 `EdgeHighlight` 在设置页已经是「浮起来的纸」的边（P12-c 的
2-7 / 2-8）；两种边各管一档，空间感才有算术来源。**底对侧不补**：抽屉的下边就是窗口
的底边，P11-av 已经证明那条线会落进窗口最底一行、压进 12u 圆角的过渡里。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 3-1 | 面板 | E0 + `panelEdge` | 保持；macOS 的 `--glass-rim-shadow` 四笔与受光边是同一件事 | `已落地` |
| 3-2 | 会话挂载块 | `RadiusMD` + 一层填充 | **故意不做**：画布是这一页的**底**，补边 = F3 的第二层框回来（见上） | `故意不做`（见理由） |
| 3-3 | 外观抽屉的缝 | `HairlineSoft` 分区线（0.055） | **E1 的 `LensRim`**（0.09，实测浅 `(450,384)` 217→232、深 51→58） | `已落地` |
| 3-4 | 标题条 hover 影 | 宽几何 + `ShadowContact` 一笔 | E2：宽影换 `ShadowAmbient` + 补 `0 1px 2px` 的 `ShadowContact`（实测浅第一行 231→193、深 16→8） | `已落地` |
| 3-5 | 底部常驻便条 / 顶部失败条 | 整宽带子，无圆角无影 | **故意不做**：贴边的带子不是浮层（P11-bc/bg 定审） | `故意不做` |
| 3-6 | 状态灯 | 1.75u 中性点 + 呼吸 | **故意不做**：不画受光边（它只有 1.75u） | `故意不做` |
| 3-7 | **新增**｜悬停的图标按钮（`barButton`） | `Lens(button)`：四层，第四层是四边 `ControlEdge` 的环；旁边还写着「≤0.04」（P12-a 已重定到 0.09） | E1：`LitFace` —— 顶受光边 + 底对侧暗线，**无环、无宽 sheen** | `已落地` |
| 3-8 | **新增**｜空态键帽（`emptyLine`） | 只有顶边一条 `LensRim`（一半的受光边），浅色档白压白 | E1 完整：`tokens.LitFace(badge)` —— 与 P12-b 的启动器键帽同一个函数（实测浅底内线 230→216） | `已落地` |

pin：`internal/terminalui/litedge_test.go` 三条（抽屉的缝 / 悬停按钮是面不是环 /
空态键帽的一对线），每条在深/浅两套外观上跑；`raycast_test.go` 的
`TestTheTitleBarHoverShadowIsShadowContact` 按新值重定基线（参照从一条变成一叠）。
`internal/shell/glasscensus_test.go` 的 `.toolbar-button:hover` 条目从「透镜必须还在」
翻成 `dropped`（新面由上面第二条 pin 逐像素量着）。
出图 `/tmp/floter-P13-d-drawer,bar,empty}-dark,light}-before,after}.png`。

### §6.4 扩展页（extensions / 设置页内的集成面）— **已落地（P13-e）**

**落地的形状**：被 `floter://register` 链接命名的那一行走 `theme.Tokens.LitFace`
（顶 `LensRim` + 底 `LensEdge`，无影），触发点是 `rowFace` 的新字段 `Lit`；动作胶囊
走复核（像素 pin），产品代码零改动。**为什么这一页此前只有「两级半」**：底（页面）与
面（分组卡，P12-c 的 2-3 已在场）都有了，缺的是**被选中的那一行**——它只是一层
`RowSelected` 平涂，与脚下的 E1 卡只差一档 alpha，读起来是「卡里又刷了一块漆」。
补上一对线之后，这一页第一次出现「底 / 面 / 选中的面」三级。

**4-4 为什么给破损行写一条反面规则**：一个面的身份是**一条**受光边（§1 的不变量）；
破损行那条线已经被它的警告色内环占住（`Inset` 四边都在）。再叠一条白冠线会画进
同一个 1px 行，把「这块坏了」洗成「这块被选中了」。这不是少做一步，是「一个面只
说一句话」。

**4-3 复核的结论**：`extensionActionFace` 的 `tokens.Lens(button)` 第一层就是
`LensRim`（P12-a 已重定到 0.09），所以「受光边随升代自动到位」成立——本轮补的是
**证据**（像素 pin），不是代码。没改成 `LitFace`：胶囊的第四层 `ControlEdge` 环与它
自己的 `Border(1, ControlEdge)` 画在相邻的两行 1px 上，是同一条边的两次上色，不是
第二层画框；退掉它会让胶囊边缘从 2px 掉到 1px，那是几何改动，超出「只改墨与面」
的红线（观察记录在 `docs/rounds/P13-e-report.md`）。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 4-1 | 集成行面（`settingsui.go` `integrationRowFace` 的 `Highlighted` 臂 + `rows.go` `cardRow`） | `RowSelected` 一档平涂，与脚下的 E1 卡只差一档 alpha | E1：`rowFace.Lit` → `tokens.LitFace(row)`（实测浅色顶边 223→224、底边 221→208；深色 65→70 / 65→61） | `已落地` |
| 4-2 | 包 chip（`rows.go` `packageChip`） | `Control` + `ControlEdge` | **故意不做**：与 2-9 同一条（30u 的方块加冠线就成噪点） | `故意不做`（见理由） |
| 4-3 | 扩展动作按钮（`rows.go` `extensionActionFace`） | `Control` + `Lens`，冠线是 `LensRim` | **复核通过 + pin**：产品代码零改动，顶边是 E1 的 `LensRim` 而非 E2 的 `EdgeHighlight`，且无外投影 | `已落地` |
| 4-4 | **新增**｜破损行（`integrationRowFace` 的 `Broken` 臂） | `RaisedWarm` + 警告色内环 + 警告前导条 | **故意不加** E1 冠线：它那条线已经是警告环，第二个面只说一句话（见上） | `故意不做`（见理由） |

pin：`internal/settingsui/liteedge_extensions_test.go` 三条（命名行的 E1 一对线 /
破损行不拿冠线 / 胶囊的冠线是 E1 的线），每条在深/浅两套外观、中灰底上跑——
`RowSelected` 在深色是白洗、浅色是黑洗，中灰是唯一两套都能读出「面站起来」的底。
`extensionrowface_test.go` 的 `TestTheRowFacePaintsItsPaneAndKeyline` 与
`TestTheIntegrationRowFaceFollowsItsState` 按新值重定基线（V）。
出图 `/tmp/floter-P13-e-integrations-{dark,light}-{before,after}.png` +
`/tmp/floter-P13-e-pill-{dark,light}-{before,after}.png`。

### §6.5 文件页 — **已落地（P13-f）**

**落地的形状**：5-1 是**复核 + 像素 pin**（产品代码零改动）——文件页的行就是 `rowBody`
的普通结果行（`files.go` 的 id 是 `drop:<action>:<path>`，不带 `bar:` 前缀），选中块在
P12-b 之后**已经**是 `tokens.LitFace`；这一页缺的从来不是代码，是**证据**。5-2 / 5-3
是这一页自己的墨：图标列此前十五个板全是**历史时钟**。

**为什么这一页的「眼睛」比它的面更重要**：每行第一眼落在那块 28u 的图标板上。板上写着
「最近用过」的时候，页面在说一件它没有的事——三个不同的动作（打开 / 进终端 / 拷路径）
被画成同一个记号，整页读起来是「一堆一样的东西」，这正是用户说的「平、糊」；面与受光
边的层级信号再准，也被一个说错话的图标列淹掉。换成语汇之后（与动作条同一组三个记号），
这一页第一次能「一眼看出这行要干什么」。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 5-1 | 文件列表行（`launcher.go` `rowBody` 的 `i == a.Selected` 臂） | 与启动器结果行同一语汇，但**从无量测** | E1：`tokens.LitFace(row)`（继承自 §6.1 第 1-2 行）。像素实测浅色顶边 `238→240`（+2）、底边 `238→224`（−14）；深色顶边 `43→62`（+19）、底边 `43→40` | `已落地` |
| 5-2 | **新增**｜图标列（`glyphs.go` `rowGlyph`） | 没有 `drop:` 分支，尾巴 `return glyphClock, true`——五个文件十五个时钟 | 与动作条同一语汇：`drop:open:` → `glyphFolder`（`bar:path-open`）、`drop:cd:` → `glyphTerminal`（`bar:path-cd`）、`drop:copy:` → `glyphFile`（`bar:path-copy`）。按 id 前缀判（id 尾上带着路径） | `已落地` |
| 5-3 | **新增**｜折叠行（`drop:more`） | 同样落进时钟 | `glyphChevronRight`——它做的是**展开**，不是「最近」 | `已落地` |
| 5-4 | **新增**｜图标板（`glyphs.go` `iconPlate`） | `tokens.Lens(plate)` 四层，第四层是四边 `ControlEdge` 的环 | **故意不做**：这块板启动器**所有页**共用（结果行 / 剪贴板 / 计算器 / 浏览器），改它等于跨表面，违反「一个表面一轮」；留给启动器表面那一轮 | `故意不做`（见理由） |

pin：`internal/launcher/filesmarks_test.go` 两条（三条动作行的记号 + 折叠行的展开符 /
选中块的 E1 三判据），后者在深/浅两套外观上跑。出图
`/tmp/floter-P13-f-files-{dark,light}-{before,after}.png`：整页只有**九块 16×16 的
字形框**变（浅色 1397 px / 深色 1353 px），行盒、文字列、字号一个像素没动。

### §6.6 弹层（toast / 通知 / 对话框）— **已落地（P13-g）**

**落地的形状**：toast 是这一支的**第五个**浮层（前四个 P11-an 就进了 `theme.ElevThrow`），
也是唯一一个还在借窗口自己那对 token 的：`ShadowAmbient` 0.62 / `ShadowContact` 0.44，
而且几何写成 `t.Space(10)`/`t.Space(26)`——`Space()` 会乘 4，于是那两笔是 **40px 偏移 /
104px 模糊**，参考的 `--elev-3` 是 `0 10px 26px`。两条毛病一起犯：框拉到 10% 时别处
重了 2.8 倍它纹丝不动；同时一层 104px 的黑雾把内容页整片染暗。改完之后近处更深
（相对压暗 0.049 → 0.156）而远处干净（y=160..240 那条「影够不到」的带子平均 alpha
深 161.0 → 157.1、浅 154.2 → 152.8）。

**为什么折叠带子要补冠线**：参考的 `--elev-3-compact` 丢的是**冠线**（20px 带子里那条 1px
亮线读不出来），不是接触影；而 §2 判据 3 说 E2 必须两层影、§3 的表说 E2 的顶边是
`EdgeHighlight`。此前这条带子两样都没有，于是它读成「贴在卡上的一块」而不是「浮起来的纸」。
接触影在真机上看不出**向下的投影**（它离窗口底边只有 6px，影落进窗口自己那笔影里，
黑上加黑读不出），能读出来的是它把第一行加重到第五行的 1.96 倍（只有环境影时是 1.44 倍）
——pin 量的就是这个比值。

| # | 位置 | 现状 | 落成 | 状态 |
|---|---|---|---|---|
| 6-1 | toast（`shell/notices.go` `toastViewport`） | `Float` + `InputStroke` + 1px `EdgeHighlight`，两层影是**窗口 token** 的固定 alpha、几何 40px/104px | 满高臂 `theme.ElevThrow`（10px/26px + 1px/2px，alpha 0.30/0.20 × `ElevScale`）；折叠臂 `theme.ElevThrowCompact`（6px/18px + 同一笔接触边）**并补上冠线** | `已落地` |
| 6-2 | 终端失败提示 | 整宽带子 | **故意不做**（同 3-5） | `故意不做` |
| 6-3 | **新增**｜toast 里的动作 chip（`.app-toast__action`） | `Control` 面 + 四层 `Lens`，**没有自己的边框**——环就是它唯一的边界，而环又把 rim 压住 | E1：`tokens.LitFace`，**不补边框**（P13-j）。面差本身已经是它的边界：实测浅色 `(194 对面 209)` = **15 级**、深色 `(45 对面 21)` = **24 级**，够读出一条边；而 `Border(1, ControlEdge)` 会把 padding box 缩 1px（`insetShadow` 从 padding edge 算起），那是几何改动，也正是 §1 禁的「画框分层」。一对线的实测：浅色顶 **219**（对面 194）/ 底 **182**；深色顶 **56**（面 45）/ 底 **43**；两侧 = 面（浅 195、深 45） | `已落地` |

pin：`internal/shell/toastthrow_test.go` 三条——影随框透明度反向缩放（100% vs 10%，同一个像素、
同一次盘点的前后帧差）/ 两条臂的冠线都是 E2 的线（深+浅两套外观，浅色是白压白那一档）/ 折叠
带子的接触层（第一行比第五行重 1.7 倍以上，量过「只有环境影」是 1.44 倍）。
**P13-j 追加**：`internal/shell/toastactionface_test.go` 一条像素 pin（6-3 的 chip：顶亮 / 底暗 /
两侧 = 面 / 块外无影 / 面差 ≥ 4 级），深+浅两套外观，量在真 toast 上；外加 opt-in 的
before/after 对照图。census 的 `.app-toast__action` 从「钉调用」翻成 `dropped`。
出图 `/tmp/floter-P13-g-{settings,launcher}-{dark,light}-{before,after}.png`。

**§6 的 `未开始` 行在 P13-g 收口之后又长出一条**：P13-i 把设置页剩下的十处 `Lens`
退成 E1 时，清点到 toast 的动作 chip 与 `control.SegmentedIndex` 的选中段——前者是
`未开始`（6-3，要连带决定边界），后者是 `故意不做`（9-11，accent 环就是它的身份线）。
**P13-j 把 6-3 落成 `LitFace` 且决定不补边框之后，表里再次没有 `未开始` 行**；
剩下的唯一一个非「已落地」条目是 9-11 的 `故意不做`。全仓 `Lens` 的调用点从 12 降到 **1**
（`control.SegmentedIndex`，它自己的 accent 环把 rim 与 edge 全盖住，像素上换不出来）。

P13-h 是收口之后的第一轮细节打磨：它给 §6.1 补了 1-8 / 1-9 两行（同一条材质规则
落到启动器的小面上），**没有开新表面**，表里仍然没有 `未开始` 行。

P13-j 是第二轮：它把 §6.6 的 6-3（toast 动作 chip）收掉——那是 `Lens` 最后一个
「控件该不该退成 E1」的悬案，也是这张表最后一个 `未开始`。

P13-p 是第五轮：它同样没有开表面，而是把「控件该不该退成 E1」这一族的**排查方式**换掉
（函数名 grep → 函数体材质 census），顺带补上最后三块面（§6.2b-4 的 9-18…9-20b）。

P13-o 是第四轮（P13-n 之后）：它没有开表面，而是补 §6.0 的 0-9——凹槽的底受光边
（`TrackRim`）。P13-n 量的是「面」的一对线，这一轮量的是「凹槽」的一对线：两者的
底边不是同一条，因为凹槽的那一行上多压了一条 `TrackEdge` 发丝。pin 是
`internal/settingsui/groovepair_test.go`（两套外观 × 三个真凹槽：分段槽 / 关的开关 / 开的开关）。

---

## §7 与 raycast 合约的关系

- **继续有效**：§1 的颜色语义 token 与两条不变量（发丝线同值、行态梯中性有序）、
  §2 半径梯、§3 高度链、§4 各表面规格（除光泽上限）、§5 accent 的四类用途与注脚、
  §6 的全部已落地条目。
- **被本文件取代**：§1 末尾的「低光泽上限全部 ≤ 0.04」（→ §1 末尾的新表）、
  「分层靠面 + 线 + 字阶，不靠高光」（→ §2 的三档，受光边与两层影进场）。
- **新增**：§0 自绘的物理边界与四条硬禁令、§2 elevation 阶梯、§6 逐表面差距表。

`docs/raycast-design.md` 的顶部已改写成「已由本文件升代，本文只保留行为契约与
已落地条目」。
