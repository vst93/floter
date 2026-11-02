# P13-ef 规格 · 最后 9 处「自己搭取景」的夹具换到出货尺

**意图一句话**：这一包（`internal/launcher`）此前有四个取景入口各自抄了一遍「装主题」两行，
最后 9 处测试干脆自己搭 `ui.NewTester` —— 抄漏的那几处用框架库存墨（`Spacing 4.0` /
`FontSize 14`）画这个应用，量的仍是用户看不到的帧。本轮把取景收成一个定义点
（`themedView`），并把这 9 处 + 折边量尺那一族全部换到出货尺。

**为什么现在做**：`docs/liquid-glass.md` §6.86 的 73-5 尾巴。P13-ec（`render`）→ P13-ed
（`renderIn`）→ P13-ee（`renderAt` + `frameHeight` 直接调用）已经收完三族，上一轮报告的
「仍待」第 1 条就是这 9 处。它们没有共同入口，所以只能逐处改，必须单独一轮。

## 先量（动手前的实测，不是旧报告的数字）

| 量什么 | 库存尺 | 出货尺 |
| --- | --- | --- |
| 帧高（十行） | 488 | 439 |
| 字段块发丝 | `y=55` | `y=49` |
| 行盒 | 40 | 36（40u × 0.9） |
| 输出页第二组标题盒 | — | `y=342.4..368.4`（h=26） |
| 状态带 30u | 30px | 27px（30 × 0.9） |

换尺后一次跑出 **3 个测试函数红**，逐条都是「pin 引用了库存尺的数」，不是产品回归：

1. `TestTheBandHasOnePredicate`：`charged - base = 27`，pin 要 30。
2. `TestTheOutputGroupHeadingAtTheFoldRecedes`：`height 404` 在出货尺上标题整行都在视口里
   （fold 1.000）；`360` 只切到 17.6/26（fold 0.68，墨 79/116，过不了「不到一半」那条线）。
3. `TestTheFurtherTheOutputRowIsCutTheMoreItRecedes`：`clippedRow` 返回**第一行**、
   `share=1.000` —— 见下面第 3 条落地项。

## 逐条落地项

1. **取景只有一个定义点**（`internal/launcher/view_test.go`）
   现状：`render` / `renderShipped` / `renderIn` / `renderAt` 各抄 `c.SetTheme(a.tokens(c).Theme)`
   + `a.View(c)`；9 处测试自己搭 `ui.NewTester(func(c){ a.View(c) }, …)`。
   应改成：新增 `themedView(a *App) func(c *ui.Context)`，上面四处与 9 处全部调用它。

2. **9 处直接开窗的测试换尺**（`bandheight_dump` / `clickdraw` / `deadlinewake`×2 /
   `footerink` / `feedbackpredicate` / `searchruns` / `spinnerframes`×2 / `outputfold` 出图）
   现状：`frameHeight(maxResultSlots)`（488）+ 部分没装主题。
   应改成：`themedFrameHeight(maxResultSlots)`（439）+ `themedView`。需要计数的三处保留自己的
   计数闭包，只把 `a.View(c)` 换成 `draw(c)`。

3. **`clippedRow` 按像素判「切」**（`outputfold_test.go:79`）
   现状：`share := visibleShare(...); if share > 0 && share < 1`。`visibleShare` 是 float32
   除法，行盒 `y=100.4 h=36` 在 318.6 高的视口里算出 **0.99999997** —— 整行在视口里的第一行
   也被判成「被切掉一条缝」，于是 pin 量的是第一行的头带（fold=1），两条读数一样。
   应改成：`top < clipBottom && bottom > clipBottom`（整数像素，行的最后一行像素真的溢出）。

4. **折边量尺那一族跟尺**（`fold_test` / `outputfold_test` / `headingfold_test`）
   现状：`inkAt(frameHeight(…))`、`frameHeight(…)+20`、`height 404 / 300 / 408`。
   应改成：`themedFrameHeight(…)/+20`；出货尺上标题落在折边上的实测窗口是
   `275`(f=0.12) / `280`(f=0.32) / `295`(f=0.89) / `320`(f=0.47)，输出页 `350`(f=0.29)；
   据此重钉为 `280`（单档）、`295` vs `275`（两档）、`350`（输出页）。

5. **新 pin**（`fixtureruler_test.go`）：`TestTheDirectTestersDrawTheShippedFrame` —— 直接开窗
   的帧高 439、发丝 49、第十行整行在帧里；库存尺把这三个数写成 488 / 55 / 溢出一行。
   配一条 opt-in 出图（`TestDumpP13EFFixturePNGs` → `/tmp/floter-P13-ef-<appearance>.png`）。

## 行为契约零改动红线

- 产品代码 **0 行**（本轮只动 `_test.go`）。高度链、键盘模型、会话管理、设置写回、i18n、
  命令接线一律不碰。
- 断言只换尺，不放宽：没有新增容差、没有 `t.Skip`（除标注 opt-in 的出图）、没有删断言。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                  # 空
go vet ./... ; GOOS=windows/darwin go vet ./...   # 净
go test -count=1 ./...                 # 全绿
--- PASS 计数                          # 基线 3332 → 本轮 3333
go build ×3                            # 过
```

## mutation 计划（每条都实跑）

- **M1**：`themedView` 里删掉 `c.SetTheme(a.tokens(c).Theme)` → 发丝回 55、行盒回 40，
  `TestTheDirectTestersDrawTheShippedFrame` / `TestTheDefaultFixtureDrawsTheShippedFrame` 红。
- **M2**：`clippedRow` 退回 `share > 0 && share < 1` →
  `TestTheFurtherTheOutputRowIsCutTheMoreItRecedes` 红（`row 0 … share=1.000`）。
- **M3**：状态带断言退回 `!= 30` → `TestTheBandHasOnePredicate` 红（27 对 30）。
