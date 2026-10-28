# P11-af 规格：终端浮条不再挡住它下面的画布

## 0 意图与为什么现在做

终端页的浮条（`titleBar`）是**浮在画布上**的一条 28u 全宽 `Row`，而且整条带
`.DragWindow()`。框架的命中规则是：只有带交互标志的元素才是命中目标
（`ui/input.go:297` 的 `interactive` 含 `flagDragWindow`；`runtime.go:1208-1218`
把非 `PassThrough`、非 invisible 的元素都记进 `rt.hits`），于是**画布最上面 28u
的第一/第二行输出收不到任何鼠标事件**：点不进、拖不了选区。终端的
`CopyOnSelect`（抽屉里就有这个开关）在这两行上完全失效。

这是用户实测反馈 F3「不要 floating/overlay 观感」在**输入侧**的后果：浮条不只是
视觉上的一层，它还是一块吃掉指针的层。本轮把「窗口拖拽手柄」从**整条浮条**收窄成
**会话标题本身**（标题在揭示时就是这一页的 title），浮条本身永远 `PassThrough`；
揭示改由**指针位置**判定（`Element.PointerPosition()` 在 `PassThrough` 元素上仍然
算包含关系，`ui/input.go:1265-1274`），因此按钮仍能靠悬停浮现。

## 1 逐条落地项（动手前实测的行号，HEAD `3c88036`）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go:1313-1316` `titleBar` 的 `bar` | `.Absolute().Top(0)…Height(7u)…DragWindow()`，整条带是命中目标 | 去掉 `DragWindow()`，改 `.PassThrough()`：浮条不再是 hit，指针落到画布 |
| `:1320` `revealed` | `bar.Hovered() \|\| bar.FocusWithin()`（依赖命中） | `_, _, over := bar.PointerPosition(); revealed := over \|\| bar.FocusWithin()` |
| `:1330` 阴影 | `if bar.Hovered()` | `if hovered`（同一个 `over`） |
| `:1340` `identity` | `Grow(1)…DragWindow()`（整行都是手柄） | 去掉 `DragWindow()`，只保留布局 |
| `:1364` 标题 `ui.Text` | 无拖拽 | `.DragWindow()`：会话标题就是这一页的 title bar，手柄只占它的字宽 |
| `internal/terminalui/barreach_test.go` | 无 | 2 条 pin（源码契约 + 像素/行为） |

**红线（零改动）**：高度链、浮条位置与 28u 高、按钮槽位、抽屉与 resident note 的
几何、键盘模型、会话管理、设置写回、i18n 文案。视觉轮只改「谁吃指针」。

## 2 门槛与基线（本轮实测）

| 门槛 | 基线（HEAD `3c88036` = P11-ae） | 本轮要求 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 | 同 |
| `go test -count=1 ./...` | 2355 PASS / 0 FAIL / 22 SKIP | PASS 只许涨 |
| 测试函数数 | 1919 | ≥ |
| 三平台 `go build ./...` | 过 | 过 |

## 3 mutation 计划（每条都要真红、真还原）

- **M1**：把 `bar.PassThrough()` 换回 `bar.DragWindow()`（浮条重新吃指针）→
  `TestTheTerminalBarIsNotTheWindowsDragHandle` 红。
- **M2**：把 `revealed` 改回 `bar.Hovered()`（揭示回到命中判定；`PassThrough` 下
  永远假）→ `TestTheBarRevealsUnderAPointerItDoesNotCatch` 红（这也会让
  `barchrome`/`barshadow` 的悬停帧一起红，正好证明整条揭示链都走这个判定）。
- **M3**（可选）：把标题的 `DragWindow` 摘掉 → 源码契约 pin 红（页面没有拖拽手柄）。
