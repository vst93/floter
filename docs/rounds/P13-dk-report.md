# P13-dk 报告：更新行的状态字，四种答案是同一档灰

**方向一句话**：About 页更新行的那条状态字是这一页唯一会变的一句话，此前四种答案共用一档
副标题墨（浅 101 / 深 94）；这一轮把它拆成三档 —— 没事（不动）/ 有个版本可取（122 / 115）/
坏了（162 / 131，暖色相）—— 而版本行与状态行的盒子一个像素没动。

**这轮为什么让它更有空间感**：空间感不是靠再加一层影，而是靠同一个 z 平面上「谁在前、谁在后」。
这一行此前四种状态齐平（全 101/94），版面把「一个字重」当成了层级；让墨去回答状态之后，
无事的那两档退到最远、有事的站出来，行没有变高、没有加面、没有多一个控件，读起来却第一次
有了前后 —— 这正是 §1 说的「分层靠面 + 线 + 字阶」里最后那一项。

选题：任务书「下一轮 = 细节打磨；下一格从状态反馈的另一半（禁用 / 只读面）挑，先量再改」
+ P13-dj「仍待」第 1 条（§6.10 的 13-5 那三个只读臂没人量过）。规格
`docs/rounds/P13-dk-spec.md`，合约新增 **§6.80**。表面 = **设置页 → About → 更新卡**。

## 先量（`inkPixel` 的 luma 跨度，深浅两套；`updatePage` 760×1400）

改前（HEAD `32bd9cea`，新 pin 首跑，红）：

| 状态 | 状态字 | 浅 | 深 |
|---|---|---|---|
| `up-to-date` | Up to date | **101** | **94** |
| `disabled` | This build has no update feed | **101** | **94** |
| `available` | Latest version: v1.4.0 | **101** | **94** |
| `failed` | Update failed, retry? | **101** | **94** |

同一行的标签 `Current version: v1.2.3` 是 202 / 198（emphasis / `TextStrong`）：
这一行上最响的字是**不变的**版本号，最安静的是**会变的**答案。四种不同的意思，像素上一模一样。

## 落地项（80-1…80-7，见合约 §6.80）

| # | 位置 | 改法 |
|---|---|---|
| 80-1 | `internal/settingsui/rows.go` | 新增 `rowInk` 三档（`rowInkQuiet` 零值 = `t.TextMuted` / `rowInkFact` = `tokens.TextSecondary` / `rowInkWarning` = `t.Warning`）与 `rowLine.Ink`；**只作用于普通单行**，`Segments` 那条仍走各自的 `rowSegmentRole` |
| 80-2 | `rows.go` 新增 `rowState` | `rowAligned` + 一条带 `Ink` 的 `rowLine`；标签列 / 44px 地板 / 分隔线 / 1px 行距 / 控制槽全沿用 `row` |
| 80-3 | `internal/settingsui/settingsui.go:4054` `updateRow` | `Failed` 打 `rowInkWarning`、`Available` 打 `rowInkFact`，`Disabled` 与 `UpToDate` 保持零值；调用点 `a.row` → `a.rowState` |
| 80-4 | `internal/settingsui/draftrowink_test.go:44` | P13-di 留在这个包里的测试局部类型也叫 `rowInk`，与新生产类型撞名 → 改名 `rowScan`（**只改名字，四条断言逐字不动**） |
| 80-5 | `internal/settingsui/updatestateink_test.go`（新） | 一条像素 pin（四状态 × 深浅）+ opt-in 出图 |

改后（同一夹具、同一读法）：

| 状态 | 状态字 | 浅 | 深 | 像素（浅 / 深） |
|---|---|---|---|---|
| up-to-date | Up to date | 101 | 94 | `[128 128 135]` / `[133 133 141]` |
| disabled | This build has no update feed | 101 | 94 | 同 |
| available | Latest version: v1.4.0 | **122** | **115** | `[107 107 112]` / `[154 154 160]` |
| failed | Update failed, retry? | **162** | **131** | `[156 31 24]` / `[216 153 142]` |

