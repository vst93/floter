# P13-bq · 间距节奏的第一张账：设置卡里四个纵向刻度混用（**先量，后不改**）

## 意图（一句话）

同一个设置卡里，行的**纵向内边距**读的是 `t.Space(2.25)` = **9u(36px)**，同一张卡里
`below` 块读 `t.Space(3)` = 8u，分组之间读 8u，而族里还有 `Space(2.75)`=11u 与
`Space(3.25)`=13u 的孤立用法——**这一轮只把账量出来并钉住，不改任何数值**。

## 为什么现在做

`docs/liquid-glass.md` §6 已无 `未开始` 行；`.pending-bugs.md` 里 P13-bo 仍待 3 ②、
P13-bp 仍待 3 都把「通用队列 · 间距节奏」列为下一格，并且**明写「先量设置卡片里
8/9/11/13u 的混用（不许先改）」**。本轮的产出是**账**，不是墨。

## 本轮侦察的结论（写下来，免得下一轮重做）

### 已排除的两个候选（附证据）

1. **P13-bo 仍待 1「把『列表从属注 / 这一页结论』的分界写成一条共享判据」——不取**。
   它要同时改 `internal/shell/detach.go` 的 `detachedListNote`（§6.24 24-5）与
   `internal/launcher/launcher.go` 的 `item.status` 臂（§6.33），**跨两个表面**，
   违反「一个表面一轮」的纪律。留作另开一轮，且必须先决定「谁是权威判据」。
2. **P13-bn 仍待 1 / §6.32 的 32-4「同一张卡上两个层级标题」——不取**。它是
   **需拍板**：审计块 `settingsui.go:1293` 写 `TextStrong` / 650 / `FontSize-1`，
   审批记录块 `settingsui.go:2476,2481` 写 `TextSecondary` / 620 / `tokens.Caption`，
   两处都是「这一层的名字」，统一要先定权威写法（审计块无参考 CSS，审批记录块来自
   `floter-main/src/styles/extensions.css:1557-1561`）。按红线不许擅自改。

### 本轮量到的实况（`internal/settingsui`，非测试源码）

`t.Space(n)` 在本仓是 `4n` 像素。设置卡的**行**由 `rows.go` 的 `cardRow` /
`stackedRow` / `actionRow` 三条路画，它们的纵向内边距**同值**：

| 位置 | 源码 | 值 |
|---|---|---|
| `cardRow` 内容列 | `rows.go:1359` `Padding(t.Space(2.25), inset-1)` | 9u / 36px |
| `stackedRow` 内容列 | `rows.go:1463` `Gap(t.Space(1)).Padding(t.Space(2.25), inset)` | 9u / 36px |
| `actionRow` 内容列 | `rows.go:1517` `Padding(t.Space(2.25), inset)` | 9u / 36px |

**三条路一致，这半边不是缺陷**——它是参考的 `.settings-row` 自己的 9px 一档
（`docs/raycast-design.md` §4 的既落地条目）。

混用出现在**这三条路之外**（同一次 grep 的全量读数，`internal/settingsui/*.go` 非测试）：

| 值 | px | 出现次数（纵向或四边） | 位置样本 |
|---|---|---|---|
| `Space(0.5)` | 2 | 3 | `uninstall.go:484`、`settingsui.go:1884`、`healthfaces.go:51` |
| `Space(1)` | 4 | — | `settingsui.go:4624`、`config.go` 的若干面板 |
| `Space(1.5)` | 6 | 4 | `uninstall.go:187`、`rows.go:457`、`healthfaces.go:173` |
| `Space(2)` | 8 | 5 | `savealert.go:102`、`healthfaces.go:246`、`settingsui.go:2323` |
| `Space(2.25)` | **9** | **3（一族）** | 上面那三条行路 |
| `Space(2.5)` | 10 | 2 | `settingsui.go:3181`、`settingsui.go:6078` |
| `Space(3)` | 12 | 4 | `uninstall.go:436`、`rows.go:1517` 外的块、`savealert.go:179` |
| `Space(4)` | 16 | 3 | `uninstall.go:192`、`settingsui.go:1507`、`pluginsheet.go:72` |
| `Space(4.25)` | 17 | 1 | `uninstall.go:160` |
| `Space(6)` | 24 | 1 | `empty.go:48` |

**读数只说一件事**：9u 那一档是**行**的刻度，其余全是**块**的刻度；族内没有任何
两处「同一个角色两个值」。所以「8/9/11/13 混用」这个猜想**在设置卡的纵向 padding 上
不成立**——9u 是唯一被行共享的那一格。

**`Space(2.75)`=11u 与 `Space(3.25)`=13u 在本包一次都没出现**（全仓只有
`rows.go:1563` 与 `shell/notices.go:372`，两处都是**图标尺寸** `Size(3.25, 3.25)`，
不是间距）。它们属于启动器/外壳，不属于设置卡。

### 还剩的真问题（**下一轮取它，仍须先量再改**）

行与**分组之间**的节奏：`sectionGroup` 用 `a.cardTheme(c).Space(2)` = **8u**
（`rows.go:81`），而 `cardMaterial` 那一支用 `t.Space(2.5)` = **10u**
（`rows.go:114`），`pageColumn` 用 `c.Theme().Space(5.5)` = **22u**（`rows.go:56`）。
8 / 10 / 22 是三个**不同的角色**（组内间隔 / 材质组自己 / 页与页），但 8 与 10
之间只差 2u，在 1x 屏上是 2 个物理像素——**这需要先出图看它读不读得出**
（`ui.Tester`，General 页两张相邻分组卡的间距，深/浅两套），不许先改。

## 落地项（本轮）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/ci/rowrhythm_contract_test.go`（新增） | 这笔账只存在于人的眼睛里 | 一个**账本 pin**：把上面那张「行刻度」表钉成三条断言——`cardRow` / `stackedRow` / `actionRow` 的纵向内边距必须**同值**，且该值必须是 `Space(2.25)`；任一被改成别的数即红 |
| 2 | 同文件 | 混用的其余刻度无人守 | 第二条断言：`internal/settingsui` 里出现 `Space(2.75)` 或 `Space(3.25)` 作**间距**即红（逃生口 `rowRhythmAllow`，落地即空表，条目数写死） |
| 3 | `docs/liquid-glass.md` §6.35（新增小节） | — | 把这张账写进合约：行的纵向刻度 = 9u 一族，块 = 8/12/16，页 = 22；并写明结论「8/9/11/13 混用」在纵向 padding 上**不成立** |

## 行为契约零改动红线

本轮**一个产品字节都不改**（只加 CI 账 + 合约一节）。高度链、键盘模型、会话管理、
设置写回、i18n、命令接线全部零接触。V 0 / B 0 / S 0。

## 门槛

`gofmt -l cmd internal` 空；`go vet ./...` 净；`GOOS=windows|darwin go vet ./...` 净；
`go test -count=1 ./...` 全绿；`grep -c -- "--- PASS"` 只许涨；三平台 `go build`。
基线（P13-bp 实测）：PASS **3205** / FAIL 0 / SKIP **115** / 测试函数 **2358**。

## mutation 计划（≥2）

1. `cardRow` 的 `Space(2.25)` 改成 `Space(2.5)`（行刻度漂一档）
   → 账本 pin 第一条红（点名 `rows.go:1359`）。
2. `stackedRow` 的 `Space(2.25)` 改成 `Space(3)`（同一族两个值）
   → 账本 pin 第一条的「同值」半边红。
3. 在 `settingsui.go` 任意一处加 `Padding(t.Space(2.75))`
   → 账本 pin 第二条红。
