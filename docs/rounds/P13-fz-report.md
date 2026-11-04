# P13-fz 报告：插件窗口的下沿 —— 同一族里最后两个刀口

**一句话**：这个应用里每一块**自由滚动**的面都有下沿，只有两个插件窗口（捕获输出页、便条）
没有——滚到中间时正文最后一行在窗口最后一行像素上被水平切断，墨一点没退。这一轮把这两块面
补上同一道 `glass.ScrollEdge`，并证明它**不吃布局、不盖字、空白上不显形**。

## 为什么是这一格

`docs/liquid-glass.md` §6 差距表没有 `未开始` 行，选题回通用队列；上一轮 P13-fy「仍待」
第 1 条 `140-5` 需拍板。同族的下沿已经收口五处（设置页 §6.16 / 启动器满态 §6.1 表 1-10 /
会话抽屉 §6.141 / 浮条 §6.20 / 集成卡 §6.143），剩下这两块面从没进过账——而它们不是
「量不出来」，是**看得出来**：一页长输出的最后一行被刀刃切在窗口底边。

**抢救 stash 的接续**：`stash@{2}`（`8a69c7f861`，2026-11-03 15:58）里有一条未提交的
`detachedBodyView` 下沿改动（33 行补丁，只碰 `internal/shell/detach.go`）。它从没进过
HEAD（`git apply --check -3` 仍然干净），注释引用的 §6.121 已过期、也没有任何 pin。
本轮把它接上并**扩成一对窗口**（`pinned.go` 是同一个 `.plugin-window` 表面、同一个滚动器、
同一个内边距），§ 号落到 §6.145，补判据 + 四条变异。

## 量（720×460，正文 90 行，`MaxY` 1151.7999）

| 量 | 改前 | 改后 |
| --- | --- | --- |
| 捕获输出页 · 窗口最后一行（19 档滚动里最坏） | **139**（浅）/ **134**（深），与它上方那一带同值 = 100% | **28** / **28**（20% / 21%） |
| 便条 · 窗口最后一行（19 档滚轮） | 满对比 | **0** / **0**（上方 137 / 132） |
| 渐变深度（行） | 0 | **9**（边 10 行） |
| 滚到底时：带上均值 vs 带上方的均值 | — | 浅 **246 / 246**、深 **20 / 20**（差 0 = 完全自门控） |
| 滚到底时最后一行墨的位置 | 窗口底边 | **y=428，离底 31px**（内边距 14 + 行盒余量） |
| 正文 `MaxY` | 1151.7999 | **1151.7999**（叠层不吃布局） |

`Space(2.5)` = 10.0 < `Space(detachedBodyInset)` = 14.0：这是「滚到底时边落在空白上」
（§6.19 的几何门）成立的**唯一**理由，写进常量注释与判据 3。

## 落地

| # | 位置 | 内容 |
| --- | --- | --- |
| 1 | `internal/shell/detach.go` `detachedBodyView` | 正文之后 `Absolute().Bottom(0).Left(0).Right(0).Height(Space(detachedEdgeInset)).PassThrough()` + `glass.ScrollEdge{Bottom: true, Background: tokens.Opaque}` |
| 2 | `internal/shell/pinned.go` `pinnedBody` | 同一道边、同一常数、同一条自门控算术 |
| 3 | `internal/shell/detach.go` 常量块 | 新增 `detachedEdgeInset = 2.5 // 10u`，注释写明必须 < `detachedBodyInset` |
| 4 | `internal/shell/pluginwindowedge_test.go` | 3 条判据 + 1 条 opt-in dump |
| 5 | `internal/ci/liquidglass_contract_test.go` | 分隔行按单元格读（见下「预存红」） |

判据：

- **fz-1** `TestTheOutputWindowFadesAtItsBottomEdge`：19 档滚动，最后一行 ≤ 上方那一带的 45%
  （实测 20%）；同帧量渐变深度（9 行）；滚到底时带上与带上方的均值差 ≤ ±6（实测 0）。
- **fz-2** `TestThePinnedNoteFadesAtItsBottomEdge`：同一个量，用滚轮（`tt.Wheel`）把便条
  滚 19 次，最后一行 ≤ 45%（实测 0%）。
- **fz-3** `TestTheEdgeCostsTheBodyNoLayout`：`band < inset`（10 < 14，两个数都从同一主题台阶读，
  改 `detachedEdgeInset` 会红）+ `MaxY == 1151.8`（边进布局就红）。

**预存红（本轮门槛跑出来的，不是本轮引入）**：`internal/ci` 的
`TestTheLiquidGlassContractSupersedesTheRaycastOne` 在 HEAD 上就是红的——P13-fy 的 §6.144 表
用 `| --- | --- | --- |` 这种带空格的分隔行，而检测器只认 `|---` 前缀，于是把分隔行当成
「状态 = `---`」的数据行。改成按单元格判定（全是 `-`/`:` 即分隔行），行数 510 → 509，
断言没有放宽。

## 台账 V / B / S

