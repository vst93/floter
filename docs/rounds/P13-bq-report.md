# P13-bq · 设置页行的纵向刻度：这一族只有一档，账是「别再长出第二格」

> 规格 `docs/rounds/P13-bq-spec.md`｜合约 `docs/liquid-glass.md` §6.35（3 行新账 + 1 行 `未开始`）
> **零产品字节**（`git diff --stat` 只有 `docs/liquid-glass.md` 的 22 行）

## 一、选题与意图

`docs/liquid-glass.md` §6 的 `未开始` 行在 P13-bp 之后已清空，选题按通用队列走。
`.pending-bugs.md` 从 P13-bo 起连着两轮把「通用队列 · 间距节奏」列成下一格，并且
**明写「先量设置卡片里 8/9/11/13u 的混用（不许先改）」**——那就先量，量的结果决定这轮
是「改」还是「钉」。

**这轮为什么让它更轻、更有空间感**（一句话）：间距节奏是空间感的**算术**——用户读到的
「平、糊」不是缺装饰，是**同一个角色有两个刻度**。这一轮先把「行」这一族的刻度数清，
并把「不许长出第二格」钉成断言；同时把下一个真问题（分组间距 8u vs 10u 差 2 像素）
从「感觉」变成一个**有坐标的 `未开始` 行**（§6.35 的 35-3）。

## 二、先量：结果与猜想相反

### 2.1 行的纵向刻度：三条路，一个值

`internal/settingsui/rows.go` 的三条行路，纵向 `Padding` 都是 `t.Space(2.25)`：
`t.Space(n)` 在本仓是 `4n` 像素，所以 9u = **36px**。

| 行路 | 源码行 | 纵向 Padding |
|---|---|---|
| `cardRow` | `rows.go:1359` | `Padding(t.Space(2.25), inset-1)` |
| `stackedRow` | `rows.go:1463` | `Padding(t.Space(2.25), inset)` |
| `actionRow` | `rows.go:1517` | `Padding(t.Space(2.25), inset)` |

**三条路同值** ⇒ 这一族**没有**「同一张卡里两行不一样高」的缺陷。

### 2.2 「8/9/11/13u 混用」这个猜想的实际读数

`internal/settingsui` 非测试源码里，`Padding(` / `Gap(` 等间距方法上的 `t.Space(n)`：

| 值 | px | 出处（样本） | 角色 |
|---|---|---|---|
| `Space(0.5)` | 2 | `uninstall.go:484`、`settingsui.go:1884`、`healthfaces.go:51` | 药丸 / chip 的贴着内缩 |
| `Space(1)` | 4 | `settingsui.go:4624`、`uninstall.go:187` | 行内部的空气 |
| `Space(1.5)` | 6 | `rows.go:457`、`healthfaces.go:173`、`uninstall.go:187` | 小格 / 小块的边 |
| `Space(2)` | 8 | `savealert.go:102`、`healthfaces.go:246`、`settingsui.go:2323` | **块**（告警带、横幅） |
| **`Space(2.25)`** | **9** | `rows.go:1359/1463/1517`、`settingsui.go:4553`、`savealert.go:179` | **行** |
| `Space(2.5)` | 10 | `settingsui.go:3181`、`settingsui.go:6078`、`savealert.go:179` | 块的第二档 |
| `Space(3)` | 12 | `uninstall.go:436`、`healthfaces.go:246`、`rows.go:1517` 的块 | **块** |
| `Space(3.5)` | 14 | `rows.go:365` `rowInset` 默认 | 行的**横向**内缩 |
| `Space(4)` | 16 | `uninstall.go:192`、`settingsui.go:1507`、`pluginsheet.go:72` | 面板 |
| `Space(4.25)` | 17 | `uninstall.go:160` | 对话框 |
| `Space(6)` | 24 | `empty.go:48` | 空态区域 |

**`Space(2.75)` = 11u 与 `Space(3.25)` = 13u 在 `internal/settingsui` 里一次都没出现作间距。**
全仓只有两处 `3.25`，都是**图标边长**：`rows.go:1563` 与 `shell/notices.go:372` 的
`Size(t.Space(3.25), t.Space(3.25))`。`2.75` 在非测试源码里**零出现**。

结论：**「8/9/11/13 混用」在纵向 padding 上不成立**——8 / 9 / 12 / 16 分属四个不同角色
（块 / 行 / 块 / 面板），11 与 13 根本不存在。这也解释了那个猜想的来源：字面 grep 会把
`Size(Space(3.25), Space(3.25))`（图标）和 `max(t.Space(11), rowFloorPx)`
（行的 **44u 高度下限**，`rows.go:1301`）读成「间距」。

## 三、落地

