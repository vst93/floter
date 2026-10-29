# P11-bo 规格 · 按住修饰键的「进入终端」只剩底部那一条带（不再是一行）

## 意图

用户 F4-3 原文：**「按住 Super 显示进入终端选项要固定底部且不重复」**，细则
「**额外固定在底部**的一行（不属于任何列表项），不受滚动影响，不占排序；按
Super+Enter 即可进入终端；当输入非空且有搜索结果时，仍保留该底部行但不重复」。

P11-bb 把这条线做成了**两个东西**：底部的常驻带（`heldHintRow`，`Absolute().
Bottom(footerBand)`）+ 空查询时**追加进 Results 的一行**（`heldTerminalItem`，
`search.go:856-862` 修前）。于是空页面 + 按住修饰键时，屏幕上同时有
`Open Terminal / A blank session — runs nothing`（列表最后一行、带选中色、
`Ctrl ↩` 徽章）**和** `Ctrl ↩ / Open a terminal session`（底部带）——同一枚和弦
画了两遍。这就是用户说的「不重复」的反面。

## 现状（本轮实测，行号为修前）

| 位置 | 事实 |
|---|---|
| `internal/launcher/search.go:856-862` | `Results()` 末尾 `if item, ok := a.heldTerminalItem(); ok { out = append(out, item) }` |
| `internal/launcher/launcher.go:3331-3365` | `heldTerminalItem()`：空字段 + 修饰键 → 一行 `system-terminal-bare` |
| `internal/launcher/launcher.go:3177-3180` | `heldHint()`：`modifierHeld && output==nil && !pluginMode`——**不看查询**，所以带在空页面也画 |
| `internal/launcher/launcher.go:1268` | `Feedback: … || a.heldHint()`：窗口**已经**为空页面上的带预留了 30u，又多算了一行 |

复现（headless，本轮实测）：

```
tt.HoldModifiers(ui.Cmd); tt.Frame()
texts = [… "Open Terminal" "A blank session — runs nothing" "Ctrl" "↩"
         "Open a terminal session"]
```

同一屏两处「进入终端」＝用户报的「重复」。

## 落地项

1. **列表不再有一行**：删掉 `Results()` 的追加与 `heldTerminalItem()`；删掉随之
   变成死代码的 `bareTerminalRowID` 常量、`assignNumbers`/`defaultSelection` 里的
   跳过分支、`heldRowShown`/`heldRowKey`/`holdHeldRowSelection`。
2. **底部带成为唯一说法**：`heldHint()` 保持原判据（空页面与有查询都画），
   注释改成「这是唯一一条」。
3. **Super+Enter 直接跑动作**：`View` 里改成
   `if a.heldHint() && c.Shortcut(ui.Cmd, ui.KeyEnter) { a.openBareTerminal() }`——
   不经过任何行、任何排序。
4. **选择不再被按动**：空页面按下修饰键不改 `Selected`（没有行可选）；有查询时
   仍把选择交给 action bar（既有行为，零改动）。
5. **i18n 收口**：`BareTerminal` / `BareTerminalHint` 两个文案随行一起死掉，
   从 `internal/i18n` 删除（`TestNoDeadCopy` 本来就会红）。

## 行为契约零改动红线

- 高度链：窗口高度照旧由 `Geometry{Feedback: …heldHint()}` 的 30u 带 + 行数决定；
   **本轮唯一的高度变化是那一行不再被算进去**（这正是要的）。
- 键盘模型：`Super+Enter`、修饰键按下/松开边沿、action bar 的选择交接不变。
- 会话管理、设置写回、命令接线、动效：零改动。

## 门槛（本轮实测，修前基线）

- `gofmt -l cmd internal` 空；三份 `go vet`（linux/windows/darwin）净；
  `go test -count=1 ./...` 绿；PASS **2454**（基线 2454）；SKIP 51；FAIL 0；
  三平台 `go build ./...` 过；测试函数 2005 → **2006**。
- 注意：`internal/i18n` 的 `TestNoDeadCopy` 在删文案前是红的（死文案），
  这是本轮必须一起收的账。

## mutation 计划

- **M1**：把行加回 `Results()`（空查询 + 修饰键追加一行 `system-terminal-bare`）→
  `TestTheHeldLineIsTheBandsAndNeverARow` / `TestHeldModifierBareTerminalRow` /
  `TestTheHeldBandIsTheEmptyPagesLastBand` 变红。
- **M2**：`heldHint()` 加上「只看有查询」的闸门 → 空页面的带消失 →
  `TestTheHeldRowWearsTheSelectedFace` / `TestTheHeldLineIsTheBandsAndNeverARow`
  变红。
