# P13-eb · 助记提示：画的与量的读同一个寄存器（`de-5` + `de-11`）

## 意图

`docs/liquid-glass.md` §6.86 的 **`de-5`**（`launcher.go` 助记提示的绘制）与 **`de-11`**
（`triggerhint.go` 的量算）是同一条处方：「画的与量的同一个名字」。这一轮把两处从
`t.FontSize - 1`（= 9.8）换成阶梯上的 `tokens.Caption`（= 9.9），并把卡住这一族的那把
**库存尺**换成出货尺——`TestTriggerHintFits` 此前一直在量 13px 的字（stock `FontSize` 14），
比生产画的大 **33%**。

为什么现在做：这是 §6.86 差距表里**第一条能单独落地**的 `未开始` 行（`de-2` / `de-3`
是输出页行盒的计费，动它们会改**高度链**——红线，必须拍板；`de-5` / `de-11` 只动字号
与取景，不碰高度链）。

## 现状（动手前实测）

- `internal/launcher/triggerhint.go:160`（P13-eb 前）：`size := t.FontSize - 1`。
- `internal/launcher/launcher.go:1636`（P13-eb 前）：`).FontSize(t.FontSize-1).SingleLine()`。
- 出厂档 `t.FontSize = round1(12 × 0.9) = 10.8`，所以偏移量给 **9.8**；阶梯的
  `tokens.Caption = round1(11 × 0.9) = 9.9`。两个数**碰巧**几乎相等，所以画与量看起来
  一致——但**名字不是同一个**：下一个改 `t.FontSize` 的人只会改到画的那一半。
- `internal/launcher/triggerhint_test.go:123-129`（P13-eb 前）：`wide` 夹具**没有**
  `c.SetTheme(...)`，`c.Theme()` 是框架 stock（`FontSize` 14）→ `t.FontSize - 1` = 13。
  同文件 `narrow` 夹具（`:152`）同样没有换尺。

## 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `triggerhint.go:160` | `size := t.FontSize - 1`（9.8） | `size := a.tokens(c).Caption`（9.9），注释写明 de-5/de-11 同进同退 |
| `launcher.go:1636` | `.FontSize(t.FontSize-1)` | `.FontSize(tokens.Caption)`（`tokens` 已在该作用域） |
| `triggerhint_test.go:123` | 库存尺夹具（量 13px） | `c.SetTheme(wideApp.tokens(c).Theme)`，并按 `tokens.Caption` 量 |
| `triggerhint_test.go:152` | 库存尺夹具 | 同上（`narrowApp`） |
| `triggerhint_test.go:126` | 宽窗 300 | **240**：出货尺下 300 宽时 `natural 114 ≤ room 135.6`，那条「被挤」的前提在生产的尺上根本不成立（实测红串 `114 <= 135.6`） |
| `triggerhint_test.go:158` | 窄窗 120 | **90**：出货尺下 120 宽的 `room` = 45.6，仍高过六格地板，narrow 臂会失去对象 |
| `triggerhint_test.go:145` | `box.W != room`（精确） | `box.W == floor(room)`：`room` 是布局的 105.6，画出来的盒是 105（P13-du 的「painter 的格子」） |
| 新增 `triggerhintdump_test.go` | — | opt-in 出图（`FLOTER_PNG=1`），240 宽、深浅各一张 |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节不动**。
`triggerHintShown` 只决定「显示/隐藏」，返回类型与语义不变；字号是 9.8 → 9.9（0.1pt）。

## 门槛（P13-ea 基线 → 本轮实测）

- `gofmt -l cmd internal` 空 / `go vet ./...` 净 / `GOOS=windows|darwin go vet ./...` 净
- `go test -count=1 ./...` 全绿
- PASS **3327**（基线 3327）/ FAIL **0** / SKIP **160 → 161**（新增的 opt-in 出图）
- 测试函数 **2495 → 2496**（新增出图那一个）
- 三平台 `go build` 过

## mutation 计划（每条都要实跑变红）

- **M1**：`triggerHintShown` 的尺换成 `Caption × 4` → 六格地板失效，`TestTriggerHintFits`
  应在「nudge is missing at 240px」红。
- **M2**：把 `wide` 夹具的 `c.SetTheme(...)` 拿掉（退回库存尺）→ 应在
  「the nudge's box = 90, want the row's room 104」红（实测红串见报告）。
- **M3**：`triggerHintMinCells` 6 → 3 → 应在「a box under six cells was kept」红。
