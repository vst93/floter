# P13-dp 报告 · 夹具的尺子：`internal/terminalui` 这一包量的一直是框架库存的字

规格 `docs/rounds/P13-dp-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.85**
选题：`§6.76` 拆轮表里 `dg-t` 那一格（`P13-df`/`P13-dg` 已量、`P13-dh` 已把缝收口）。

**这轮为什么让它更轻/更有空间感**：本轮不动一个像素（产品代码零改动）。它修的是**尺子**——
这个包里 6 条几何 pin 此前量的是框架库存主题（`Spacing 4.0` / `FontSize 14.00`），
而用户看到的画面是 `3.6` / `10.80`。尺子错了，后面每一次「间距节奏 / 层级 / 状态反馈」
的打磨都会先改到**没人渲染的那张画**，然后在真机上重新错一遍。空间感的账要能记账，
先得让尺子和画面是同一把。

## 1. 先量：8 条红，逐条归类

把 15 个测试文件里 **51 处** `render(t,` 全换成 `renderThemed(t,`（P13-dh 加的夹具，
装 `tokensOf(a,c).Theme`）后实跑，红 8 个函数，与 `§6.76` 的「≥8 条」对上：

```
textheight_test.go:66: zh light 640x360: no run of height 20.0 was painted (heights [15 17 25.2 27])
textheight_test.go:121: light: "floter --version" (a line of the palette preview) is 15.0 tall, not 20.0
textheight_test.go:121: light: "Line height" (a row's label) is 17.0 tall, not 22.0
textheight_test.go:121: light: "Close terminal settings" (the drawer's close button) is 25.2 tall, not 28.0
textheight_test.go:121: light: "Regular" (a segmented control) is 27.0 tall, not 30.0
pageheight_test.go:70:  empty en light 640x360: no run of height 22.0/23.0/28.0 was painted (heights [15 17 18 25.2])
drawerscroll_test.go:61: en 640x360: the drawer reports no scroll range (MaxY=0.0)
tokensmemo_test.go:37:   the first frame built 2 palettes, want 1
```

比例逐条对上 `§6.76` 的两个数：**行盒 ×0.77**（22→17、23→18、20→15）、
**空格 ×0.90**（28→25.2 = 7×3.6、30→27 = 7.5×3.6）。

### 1.1 抽屉的阶梯从五档塌成四档（本轮最值钱的一条）

库存尺上抽屉是 `17 / 20 / 22 / 28 / 30` 五档，设计尺上是 `15 / 17 / 25.2 / 27` 四档。
塌掉的那一档不是丢了一行，是**两条 pin 的判别力**：

- **读数**（`tokens.Body` + mono）与**行标签**（`t.FontSize`）在设计尺上**都是 10.80**，
  于是都是 17。库存尺给的 17 vs 22 是那把尺子（正文 14.00）自己造的差，不是设计。
  这两者在画面上靠 **mono 字栈**分开，不靠字号——pin 照实写 17/17，
  并在注释里点名「它们之间不再有几何判别力」。
- **预览行**（`t.FontSize - 1` = 9.8pt）在设计尺上落到 15，也就是 `tokens.Caption` 那一档。

### 1.2 两个 S：前提消失了，换窗口而不是放宽

640×360 / 720×360 是 `§P11-au` 的最小窗口。设计尺下行短 10%，**条带在最小窗口上装得下了**
（实测 `MaxY=0.0`，条带 309.2u）——「条带比视口高」这个前提不复存在，pin 无从说起。
探针实测：

| 窗口 | `MaxY` | 最后一行（`Safe paste`）在 Y=0 时 | 滚到末 |
|---|---|---|---|
| 640×360 | **0.0** | 整行（H=17） | — |
| 720×360 | **0.0** | 整行 | — |
| 640×280 | 54.4 | 被裁（H=0） | H=17 |
| 720×240 | 36.0 | 被裁（H=0） | H=17 |

所以窗口改成 `640×280` / `720×240`（两个宽度都留：720 宽时调色板预览挪到行旁，
260 就装得下，所以两条宽度不能共用一个高度）。这是 **S**：换窗口让被测状态继续存在，
不是把「最后一行必须整行」放宽成「非空」。

### 1.3 两条 pin 留在 `render` 上，并写明理由

`TestOnePalettePerTerminalFrame` / `TestASwappedStoreIsANewPalette` 量的是
**一帧建几次调色板**，不是几何。而 `renderThemed` 自己也要读一次 `tokensOf` 来装主题：
第一读看到的是库存主题的 `c.Theme().Dark`，app 那一读看到的是刚装上的设计主题的
`Dark`，两者不同就是两个 memo key —— 换 store 那一帧因此多出一笔（实测 2）。

顺带修掉夹具里的一个真毛病：`renderThemed` 原先每次渲染都 `SetPreferences{ReduceMotion}`
一次，而 `SetPreferences` 自己 settle 一帧，于是「第一帧」之前已经画过一帧
（`the first frame built 2 palettes, want 1`）。删掉那一行之后这两条 pin 回绿，
不需要 `Reset`、不需要改产品。

## 2. 落地（全部是 `_test.go`）

| 文件 | 改动 |
|---|---|
| `view_test.go` | 删 `SetPreferences`；新增 `const designSpace = 3.6`；`renderThemed` 的注释改成「本包已全线收口，唯一例外是数构建次数的那两条」 |
| `textheight_test.go` | 阶梯 `[15 17 25.2 27]`；五个产生者 17 / 15 / 17 / 25.2 / 27；头注重写（含「读数与标签同档」的判据） |
| `pageheight_test.go` | 阶梯 `[15 17 18 25.2]`，must `{17, 25.2}`；五个产生者同上 |
| `drawerscroll_test.go` | 窗口 `640×280` / `720×240`；`wholeRow 20 → 17`；`TestReopening...` 同窗口 |
| `tokensmemo_test.go` | 两臂回 `render`，注释写明原因 |

**行为契约零改动**：`git diff --stat` 里没有一个非测试的生产文件；高度链、键盘模型、
会话管理、设置写回、i18n、命令接线一字未动。

## 3. 台账

**V = 4 条**（测试函数）：`TestTheDrawerStaysOnTheHeightLadder`、`TestEachDrawerRungHasItsOwnProducer`、
`TestTheTerminalPageStaysOnTheHeightLadder`、`TestEachTerminalPageRungHasItsOwnProducer`
（按断言行共 12 处）；逐条注释写
`P13-dp 取景收口：旧值 → 新值`）。
**S = 2 条**（两个抽屉滚动 pin，换窗口，逐条写实测 `MaxY`）。
**B = 0 条**（没有任何一条红是产品回归：三条变异证明产品改坏时这些 pin 仍然会红，见 §4）。

## 4. mutation（实跑，全部还原并逐字节核对）

| # | 注入 | 实测红串 |
|---|---|---|
| M1 | `terminalui.go:898` `t.FontSize + 1` → `t.FontSize` | `pageheight_test.go:133: light: "No terminal session yet" (the empty state's title) is 17.0 tall, not 18.0 — the rung moved, or two runs swapped their heights`（zh dark 同） |
| M2 | `terminalui.go:1467` `barButton` `Space(7)` → `Space(7.5)` | `light: "New command" (a bar button) is 26.1 tall, not 25.2`（dark / resident 同） |
| M3 | 夹具回退：`textheight_test.go` 的 `renderThemed` → `render` | `textheight_test.go:74: en light 900x600: height 20.0 is not on the ladder [15 17 25.2 27]` |

还原：M1/M2 用 `cp` 回原件，`md5sum -c /tmp/m.src` → **成功**；M3 回 `cp /tmp/th.bak`，
随后整包 `go test` **ok**。每条都在还原后 `touch internal/terminalui/*_test.go` 复跑，
避免测试缓存复用。

## 5. 门槛（实测）

```
gofmt -l cmd internal                → 空
go vet ./...                         → 净
GOOS=windows go vet ./...            → 净
GOOS=darwin  go vet ./...            → 净
go test -count=1 ./...               → 全绿（FAIL 包 0）
go test -v ./... | grep -c -- "--- PASS" → 3322（基线 3322）
GOOS=linux/windows/darwin go build ./... → 过
测试函数 → 2485（基线 2485；本轮只改既有 pin，不新增函数、不新增 SKIP）
```

## 6. 出图

`FLOTER_PNG=1 go test -run 'TestDumpP11AUDrawerScrolledPNGs|TestDumpP13' ./internal/terminalui/`：

- `/tmp/floter-P11-au-drawer-end-light.png`（29 073 B）、`…-dark.png`（28 677 B）——
  设计尺 640×360 上抽屉滚到末的整页。

这两张图与改动前**逐像素相同**（产品代码零改动），这正是本轮的证据：只搬了尺子。
图上看到的一处：抽屉滚到末之后 `Safe paste` 那一行整行落在视口里（H=17），
上面一屏是调色板预览与滑轴读数两行——设计尺下它们分别是 15 与 17，**不是库存尺的 20 与 22**。

## 7. skill 复核（`qiaomu-design`）

读：`SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`（只读本轮用得上
的四份，没通读全库）。走「已有方向，只要打磨」那条车道，不出 HTML 预览、不停下来等点选。

**偷了什么**（逐条）：

1. `craft-loop.md` 的「工艺密度菜单」第 3 项（**尺子/单位先于像素**）：落成 `const designSpace = 3.6`
   —— 本包所有 `t.Space(n)` 撑出的盒子写成 `n * designSpace`，于是 25.2 / 27 是**算出来**的
   而不是抄来的读数。
2. 同菜单第 6 项（**一个判据只许有一个来源**）：落成「阶梯 = 产生者之和」这条自检——
   抽屉 `[15 17 25.2 27]` 与五个产生者逐条对齐，页 `[15 17 18 25.2]` 同。
3. 同菜单第 9 项（**换窗口不是换断言**）：落成 §1.2 的两个 S，窗口 360 → 280 / 240，
   断言强度一字不变。

**preflight**：A（反 AI 套路）过——本轮无新增文案、无占位符、无「赋能/无缝」类词；
B（设计读取一致性）过——所有几何读数现在与生产同一把尺子（这正是本轮的全部内容）；
C（动效抽查）不适用——产品无改动，无动效可抽查（`renderThemed` 里删掉的 `ReduceMotion`
正是为了不再伪造一个动效前提）；D2（中文页）**仍未过**：`tokens.Caption` 9.9px / 正文 10.8px
与「正文 ≥ 14px」冲突，第 41 份证据，仍挂在「需拍板」。

## 8. 仍待（P13-dp）

1. **下一格（`§6.76` 拆轮表的下一格）：`P13-dg-l` = `internal/launcher` 的 21 条。**
   同一条手续，但多一件：`Geometry{Font, TitleLine, SubtitleLine}` 必须喂**实测行盒**，
   否则帧高按 4.0、画面按 3.6 排，两条腿不是一套（`§6.76` 已实测编号族 9 → 10 条）。
2. `P13-dg-s` = `internal/settingsui` 四十余条，`renderThemed` 已有，只差整体收口。
3. **需拍板（沿用 + 第 41 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十三次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
