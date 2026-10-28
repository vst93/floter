# P11-aq 报告 · 输出面板的标题带占自己的位置，不要压在正文上

## 1 意图

捕获输出面板的标题/状态带从**绝对定位浮层**放回正常流：它是这一页的第一条带子，正文
（文字块或列表）排在它下面。修掉两件同源的事——**状态行印在第一行正文上**，以及**那 26u
带宽里的正文选不动**。

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（`§6` 全表已落地，
只剩 `⌘K`＝拍板）→ **上一轮「仍待」第 1 条**。P11-ap 明写：

> `internal/launcher/output.go:467` 的输出面板标题带也是「绝对定位 + 无填充 + 跨满宽」浮在
> 滚动内容上，按同一判据也该 `PassThrough`。本轮没做：要量「底下那行文字能不能 drag-select」
> 得先有输出面板的夹具。

夹具现在有了（`runListOutput` + `auditLauncherFrame`）。量出来的结论比预想的严重一档：
**不是带子挡了正文，而是正文根本没有被让出位置**——带子 42u 高，正文只给它留了 16u。
所以本轮不做 `PassThrough`（那样只会让用户去选一段被字压住的正文），而是把带子放回流里。

## 2 证据（修前实测，720×488，`Tester.Find`）

| 元素 | 矩形 |
| --- | --- |
| 标题 `tool` | `x=12 y=60 w=696 h=22` |
| 状态 `Finished  ·  ↑↓ choose · Enter runs · Esc closes` | `x=12 y=82 w=696 h=20` |
| 列表页分组头 `Group A` | `x=54 **y=82** w=58 h=22` |
| 文字页正文块（80 行） | `x=12 **y=76** w=696 h=412` |

正文从 76 起算（`edge + t.Space(4)` = 60+16），带子占到 102：**26u 的正文就画在状态行底下**。
逐像素复核（把 `view.Status` 置空再画一帧，比 y=82…103 / x=40…130 的墨）：22 行里 13 行变了。

指针（`Tester.Move` + `Tester.Cursor`，`CursorText`=可选文本）：

```
x=30/300/600，y=64 / 70 / 76 / 80 / 84 / 90 / 96 / 100 → CursorDefault（选不动）
x=30/300/600，y=110 / 130 / 200                       → CursorText
```

正好是「看得见的那几行选不动」。

参考侧的形状（`main:src/plugin-window/DetachedPluginApp.tsx:296-329`）：
`<header className="plugin-window__bar">` 与 `<div className="plugin-window__body">` 是
正常流里的兄弟，`body` 在 `header` 之下滚动，没有绝对定位叠层。

## 3 落地（`internal/launcher/output.go` 的 `outputBody`）

- 标题带：`ui.Column(c).Absolute().Top(edge).Left(t.Space(3)).Right(t.Space(3))` →
  `ui.Column(c).FillWidth().Padding(edge, t.Space(3), 0, t.Space(3))`。同样的上边距 `edge`、
  同样的左右 12u，但**高度由自己的内容决定**（标题行 22 + 状态行 20 = 42），不再有
  「预留高度」这个会和内容漂开的数。
- 正文：外层列里紧跟一个 `ui.Column(c).Grow(1).FillWidth().MinHeight(0)`，正文自己的顶内边距
  从 `edge+header` 降到 `header`（16u 的呼吸）——`edge` 那 60u 已经由带子的 `Padding` 顶掉了。
  列表页与文字页同一条带（两处 `Padding` 的顶值都改）。
- 墨、字阶、按钮几何（`Space(7)` / `RadiusSM`）、`AtEnd()` 分页、`outputFooter`、
  `a.Output` 的滚动状态、`leaveOutput`/`OutputFor` 的会话管理：一个字节没改。

## 4 台账

- **B 类**（行为/接线）：既有 pin 一条没改，全绿（`outputheight`、`output_pagination`、
  `outputnumbers`、`wheel`、`textcolumn`、`headingcolumn`、`outputaccent` 全过）。
- **V = 0**：没有视觉数值被重定基线。位置变了，但没有一条 pin 钉过输出面板的纵向坐标。
- **S = 1 类**：标题带从「绝对定位的浮层」变成「列里的第一个孩子」——两条新 pin 的注释写明
  旧结构 → 新结构。
