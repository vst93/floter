# P13-bs · 报告：行的文字原点——一张卡片里的两条线，并回一条

> 规格 `docs/rounds/P13-bs-spec.md`｜合约 `docs/liquid-glass.md` §6.37（新）
> 产品改动 2 处：`internal/settingsui/rows.go`（`cardRow`）、`internal/settingsui/settingsui.go`（`keyValue`）

## 一、这轮为什么让它更轻、更有空间感

一张卡片里出现**两条文字原点线**（有提示的行 8.1px、没有提示的行 15.6px），第一行就读成
「挂在半空」：它既不属于它下面的行，也不属于别的卡片的第一行。并回一条 8.1px 的原点线之后，
卡片的第一行与它下面两行、以及与别的卡片的第一行共线，**卡片之间的 20px 节奏才读得出来**——
之前那 6px 的落差一直在偷走这 20px 的信噪比。

## 二、先量（改前，General 页 720×2000，深/浅完全同值）

| 组 | 首行控件 | 组头底 → 首行标签顶 | 组盒高 |
|---|---|---|---|
| Window | `checkbox` + 提示 | 16.3 | 240.0 |
| Menu bar | `checkbox` + 提示 | 16.3 | 170.4 |
| **Terminal** | `choose`（**无提示**） | **22.2** | 355.0 |
| Material | `glassPick` + 提示 | 16.3 | 380.0 |

根因（`rows.go` `cardRow` 内层 `Row` 的居中）：有提示的行文字列 ≈ 36.6px，比控件高，行高由
文字列决定，居中是恒等变换；没有提示的行文字列只有 18px，而 `ui.Select` 的触发框 33px，
行高由控件决定，18px 的块在 33px 的盒里居中 → 标签顶被推到 8.1+(33−18)/2 = **15.6px**。
同一个 Terminal 卡的第 2、3 行是 `stackedRow`（标签恒在自己列的顶边），于是同卡两线。

## 三、落地

| # | 位置 | 落成 |
|---|---|---|
| 1 | `cardRow` 内层 `Row` | `AlignItems(align)` → **`AlignItems(ui.Stretch)`**：文字列撑满内容盒，标签从内容盒顶边（8.1px）起 |
| 2 | `cardRow` 的前导记号 | 包一层 `Row.AlignItems(align)`——图标不参与拉伸（框架 `stretches()` 跳过 `kindIcon`），不包会掉到行顶 |
| 3 | `cardRow` 的控件槽 | 硬写的 `AlignItems(ui.Center)` → `AlignItems(align)`；除 `settingsui.go:2277`（`ui.Start` 且无控件）外全是 `ui.Center`，像素零变化 |
| 4 | `settingsui.go` `keyValue`（About 页只读事实） | 名字包一层 `Row.FillHeight().AlignItems(ui.Center)`：这一行的名字是**旁注**，要与右边的值同一条光学线（P13-an 的 pin）。不包它就会被顶到行顶、旁注掉到值下面 |
| 5 | `internal/settingsui/roworigin_test.go`（新） | 「组头底 → 首行标签顶」四组同值，spread ≤ 1.5px；另带 1 条 opt-in dump |

改后：**16.2999 / 16.2999 / 16.2999 / 16.2999**（spread 0.0001px）；五个组盒高
**297.2 / 240.0 / 170.4 / 355.0 / 380.0——一个像素都没变**，说明只挪了一张卡片里的一行字。

## 四、V / B / S 台账

- **V 0**：没有任何一条钉旧数值的 pin 变红（`go test ./internal/settingsui/` 一次跑通，
  唯一红的是被本轮**故意**改语义的 `keyValue`，改在同一个提交里）。
- **B 0**：行为/度量/接线 pin 零改动全绿。
- **S 0**：没有结构断言需要改。

## 五、门槛（全部实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS 计数                → 3210（基线 3209，+1）
--- SKIP 计数                → 117（基线 116，+1 = 本轮 opt-in dump）
测试函数数                   → 2365（`git grep '^func Test' HEAD` = 2363，+2）
GOOS=linux|windows|darwin go build ./... → 三平台全过
```

## 六、变异（2 条实跑变红 + `md5sum -c` 逐字节还原）

还原基线：`rows.go` `1d3421afb0c85c3c39f7d71534c35a31`、
`settingsui.go` `2d3535ee91521377ee213e7dcc45373e`、
`roworigin_test.go` `8835ff3a0ee91ebe99b9acbcf1b17e04`（`md5sum -c` 三行全「成功」）。

| # | 注入 | 实测失败串（原样） |
|---|---|---|
| M1 | `cardRow` `AlignItems(ui.Stretch)` → `ui.Center` | `roworigin_test.go:69: [light] the groups' first rows do not share one text origin: map[material:16.2999 menubar:16.2999 terminal:22.2000 window:16.3000] — spread 5.9px, want ≤ 1.5px` |
| M2 | `keyValue` 去掉 `FillHeight()` | `keyvalueink_test.go:129: light: "Framework"'s ink spans rows 142-150 and its value "mygo 0.9" rows 149-161 — they overlap by 1 of the shorter band, so the pair reads as two lines, not one` |