| # | 位置 | 现状 → 落成 |
|---|---|---|
| 1 | `internal/ci/rowrhythm_contract_test.go`（新，2 条测试） | ①三条行路的**第一条 `Padding` 参数**必须同值且 = `Space(2.25)`；②`internal/settingsui` 的**间距方法第一参数位**（`Padding`/`Gap`/`GapX`/`GapY`/`Margin`/`MarginX`/`MarginY`/`InsetShadow`）不许出现 `Space(2.75)` / `Space(3.25)`。逃生口 `rowRhythmSpaceAllow` **落地即空表**，过期条目同样变红（与 `litarm_contract_test.go` / `flatface_contract_test.go` 同一约定） |
| 2 | 同文件：判据按「方法名 + 参数位」，**不按字面文本** | 这是这轮唯一的设计判断：`Size(t.Space(n), t.Space(n))` 是图标、`max(t.Space(11), rowFloorPx)` 是地板，两者都不是间距；字面 grep 会把它们读成混用 |
| 3 | `docs/liquid-glass.md` §6.35（新增） | 4 行：35-1 / 35-2 / 35-4 `已落地`（其中 35-2 是 `故意不做（量过，无需改）`）+ 35-3 **`未开始`**（分组间距 8u vs 10u，先出图再决定） |

**零产品字节**：`git diff --stat` 实测只有 `docs/liquid-glass.md | 22 +`。高度链、键盘模型、
会话管理、设置写回、i18n 文案、命令接线**一个字节没动**。**V 0 / B 0 / S 0**。

## 四、门槛（全部实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS 计数                → 3207（基线 3205，+2）
--- SKIP 计数                → 115（不变）
测试函数数                   → 2360（基线 2358，+2）
GOOS=linux|windows|darwin go build ./... → 三平台全过
```

## 五、变异（3 条实跑变红 + `md5sum -c` 逐字节还原）

还原基线：`rows.go` `fb706b5cbaa30f2a057326874f2f6f3c`、
`settingsui.go` `337086d22d63ebb606de1261758e4c6e`、
`rowrhythm_contract_test.go` `301c34db40d4a4ea599dbd96a7c32188`。

| # | 注入 | 实测失败串 |
|---|---|---|
| M1 | `cardRow` 的 `Padding(t.Space(2.25), inset-1)` → `Space(2.5)` | `cardRow: the row's vertical step is Space(2.5), want Space(2.25) — the reference's own '.settings-row' step (9u / 36px)` + `the row paths disagree: cardRow has Space(2.5) but stackedRow does not` |
| M2 | `stackedRow` 的 `Padding(t.Space(2.25), inset)` → `Space(3)` | `stackedRow: the row's vertical step is Space(3), want Space(2.25)` + `the row paths disagree: cardRow has Space(2.25) but stackedRow does not` |
| M3 | `settingsui.go:2323` 的 `Padding(t.Space(2), 0)` → `Space(2.75)` | `` `Space(2.75)` / `Space(3.25)` have no role as spacing in this package (they are icon sizes here): internal/settingsui/settingsui.go:2323 (Space(2.75)) `` |

三条都先 `cp` 备份、注入、跑红、`cp` 还原、`md5sum -c` 逐字节核对（三行「成功」）。
M1 / M2 分别只红一条行路 + 「不一致」那半边，说明判据的两个方向（**同族一档** 与
**三路互相同值**）都在工作。

## 六、出图

**本轮不出图**，理由是这份账**一个像素都没动**（`git diff` 里没有任何 `internal/` 产品
文件）——与 P11-cr 同一处置：零像素轮的正确读数是「像素一个没变」，此时出 before/after
只会得到两张逐字节相同的图，不构成证据。

图留给 §6.35 的 **35-3**：那一行要求先看 General 页两张相邻分组卡之间的 8u 与材质组
自己的 10u 在 1x 屏上读不读得出（深/浅两套），**看图之后才决定改不改**。

## 七、用 `qiaomu-design`：偷了什么

按「已有方向，只要打磨」那条车道（`craft-loop.md` + `preflight.md`），**不走** Phase 2
的四方向预览（这个自动车道没人点选，且用户已经把方向给死：轻便液态玻璃）。

