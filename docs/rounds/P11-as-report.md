# P11-as 报告：外观抽屉里的宽控件占自己的一行

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩 `⌘K` 动作
面＝拍板）→ 上一轮「仍待」第 1 条（同一条拍板）→ **界面质感与交互逻辑**（用户 2026-10-28 定的
优先档）。这一档自己找，找到的是画面上**两段字压在同一块像素**——终端外观抽屉里三个宽控件
越出自己的格子，压住邻行的字、甚至压住抽屉自己的关闭钮。

## 2. 根因（实测定位）

`internal/terminalui/drawer.go:99` 的 `drawerGrid` 是 `ui.Row(...).Wrap()`：每行按**自己的
`MinWidth`**（`drawerCellFloor` = 参考 `minmax(230px,1fr)` = 57.5u）参与折行，折完一行只分到
「本行 floor 之和」的宽度。而 `drawerRow`（`drawer.go:285`）里是
`label(Shrink 0) + slot(Grow 1, Justify End)`：控件按**内容宽度**画，格子给多少它不管。于是
「标签 + 三段选择器」这种内容比格子宽的行，就从格子右边缘画进邻格。

参考侧没有这个形状：`.terminal-settings__control { min-width: 0 }`
（`main:src/styles/terminal.css:677-683`）让控件收缩进格子，`.terminal-settings` 的
`repeat(auto-fit, minmax(230px, 1fr))`（`:654-658`）只因为 floor 折行。

改前实测（`ui.Tester`，`ui.Find` 的绘制盒）：

| 行 | 窗口 | 报错串（原文） |
| --- | --- | --- |
| 光标形状 | 900×600 | `"Underline" {X:509.33334 Y:371 W:78.00003 H:30} paints over "Blinking cursor" {X:582.6667 Y:375 W:111 H:22} (overlap 4.7x22.0)` |
| 调色板 | 720×600 | `"Palette preview" {X:286 Y:465 W:148 H:88} paints over "Copy on select" {X:410 Y:498 W:102 H:22} (overlap 24.0x22.0)` |
| 终端内边距 | 1100×600 | `"Relaxed" {X:1011 Y:384 W:65 H:30} paints over "Close terminal settings" {X:1060 Y:381 W:28 H:28} (overlap 16.0x25.0)` |

## 3. 落地

- 新增 `drawerRowStacked`（标签一行、控件一行且 `FillWidth`）：控件没有由内容决定的宽度，就没
  有可溢出的宽度。
- 光标形状行、终端内边距行：`drawerRow` + `control.Segmented` → `drawerRowStacked` +
  `control.SegmentedFill`（设置页画同一个选择用的就是这一形）。
- 调色板行：新增 `(*App).drawerPalette`，选择器留在标签那一行，`PalettePreview` 走 **width 0**
  铺满格子落在下一行（`PalettePreview` 的注释本来就写明「width 为零就铺满父级」，设置页正是
  这么画的）。固定 `t.Space(37)` 的预览宽度与 `paletteCellFloor` 这个「猜」不再承担布局。
- 墨 / 字阶 / `Control` 面 / `HairlineSoft` 上边线 / 关闭钮 28u / 行 gap / `drawerCellFloor`
  零改动；`options()`、会话、设置写回、i18n 文案、命令接线零改动。

改后实测（同一个夹具）：

| 窗口 | 光标形状三段（各自宽 / 起点） | 调色板预览 | 内边距三段 |
| --- | --- | --- | --- |
| 720×600 | 104u 宽，x=14 / 120 / 226（在其格内均分） | x=26 **w=380**（铺满格子） | `Relaxed` x=564，格内 |
| 900×600 | 86.4u 宽，x=299.3 / 387.8 / 476.2（全在第二格 297…573 内） | x=26 **w=410** | `Relaxed` x=190.9 |
| 1100×600 | 101u 宽，x=14 / 117 / 220 | x=684 **w=380** | `Relaxed` x=968.7（结束 1048 < 关闭钮 1060） |

## 4. 台账（V / B / S）

- **S = 1**：`TestTheDrawerGridCellsKeepTheReferenceFloor`（`internal/terminalui/drawermetrics_test.go`）
  的观测量。旧判据是「(只有 9 个标签) 落在多少条不同的 y 上」——它要求**每个格子等高**；上下两行
  之后格子高度不齐、标签按各自格子中线落，两个 floor 量出同一个断点（实测都是 1400），信号全丢
  （红：`the three-line breakpoint at the reference's floor (1400) is not below the old 60u floor's
  (1400)`）。新判据量「网格自己有多高」＝最后一排落墨的 y，语义不变（floor 决定折几行），且更
  难糊弄：900 宽时 57.5u → **170**、60u → **199**（实测），断言 `reference < old` 是严格不等式，
  没有容差、没有 `Skip`、没有弱化成「非空」。floor 常量本身仍是 `57.5` 的硬断言。
