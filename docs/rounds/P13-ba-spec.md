# P13-ba · 扩展页行忙时，整簇动作键还亮着灯

## 意图（一句话）

`actionButton` 把 `extensionActionFace` 的 `disabled` 那格写死成 `false`，于是全仓
**6 处** `.Disabled(...)` 的胶囊绕过了 P13-s 建好的禁用面——它们保留「这块面可以被碰」
的受光边（`LitFace` 的 `LensRim`）与手型指针，只是被框架的 0.5 淡了一道。

## 为什么现在做（证据）

- `internal/settingsui/rows.go:489` `actionButton` → `:534` `extensionActionButton` →
  `:546` `extensionActionFace(c, height, radius, false)`：**`disabled` 参数在这里被写死**。
- `internal/settingsui/rows.go:533`（同文件 `extensionActionFace` 的注释）已经把契约写死：
  「禁用面的材质换成 `DimFace`——底对侧还在，**受光边熄了**（受光边是「这块面可以被碰」的记号）」。
  这条只对 `busyActionButton` / `extensionGlyphActionButton(..., true)` / `configOpenToolRow`
  生效；`actionButton` 这条最常走的路（扩展行的一簇键 + 导入导出 + 检测到的工具行）
  一个都没走到。
- 6 个调用点：`settingsui.go:1977`（导出）、`:1983`（导入）、`:2757`（行忙时整簇
  `rowAction`）、`:2773`（Run 不可用 / 行忙）、`:3564`（`repairAction` 被别的操作锁住）、
  `:5866`（`detectedAction` 无可执行项）。
- 实测（`dimProbe`，本轮新增 pin 的 before 读数）：禁用胶囊顶内线仍**高于面**（见报告数字）。

这与 §3 的判据直接冲突：「受光边是「这块面可以被碰」的记号」。一个因为**别的**操作在飞
而锁住的键，戴着这枚记号，用户就会去按它。

## 落地项（位置 → 现状 → 应改成）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `rows.go` `actionButton` 旁 | 只有「永远可用」这一条路，`disabled=false` 写死在 `extensionActionButton` | 新增 `disabledActionButton(c, label)`：同一枚胶囊，`extensionActionFace(..., true)` + `.Disabled(true).Cursor(ui.CursorDefault)` |
| 2 | `settingsui.go:1977/1983`（`transferRow`） | `a.actionButton(...).Disabled(busy)` | `busy` 时走 `disabledActionButton`，否则 `actionButton` |
| 3 | `settingsui.go:2757`（`rowAction`） | 建完再 `.Disabled(true)` | 同上 |
| 4 | `settingsui.go:2773-2778`（Run） | 建完再 `.Disabled(true)`（两次） | 同上；`:Disabled` 从 `Tip` 那一行移到建面时 |
| 5 | `settingsui.go:3564-3569`（`repairAction`） | 建完再 `.Disabled(true)` | 同上 |
| 6 | `settingsui.go:5866`（`detectedAction`） | `a.actionButton(...).Disabled(disabled)` | `disabled` 那一臂走 `disabledActionButton` |
| 7 | 新 pin | — | `disabledpill_test.go`：①复用 `assertDisabledFace`（面还有台阶 / 顶内线不亮 / 底对侧还在 / 只淡一道）×深浅两套；②普查：`internal/settingsui` 的非测试源码里 `actionButton(` 不得与 `.Disabled(` 同行 |

## 行为契约零改动红线

- 只改**墨与面**：胶囊的 `MinHeight` / `Padding` / `Radius` / `Gap` / 字号 / 标签文案
  一个字段不动（`extensionActionFace` 的几何由调用方传，本轮不动）。
- 点击语义不变：`Disabled` 的指针拒绝、`Clicked()` 的返回、`.Cursor` 的取值只从
  「手型」变成「默认箭头」——那正是参考实现 `:disabled { cursor: default }` 的那一句。
- 不改 `busyActionButton`（in-flight 那枚）与 `extensionGlyphActionButton(..., disabled)`
  这两条已经合阶的路。

## 门槛

`gofmt -l cmd internal` 空；`go vet ./...` 净；`GOOS=windows/darwin go vet ./...` 净；
`go test -count=1 ./...` 全绿；`grep -c -- "--- PASS"` 只许涨；三平台 `go build`。
基线（P13-az 实测）：PASS **3117** / FAIL 0 / SKIP 102 / 测试函数 **2297**。

## mutation 计划（≥2）

1. `disabledActionButton` 里把 `extensionActionFace(..., true)` 改回 `false`
   → `TestTheRowLockedActionPillTurnsItsCrownOff` 的深/浅两套同时红（顶内线亮）。
2. `rowAction` 的 busy 臂改回 `a.actionButton(c, label)` + `.Disabled(true)`
   → 普查那条 pin 红（点名 `settingsui.go:NNN`）。
3. `disabledActionButton` 去掉 `.Cursor(ui.CursorDefault)`（用光标普查 / 或删掉
   `.Disabled(true)`）→ 行为 pin 红（`Clicked()` 在禁用后仍为真）。
