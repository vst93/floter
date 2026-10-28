# P11-ai 规格 — 指针不动就不算"进入"（R21 的入口边只认指针自己的动作）

## 意图

R21「指针进入即选中」在本仓被实现成「指针底下那一行变了就选中」。**行变了有两种原因**：
指针动了，或者**内容动了**。第二种在键盘步进时每跨过窗口末行就发生一次——于是键盘一步步
往上走，被滑到静止指针底下的那一行逐步把选中抢回去。本轮把入口边改成只认指针自己的动作，
并把「用户自己滚的」与「app 替键盘滚的」分开。

为什么现在做：用户 2026-10-28 定的优先级是「界面质感与交互逻辑 + 用户报的 bug 优先」，
P11-ah 的「仍待」第 2 条就是这个交互 bug，且当轮留了实测数字（15 次 Down 只前进 8 行）。

## 证据（本轮实测，不是旧报告数字）

- `internal/launcher/launcher.go:2250`（本轮行号）——`if row.Hovered() { a.hoverFrameRow = i; if i != a.hoveredRow && … { a.Selected = i; a.pointerChose = true } }`。
  判据只有「下标变了」，没有任何「指针动了」的信号。
- 同一个 `View` 的 reveal 段（`launcher.go:1528` 一带）在选中变化时发
  `a.List.ScrollIntoView(row)`——**注释写的是键盘的规则**——列表因此滚动，行在静止指针底下换人。
- 复现（本轮新增 pin 的失败输出，修前）：
  ```
  --- FAIL: TestTheKeyboardWalksTheListUnderARestingPointer
      pointeridle_test.go:60: fifteen arrow steps from row 3 landed on 6, want 18:
      a resting pointer took the choice back
  ```
  即 15 步只走了 **3** 行（P11-ah 记的 8 行是它自己那组数字；本轮这组是 20 行列表 + 步进 15）。
- 参考侧为什么能给答案：`LauncherResults.tsx:859-862` 的读法是 `onPointerEnter`——**一个指针事件**。
  浏览器不会因为内容在静止光标下滑过就补发 `mouseenter`（打开在光标下的窗口也不发），
  所以「内容动了」在参考里根本不是一个入口。

## 逐条落地项

### 1. 每帧（每个 build pass）读一次指针位置与列表首行

位置 → `internal/launcher/launcher.go` 的面板（`panel := ui.Box(c).Fill()…`，`trackPointer` 之前是匿名表达式）→
现状：面板不知道自己被谁指着，也没有任何 pass 级指针状态 →
应改成：面板具名，`panel.Children` 第一件事是 `a.trackPointer(panel)`：

- `panel.PointerPosition()`（`mygo` `ui/input.go:1265`，终端浮条 P11-af 已用同一 API）给出
  相对面板的坐标与「是否在面板内」。
- `moved`：**第一次读到指针（`pointerSeen` 为假）不算动**——窗口开在光标下不该把选中交给
  它压着的那一行；此后坐标变化或进出面板都算动。
- `scrolled`：`a.List.Visible()` 的首行与上一 pass 不同 = 内容在指针底下移动了。
- `a.appScrolled` 由 reveal 置位、列表静下来才复位；它站着时内容移动算 **app 的**，
  否则算 **用户的**（滚轮）。
- 结论写进 `a.hoverFresh`：指针的位置是用户自己给的。

### 2. 入口边加一道门

位置 → `launcher.go` 的 `rowBody` 行尾 R21 段（本轮 `:2250` 一带）。
现状：`i != a.hoveredRow` 就交选中。
应改成：`a.hoverFresh && i != a.hoveredRow && rowSelectable(item) && item.Run != nil`。

### 3. reveal 自报家门

位置 → `launcher.go:1528` 的 `a.List.ScrollIntoView(row)`。
应改成：调用后 `a.appScrolled = true`（键盘这一滚不是指针的）。

## 行为契约零改动红线

- **滚轮不动**：`TestTheWheelScrollsTheListUnderThePointer` 断言滚过之后
  `a.Selected == a.hoveredRow`——滚轮**是**指针手势（用户用同一只手瞄），滚过的行逐行交权。
  本轮不许动它，测试零改动。
- 键盘模型、高度链、会话管理、设置写回、i18n 文案、命令接线零改动。视觉零改动（无墨、无面）。
- R21 的「静止指针不抢键盘」原行为保留（`TestARestingPointerDoesNotFightTheKeyboard` 零改动）。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                      # 空
go vet ./...                               # 净
GOOS=windows go vet ./...                  # 净
GOOS=darwin  go vet ./...                  # 净
go test -count=1 ./...                     # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
```

基线（P11-ah 收尾实测）：**PASS 2360**、SKIP 25、FAIL 0、测试函数 1927。
本轮新增 3 条 pin + 1 条 opt-in dump → 函数数 **1931**、PASS 应 **2363**、SKIP 26。

## mutation 计划（≥2 条，实跑）

1. 把入口边的门去掉（`a.hoverFresh &&` 删掉）→ `TestTheKeyboardWalksTheListUnderARestingPointer`
   必须变红（15 步走不完）。证明门是这条 pin 的唯一原因。
2. 把 `a.appScrolled = true` 删掉 → 键盘滚的那一滚被读成「用户的」→ 同一 pin 变红。
3. 把 `moved` 的首帧判断反过来（`!a.pointerSeen` 时也当作动过）→
   `TestThePointersWordCountsAgainOnceItMovesAfterAScroll` 不该红，但
   `TestThePointerTakesTheSelectionOnEntry` 仍绿——这条只作对照，不进台账。
