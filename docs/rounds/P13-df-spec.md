# P13-df 规格：三个画面包的夹具都在量框架库存的字（先量，不改）

## 意图一句话

把「测试夹具没装设计主题」这件事从**偶发发现**（P13-cm / P13-dc / P13-dd / P13-de 四次）
变成**一次量清的账**：三个绘制包各有多少条 pin 在量一个生产上不存在的帧，重钉它们要付多少
代价，以及为什么它必须独占一轮而不是塞进字号轮。

## 为什么现在做

P13-de「仍待」第 1 条就是它（`internal/launcher` 的夹具取景收口）。上一轮实体只做了 4 处
字号搬迁，剩下 5 处卡在尺子上；`internal/terminalui` 与 `internal/settingsui` 是同一个病。
不动它，字阶这一族（P13-dd 24 处 / P13-de 4 处 / 终端页 5 处）就到此为止。

## 证据（本轮实测，不是旧报告的数字）

生产：`internal/shell/shell.go:4262` 在每个 surface 画之前 `c.SetTheme(surface.Tokens.Theme)`。
夹具：`internal/launcher/view_test.go:56` 的 `render()`、`internal/terminalui/view_test.go:22`
的 `render()`、`internal/settingsui` 的 `render()` **都没有** `SetTheme`。

同一个 `*ui.Context` 上量两种取景（`ui.Tester`，参考字体）：

| | 库存（夹具默认） | 设计（生产） |
|---|---|---|
| `c.Theme().FontSize` | **14.00** | **10.80** |
| `c.Theme().Spacing` | **4.0** | **3.6** |
| 寄存器 | — | Body 10.80 / Caption 9.90 / Emphasis 11.70 / Title 12.60 / Display 17.10 |

更糟的是**库存这把尺子是量不动的**（`MeasureText("Ag")`，同一个 context）：

```
库存: 9.90→22.0  10.80→22.0  11.70→22.0  12.60→22.0  13.00→22.0  14.00→22.0  16.00→24.0  17.10→25.0
设计: 9.90→17.0  10.80→17.0  11.70→18.0  12.60→19.0                             17.10→25.0
```

库存主题下 9.9~14.0 全部落在同一个 22 —— 所以启动器那条「六档阶梯」里
`18 / 22 / 24` 不是三个字号，是那把尺子的**量子化产物**。

## 量出来的代价（实跑：给 `render()` 装上 `c.SetTheme(a.tokens(c).Theme)`）

### `internal/launcher`（21 个测试函数变红）

- 帧高也得跟着换：`frameHeight()` 写的是 `ui.LightTheme().Spacing`（4.0），装主题之后
  画面按 3.6 排，帧高按 4.0 算 —— 两条腿不是一套。改成 3.6 之后仍是 21 个。
- 阶梯账：pin 写 `[15 18 22 24 25 28]`，实际画出来 `[15 17 18 19 25 25.2]`。
- 列：`textcolumn_test.go` 的标题列 pin `54.0`，生产 `48.6`。
- 高度链：`resultsedge_test.go` / `raycast_test.go` 的字段块floor `y=55`，生产 `y=49`。
- 前提破掉的臂：`outputedges_test.go:184`「没有一行被切在两端」、`outputnumders` 的
  「十行列表没有滚动」、`outputfold_test.go:171`「第一行只露 1.000」——帧变了之后夹具
  不再进入它要测的那个状态。这一类**不能靠改数字收口**，得给它们一个能进入该状态的窗口。

### `internal/terminalui`（8 个测试函数变红）

`TestASwappedStoreIsANewPalette`、`TestEachDrawerRungHasItsOwnProducer`、
`TestEachTerminalPageRungHasItsOwnProducer`、`TestReopeningTheDrawerStartsAtItsTop`、
`TestTheDrawerScrollsItsLastRowsIntoView`、`TestTheDrawerSeamIsOnItsTopEdge`、
`TestTheDrawerStaysOnTheHeightLadder`、`TestTheTerminalPageStaysOnTheHeightLadder`。

- 终端页阶梯：pin `[15 22 23 28]`，生产 `[15 17 18 25.2]`（空态标题 23 → 18、条带身份行
  22 → 17、`New command` 按钮 28 → 25.2）。
- 抽屉阶梯：pin `[17 20 22 28 30]`，其中 `20 —— 迷你终端预览的一行（t.FontSize-1 + mono）`
  是**库存取景的产物**：那一行在生产里是 9.8pt（`10.80-1`），画出来 **15**，不是 20。
- 抽屉滚动：640×360 下 `MaxY=0.0` —— 换 3.6 间距后内容装得下了，「重开抽屉回到顶部」
  这条规则在夹具里**没有被执行的机会**。

### `internal/settingsui`

P13-dd 记的是「四十多条 pin 在量库存字 14」，本轮未重跑（该包只对两条需要真数的 pin
用了 `renderThemed`，其余未动）。同一份账，同一轮收。

## 本轮做什么 / 不做什么

**做**：把上面这张账量清楚、写进合约（§6.75）与 `.pending-bugs.md`，并把
「重钉 = 下一轮独占」的理由钉住。**不做**：不在这一轮里改夹具 —— 21 + 8 + 40 条 pin 的
重钉需要逐条判定 V/B/S，且有一类要先修前提（窗口尺寸），塞进一轮只会把「产品回归」
和「夹具换了尺子」混成同一批红。

**产品代码零改动**（`git diff --stat` 为空是这一轮的验收条件之一）。

## 下一轮（P13-dg）的做法，先写死再动手

1. 每包一个 `renderThemed(t, a)`（settingsui 已有，照抄），生产取景 + **帧高按设计间距**。
2. 逐条重钉，每条只许二选一：**V**（数字变了，注释写 `P13-dg 取景收口：旧值 → 新值`）
   或 **S**（前提破了：把窗口放大到能进入该状态，**不许**放宽断言、不许加容差）。
3. 判不准按 B 先跑；红了是产品回归就改产品。
4. 只有当整轮的「变红清单」全部有归类，才允许提交。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal                                  # 空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 基线 3311
测试函数数                                             # 基线 2467
```

## mutation 计划

本轮**无产品代码、无新 pin**（纯账），没有可注入的缺陷 —— mutation 不适用。
这一点在报告里明写，不用「跑一次绿」冒充变异。