两次注入后都 `cp` 回备份、`md5sum -c` 逐字节核对成功，并 `touch` 复跑一次确认回到绿。

## 七、出图

| 文件 | 说明 |
|---|---|
| `/tmp/floter-P13-bs-before-{dark,light}.png` | 改前（HEAD 的 `rows.go` + `settingsui.go`） |
| `/tmp/floter-P13-bs-after-{dark,light}.png` | 改后 |
| `/tmp/floter-P13-bs-general-{dark,light}.png` | 本轮 dump 的默认名（= 改后） |

逐像素 diff：**dark 606 / light 606**，差异盒 `x[200..268] y[935..955]`——**这张图上看到的一处
变化**：「Font family」这一行的那串字整体上移了 6px，落到与「Font size」「Line height」两行
同一个左列原点线上；右边的下拉框一个像素都没动，组与卡的高度也没动。

## 八、用了 `qiaomu-design`（打磨车道，不走 Phase 2 预览）

读的路：`SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性 / P-7 功能契约
→ 本轮都不触碰）→ `references/craft-loop.md`（材质·空间）。走了「已有方向，只要打磨」那条路，
没有出四方向预览页、没有等点选。

**偷了什么（逐条对工艺密度菜单）**

1. **第 6 项「编辑细节：章节编号、页边注、hairline、大写字距标签」** → 落成**一个具体数值**：
   行的文字原点 = 行内边距 `Space(2.25)` = **8.1px**，全页、全卡、全行类（`cardRow` /
   `stackedRow` / `actionRow`）**同一个值**；量到的旧值是 8.1 与 15.6 两条。
2. **第 6 项的延伸（网格而不是画框）** → 落成**逐组盒高不变**的验收：297.2 / 240.0 / 170.4 /
   355.0 / 380.0——「对齐靠网格移动，不靠加 padding」。
3. **两轮制（Render → Critique → Refine）的工程师视角**（「哪里对不齐 / 间距不一致」）→ 这一轮
   的唯一一处 refine 就是它；艺术总监视角看到的「Terminal 卡首行挂在半空」是同一处的另一面。

**preflight**

- **A（反 AI 套路）**：过。本轮没有新增任何渐变、白边、模糊或 `glass` 叠 `glass`；没有加装饰，
  只改了一条对齐轴。
- **B（设计读取一致性）**：过。改动只依赖既有 token（`Space(2.25)`、`align`），没有写死色值/尺寸；
  深浅两套读数逐位相同（16.2999 ×4），说明没有引入明度相关分支。
- **C（动效抽查）**：**不适用（本轮零动效）**——`cardRow` 没有 transition/animation，
  设置行的 hover/press 只换面，帧数不变（`go test` 里 `FramePending` 相关 pin 全绿）。
- **D2（中文页）**：**过一半、一条老账**。本轮只动几何，不改字号，所以中文页的字号问题不因本轮
  变好也不变坏；`tokens.Caption` 9.9px 与 preflight「正文 ≥ 14px」的张力仍是**需拍板**项
  （第 7 份证据：`keyValue` 的旁注就是 `Caption`，本轮它的位置刚被钉成「与值同一光学线」，
  动字号会连带动这条线，所以更要先拍板再动）。

## 九、仍待

1. **需拍板（第 7 份证据）**：`tokens.Caption` 9.9px / `tokens.Body` 10.8px 与 D2 的「正文 ≥ 14px」。
   本轮把「旁注与值同一光学线」钉成了像素事实，改字号必须同时重定这条线。
2. **下一格候选**：同一条行框架里，`stackedRow` 与 `actionRow` 的**左右内边距**是 `inset`、
   `cardRow` 是 `inset-1` + 1px 规则槽 = `inset`——三者读数一致，但表达式有三种写法（本轮顺手量到，
   **未改**）。再下一格可从「状态反馈」的只读面（`factRow` / `codeRow` 的第二种只读形态）挑。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾；§6.32 的 32-4。
4. **P13-bo 仍待 1 原样挂着**：列表从属注 / 这一页结论的分界要写成共享判据，跨表面，另开一轮。
