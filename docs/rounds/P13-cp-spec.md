# P13-cp 规格 · 捕获输出页便条行的图标板：一道淡，不是两道

## 0. 意图（一句话）

便条行的 28u 图标板在「材质熄灯」（`DimFace`）之外**又**挂了一道 `Opacity(0.62)`，
两道淡乘起来把一块 E1 的面压成地面上的一层斑；这一轮把它收成**一道**——
材质熄灯 + 墨低一档，与 P13-s / P13-aq / P13-u / P13-y 同一个算术。

## 1. 为什么现在做（证据）

选题来自 `.pending-bugs.md` 的 P13-ch「仍待 1」（本轮第一份**实测**证据）：

> 便条行的图标板叠 `DimFace` + `Opacity(0.62)` 两道（`launcher/output.go:907-910`
> + `launcher/glyphs.go:286-309`）——先量图标淡两道之后还剩多少级差，再决定收不收成一道。

行号已漂，本轮核对后的真实位置：

| 位置 | 内容 |
|---|---|
| `internal/launcher/output.go:928-938` | `dim := row.Status \|\| row.Disabled`；`plate := a.iconPlate(c, false, dim)`；`if dim { plate.Opacity(0.62) }` |
| `internal/launcher/glyphs.go:292-309` | `iconPlate`：`if dim { return tokens.DimFace(plate) }`，`DimFace` 见 `internal/theme/theme.go:1254`（只留底对侧 `LensEdge`，不画冠线） |

改前实测（`TestNotePlateDump`，720×488 的真捕获输出页，三行：选中门 / 静息门 / 便条）：

```
light Door2: ground={253 253 254} face={235 235 236} step=18 crown=5  foot=14 glyph=122
light Note : ground={253 253 254} face={242 242 243} step=11 crown=0  foot=9  glyph=93*
dark  Door2: ground={23 23 25}   face={47 47 48}   step=24 crown=19 foot=10 glyph=122
dark  Note : ground={23 23 25}   face={38 38 39}   step=15 crown=0  foot=5  glyph=70*
```

（`*` 是**改后**的字形读数；改前字形还被 0.62 乘过一道。）

- 面差：18 → **11**（浅）、24 → **15**（深）——便条板只剩门板面差的 **61% / 63%**。
- 底对侧：14 → **9**、10 → **5**——`DimFace` 保留的那条暗线也被淡掉了。
- 冠线：两侧都是 **0**，这是 `DimFace` 的功劳，与 0.62 无关——也就是说第二道淡
  **没有做任何材质上的事**，它只是把第一道的结果又乘了一次。

## 2. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `output.go:930-932` | `if dim { plate.Opacity(0.62) }` | 删掉。`DimFace` 就是「便条」的材质：面在、底对侧在、冠线熄了 |
| 2 | `output.go:934` | 字形无条件 `t.TextMuted`（便条靠 0.62 变淡） | 便条那一臂换 `tokens.TextTertiary`（墨阶的下一档，`theme.go:692` 浅 `#8E8E94` / `:827` 深 `#6E6E76`） |
| 3 | `glyphs.go:278-281` | 注释说「参考的 0.62 由调用点施加」 | 改成「没有自己的淡：登记 = 材质熄灯 + 墨低一档」 |

**不动的**：行高、行几何、字阶、字重、i18n 文案、键盘模型、会话管理、命令接线。
这一轮只改**一块 28u 板的面与它里面那枚字形的墨**。

## 3. 行为契约零改动红线

- 高度链：`outputRow` 的盒子、`iconPlate` 的 `Space(7)` 一个字节不动（pin 断言板宽 = 28px）。
- 墨：便条行的**标题**仍是 `TextMuted`（`output.go:941-943`），本轮不碰。
- 选中 / 游标：`RowSelected` / `LitFace` 两条臂一个字节不动。

## 4. 门槛命令与 PASS 基线（本轮开工前实测）

```
gofmt -l cmd internal                        → 空
go vet ./...                                 → 净
GOOS=windows go vet ./...                    → 净
GOOS=darwin  go vet ./...                    → 净
go test -count=1 ./...                       → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 3277（基线）
--- SKIP 计数                                  → 133（基线）
func Test 计数                                 → 2426（基线）
三平台 build                                   → 过
```

## 5. mutation 计划（≥2 条）

| # | 注入 | 期望变红的 pin | 期望报错串 |
|---|---|---|---|
| M1 | 把 `if dim { plate.Opacity(0.62) }` 放回去（= HEAD 的原样） | `TestTheNoteRowPlateCarriesOneFadeNotTwo` | `the note plate's face steps 11 levels off its ground and the door plate's steps 18` |
| M2 | 字形墨回退成 `t.TextMuted`（保留删掉的 0.62） | 同上 ④ | `the note plate's glyph reads 122 levels off its face and the door's 122 (a step of 0)` |
| M3 | `iconPlate` 的 `dim` 臂换成 `tokens.LitFace` | 同上 ② | `the note plate is lit: its top row lifts 5 levels above its face` |

每条都要 `md5sum` 逐字节还原 + `touch` 复跑绿。
