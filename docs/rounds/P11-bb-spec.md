# P11-bb · 编号家族只落在「读得全的那一段」+ 按住修饰键时底部常驻终端提示

## 意图（一句话）

搜索列表的 `Super+1…0` 徽章**只发给看得见的那几行**：滚出可见范围 **≥30%** 的行不再领号，
编号从**第一行读得全的结果**开始数 1；按住修饰键（`Super`）时底部那条带**常驻**一行
`Super+Enter 快捷进入终端`。

## 为什么现在做（证据）

用户实测反馈（2026-10-28，最高优先级，`.pending-bugs.md` 顶部）：

> 「Super+N 按键在滚动时的渲染是有 bug 的，应该从列表的可见项开始赋予 1，一直到 9 或者 0」
> 「滚动时快捷键跟着跳，且超出屏幕的项也显示快捷键」
> 「当按着 M 键时，底部固定显示一行「Super+Enter 快捷进入终端」」
> 「需要设一个阈值，比如某一项滚出可见范围达到 30% 就算作不可见项」

现状（`internal/launcher/launcher.go`，本轮实测行号）：

- `assignNumbers`（`launcher.go:582`）把 `a.List.Visible()`（`launcher.go:611`）当作
  可见区间。`ui.ListState.Visible()` 的定义是「**任何一个像素**还在框里」
  （`mygo/ui/list.go:1483-1492`：`y+h > top && y < bottom`）。于是一行只露出 2px 也算
  「可见」，拿到 `1`；它的徽章在行中线，早被裁掉——用户看到的第一行徽章是 `2`，
  按 `Super+1` 跑的却是屏幕外那一行。参考实现 `main:src/launcher/result-budget.ts:768-810`
  的 `visibleRowRange` 明写「跳过被上边缘裁掉的行（`start` 是第一行读得全的）」，
  本仓移植时把这条丢了。
- 底部那条带（`launcher.go:3013` 的 `statusBand` / `launcher.go:2933` 的 `feedbackRow`）
  只有 toast 与关机问句两个句子，按住修饰键时什么都没有；而按住修饰键在**有查询**时
  不画 `system-terminal-bare` 行（R64，`launcher.go:3164` 注释），用户于是没有任何
  「`Super+Enter` 进终端」的提示。

## 落地项

### A. 编号家族按「读得全」判可见（30% 阈值）

- 位置：`internal/launcher/launcher.go` `assignNumbers`（`:582-631`）。
- 现状：`first, last = a.List.Visible()`，区间端点允许「一个像素」的行。
- 应改成：用**上一帧的几何**（行盒 ∩ 列表视口盒）算每行的可见比例；比例 `< 0.7`
  （即被裁掉 ≥30%）的行不领号；`first/last` 是比例达标的第一行/最后一行。
  几何来源：
  - 行盒：`rowBody` 里 `ui.Row(...)` 的 `Element.Bounds()`（框架给的是**上一帧**的矩形，
    `mygo/ui/api_gen.go:3026` 的注释就是这么写的），逐帧写进 `App.rowRects`。
  - 视口盒：`View` 里 `ui.List(...)` 返回的元素的 `Bounds()`，写进 `App.listRect`；
    **顶边再抬到 `listEdge`**（字段行 + 呼吸 + chips）——搜索框是浮层，行从它底下滚过去，
    那一条带在不透明字段后面，读不到。
  - 没有任何几何（第一帧、`Bounds()` 还是空盒）时**回落到旧规则**（`List.Visible()`），
    首帧不会出现「一个徽章都没有」。
- 不动：`resultCount > maxResultSlots` 那道闸（R72，短列表整表编号）保留；
  编号仍然是 `1..9` 然后 `0`，`runResultDigit` 的读法不变。

### B. 按住修饰键时底部常驻一行终端提示

- 位置：`launcher.go:3013`（`statusBand`）、`:2933`（`feedbackRow`）、`:1170`（`Geometry.Feedback`）、
  `:3022`（`FeedbackVisible`）。
- 现状：带子的判据是 `toast != "" || pendingPower != ""`。
- 应改成：加第三个判据 `a.heldHint()`（`modifierHeld && output == nil && !pluginModeActive()`）；
  优先级 `pendingPower > toast > heldHint`（P11-az 的「一条带一个句子」不变）；
  句子由 `rowKeycap(a.enterBadge())` + `copy.HeldTerminalHint` 组成（`Super+Enter` 两块键帽
  + 「快捷进入终端」/「Open a terminal」）。
- 不动：带子高度仍是 `launcherStatusUnits`（30u），窗口高度链、列表底部让位、键盘模型零改动。

### C. i18n

- `internal/i18n/i18n.go` 加 `HeldTerminalHint`；`copy.go` 的 en/zh 各一句。
  `deadcopy_test` 要求字段在包外被读——`feedbackRow` 读它。

## 行为契约零改动红线

高度链（`Geometry`/`height.go`）、键盘模型（`fieldkeys.go`/`appChordKeys`）、会话管理、
设置写回、i18n 既有文案、命令接线：**一行不改**。本轮只动墨、带子的第三个句子与
编号的可见判据。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2427，只许涨
三平台 build
```

## mutation 计划（≥2）

1. 把 `numberVisibleShare` 改成 `0.0`（= 任何像素都算可见，即旧行为）→ 新 pin
   `TestTheNumberedFamilyStartsOnARowYouCanRead` 红（第一行徽章是 2 而不是 1）。
2. 把 `heldHint()` 的返回值改成 `false` → 新 pin `TestHoldingTheModifierKeepsTheTerminalHint`
   红（带子高度 0、句子不在）。
3. （可选）`statusBand` 里去掉 `heldHint()` → 同一 pin 的另一臂红（窗口长了 30u 但列表
   没让位，最后一行压到带子底下）。