| 从 `craft-loop.md` 的工艺密度菜单取了第 N 项 | 落成哪个具体数值 |
|---|---|
| **第 12 项 · 材质细节**（「内边框高光、glass 折射边…」，即「材料之间必须有可量的接缝」） | 本轮把它读成「**接缝必须可量化**」：行刻度 = `t.Space(2.25)` = **9u / 36px**（三条行路同值），块的刻度 = 8 / 12 / 16 / 24u，页 = **22u**。这张「角色 → 数值」的对照表是 §6.35 的核心资产 |
| **第 6 项 · 编辑细节**（「hairline、大写字距标签」——精度类细节） | 同一个数字在三条轴上不是一件事：`Space(3.25)` 在 `rows.go:1563` 是**图标 13px**、`Space(11)` 在 `rows.go:1301` 是**高度地板 44u**，都不是间距。判据因此收在「方法名 + 参数位」——三个轴分别是 36px / 13px / 44u |
| **第 1 项 · 氛围层**（「透明度 <6%」，即「加东西必须能被量出来，不能被感觉到」） | 35-3 的门槛就是这条的推论：**8u 与 10u 只差 2 个物理像素**，所以它不能凭「看起来更透气」改，必须先出图证明这 2 像素读得出 |

**为什么这一轮不落更多项**：`craft-loop.md` §四 的优先级是「两轮制 > DNA 注入 > 工艺密度」，
而这一轮的性质是 **audit（量）**，不是 render（画）——按 `SKILL.md` 的打磨模式
（Audit / Critique / Polish / Animate / Harden / Live），本轮落在 **Audit** 上；
把 Polish 提前到量清楚之前，就是任务书明令禁止的「先改」。

### preflight 逐条结论

| 门 | 结论 |
|---|---|
| 第 0 步 · 偏好账本 | 已读 `user-preferences.md` 全部 49 条。适用本轮的：**P-1（禁斜体）** 无斜体新增；**P-5（可读性优先）** 本轮不降任何对比度；**P-14（加载/成功状态不占版面）** 未新增常驻文案。其余（封面、预览页、K3）**不适用**（本仓不是 web 页，无 Phase 2） |
| A · 反 AI 套路 | **过**：无渐变、无纯黑、无外发光、无左竖条（P-38）、无三等分卡片（`cardRow` 是单列）、无「革命性/无缝/赋能」类文案（本轮零文案） |
| A2 · 进攻三步 | DNA 注入 / 工艺密度 ≥5：**本轮只落地 3 项且写明理由**（audit 轮不强行凑 5，见上）；两轮制：**不适用**（无渲染改动） |
| B · 设计读取一致性 | **过**：一句设计读取 =「设置页，面向已经在用 floter 的人，用轻便液态玻璃语言，落在 elevation 阶梯上」；产出（一张刻度对照表 + 一条不许长第二格的断言）与它一致；**记忆点** = 「行只有一档 9u」 |
| C · 动效抽查 | **不适用**：本轮零动效改动（§5 的 `ColorStep` 120ms、按下面档 `Scale(0.986)` 一个字节没动） |
| D2 · 中文页 | **中文行高 ≥1.5：过**（`empty.go` 的 `LineHeight(1.5)` 未动）；**中西文空格：过**（本轮零文案）；**D2「正文 ≥14px」：不过 —— 这是既有挂账，不是本轮新增**：`tokens.Caption` = 9.90px / `tokens.Body` = 10.80px / `tokens.Emphasis` = 14px，全仓大量副标题用 `Caption`。抬它们会连带动**高度链**（行为契约红线，P13-bd 已实证：`tokens.Title` 把盒高 23 → 19、`TestTheTerminalPageStaysOnTheHeightLadder` 直接红），所以它一直挂在 `.pending-bugs.md` 的「需用户拍板」里，本轮**继续沿用挂账，不擅自改** |

## 八、仍待（P13-bq）

1. **§6.35 的 35-3 是新的 `未开始` 行，也是下一格**：分组间距 8u（`rows.go:81`
   `sectionGroup`）vs 材质组 10u（`rows.go:114` `sectionMaterial`）vs 页 22u
   （`rows.go:56` `pageColumn`）。**先出图**（`ui.Tester`，General 页两张相邻分组卡，
   深/浅两套），看图再决定——不许先改。
2. **需拍板（沿用）**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾（P13-bm 仍待 1）；终端弧 S1b / S1c / S2 尾；
   §6.32 的 32-4（同一张卡两个层级标题）。
3. **P13-bo 仍待 1 原样挂着**：把「列表从属注 / 这一页结论」的分界写成一条共享判据，
   要同时改 `shell/detach.go` 与 `launcher`，跨表面，**另开一轮**。
4. **抢救 stash 复核（本轮做了）**：`stash@{0}` = `33e03d1f6a`（2026-11-01 11:58）的
   tree 与 HEAD **逐字节相同**（`git rev-parse stash@{0}^{tree}` == `HEAD^{tree}` ==
   `6d442dc5310bc8bb0712d442e9d9ba9b218a24ca`，`git diff HEAD stash@{0}` 输出为空）
   ——那次抢救没有带来任何未提交工作，**无需接续**，也不会静默丢弃。
