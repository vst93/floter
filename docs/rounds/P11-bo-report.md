# P11-bo 报告 · 按住修饰键的「进入终端」只剩底部那一条带

## 1. 根因（file:line 为修前）

用户 F4-3 的验收句是「**固定底部且不重复**」。P11-bb 落地时把这条线做成了两半：

- **底部常驻带**：`internal/launcher/launcher.go:3141` 的 `heldHintRow`，
  `Absolute().Bottom(a.footerBand)`，三十单位高，画 keycap + `HeldTerminalHint`；
- **列表里的一行**：`internal/launcher/search.go:856-862` 在 `Results()` 末尾
  `if item, ok := a.heldTerminalItem(); ok { out = append(out, item) }`，
  `heldTerminalItem`（`launcher.go:3331`）只在**空字段**成立。

`heldHint()`（`launcher.go:3177`）的判据是
`modifierHeld && output == nil && !pluginModeActive()`——**不看查询**，所以空页面上
两半同时在屏上：列表最后一行 `Open Terminal / A blank session — runs nothing`
（带选中色与 `Ctrl ↩` 徽章），底下又一条 `Ctrl ↩ / Open a terminal session`。
同一枚和弦画了两遍 = 用户报的「重复」。

实测证据（headless，修前）：

```
tt.HoldModifiers(ui.Cmd); tt.Frame()
texts = ["Launcher results" "Recently launched" "Recent 00" "recent application 00"
         "Ctrl" "1" … "Open Terminal" "A blank session — runs nothing" "Ctrl" "↩"
         "Search" "Type to search" "Settings" "Ctrl" "↩" "Open a terminal session"]
selected = 4 = 最后一行（那一行）
```

高度上也是双份：`launcher.go:1268` 的 `Feedback: … || a.heldHint()` 已经为空页面的
带预留了 30u，而行又被算进 `Geometry.Rows`，于是窗口为同一条线付了「一行 + 一带」。

## 2. 落地

- `internal/launcher/search.go`：`Results()` 不再追加持有行（注释写清「它不是本
  列表的一行，是面板的底部带」）。
- `internal/launcher/launcher.go`：
  - 删 `heldTerminalItem`（连同 `bareTerminalRowID` 常量）；
  - 新增 `openBareTerminal()`：`a.Actions.OpenTerminal()`，不经任何行/索引；
  - `View` 的快捷键改为 `if a.heldHint() && c.Shortcut(ui.Cmd, ui.KeyEnter) {
    a.openBareTerminal() }`——空字段与有查询都能用（用户原文「按 Super+Enter 即可
    进入终端」）；
  - `syncHeldModifier` 缩到「按下且有查询 → 交给 action bar；松开且有查询 → 回页面
    默认」，删 `heldRowShown`/`heldRowKey`/`holdHeldRowSelection`；
    `heldRowKey` 改名 `listShapeKey`（只剩 bar 用）；
  - `assignNumbers`、`actionbar.go` 的 `defaultSelection` 去掉对那一行的跳过（行没了，
    判据也就没了）；
  - 选择滚入视口的分支不再为「持有一行」开特例（那一行不再会长出窗口）。
- `internal/launcher/heldHint` 注释改成「这是唯一一条」，判据零改动：空页面与有查询
  都画带。
- `internal/i18n`：`BareTerminal` / `BareTerminalHint` 随行一起死掉，删字段与两份
  文案（`TestNoDeadCopy` 先红后绿）。

## 3. V / B / S 台账

- **V = 0**：没有一条像素数值需要重定基线。带自身的墨、几何、keycap、
  `Absolute().Bottom(footerBand)` 一个字节没动。
