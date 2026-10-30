# P13-v 规格：满态的下折边——只露一半的行**退后**，不是被切

**意图一句话**：启动器列表没有滚动条（用户定的），所以「下面还有」只能靠折边说话；
而折边今天是**把行硬切**——视口下沿落在行的中间，一行字被水平切成两半。折边之外
的东西应该是**退后**，不是**被切**。

**为什么现在做**（证据，`docs/liquid-glass.md` §6 已无 `未开始` 行，按纪律转通用队列的
「空态满态」格；P13-s/t/u 已把「状态反馈」的禁用那一半收完）：

- `internal/launcher/launcher.go` 的 `rowBody`：行盒固定 `rowUnits`（40u），列表
  `Grow(1)` 拿到的高度不可能是 40 的整数倍（窗口高度 = `Geometry.Height()`，字段带 56u
  呼吸 4u 面板上沿 4u 尾 2u + 行 × 40u + 页脚带，且窗口可被用户缩放）。
- 实测（`ui.Tester`，720 × 468，20 条 `walk` 结果，浅色档）：列表视口 `{0,60,720,383}`，
  第 10 行盒子 `{8,420,704,40}`，视口下沿 443 —— 这一行只露 23/40。
  它的文字盒因此是 `{54,428,63,15}`（整行可见时是 `{54,409,60,24}`），
  墨在 y=442 一刀断掉（`docs/rounds/P13-v-report.md` 有逐行墨计数）。
- 断口是**字**：24px 的行文字被切到 15px，即「上半截字母」。这不是「还有更多」，
  这是「界面坏了」。而列表既没有滚动条、也没有渐变遮罩（后者是拿背景色回放冒充玻璃，
  `docs/liquid-glass.md` §0 禁），折边于是是唯一能说话的地方。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/launcher.go`（`rowTransition` 上方新增 `foldAlpha`） | 没有「这一行露出多少」的判据 | 纯函数 `foldAlpha(viewport, box) float64`：整行在视口内 → 1；被折边切 → `可见高度/行高`；滚出视口 → 0；首帧没有盒子 → 1 |
| 2 | `internal/launcher/launcher.go` `rowBody` | 行只有历史档 `Opacity(0.92)` | 行墨 = min(静息档, 折边比例)，只在 `< 1` 时下发 `row.Opacity`；两个盒子都用**上一帧**的（`a.listRect` / `a.rowRects[i]`），互相自洽 |
| 3 | 同上，页脚那一行 | —— | **不参加折边**：动作栏是钉在面板底部的 footer，只是被记成列表最后一行好让高度链算进去，它的盒子永远在视口之外，按折边算就是 0 —— 那会把整条动作栏抹掉（实测：选中块 tint 全丢、块从 `x=-1` 起算，`actionbarfooter_test.go` 四条直接红） |

**不做**：不给折边加渐变遮罩、不补滚动条、不改行高/视口高度（高度链是红线）、
不动 `output.go` 的捕获输出列表（另起一轮，写进「仍待」）。

## 行为契约零改动红线（逐条核对）

- 高度链：`Geometry.Height()` / `rowUnits` / 列表 `Padding` 一行未动。
- 键盘模型：`Selected` / `ScrollIntoView` / 编号家族一行未动（折边只改墨）。
- 会话管理、设置写回、i18n 文案、命令接线：零改动（本轮没有新增任何文案）。

## 门槛命令 + PASS 基线（本轮实测，HEAD `784fcf6c`）

```
gofmt -l cmd internal                                  → 空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...  → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2981（基线）→ 2985
SKIP 78 → 79（新增一条 opt-in 截图 dump）｜测试函数 2198 → 2203（+5，同一 grep 口径）
GOOS=linux/windows/darwin go build ./...               → 过
```

## mutation 计划（≥2，每条说明会让哪个 pin 变红）

| # | 注入的缺陷 | 预期变红的 pin |
|---|---|---|
| M1 | 折边不下发（`false && fold < opacity`） | `TestTheRowAtTheFoldRecedes`（折边上的行墨不再变淡）、`TestTheLessOfARow…`（比例不再生效） |
| M2 | `foldAlpha` 对整行可见也返回 0.5 | `TestTheRowsInsideTheViewportKeepTheirInk`（首行墨被淡）、`TestTheRowAtTheFoldRecedes` |
| M3 | 页脚那一行也参加折边（去掉 `!isBar`） | `actionbarfooter_test.go` 的 `TestTheActionBarTakesTheSelectionOnACommandLine` / `TestTheFooterIsABlockWithTheWeakRule` |
| M4 | 分母用视口高而不是行高 | `TestFoldAlpha`（表格三条） |
