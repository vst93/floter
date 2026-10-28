# P11-as 规格：外观抽屉里的宽控件占自己的一行

## 意图

终端外观抽屉（`internal/terminalui/drawer.go` 的 `drawerGrid`）里，凡是「三选一」这类**比一个格
子还宽**的控件，都不许再往邻格里溢出——让它们（光标形状、终端内边距、调色板预览）占自己的一
行，控件铺满格子，而不是由内容决定宽度。

## 为什么现在做

用户 2026-10-28 定的顺序：界面质感与交互逻辑 + 用户报的 bug 优先。F1/F2/F3 已由 P11-y/z/aa 收
完，合约 §6 只剩 `⌘K` 动作面（拍板），上一轮「仍待」第 1 条也是拍板项。于是本轮在「界面质感
与交互」这一档自己找，找到的是**画面上两段字压在同一块像素**。

证据（`ui.Tester` 实测，先复现后改；`ui.Find` 给的是绘制盒）：

| 表面 | 窗口 | 压在一起的两段字 | 重叠 |
| --- | --- | --- | --- |
| 光标形状那一行 | 900×600 | `Underline` `{509.3, 371, 78, 30}` 压 `Blinking cursor` `{582.7, 375, 111, 22}` | 4.7×22u |
| 调色板那一行 | 720/900×600 | `Palette preview` `{286, 465, 148, 88}` 压 `Copy on select` `{410, 498, 102, 22}` | 24×22u |
| 终端内边距那一行 | 1100×600 | `Relaxed` `{1011, 384, 65, 30}` 压抽屉自己的关闭钮 `{1060, 381, 28, 28}` | 16×25u |

## 根因

`drawerGrid` 是一个 `Wrap()` 的 Row，每个格子按**行自己的 `MinWidth`**（`drawerCellFloor` = 参考
的 `minmax(230px, 1fr)` = 57.5u）参与折行，折完每一行只剩「本行所有 floor 之和」的宽度——多出来
的空间不会再分给某个格子。而 `drawerRow` 的布局是 `label(Shrink 0) + slot(Grow 1, Justify End)`：
控件按**自己内容的宽度**画，格子给多少它不管。于是「标签 + 三段选择器」这种内容比格子宽的行，
就从格子右边缘**画到邻格里去**：邻格那行的标签落在同一块像素上。

这不是「浮层挡住了」也不是「预留高度不够」，是行内自带的内容宽度越过了容器给的宽度——参考侧
没有这个形状，因为参考的 `.terminal-settings__control { min-width: 0 }` 让控件**可以收缩**进格
子（`main:src/styles/terminal.css:677-683`），而 `.terminal-settings` 的 `repeat(auto-fit,
minmax(230px, 1fr))` 只会因为 floor 折行、不会让内容溢出。

## 落地项

1. `internal/terminalui/drawer.go` 新增 `drawerRowStacked(c, minWidth, label, build)`：标签占一行、
   控件占下一行并 `FillWidth`。控件因此**没有由内容决定的宽度**可溢出。
2. 光标形状行（`control.AxisCursorShape`）：`drawerRow` + `control.Segmented` →
   `drawerRowStacked` + `control.SegmentedFill`（三段按格子均分，正是设置页同一个选择用的那
   一形）。
3. 终端内边距行（`control.AxisPadding`）：同上（它也是三选一，实测在 1100 宽压住关闭钮）。
4. 调色板行（`control.AxisPalette`）：新增 `(*App).drawerPalette`——选择器留在标签那一行，
   `PalettePreview` 走 **width 0**（铺满格子）落在下一行，即设置页画它的同一形
   （`terminalui.go` 的 `PalettePreview` 注释写明「width 为零就铺满父级」）。
   固定的 `t.Space(37)` 预览宽度与 `paletteCellFloor` 这个「猜」就此不再承担布局。
5. 抽屉的墨、字阶、`Control` 面、`HairlineSoft` 上边线、关闭钮几何、行 gap、`drawerCellFloor`
   全部零改动；`options()`/会话/设置写回/i18n 文案零改动。

## 行为契约零改动红线

高度链（抽屉是画布的兄弟，开合会 Resize PTY）、键盘模型、会话管理、设置写回（`a.set` →
`Store.Update`）、i18n 文案、命令接线一律不动。本轮只改**格子里的排版**。

## 门槛命令与 PASS 基线（实测，本轮开工时）

```
gofmt -l cmd internal          → 空
go vet ./... / GOOS=windows|darwin go vet ./... → 净
go test -count=1 ./...         → 全绿
PASS 计数（go test -v | grep -c "--- PASS"）     基线 2384
SKIP 计数                                        基线 29
测试函数数（grep -rhn '^func Test'）             基线 1947
GOOS=linux|windows|darwin go build ./...         → 过
```

## 台账预案

- **S 类**（结构真的变了）：`TestTheDrawerGridCellsKeepTheReferenceFloor` 的观测量。它以前数
  「标签落在多少条不同的 y 上」，那要求**每个格子等高**；上下两行之后格子高度不齐，标签按各自
  格子中线落，两个 floor 量出同一个断点（1400），信号全丢。改成量「网格自己有多高」（最后一排
  落墨的 y）：语义仍是「floor 决定折几行」，判据反而更难糊弄。注释写 `P11-as 重定基线`。
- **V 类**：无。几何里的 floor/gap/半径/墨一个没动。
- **B 类**：其余全部 pin 零改动必须全绿（`TestTheTerminalViewsPaintFromTokensNotLiterals`、
  `TestTheDrawer*`、barink/barchrome/redframe 等）。

## mutation 计划（≥2 条，实跑）

1. 把光标形状行改回 `drawerRow` + `Segmented` → `TestTheDrawerRowsKeepTheirWordsInsideTheirCells`
   必须在 900 宽红，报 `Underline … paints over "Blinking cursor" (overlap 4.7x22.0)`。
2. 把调色板行改回「预览固定 37u 挤在尾部 slot 里」→ 同一条 pin 在 720 宽红，报
   `Palette preview … paints over "Copy on select" (overlap 24.0x22.0)`。
3. 把内边距行改回尾部 `Segmented` → 1100 宽红，报 `Relaxed … paints over "Close terminal
   settings" (overlap 16.0x25.0)`。