- **B = 7（行为重定基线，全部由用户 F4-3 原话授权）**：
  `TestTheHeldRowIsTheEmptyPagesAlone` → `TestTheHeldLineIsTheBandsAndNeverARow`；
  `TestHeldModifierBareTerminalRow`、`TestHeldModifierRowCarriesTheTerminalGlyph`、
  `TestHeldModifierRowIsNotScrolledIntoView`、`TestTheHeldRowWearsTheSelectedFace`
  （改为「带**不**上选中色」）、`TestTheModifierEdgesStillOwnTheSelection` →
  `TestTheModifierEdgesMoveNoRowOnTheEmptyPage`、`TestTheActionBarTakesNoNumber`
  的第二臂（从「那一行不领号」改成「按下修饰键不加行、不加号」）；
  shell 侧 `TestHeldModifierRowGrowsTheWindowAndRecedes`、
  `TestTheHeldRowIsTheEmptyPagesLastLine` → `TestTheHeldBandIsTheEmptyPagesLastBand`、
  `TestHeldModifierReleaseRestoresThePageDefault`、`TestTheHeldRowOnlyExistsOnTheEmptyField`、
  `TestTheHeldRowIsTheLastLineOfTheShippedList`、
  `TestTheTerminalSurfaceAndTheHeldActionAreTwoRows`。
  **没有一条是放宽断言换绿色**：每一条都换成新契约的正面/反面断言（行不存在、带在屏
  一次、带在帧底、选择不动、和弦能跑）。
- **S = 0**：元素树结构没动（带本来就在那儿）；删掉的是列表里的一行。

## 4. 门槛（本轮实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 三份净 |
| `go test -count=1 ./...` | 绿，FAIL 0 |
| `--- PASS` | **2454**（基线 2454；本轮先修掉 `internal/i18n` 的死文案红，再 +1 新 pin） |
| `--- SKIP` | 51 |
| 测试函数 | 2005 → **2006**（+1 新 pin；其余是同位置改写） |
| 三平台 `go build ./...` | 全过 |

## 5. 变异（实跑，逐条变红后还原，`md5sum -c` 核对）

- **M1**：把行加回 `Results()`（空查询 + 修饰键 `append Item{ID:
  "system-terminal-bare", Title: "Open Terminal"}`）
  ```
  --- FAIL: TestTheHeldLineIsTheBandsAndNeverARow (0.03s)
      actionbarfooter_test.go:627: the held modifier changed the list: [… system-terminal-bare], want the recents alone […]
  --- FAIL: TestHeldModifierBareTerminalRow (0.04s)
      filters_test.go:352: the modifier changed the list: …|system-terminal-bare, want …
  --- FAIL: TestTheHeldBandIsTheEmptyPagesLastBand (0.10s)
      heldmodifier_test.go:99: the modifier added 1 row(s) to the empty page: […]
  --- FAIL: TestTheHeldRowIsTheLastLineOfTheShippedList (0.04s)
  ```
- **M2**：`heldHint()` 加 `&& strings.TrimSpace(a.Query) != ""`（带只在有查询时画）
  ```
  --- FAIL: TestTheHeldRowWearsTheSelectedFace/light|dark
      actionbarfooter_test.go:589: the held line was not drawn: […]
  --- FAIL: TestTheHeldLineIsTheBandsAndNeverARow (0.02s)
      actionbarfooter_test.go:636: the held line is on screen 0 times, want exactly 1
  --- FAIL: TestHeldModifierBareTerminalRow (0.01s)
  ```
- 还原：`md5sum -c` → `search.go: 成功`、`launcher.go: 成功`；`touch` 后复跑三条
  pin 全绿（避开测试缓存）。

## 6. 视觉证据（说明，不糊弄）

本轮是**交互/结构**轮，不是墨与面的轮：带的像素、keycap、位置一个字节没动，动的
是「列表里那第二份」消失。所以没有新的 `FLOTER_PNG` dump；上一轮 P11-bb 的带截图
`/tmp/floter-P11-bb-*.png` 仍然是本轮修后的带（同一段代码，未改）。想看差别只需看
**列表最后一行是否叫 `Open Terminal`**：修后它不再存在，屏幕上只剩底部那条
`Ctrl ↩ / Open a terminal session`。

## 7. 仍待（P11-bo）

1. 老拍板项原样有效（`.pending-bugs.md`「需用户拍板」表）。
2. 与参考的有意分歧：参考（`main`）在空字段**会**把这条线追加成列表最后一行
   （R64 `bareTerminalPlacement`），本仓按用户 F4-3 改成了「只在底部带、永远不是行」。
   想要回参考那份（行 + 选择）需用户拍板。
3. 同族 perf 项仍在：`footerSpinner`（`internal/launcher/output.go:312`）与设置页
   `actionSpinner`（`internal/settingsui/rows.go:567`）还是构建期 `Rotate` + `Loop`，
   只在加载中跑；搬 `Draw`+`p.Now()`+`AnimationFrameRate` 时 `Rotate` 是元素变换，
   像素会动（V 类），下一轮可领。