盒子四种状态逐状态相同：版本行 18px / x=200.4 / y=366.4、状态行 17px / x=200.4 / y=385.4
（差 ≤0.5px），版本行的墨四种状态都是 202 / 198。

**深色档为什么还要钉色相**：`warning` 在浅色是 `#9C1F18`、在深色是 `rgba(255,178,164,0.82)`，
同一档墨量差 31 级（162 对 131）；深色档里「失败」131 与「有新版」115 只差 16 级 ——
单靠明度读不出来，所以 pin 同时钉 `R−B ≥ 30`。这是 §6.26 那句「意思允许有色相」的可验证版。

## 台账：V 0 / B 0 / S 1

- **B 0**：`TestAboutUpdateRowFollowsTheOldStateMachine`（状态机四条）**零改动**、仍绿 ——
  它量的是哪句话在不在，不是哪档墨。
- **V 0**：没有任何既有 pin 断言过这条状态字的墨（这也是这一轮存在的原因）。
- **S 1**：`draftrowink_test.go` 的局部类型 `rowInk` → `rowScan`（生产代码新增了同名类型），
  该文件四条断言逐字未动。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./... / GOOS=windows / GOOS=darwin              → 净
go test -count=1 ./...                                 → 全绿（exit 0）
--- PASS 计数                                          → 3315（基线 3314）
--- SKIP 计数                                          → 151（基线 150，+1 = 本轮 opt-in dump）
--- FAIL 计数                                          → 0
测试函数                                               → 2474（基线 2472）
GOOS=linux/windows/darwin go build ./...               → BUILD-OK
```

## 变异（2 条实跑变红并还原，`md5sum` 逐字节核对 + `touch` 后复跑 `ok`）

- **M1**：删掉 `Failed` 那一支的 `ink = rowInkWarning`。原始报错（4 条红，深浅各两条）：
  `light: the failed state's ink is [128 128 135] — a failure is a meaning and a meaning may
  take a hue (§6.26), but this one is neutral` 与
  `light: the failed state resolves 101 against the available state's 122 (-21) — the two are
  different answers and may not share a rung (§6.80)`（深色同两条）。
- **M2**：删掉 `Available` 那一支的 `ink = rowInkFact`。原始报错（2 条红）：
  `light: the available state resolves 101 against the quiet rung's 101 (+0) — a version to
  fetch is a fact, one rung above a subtitle (§6.80)`（深色同一条）。
- 还原：`cp` 备份回去，`md5sum` = `9d88d4cbf476225b7803490db82b95cf`（改前 / 改后同一串），
  `touch` 后复跑 `ok floter/internal/settingsui 0.615s`。

## 出图（8 张 / 前后两套）

`FLOTER_P13DK_DUMP=1` → `/tmp/floter-P13-dk-update-<light|dark>-<state>-<before|after>.png`
（760×900，四状态 × 深浅 × 前后 = 16 张里本轮落了 8 before + 8 after）。

**这张图上看到的一处变化**：同一张 `light failed` 图，状态字那一条带（y 383–403、x 198–470）
在 before 里**暖色像素 0 枚**、after 里 **342 枚**（深色 0 → 250）——那条「Update failed, retry?」
从一行灰字变成了这一页唯一的暖色。而 `available` 那张图上，同一带的最暗像素从 128 走到 107
（副标题档 → 事实档），位置、行高、缩进都没动。

## preflight（本轮逐条过）

- **A 反 AI 套路**：无渐变 / 无霓虹 / 无斜体（P-1）/ 无 emoji / 无假数据（用的是 `aboutFixture`
  的真实形状 `1.2.3` / `v1.4.0`）/ 文案没有空词，新增的只有颜色。**过**。
- **B 设计读取一致性**：读取 = 自绘 Go 桌面工具（mygo canvas）、面向调设置的人、用「近黑底 +
  一条受光边」的轻便玻璃语言、倾向本仓 §1 的三档墨阶梯。这一轮的三档墨都是从**本仓自己的**
  token 上取的（`TextMuted` / `TextSecondary` / `Warning`），没有引入新颜色、没有新字号。
  **过**。
- **C 动效抽查**：这一行没有动效（状态切换是纯墨替换，四状态盒子逐像素相同，所以不存在
  「换状态时跳一下」）。`ReduceMotion` 与本轮无关。**过**。
- **D2 中文页**：中文文案未改（`已是最新版本` / `更新失败，重试？` / `此构建没有更新源` 逐字不动），
  仍走系统中文栈。**中文页的 D2 大字要求仍不过**：`tokens.Caption` 9.9px / 正文 10.8px 对
  preflight D2「正文 ≥ 14px」——这是**第 36 份证据**，属「需拍板」，本轮不擅自改字阶。
- **A2.1 执行者路由**：默认由当前 pi 直接执行，未调用 K3（P-46）。**过**。

## 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落数）

这一轮只改**一行字**，所以菜单里与「面 / 材质 / 氛围」有关的项本来就不适用；取到的是这三项：

1. **第 6 项「编辑细节（hairline / 大写字距标签 / 页边注）」** → 落成这一行的位置账：
   状态字与标签同列（x=200.4）、差 `rowLineGap` 的 **1px**、行盒 **17px**、总高 **36px**
   （18 + 1 + 17）—— 一条页边注该有的位置，一个都没挪。
2. **第 10 项「收尾彩蛋 / 只在特定时刻出现的细节」** → 落成失败档的暖色相：`[156 31 24]`
   （浅）/ `[216 153 142]`（深）**只在 `failed` 出现**，其余三态仍是同一档灰（101 / 101 / 94 / 94）。
   这不是装饰，是「有事发生」那一下才亮的一笔。
3. **第 5 项「多层染色阴影」的替代物：同一平面上的墨梯队**（菜单里没有这一项，但它是本仓
   §1 的等价机制）→ 落成 **101 → 122 → 162**（浅）/ **94 → 115 → 131**（深）三段，相邻差
   21 / 40 与 21 / 16 级，全部来自既有 token，没有一个新数值。

不适用并写明理由：第 1 项氛围层（这是桌面自绘面板，纹理上限 3%，且本轮不碰面）、第 2 项
`::selection`（mygo 自绘，选中态由框架管）、第 3 项品牌 focus 环（行内两个按钮已有，不在本轮）、
第 4 项签名交互（这一行的「交互」就是它不变：四状态盒子逐像素相同）、第 7/8/11 项（展示排版 /
流体字号 / 图形资产，一个文本行没有这些）、第 9 项滚动条、第 12 项材质细节（字没有面）。

**两轮制**：第一轮 = 改墨（三档）+ 出 before/after 图与逐状态像素账 → 看图时发现深色档
「失败」131 与「有新版」115 只差 16 级（浅色档同一条是 40 级），也就是**深色档单靠明度
读不出「坏了」** → 第二轮只修这一处：pin 的判据从「明度差 ≥12」补成「明度差 + 色相
`R−B ≥ 30`」（§6.80 的 80-6）。没有推倒重做，也没有加任何新墨。

## 仍待（P13-dk）

1. **下一格（细节打磨 · 只读面剩下的两只）**：`PermissionBoundary`（`settingsui.go:5552`，一句
   讲边界的散文写在副标题槽）与工具编辑器的 `LocalToolId`（§6.11 的 14-7 已换墨）**盒子**
   仍没人量过；先量再改。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口（§6.75/§6.76/§6.77），
   仍未动，在队里。
3. **需拍板（沿用 + 第 36 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨（**与本轮不同族**：那是抽屉的整条面，
   本轮只给一句话换档）/ 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第三十八次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
