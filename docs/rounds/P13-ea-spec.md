# P13-ea · 会话行 56 地板：不是「行不拥有盒」，是 14u 不等于 56

**意图一句话**：把设置页会话行的盒做到参考的那条 **56px 地板**（`.session-manager__row
{ min-height: 56px }`，`main:src/styles/settings.css:1237-1244`），让「标题 + 状态行」两行字
下面真正留出呼吸 —— 现在它 50.4px，两行字顶到离底边 8.1px 就完，比参考少 5.6px。

**为什么现在做**：`docs/liquid-glass.md` §6.95 的 **73-6c** 是 §6 差距表里最后一条 `未开始`
的**产品**行，也是 P13-dz「仍待」第 1 条。P13-dz 按 73-6 的处方注入了
`min-height = Space(14)`，**像素一个都不动**，于是把它记成「这一行的盒不由行元素拥有，
要找卡片几何的主人」。这一轮先量那个「不动」，结论反过来了。

## 先量（证据，`720×760`，`renderThemed`，en/浅色）

用同一张会话卡，只改 `cardRow` 的地板，量 `sessionRowFrame` 的读数：

| 注入 | 帧读数（`cardTop+1..cardBottom-2`） |
|---|---|
| `MinHeight(44)`（现状） | 49 |
| `MinHeight(50.4)`（= `Space(14)`，P13-dz 的处方） | **49**（不动） |
| `MinHeight(52)` | 51 |
| `MinHeight(56)` | **55** |
| `Height(44)` | 43 |
| `Height(50.4)` | 49 |
| `Height(60)` | 59 |
| `Height(70)` | 69 |

两件事：

1. **行盒确实由行元素拥有**（`Height(60) → 59`、`Height(70) → 69`，一字不差地跟着走），
   P13-dz 的「盒不是行给的」是**误判**。
2. `Space(14)` 落进了一个**盲区**：这一行的内容自身就有 **50.4**（`Space(2.25)` 8.1 × 2 +
   标题 18 + 行距 1 + 状态行 15），所以 `MinHeight(50.4)` 与「不打地板」是同一个像素；
   而帧是用**整像素**扫描读的（`cardTop+1..cardBottom-2`，比布局盒整体少 1：
   `44 → 43`、`60 → 59`、`70 → 69`），50.2 与 50.4 都截到 **49**。
   → **错的不是「行级地板」这条思路，是 `Space(14)` 这个值**。

**处方（本轮的落地项）**：

- `internal/settingsui/rows.go`
  - `rowFace` 增一个 `Floor float32`（行自己的高度地板，逐行给出，不是卡片级覆盖）。
  - `cardRow` 里 `if face.Floor > floor { floor = face.Floor }`，排在
    `cardTitleFloor` 之后（卡片级的地板是卡片的第一个行，行级地板是这一行自己）。
  - `const sessionRowFloorPx = 56`，与 `rowFloorPx = 44` 并列：参考写的是
    `min-height: 56px`，那是**字面 px**，不是 `--u * n` —— `u = 3.6` 时
    `Space(14) = 50.4`，正是 73-6 那条处方量不到的那 5.6px。
- `internal/settingsui/settingsui.go:1683`：会话行 `rowFace{}` → `rowFace{Floor: sessionRowFloorPx}`。

**只动会话行**：设置页其它行仍是 44 地板（`rowFloorPx`），集成卡的 66u 首行地板不变。

## 为什么这轮让它更有空间感

参考那条地板的注释写着「a taller floor, because a session is two lines of text plus a footer
of metadata」——地板不是给内容的，是给内容**下面**那口气的。56 之后多出的 5.6px
**全部落在状态行下方**（标题起点仍是行顶 + 8.1，逐像素核对：`title.y` 一个字节不动），
所以这一行读起来是「一行块 + 一段空隙」，而不是「块贴着盒底」。

## 行为契约零改动红线

只改 `cardRow` 的一个高度预算，不动高度链（窗口/列/卡片的地板）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线。会话行仍是 `rowValue` → `cardRow` 的同一帧，控件槽、
点击区、arm 状态、截断名提示全部原样。

## 门槛命令（PASS 基线 = 本轮实测当前值）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3326（不许减）
GOOS=linux/windows/darwin go build ./...               # 过
```

## mutation 计划（≥2 条，说明每条让哪个 pin 变红）

- **M1**：`staticSessionFloor` 56 → `Space(14)`（50.4，把 73-6 的旧处方放回去）
  → `TestTheSessionRowsHeightAgainstTheReferencesFloor` 的盒读数 55 → 49 变红。
- **M2**：`cardRow` 删掉 `if face.Floor > floor` 那一行（行级地板失效）
  → 同一条 pin 变红（55 → 49），且新的 `TestTheSessionRowsFloorIsTheReferencesLiteral`
  的「地板落在状态行下方」变红。
- **M3**：把地板挂到卡片的第一个行（`cardGeometry` 的 `titleFloor`）而不是会话行
  → 集成卡首行 66u 与设置卡其它行一起变，`extensionrowface_test.go` / 行高 pin 变红。
