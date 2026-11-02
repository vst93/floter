# P13-dp 规格 · 夹具的尺子：把 `internal/terminalui` 剩下的 pin 收到设计尺上

## 意图

`§6.76` 把夹具债拆成三格（`dg-t` / `dg-l` / `dg-s`），`dg-t` 是终端画面包那一格。
本轮把它做完：这个包里除了两条**数调色板构建次数**的 pin，其余每一条几何/高度 pin
都改骑**生产尺**（`Spacing 3.6`、`FontSize 10.80`）。

为什么现在做：`§6.75` 实测 `MeasureText("Ag")` 在库存主题下对 9.90 / 10.80 / 11.70 /
12.60 / 13.00 **全返回 22.0** —— 也就是说这个包里的「字号阶梯」pin 一直在量**框架库存
的字**，而用户看到的是 15 / 17 / 18 / 25.2。一条 pin 若量的是没人渲染的那套尺子，它变红
时说不出产品坏在哪，变绿时也保不住任何东西。

## 现状（动手前实测，P13-dp）

- `internal/terminalui/view_test.go` 的 `render()` 不装主题；`renderThemed()`（P13-dh 加）
  只被 `drawerseam_test.go` 用。
- 把 15 个文件里 51 处 `render(t,` 全换成 `renderThemed(t,` 后有 **8 个测试函数**变红，
  与 `§6.76` 预言的「≥8 条」一致：

| 测试 | 类别 | 实测失败串（节选） |
|---|---|---|
| `TestTheDrawerStaysOnTheHeightLadder` | V | `height 15.0 is not on the ladder [17 20 22 28 30]`、`no run of height 20.0/22.0/28.0/30.0 was painted (heights [15 17 25.2 27])` |
| `TestEachDrawerRungHasItsOwnProducer` | V | `"floter --version" 20.0 → 15.0`、`"Line height" 22.0 → 17.0`、`"Close terminal settings" 28.0 → 25.2`、`"Regular" 30.0 → 27.0` |
| `TestTheTerminalPageStaysOnTheHeightLadder` | V | `no run of height 22.0/23.0/28.0 was painted (heights [15 17 18 25.2])` |
| `TestEachTerminalPageRungHasItsOwnProducer` | V | 同上四条逐条 |
| `TestTheDrawerScrollsItsLastRowsIntoView` | **S** | `en 640x360: the drawer reports no scroll range (MaxY=0.0)` |
| `TestReopeningTheDrawerStartsAtItsTop` | **S** | `no scroll range (MaxY=0.0) at 640x360` |
| `TestOnePalettePerTerminalFrame` | 夹具自身 | `the first frame built 2 palettes, want 1` |
| `TestASwappedStoreIsANewPalette` | 夹具自身 | 同上（换 store 后那一帧） |

- 比例确认（`§6.76` 的两个比例在本包逐条对上）：行盒吃 **0.77**（22→17、23→18、
  20→15），空格吃 **0.90**（28→25.2 = 7×3.6、30→27 = 7.5×3.6）。`tokens.Caption`
  的 15 两边同值，是全包唯一没被库存尺污染的档。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `view_test.go` `renderThemed` | 每次渲染额外 `SetPreferences{ReduceMotion}` | 去掉那一行：`SetPreferences` 自己会 settle 一帧，于是在「数调色板构建次数」的 pin 里多出一次构建（实测 2 而不是 1） |
| 2 | `view_test.go` | 无 `designSpace` 常量 | 加 `const designSpace = 3.6`（`§6.76` 实测的设计尺间距），凡是 `t.Space(n)` 撑出的盒子都用 `n * designSpace` 写 |
| 3 | `textheight_test.go` 阶梯 | `[17 20 22 28 30]` | `[15 17 25.2 27]`（15 = 预览行 9.8pt，17 = 读数/行标签，25.2 = `Space(7)`，27 = `Space(7.5)`） |
| 4 | `textheight_test.go` 产生者 | 读数 17 / 预览 20 / 标签 22 / 关闭 28 / 分段 30 | 17 / 15 / 17 / 25.2 / 27，逐条写 `P13-dp 取景收口：旧值 → 新值` |
| 5 | `pageheight_test.go` 阶梯 | `[15 22 23 28]`，must `{22,28}` | `[15 17 18 25.2]`，must `{17, 25.2}` |
| 6 | `pageheight_test.go` 产生者 | 身份行 22 / 按钮 28 / 空态标题 23 / 空态标签 22 / 便条 22 | 17 / 25.2 / 18 / 17 / 17 |
| 7 | `drawerscroll_test.go` 窗口（**S**） | 640×360 / 720×360 | 640×**280** / 720×**240**：设计尺下行短 10%，360 高的最小窗口上条带内容已装得下（实测 `MaxY=0.0`），前提消失 |
| 8 | `drawerscroll_test.go` `wholeRow` | 20 | **17**（一整行文字盒在设计尺上的高度） |
| 9 | `tokensmemo_test.go` 两臂 | `renderThemed` | 回 `render`：这两条量的是**构建次数**不是几何，而夹具自己装主题的那次读是这一帧的**第二个读者**（第一读看到库存主题的 `Dark`，第二读看到设计主题的），会在换 store 那一帧多出一笔 |

**不许做的事**：加容差、放宽断言、`t.Skip`、把「没画出来」改成「非空即可」。

## 行为契约零改动红线

产品代码 `git status --porcelain` 在本轮**只剩 `docs/` 与 `*_test.go`**：高度链、键盘模型、
会话管理、设置写回、i18n 文案、命令接线一字不动。视觉轮才动墨与面，本轮只搬尺子。

## 门槛命令与 PASS 基线（实测）

```
gofmt -l cmd internal                  # 空
go vet ./...                           # 净
GOOS=windows go vet ./...              # 净
GOOS=darwin  go vet ./...              # 净
go test -count=1 ./...                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3322（基线 3322）
GOOS=linux/windows/darwin go build ./...            # 过
```

测试函数 **2485**（基线 2485：本轮只改既有 pin，不新增函数）。

## mutation 计划（≥2，实跑）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `terminalui.go:898` 空态标题 `t.FontSize + 1` → `t.FontSize` | `TestEachTerminalPageRungHasItsOwnProducer`：`"No terminal session yet" ... is 17.0 tall, not 18.0` |
| M2 | `terminalui.go:1467` `barButton` 的 `Space(7)` → `Space(7.5)` | 同测试（按钮）：`is 26.1 tall, not 25.2` |
| M3 | 夹具侧回退：`textheight_test.go` 的 `renderThemed` → `render`（即本轮的「尺子」） | `TestTheDrawerStaysOnTheHeightLadder`：`height 20.0 is not on the ladder [15 17 25.2 27]` |