**V 0 / B 0 / S 0**。生产改动是**新增**一个叠层元素，没有一条既有 pin 需要改；
`internal/ci` 那处是**解析修正**（测试自己的 false positive），不是视觉基线变更。

## 变异（实跑 + 还原 + `md5sum -c` + `touch` 复跑）

`internal/shell/detach.go` `7ad4fed5827e4e2dcbed6a93883beb82` → 四条，全部变红并还原：

- **M1** `Height(t.Space(detachedEdgeInset))` → `Height(t.Space(0))`（边消失）：
  `the last row of the window reads at 139 while the rows above the edge read at 139 (100% of it)`（浅）
  / `134 … 134 (100% of it)`（深）。
- **M2** `detachedEdgeInset = 2.5` → `4`（16 > 内边距 14）：
  `the edge band is 16 px while the body's own padding is 14 … (want band < inset)`。
- **M3** `Background: tokens.Opaque` → `tokens.Control`：
  `the last row … reads at 131 … (94% of it)` + `at the end of the scroll the bottom 10px read 239
  while the rows above them read 246 (-7 levels): the edge is not self-gating`（深色 `29` 对 `20`）。
- **M4** 去掉 `.Absolute()`（边进布局）：`the body's MaxY is 1160.7999, want 1151.8`（+9 = 边高减 1）。

还原后 `md5sum -c` 逐字节通过（`成功`），`touch` 测试文件再跑一次全绿。

## 出图

`FLOTER_P13FZ_DUMP=1 go test ./internal/shell/ -run TestDumpP13FZPluginWindowEdge` →
`/tmp/p13fz/`（6 张，73–80KB）：`floter-P13-fz-output-{rest,mid}-{light,dark}.png` +
`floter-P13-fz-pinned-mid-{light,dark}.png`。

**图上看到的一处变化**：`output-mid-light.png` 里正文一直写到窗口最下一行，但最后约 9 行
的字是**渐隐**的——最底一行几乎读不出字形（spread 28 对上方 139），而 `output-rest-light.png`
里同样位置（正文还没滚）仍然是满对比的字。两张并排看，变的是「快到底时那一行怎么退场」，
不是「有没有内容」。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单逐项）

- **第 12 项「材质细节：内边框高光 / glass 折射边」**：落成「同一族的面用**同一条**边」——
  `glass.ScrollEdge` 的 85% → 0 渐变，`Background` 取这块面自己的地（`tokens.Opaque`，
  不是主题 `Background`），所以它不是一条贴上去的灰带，是这块面自己的底在透上来。
- **第 5 项「多层染色阴影（拒绝单层灰）」的反向用法**：这道边**不许**用阴影模拟。
  插件窗口是 E2（浮）但窗口层不做 elevation（它没有父面），所以下沿只用「面淡出去」，
  不叠 `ShadowAmbient`/`ShadowContact`；一个 10px 的投影带在深色档会读成一条脏边。
- **第 6 项「编辑细节：hairline」**：10.0 这个高度不是拍的——它由两个既有刻度夹出来
  （`Space(2.5)` < `Space(3.5)` 内边距），并且实测渐变恰好铺 9 行。
- **两轮制**：第一轮按 stash 给的 `Space(1.5)`（5.4）写，量出「最后一行 28 对上方 139」——
  退是退了，但 5.4 只够淡 4 行，读起来像一条糊边而不是渐变；第二轮换成 `Space(2.5)`（10），
  渐变 9 行，同时仍严格小于内边距 14（自门控不破）。

## preflight 抽查

- **A（反 AI 套路）**：没有渐变白边冒充高光、没有毛玻璃贴图、没有玻璃叠玻璃；边只用一种材质，
  且只在**滚动面**的下沿出现。**过**。
- **B（设计读取一致性）**：读取为「自绘桌面应用的插件窗口 → 长输出 → 轻便液态玻璃
  （面差 + 受光边，无模糊）」。层级仍由三段面色说（窗口地 / 正文墨 / 边），这道边是
  **退场语法**而不是新层级——与 §2 的 elevation 阶梯一致（窗口层不做 elevation）。**过**。
- **C（动效抽查）**：不涉及动效（静止帧 + 滚动位移，`ReduceMotion` 无关）。**N/A**。
- **D2（中文页）**：本轮判据是像素行数，不涉及中文排版；正文用 fixture 的 ASCII 长行，
  zh 侧同一结构（`i18n` 只影响状态行，不影响正文）。**N/A（不适用，非遗漏）**。

## 仍待

1. **下一格（首选）**：`140-5`（同一条顶边在深浅下读成相反的东西）**仍需拍板**。
2. 沿用 §6.137 的 **137-4** 拍板证据（About 页只读事实行：路径 92 字时标签盒 0×0）。
3. 沿用 §6.142 的「故意不做」（`sectionHeading` 的 1.8px）。
4. 本轮新增一条**量出来的事实**（不是缺陷，记下来防误判）：软边的 alpha 斜坡让
   「更高的边」在每一行**更淡**，所以像素剖面无法区分「边更高」与「斜坡更缓」——
   `band < inset` 这条只能靠**同一常数**的两个刻度来守（判据 3 就是这么写的）。
