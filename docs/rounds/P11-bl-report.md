# P11-bl 报告 · 捕获输出页的 `⌘N` 编号也按「读得全的视口」发

## 1. 根因（本轮实测）

`internal/launcher/output.go` 的 `assignOutputNumbers` 是搜索列表编号家族的**第二份实现**，
它比 P11-bb/P11-bi 收口的那份落后两步：

1. **闸门**：`if len(rows) > maxResultSlots { … }`。搜索列表的同一个闸门在 P11-bi 已被删掉，
   理由实测过：窗口把带子（搜索页 60u 字段带 / 输出页 42u 状态带）算在列表上方，**恰好十行**
   的列表也滚得动，`len(rows) == 10` 时闸门为假、视口一次不读、徽章钉在顶部。
2. **度量**：`a.OutputList.Visible()`。框架的实现（`mygo v0.7.0 ui/list.go:1484-1494`）判据是
   `y+ht > top && y < bottom` —— **还有一个像素**就算「可见」。P11-bb 已经为搜索列表判过这个
   答案不合格（用户原话「某一项滚出可见范围达到 30% 就算作不可见项」），输出页一直沿用。

后果与用户报的搜索页现象逐字相同：滚一下，**滚出屏幕大半的行还带着号**，后面的号跟着往前
跳一格（`1` 出现在读不到的行上）。

## 2. 落地

- **`internal/launcher/launcher.go`**
  - `numberedViewport` 现在只是一层转发；「哪几行可以带号」的判据搬进
    `viewportRange(count, rects, viewport)`（70% 起点、末端只要求「有一个像素」、循环封顶十枚
    ——**规则一个字没改**，`assignNumbers` 的调用点与行为零改动）。
  - 新增字段 `outputRowRects map[int]ui.Rect` / `outputListRect ui.Rect`（与 `rowRects`/`listRect`
    同形、同一时机填）。
- **`internal/launcher/output.go`**
  - `outputNumberedViewport` = `viewportRange(count, a.outputRowRects, a.outputListRect)`。
  - `assignOutputNumbers`：删掉闸门与 `Visible()`，只要上一帧留下几何就用视口，没有几何回落
    到整段（与 `assignNumbers` 同一个回退）。
  - `outputRow` 记录 `style.Bounds()`；输出列表建之前 `clear(a.outputRowRects)`、建之后
    `a.outputListRect = list.Bounds()`（与搜索列表的行盒/视口盒同一个时机：读上一帧的盒子）。
  - `showOutput`（P11-ar 的重置点）顺带清空行盒、归零视口盒：一份新的捕获输出不能拿上一份
    的几何发号。

**零改动**：列表高度链、行几何、`1`-`9`/`0` 的形状、只给 `Runnable()` 行发号、`⌘N` 的键接线、
会话管理、设置写回、i18n 文案。

## 3. V / B / S 台账

- **V = 0**：没有任何像素值/几何常数被重定基线。
- **B = 0**：产品行为按既有契约（70% / 末端像素 / 十枚）执行；既有测试**零改动**全绿。
  `TestOutputNumbersFollowTheOutputViewport` 的**度量手段**从 `OutputList.Visible()`（旧实现
  自己的读者）换成「行盒 ∩ 视口」这一手实测几何，断言强度不变且更严（现在会检查「滚走的行
  一个号都不许留」和「第一个可读行必须是 1」）。
- **S = 0**：没有结构变化（列表、行组件、几何链都没动；只多记了一对盒子）。

## 4. 门槛（本轮实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `GOOS=windows go vet ./...` | 净 |
| `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，FAIL 0 |
| `--- PASS` 计数 | **2450 → 2452** |
| `--- SKIP` 计数 | 49（本轮 +1，是 opt-in 的截图 dump） |
| 测试函数 | 1998 → **2001**（+2 pin、+1 dump） |
| 三平台 `go build ./...` | 全过 |

## 5. mutation（每条都实跑、看到红、还原后 `md5sum -c` 逐字节核对）

**M1** — 把闸门 `len(rows) > maxResultSlots` + `a.OutputList.Visible()` 原样加回
`assignOutputNumbers`。三条 pin 同时红，原始串：

```
--- FAIL: TestOutputNumbersFollowTheOutputViewport
    outputnumbers_test.go:168: after scrolling, the first visible output row 10 number = 2, want 1
      (map[9:1 10:2 11:3 12:4 13:5 14:6 15:7 16:8 17:9 18:0])
    outputnumbers_test.go:175: output row 9 scrolled away but kept its number: map[9:1 … 18:0]
