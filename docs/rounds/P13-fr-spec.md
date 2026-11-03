# P13-fr 规格：字体族 picker 的天花板，是在 picker 上还是在槽上

**意图**：`chooseSelect`（设置页字体族 picker，`internal/settingsui/settingsui.go:4718`）没有
`paletteRow` 在 P13-fp 拿到的 `Shrink(1)`（§6.135）。§6.136 的「仍待」第 1 条把它列成下一格，
但同时写着「现在没病」。这一轮的任务就是**把「没病」量成一句话**：长值会不会把它顶出卡外，
`Shrink(1)` 到底改不改变画面。

**为什么现在做**：P13-fn/fo/fp/fq 四轮把同一张 Terminal appearance 卡上「两个 picker 的盒子」
量到了地板与天花板，只剩这一枚 picker 的**上限**没量。它是这条链的最后一格。

## 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:4718-4737` `chooseSelect` | `MinWidth(220)`，无 `Shrink` | **先量**：加 `.Shrink(1)` 是否改变画面（八 arm 整帧 CRC32 逐位比） |
| 2 | 同上 | 长值的可见范围没人量过 | 出厂帧 vs 92 字帧逐列比，最后一列差应 ≤ 行内容盒右沿 |
| 3 | `rows.go:1617` 控制槽 `ui.Row(c).Shrink(0)` | 「控件不收缩，让标签列先让」（§6.132 pin 过出厂值） | 92 字时量标签列是否退到自己的墨；槽本身**不改** |
| 4 | `docs/liquid-glass.md` | 缺 §6.137 | 新增：量的表 + 两条答案 + 一条 `需拍板` |

## 行为契约零改动红线

只读视觉：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动。
`Shrink(1)` 若被证明有像素影响，也只是「值太长时」的收口方式，出厂内容零像素。

## 门槛命令与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净（三平台）
go test -count=1 ./...         # 全绿
PASS 3422 / FAIL 0 / SKIP 196 / 测试函数 2596
```

## mutation 计划（每条会让哪个 pin 变红）

- **M1**：`chooseSelect` 的 `MinWidth(220)` → `MinWidth(220).Shrink(1)` →
  `TestChooseSelectHasNoShrinkOfItsOwn` 红（源码 pin）。
- **M2**：`MinWidth(220)` → `MinWidth(2200)` → `TestThePickerNeverPaintsPastTheRowColumn` +
  `TestTheLabelColumnYieldsFirstAndThePickerHasNoCeilingOfItsOwn` 红（长值不再改变画面、
  标签列不再让位）。
- **M3**：`MinWidth(220)` → `Width(220)` → `TestThePickerNeverPaintsPastTheRowColumn` 红
  （长值的画面停在行内容盒右沿左侧 28.4…38.8px 处）。
