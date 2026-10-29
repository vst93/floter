# P11-bl 规格 · 捕获输出页的 `⌘N` 编号也按「读得全的视口」发（P11-bb/bi 的同族漏网）

## 意图

搜索列表的 `Super+N` 编号在 P11-bb/P11-bi 已改成「行盒 ∩ 列表视口 ≥ 70% 才有号」；**同一个
问题在捕获输出页原样还在**——`assignOutputNumbers` 读的是框架 `OutputList.Visible()`
（判据是「还有一个像素在框里」），且外面还套着 `len(rows) > maxResultSlots` 的闸门。于是
输出页滚一下就出现用户报过的同一个现象：**滚出屏幕的行还带着号、号跟着跳**。

为什么现在做：`.pending-bugs.md` 的 P11-bi「仍待」第 2 条明写「同族不同表面（未做）：
`assignOutputNumbers`（`output.go:826`）仍是 `OutputList.Visible()` + `> maxResultSlots`
闸门」；用户 2026-10-28 定的优先档是「界面质感与交互（含用户报的 bug）」，而这条就是
用户原话「现在滚动时快捷键跟着跳，且超出屏幕的项也显示快捷键」在另一个表面上的同一处。

## 证据（本轮实测，行号已核对）

- `internal/launcher/output.go:824-853`（`assignOutputNumbers`）：闸门
  `if len(rows) > maxResultSlots { … a.OutputList.Visible() … }`。
- 框架 `Visible()`（`mygo v0.7.0 ui/list.go:1484-1494`）的判据是
  `y+ht > top && y < bottom`——**一个像素**，正是 P11-bb 在搜索列表里判为不可用的那个答案。
- 闸门在这个表面同样会漏：搜索页的列表上方有 60u 字段带，输出页的列表上方有 42u 状态带
  （`internal/launcher/output.go:512-520` 的 `Padding(header, …)`），所以**恰好十行**的输出
  列表在 `frameHeight(maxResultSlots)`（`internal/launcher/view_test.go:42`）下也滚得动，
  而闸门为假、视口一次不读。
- 搜索列表的判据本身只有一份（`internal/launcher/launcher.go:669-700` 的
  `numberedViewport` + `visibleShare` + `numberVisibleShare`）；本轮把「读哪个盒子」这一层
  抽出来共用，规则（70%、上下两端读法不同、循环封顶十枚）一个字不改。

## 逐条落地项

1. **位置** `internal/launcher/launcher.go:669`（`numberedViewport`）与其上方字段注释
   （`:445-458`）。
   - 现状：`numberedViewport` 把「盒子从哪来」和「怎么量」写在一起，只有搜索列表能读。
   - 改成：抽出 `viewportRange(count int, rects map[int]ui.Rect, viewport ui.Rect)`，
     `numberedViewport` 转发它（`a.rowRects`/`a.listRect`）；`assignNumbers` 的调用点与
     行为零改动。
2. **位置** `internal/launcher/launcher.go:455-458`（`outputNumbers` 旁边）。
   - 现状：输出页只有 `outputNumbers`，没有行盒/视口盒。
   - 改成：加 `outputRowRects map[int]ui.Rect` 与 `outputListRect ui.Rect`，注释写清它们是
     搜索列表 `rowRects`/`listRect` 的同形答案（同一份 `viewportRange` 读）。
3. **位置** `internal/launcher/output.go:514-523`（输出列表的构建）与 `:829`（`outputRow`）。
   - 现状：`ui.List(c.Key("launcher.output.list"), …)` 的盒子无人保存；行盒无人保存。
   - 改成：列表建之前 `clear(a.outputRowRects)`，建之后 `a.outputListRect = list.Bounds()`；
     `outputRow` 里 `style.Bounds()` 记进 `a.outputRowRects[i]`（与 `rowBody` 同一行位置、
     同一时机：上一帧的盒子，首帧为空矩形由 `viewportRange` 跳过）。
4. **位置** `internal/launcher/output.go:824-853`（`assignOutputNumbers`）。
   - 现状：闸门 `len(rows) > maxResultSlots` + `OutputList.Visible()`。
   - 改成：删闸门与 `Visible()`；只要上一帧留下几何就用 `a.outputNumberedViewport(len(rows))`，
     没有几何回落到整段（循环仍封顶十枚）。形状（`1`-`9`、`0`、只发 `Runnable()` 行）不动。
5. **位置** `internal/launcher/output.go:110-119`（`showOutput` 的重置）。
   - 现状：只重置 `Output`/`OutputList`（P11-ar）。
   - 改成：同一处把 `outputRowRects` 清空、`outputListRect` 归零——一份新的捕获输出不能拿
     上一份的行盒去发号（与 P11-ar 同一条规则：新内容从自己的开头开始）。

**行为契约零改动红线**：列表高度链、行几何、键盘模型（`assignOutputNumbers` 的键接在
`launcher.go:794`，形状不变）、会话管理、设置写回、i18n 文案、命令接线；本轮的十行/七十
等价数字一个不动。

## 门槛命令 + PASS 基线（本轮实测）

- 改前基线：`go test -count=1 ./...` 全绿，`--- PASS` **2450**，测试函数 **1998**，
  FAIL 0（`grep -c -- "--- PASS"` 与 `grep -rhn '^func Test' internal/ cmd/ --include=*_test.go`）。
- 门槛：`gofmt -l cmd internal` 空 · `go vet ./...`（linux/windows/darwin）净 ·
  `go test -count=1 ./...` 全绿 · PASS 计数只许涨 · 三平台 `go build ./...` 过。

## mutation 计划

- **M1**：把闸门 `if len(rows) > maxResultSlots` + `a.OutputList.Visible()` 原样加回
  → 新 pin `TestATenRowOutputListRenumbersWhenItScrolls` 必须红（十行列表滚过之后 row 0
  仍带着号 1）。
- **M2**：`viewportRange` 的 `share >= numberVisibleShare` 改成 `share > 0`（退回「一个像素」）
  → 新 pin `TestAnOutputRowMostlyScrolledAwayTakesNoNumber` 必须红（被裁掉大半的首行仍领号 1）。
- **M3**：`outputNumberedViewport` 改成读搜索列表的 `a.rowRects`/`a.listRect`
  → 输出页的几何 pin 必须红（输出页的视口与搜索列表不是同一个盒子）。