--- FAIL: TestAnOutputRowMostlyScrolledAwayTakesNoNumber
    outputnumbers_test.go:216: row 0 is only 0.40 on screen but carries number 1
      (map[0:1 1:2 2:3 3:4 4:5 5:6 6:7 7:8 8:9 9:0])
--- FAIL: TestATenRowOutputListRenumbersWhenItScrolls
    outputnumbers_test.go:248: row 0 is 0.65 on screen but still numbered 1
      (map[0:1 1:2 2:3 3:4 4:5 5:6 6:7 7:8 8:9 9:0])
```

**M2** — `viewportRange` 的 `share >= numberVisibleShare` 改成 `share > 0`（退回「一个像素」）。
搜索列表的旧 pin 与输出页的新 pin 一起红：

```
--- FAIL: TestATenRowListRenumbersWhenItScrolls
    numberband_test.go:267: row 0 is numbered 1 while only 0.53 of it is on screen
--- FAIL: TestOutputNumbersFollowTheOutputViewport
    outputnumbers_test.go:168: after scrolling, the first visible output row 10 number = 2, want 1
--- FAIL: TestAnOutputRowMostlyScrolledAwayTakesNoNumber
    outputnumbers_test.go:216: row 0 is only 0.40 on screen but carries number 1
--- FAIL: TestATenRowOutputListRenumbersWhenItScrolls
    outputnumbers_test.go:248: row 0 is 0.65 on screen but still numbered 1
```

**M3** — `outputNumberedViewport` 改读搜索列表的 `a.rowRects`/`a.listRect`（两个表面共用一个
盒子）。五条输出页 pin 红：

```
--- FAIL: TestOutputRowsCarryTheNumberedFamily
    outputnumbers_test.go:53: output row 2 number = 0, want 3 (map[0:1 1:2])
--- FAIL: TestOutputStatusRowsAreNotNumbered
    outputnumbers_test.go:83: the second runnable row number = 0, want 2 (map[1:1])
--- FAIL: TestOutputNumbersFollowTheOutputViewport
    outputnumbers_test.go:171: output row 0 is off screen but still numbered: map[0:1 1:2]
--- FAIL: TestAnOutputRowMostlyScrolledAwayTakesNoNumber
--- FAIL: TestATenRowOutputListRenumbersWhenItScrolls
```

三条全部还原后 `md5sum -c /tmp/P11-bl.md5` 三个文件逐一 `成功`，`touch` 后重跑全包测试绿
（`ok floter/internal/launcher 10.676s`）。

## 6. 视觉证据（本轮实测）

两张一对，同一帧同一个滚动位置（`row 0` 只剩 **0.65** 在屏幕上）：

- `/tmp/floter-P11-bl-output-before-light.png` / `-before-dark.png`
- `/tmp/floter-P11-bl-output-after-light.png` / `-after-dark.png`

**我在这两张图上实测看到的**：整幅图只有一处不同——右侧徽章列
`x=671..703`、`y=103..472`，深浅各 **500 px** 差异（包围盒完全一致）。修前那一列从第一行
（已滚掉三分之一）起是 `Ctrl+1 Ctrl+2 … Ctrl+9 Ctrl+0`，十行全带号；修后**最上面那一行没有
徽章**，`Ctrl+1` 落在它下面第一行完整可见的行上，一直排到 `Ctrl+9`（十枚预算内的第十行不在
视口里，因此没有 `Ctrl+0`）。行文字、状态带、面板墨色一个像素没动。

dump 里同时打出的实测事实：
`row 0 is 0.65 visible; numbers map[1:1 2:2 3:3 4:4 5:5 6:6 7:7 8:8 9:9]`（修后）
vs `map[0:1 … 9:0]`（修前）。

## 7. 仍待

1. 老拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` / `AccentEdge` 3:1 /
   便条投影 / 拖拽手柄接不接指针 / `Permissions` 行的词 / `configDiscardBar` 12u margin /
   终端顶边失败带覆盖画布顶部两行）。
2. 编号家族到此**两个表面都读同一份 `viewportRange`**：`grep -rn 'ui.List(c.Key' internal/launcher`
   全包只有两个列表（`launcher.results`、`launcher.output.list`），两个都已接同一份判据。
3. `.pending-bugs.md` 的 P11-bi「仍待」第 2 条已由本轮收口，别再当 TODO 重领。