- 新 pin：
  - `TestTheOutputPanelPaintsNoTextOverText`（`internal/launcher/outputframe_test.go`）：
    把 P11-aj 的 `auditLauncherFrame`（「同一块像素上不许两段字 / 一段字不许跑出窗口」）
    用到**输出面板**上——列表页与文字页、浅深两外观，4 张面。
  - `TestTheOutputBodyStartsBelowTheStrip`：正文第一个盒子不低于状态行下沿；带子中段
    （默认箭头）与正文首行（`CursorText`；列表页是行命中，箭头）各自归属正确。
- **pin 的边界**（实测，别误读）：这两条只保证「正文不许高于带子下沿」。把带子整体往上飞
  24u（留一条缝）它们**仍绿**；去掉正文列的 `Grow(1)` 也绿（`Fill()` 自己撑满）。纵向占满
  归 `outputheight` / `output_pagination` 那组管。

## 5 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2381（基线 2377）
--- SKIP                                               → 29（基线 28，+1 opt-in dump）
测试函数数                                             → 1946（基线 1943）
GOOS=linux/windows/darwin go build ./...               → 三平台过
```

## 6 mutation（2 条实跑变红 + 2 条实跑仍绿，全部还原并 `md5sum -c` 逐字节核对）

1. 标题带改回浮层 → 红：
   ```
   light output list: "Beta" {X:54 Y:97 W:650 H:22} is painted over
     "Finished  ·  ↑↓ choose · Enter runs · Esc closes" {X:12 Y:82 W:696 H:20} (overlap 650.0x5.0)
   the body starts at y=16.0, the strip ends at y=102.0: 86.0u of the body is painted under
     the title/status line
   ```
2. 给标题带钉一个 24u 的固定高度（`Height(t.Space(6))`）→ 红：
   ```
   "Finished  ·  ↑↓ choose · Enter runs · Esc closes" {X:12 Y:82 W:696 H:20} is painted over
     "Group A" {X:54 Y:82 W:58 H:22} (overlap 58.0x20.0)
   the body starts at y=76.0, the strip ends at y=102.0: 26.0u of the body is painted under
     the title/status line
   ```
3. 仍绿（已记录，见 §4）：带子上边距 `edge` → `edge-t.Space(6)`；正文列去掉 `Grow(1)`。

还原后 `md5sum -c` = `internal/launcher/output.go: 成功`，`touch` 后复跑两条 pin 绿。

## 7 图（我实测看到了什么）

四张：`/tmp/floter-P11-aq-output-{list,text}-{light,dark}.png`（720×488，`FLOTER_PNG=1`）。
每张都在同一夹具下修前/修后各导一次，用像素统计核对：

| 面 | 带子带（y=60…102）的墨 | 带子下第一行墨（y=103…119） |
| --- | --- | --- |
| 列表页 浅色 修前 → 修后 | **1036 → 857** | 0 → 0 |
| 文字页 浅色 修前 → 修后 | **1301 → 730** | 667 → **0** |

- 文字页：修前 y=76…102 那 26u 是**两段字叠着**（正文 80 行的头两行钻进标题与状态行里），
  修后带子里只剩标题 + 状态行（730 个墨点，正好是这两段字），正文第一行的墨从 y=103 挪到
  **y=123**（正文块 118 起，字身 123 起）。
- 列表页：修前 1036 个墨点里含着**分组头 `Group A`**（它在 y=82…104，与状态行同一条线），
  修后带子降到 857（只剩标题 + 状态行），分组头的墨挪到 y=130 那一行。
- 深色两张同一组数值，形状一致。

一句话：**那圈「标题 + 状态」的带子下面，像素上是干净的**——之前它下面压着一行分组头和
两行正文。

## 8 仍待（下一轮）

1. 同族下一条（本轮未做）：**面板与卡之间还有没有别的「浮层 + 猜高度」**。全仓 `Absolute()`
   的地方（`fieldRow`、`filtersHead`、`actionBarFooter`、`terminalui` 的标题栏）逐个量
   「预留高度 vs 自然高度」；`fieldRow` 是 56u 不透明底、行从它下面过（合法），`filtersHead`
   与终端标题栏已由 P11-ap / P11-af 改成 `PassThrough`。
2. 五条拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
3. 侦察结论（本轮实测，别当 TODO 重领）：正文不再需要 `PassThrough`——带子下已经没有东西，
   而带子自己的 `Grow(1)` 标题文本占的是「无物区」，吃掉指针不掉任何东西；`outputFooter`
   与 `AtEnd()` 的分页触发在重排后仍绿（`output_pagination` 全过）。
