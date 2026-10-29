# P11-cb 报告 · 终端页本身进「字盒高度」尺子

规格：`docs/rounds/P11-cb-spec.md`。**产品代码零改动**——`git diff HEAD -- internal/` 除新增
测试文件外为空；本轮只加尺子与文档。

## 1. 这一轮做了什么

选题：用户实测反馈 F1–F4 已收完（`.pending-bugs.md` 顶部三条都标了落地轮号）→ 合约
`docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→ P11-ca「仍待」第 1 条是 13 条拍板项
（不能擅自动），落到第 2 条：**终端页的标题栏 / 浮条那两档还没进高度尺子**。

P11-bz/P11-ca 已经给启动器（六档）、设置页（八档）、终端外观抽屉（五档）各钉了一份
「字盒 / 行盒高度」阶梯。终端页本身是第四个表面，档位来自**另一批产生者**，所以这一份
是实测出来的，不是抄的。

## 2. 实测的阶梯（本轮 HEAD 上量的）

`frameaudit.Heights`（叶子 run 的去重高度，`MinHeight=10` 裁碎片）+ `tt.Find(文字)`。

| 档 | 产生者 | 哪段字 |
|---|---|---|
| **15** | `terminalui.go:964` `tokens.Caption` | 空态键帽的图例（`↩` / `Ctrl+W`） |
| **22** | `terminalui.go:1485` `t.FontSize` | 浮条的身份行（`Terminal` / `Terminal · Exited · code 3`） |
| **22** | `terminalui.go:966` | 空态两行的标签（`New blank session` / `Back to search`） |
| **22** | `terminalui.go:1262`、`:1264`、`:1336` | 常驻便条两句 / 失败便条 |
| **23** | `terminalui.go:861` `t.FontSize + 1` | 空态的标题（`No terminal session yet`）——全页唯一一处 |
| **28** | `terminalui.go:1366` `Size(t.Space(7), t.Space(7))` | 浮条上的按钮（28u 方钮，文字就是按钮标签） |

- 四臂：`empty` / `live` / `resident`（`ResidentAt(0,3)`）/ `feedback`（`ShowFeedback`）。
- **64 帧**（4 臂 × en/zh × 深/浅 × 640×360 / 720×360 / 720×560 / 1100×700）**逐帧同一组**：
  空态是全集 `{15,22,23,28}`，另外三臂是子集 `{22,28}`（没有键帽，也没有空态标题）。
- 两条新 pin（`internal/terminalui/pageheight_test.go`）：
  - `TestTheTerminalPageStaysOnTheHeightLadder`：每帧 `Ladder` + `Missing`（`tol=0.5`）；
  - `TestEachTerminalPageRungHasItsOwnProducer`：每档钉到它自己的那段字（empty-en-light /
    empty-zh-dark / resident-en-light 三臂）。**只钉集合会漏掉「两档互换」**，所以两条都要。
- 既有测试**一行没改**。

## 3. 这把尺子看不见的那一半（重要，别当已覆盖）

P11-ca「仍待」第 2 条要的是两段：浮条 22/28（本轮量到）与 **PTY 自己画的行 20**（本轮
**实测证明量不到**）：

- `live` 臂的阶梯只有 `{22,28}`；画布那条 run 是 `Terminal input`，盒子 `w×h` = 整块画布
  （900×600），是**祖先**，被 `Heights` 的叶子规则排除；
- 插件按单元格画出来的字**一条都没进** `tt.Texts()`（live 臂 `Texts()` 里只有画布、浮条
  身份行、浮条两个按钮）。

所以「终端里的字变高 / 被裁 / 行距失控」这把尺子**量不到**，别以为加了这轮就守住了。
另外 `TerminalLineHeight` 跟用户设置走，就算看得见也不该钉成常量。这两句同时写进
`.pending-bugs.md` 的「仍待」。

## 4. mutation（4 条实跑，3 红 1 绿）

注入 → 跑 → 抄原始报错 → `cp` 还原 → `md5sum -c` 逐字节核对（每条都「成功」）→ `touch`
测试文件再跑（`-count=1`，不吃缓存）。

| # | 注入 | 结果 |
|---|---|---|
| M1 | `:861` `t.FontSize + 1` → `t.FontSize` | **红**：`pageheight_test.go:70: empty en light 640x360: no run of height 23.0 was painted (heights [15 22 28])` |
| M2 | `:964` `tokens.Caption` → `tokens.Body`（15→17） | **红**：`height 17.0 is not on the ladder [15 22 23 28] (±0.50)` + `no run of height 15.0 was painted (heights [17 22 23 28])` |
| M3 | `:1366` `Space(7)` → `Space(8)` | **红**：`height 30.0 is not on the ladder [15 22 23 28] (±0.50)` + `no run of height 28.0 was painted (heights [15 22 23 30])`——**实测是 30，不是我预猜的 32**：按钮是 `Shrink(0)` 的 32u，但那一行只有 28u，盒子被行夹到 30。报告按实测写。 |
| M4（预期绿，也是账） | `:1407` `barHeight` `Space(7)` → `Space(9)` | **绿**：条的**高度**不是任何一段字的高度，整帧阶梯一个字节不动。所以「浮条变高」这把尺子看不见——它不是这条 pin 的守卫。 |

## 5. 门槛（实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows go vet ./...        → 净
GOOS=darwin  go vet ./...        → 净
go test -count=1 ./...           → 全绿，0 FAIL
PASS 计数                        → 2675 → 2744（+69 = 2 条函数 + 67 子测试：64 ladder + 3 producer）
SKIP                             → 54（不变）
测试函数数                        → 2041 → 2043
三平台 build                      → BUILD_OK
```

基线是**同一棵 HEAD 上把新测试文件挪走后重跑**量到的 2675（不是抄 P11-ca 报告的数字，
两者对上了）。

## 6. 图

本轮产品零改动，帧就是审计读的那 64 帧。要一张静态证据：
`FLOTER_PNG=1 go test -run TestDumpTerminalPagePNGs ./internal/terminalui/` →
`/tmp/floter-P11-cb-page-{light,dark}.png`（从既有的 P8-e dump 拷来，文件名带轮号）。
**这张图上看到的一件事**：浮条的身份行与两个 28u 按钮在 y=0..28 的带里垂直居中、
空态标题在 23u 那一档单独一行——正是 §2 表里那两档，没有多出来的第三档。

## 7. 仍待（P11-cb）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **PTY 自己画的行（20）依然不在尺子上**（§3 实测）：`ui.Tester` 看不到插件按单元格画的字。
   要守「终端里的字被裁 / 行距失控」得换手段（插件侧的行数 vs 画布高度、或 renderer 侧
   的像素），不是再加一档。
3. **`barHeight` 变高/变矮这把尺子看不见**（M4 绿变异）：浮条压住输出的边界仍只有
   P11-at/P11-af 那两条 pin 守着。
4. 只量字盒（图标 / 面 / 描边互压看不见）；`Tester.Find` 按字符串去重（同一串画两次只
   返还第一个盒子）；滚动容器里被裁掉的行不进 `Texts()`。
5. 设置页八档里 30 与 30.6 只差 0.6u（容差 0.5），分辨力已很薄，别再往里加档。
6. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」。
