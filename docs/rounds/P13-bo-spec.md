# P13-bo · 启动器页级注的寄存器：插件关闭那一页的答案，此前画在列表行的静息墨里

## 0 本轮意图（一句话）

**剪贴板插件关闭时，整页只有一句话，而这句话被画在列表行的静息档——与它上面输入框的
占位句逐级相同，比同一件事在浏览器页的说法低 89 / 73 级。** 本轮把这一句放回「一页的
陈述」那一档（`TextStrong`），并让它的图标列退到该档的次一级（`TextSecondary`）。

为什么现在做：`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用队列；这一条同时属于
**状态反馈**（一句话说一件事，不该两种画法）与 **空态/满态**（一页只有一句话时，那句话就是
这一页的结论）。它是「差值太小，全糊在一起」这句话在启动器上的最后一处。

## 1 证据（实测，`ui.Tester`，720 帧、真启动器地面；两套外观、中英两语同形）

同一件事（「这个插件关着」）在本仓有**两条路径**，落在**两档墨**上：

| 表面 | 代码路径 | 字阶 / 字重 / 墨 | 最暗（浅） | 最亮（深） | 对地面（浅 / 深） |
|---|---|---|---|---|---|
| 剪贴板关闭（`clipboard-disabled` 行） | `launcher.row()` 的 `item.status` 臂 | `FontSize` / 480 / `TextMuted` | **116** | **164** | **137 / 141** |
| 浏览器关闭（`BrowserDisabled`） | `emptyMessage` → 居中陈述 | `Emphasis` / 400 / `TextStrong` | **27** | **237** | **226 / 214** |
| 浏览器无配置（`BrowserNoProfile`） | 同上 | 同上 | 27 | 237 | 226 / 214 |
| 普通「没有匹配项」 | 同上 | 同上 | 27 | 237 | 226 / 214 |
| 输入框占位句「Type to search」 | 字段自己的占位 | `FontSize` / — / `TextMuted` | **116** | **164** | 137 / 141 |
| 最近区的分组标题「Recently launched」 | `row()` 的 heading 臂 | `FontSize` / 500 / `TextSecondary` | 108 | 156 | **145 / 133** |
| 结果行标题「Recent 00」 | `rowBody` | `FontSize+2` / 580 / `TextStrong` | 27 | 242 | 226 / 219 |

地面：浅 **253** / 深 **23**。

**三条读数**：

1. **这一页的答案与输入框的占位句是同一个数**（浅 116 / 深 164，逐字节相同）。用户在这一页
   看到的唯一内容，读起来和「还没输入」的提示一样轻。
2. 同一件事的两条路径相差 **89 / 73 级**。
3. **`TextSecondary`（145 / 133）不是中间那一档**：它离 `TextMuted`（137 / 141）只有
   **8 / 8 级**——正是 §6.26 判掉的那条「差值太小」的线。启动器这一页的墨阶梯是**两颗齿**，
   不是三颗：要么留在安静那颗（137 / 141），要么进陈述那颗（226 / 214）。所以这一轮不是
   「抬一档」，是**换一颗齿**。

`TextStrong` 是否会让它读成一条结果？不会：结果行的标题是 `FontSize+2` / **580**
（实测墨盒高 24），这一句是 `FontSize` / **480**（墨盒高 22），且它没有底板、没有编号、
不在数字族与箭头路径里（`rowSelectable`）。同色不同齿，且形态上仍是最轻的一行。

## 2 逐条落地项

| # | 位置 | 现状（`internal/launcher/launcher.go`） | 应改成 |
|---|---|---|---|
| bo-1 | `row()` 的 `item.status` 臂，句子那一句 | `ui.Text(c, item.Title).FontSize(t.FontSize).FontWeight(480).TextColor(t.TextMuted).SingleLine()`（实测浅 116 / 深 164） | `TextColor(tokens.TextStrong)`（实测浅 27 / 深 237）：一页的陈述（§4.7 的 `Emphasis` 档在本仓由 `emptyMessage` 的居中句承担；这一句的**盒子**是列表行的 30u 带，不动，只换墨） |
| bo-2 | 同臂，图标列 | `ui.Icon(c, glyph).Size(14, 14).TextColor(tokens.TextTertiary)` | `TextColor(tokens.TextSecondary)`：§4.7 注脚「图标次文字色」（`settingsui.emptyState` 的同一个数、同一条规则） |
| bo-3 | 同臂的**盒子** | `MinHeight(t.Space(launcherStatusUnits))`、`Padding(0, t.Space(2))`、`Gap(t.Space(2.5))`、28u 图标列、`Size(14, 14)` 的字形、`SingleLine()` | **一个像素不动**（高度链红线：这一行是 `RowInfo.Status` 的 30u 带） |
| bo-4 | 指针 / 键盘 / 编号 | 无底板、无指针态、`rowSelectable` 排除、`assignNumbers` 不给编号 | **不动**（行为契约红线） |

**行为契约零改动红线**：高度链（30u 带）、键盘模型（`rowSelectable` / `assignNumbers`）、
会话管理、设置写回、i18n 文案、命令接线，**全部不动**。本轮只改墨。

## 3 参考与合约的关系（必须写明，别当成随手改）

参考的 `.launcher-status`（`main:src/styles/launcher.css:1480-1491`）确实是
`color: var(--text-muted); font-weight: 480`，`clipboardStatusRow`
（`main:src/plugins/clipboard/mode.ts:52-55`）也确实走 `statusRowBase` 的 muted 注。
**这一轮显式偏离参考，并写进合约**：本仓的 §4.7 已经把「一页的陈述」定成 `TextStrong`
（P8-b 落在设置页、P13-bd 落在终端页、P13-be 落在插件窗口、P13-bg 落在捕获输出页），
而启动器这一页的陈述因为走的是**行**的代码路径（`item.status`）而漏在了参考的旧值上。
旧值 → 新值：`TextMuted` → `TextStrong`（句子）、`TextTertiary` → `TextSecondary`（图标）。

## 4 门槛命令与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（本轮动手前实测）：PASS **3198** / FAIL **0** / SKIP **113** / 测试函数 **2349**。
本轮新增 3 条 pin + 1 条 opt-in dump ⇒ 目标 PASS ≥ **3204**、SKIP **114**、函数 **2352**。

## 5 mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | 把 bo-1 改回 `t.TextMuted` | `TestTheOffPluginsNoteIsOnTheStatementsTooth`（对比 137 < 180）与 `TestTheNoteIsAboveThePagesQuietRegisters`（比占位句只高 0 级） |
| M2 | 把 bo-2 改回 `tokens.TextTertiary` | `TestTheNoteIconIsOneStepUnderItsSentence`（图标 − 句子 ≥ 40 且 ≤ 编… 实测句子 226 / 图标 145，差 81 级） |
| M3 | 把 bo-1 的字重 480 改成 580（冒充结果行标题） | `TestTheNoteKeepsTheListsOwnBand` 的**结构**那一半（AST/字面：句子仍是 `FontWeight(480)` 与 `FontSize(t.FontSize)`，不许跟结果行同齿） |

## 6 本轮为什么让它更轻、更有空间感

不是「更亮」，是**层级只由差说话**。同一页上，一句话要么是背景（占位句、分组标题），要么是
结论（这一页的答案）。此前这一页的结论与它的占位句同墨，用户读到的是「一张空卡 + 一句灰字」；
换到陈述齿之后，页面上只有一样东西是「答案」，其余都在它下面 ≥ 61 级（浅 226 − 145 = 81，
深 214 − 133 = 81）。层级不是靠加线、加底、加框做的——这正是轻便液态玻璃这一支的做法。
