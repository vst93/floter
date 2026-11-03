# P13-fq 规格：两枚 picker 之间的那片空，是余数还是间距

**意图一句话**：`paletteRow` 中间那片 89…160px 的空，要么是一条**设计过的间距**，要么是
两个定值夹出来的**余数**——先量出来是哪一种，再决定动不动。

**为什么现在做**：P13-fp 报告「仍待」第 1 条（原话：「`paletteRow` 的**行内间距**——
`Justify(ui.SpaceBetween)` 把 picker 与预览块推到行的两端，出厂值下两者之间是 175…300px
的空；而参考的 `.terminal-palette` 是 `justify-content:space-between` 同款，所以这是**照抄**，
不是漏改。**先量**：这个空在四个台阶上的实际值，以及它和卡片其它「两端对齐」的行是不是
同一条规则，再决定动不动。」）。这是差距表 §6.135 的 135-3 之后唯一还挂着的同一行问题。

## 1. 先量（已实测，720 宽真实 General 页，四台阶 × 深浅）

行内容盒 = 卡内宽 − 2×`rowInset`（`rows.go:418-423`，`3.5u`，随台阶缩放）；picker 盒 =
`MinWidth(220)`（字面，不缩放，§6.133）；预览块 = `Space(37)`（随台阶缩放，§6.135）。

| 台阶 | 行内容盒 | picker 盒 | 预览块 | 空 | 空/块 |
|---|---|---|---|---|---|
| tiny | 498.6 | 220 | 118.4 | 160.2 | 1.35 |
| small | 489.8 | 220 | 133.2 | 136.6 | 1.03 |
| default | 481.0 | 220 | 148.0 | 113.0 | 0.76 |
| large | 472.2 | 220 | 162.8 | 89.4 | 0.55 |

- 预览块右沿 = `14 px` 读数（同卡 `slider` 标题行的第二件）右沿：693.4 / 690.2 / 687.0 /
  683.8，四个台阶**两边同值**（深浅同值）。
- picker 左沿 = 行名左沿：194.8 / 200.4 / 206.0 / 211.6。
- 判：**余数**。它不是任何 token（`Space(n)` 或 `rowLineGap()`），随台阶摆 70.8px。

## 2. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/palettepair_test.go`（新） | 无 pin | 三条 pin：块右沿 = 卡片控制列（4 台阶 × 深浅）；两枚 picker 同一条地板（源码 + 画面 214…222）；空是余数（四个值 + 单调递减） |
| 2 | `internal/settingsui/settingsui.go` `paletteRow` / `palettePreview` | 现状 | **一个像素不改**（结论是「不改」，理由是两端各自有锚） |
| 3 | `docs/liquid-glass.md` §6.136 | 无 | 这一轮的账：量的表 + 两条答案 + 为什么不改 |
| 4 | `.pending-bugs.md` | 无 | 「需拍板」新增：让预览块吃掉余数（空恒定、块 250…280） |

## 3. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**全部不动**。视觉轮只改墨与面，
这一轮连墨都不改（零像素）。

## 4. 门槛命令与 PASS 基线（实测当前值）

```
gofmt -l cmd internal                         # 空
go vet ./...                                  # 净
GOOS=windows go vet ./...                     # 净
GOOS=darwin  go vet ./...                     # 净
go test -count=1 ./...                        # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...      # 三平台过
```

基线（HEAD = `cb62e76d` P13-fp，实测）：**3416 PASS / 0 FAIL / 195 SKIP / 2589 测试函数**。

## 5. mutation 计划（每条会让哪个 pin 变红）

- **M1**：删掉 `paletteRow` 的 `.Justify(ui.SpaceBetween)`（两端不再钉住）→
  `TestThePreviewBlockEndsOnTheCardColumn` 变红（块右沿 −160.2…−89.4px）。
- **M2**：预览块宽度 `Space(37)` → `Space(30)`（余数变了）→
  `TestThePaletteHoleIsARemainderNotAToken` 变红（空 +22.4px）。
- **M3**：`paletteRow` 的 `.MinWidth(220)` → `.Width(220)`（地板退回上限）→
  `TestThePalettePickerSharesTheCardsPickerBox` 变红（源码两处 + 画面）。