- **V = 0**：几何里的 floor / gap / 半径 / 墨一个没动。
- **B 类**：既有 pin 零改动全绿（`TestTheTerminalViewsPaintFromTokensNotLiterals`、
  `drawermetrics` 的其余三条、barink/barchrome/redframe/drawer 系列）。

## 5. 新 pin

`internal/terminalui/draweroverlap_test.go`：

- `TestTheDrawerRowsKeepTheirWordsInsideTheirCells`：把 P11-aj 的「字不许压在字上」判据用到抽屉
  自己的带内——3 个宽度（720 / 900 / 1100）× 深浅两外观，任何两段字不许重叠、不许出窗口。
  带子以上的浏览器条与它的 toast 被排除：那条 toast 是**不透明**的浮动面、最后画，压在标题上
  是设计（不是字压字）。
- `TestDumpDrawerPNGs`（opt-in，`FLOTER_PNG=1`）：本轮视觉证据。

## 6. 门槛（实测）

```
gofmt -l cmd internal                                       空
go vet ./... / GOOS=windows / GOOS=darwin go vet ./...      净
go test -count=1 ./...                                      全绿，FAIL 0
PASS 计数  --- PASS                                          2384 → 2385
SKIP                                                         29 → 30（+1 opt-in dump）
测试函数数                                                   1947 → 1949
GOOS=linux|windows|darwin go build ./...                    过
```

## 7. mutation（实跑，3 条）

1. 光标形状行改回 `drawerRow` + `Segmented` → 900 宽红：
   `"Underline" {X:509.33334 Y:371 W:78.00003 H:30} paints over "Blinking cursor" {X:582.6667
   Y:375 W:111 H:22} (overlap 4.7x22.0)`。
2. 调色板行改回「预览 37u 挤在尾部 slot」→ 720 宽红：
   `"Palette preview" {X:286 Y:465 W:148 H:88} paints over "Copy on select" {X:410 Y:498
   W:102 H:22} (overlap 24.0x22.0)`。
3. 内边距行改回尾部 `Segmented` → 1100 宽红（`Relaxed` 压关闭钮 16×25u）。

每条都 `cp` 回原文件 + `md5sum -c` 逐字节核对（`internal/terminalui/drawer.go: 成功`）+ `touch`
后再复跑一次绿。

## 8. 视觉证据与我实测看到了什么

`/tmp/floter-P11-as-drawer-light.png`、`/tmp/floter-P11-as-drawer-dark.png`（900×600，终端页 +
一个会话 + 抽屉打开）。

图上看到的三处变化：**① 光标形状那三格（Beam / Block / Underline）从「挤在标签右边、右端探进
下一格」变成标签下面一条均分的三段轨道**，右端离下一行 `Blinking cursor` 有 6u 空隙；**② 调色
板预览从「选择器右边一块 148u 的小方块」变成标签与选择器下面一块 380–410u 宽、铺满整个格子的
迷你终端**，右边不再压着 `Copy on select`；**③ 终端内边距（Compact / Regular / Relaxed）同样的
均分轨道，右端不再贴住抽屉右上角的关闭钮。墨与面没有变化：纸色 `Float`/`Control` 面、发丝上边
线、12u 圆角、标签仍是次级墨，读者一眼看不到新的高光或渐变。**

## 9. 仍待（P11-as）

1. **同族下一条（需先量，不许猜）**：抽屉里还有两处「尾部 slot + 内容定宽」的形状——字体家族
   `ui.Select` 与字号/行高的滑轴 + 读数。本轮实测它们在最窄 720 与最宽 1100 都没越界（读数
   `14 px` / `1.20×` 都在格内），所以没动；要动先得像本轮一样量出重叠再改。
   下一个可疑点是**本地化**：`zh` 的标签更长（「终端内边距」「光标形状」），`SegmentedFill` 已
   与标签宽度解耦，但 `Select` 的选项文字（调色板名）仍是内容定宽。
2. 五条老拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1），
   外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
3. 侦察结论（本轮实测否掉，别当 TODO 重领）：`drawerGrid` 里「一个格子给不足宽度」这件事**不是**
   `Wrap()` 能解决的——给行加 `Wrap()` 后内容仍按原宽画（实测重叠一字未变），给行加
   `FillWidth()` 反而让它与同行的其他行重叠（`Cursor shape` 压 `Blinking cursor` 76×22u）。唯一的
   解法是让控件自己**没有内容宽度**（`SegmentedFill` / 铺满的预览），本轮就是这么做的。
4. 拍板候选（本轮新增）：外观抽屉要不要干脆在窄窗口（< 900）折成**单列**（每行一格）。
   现在仍然靠 floor 折行；单列会让宽控件更稳，但会改变抽屉的高度节奏。
