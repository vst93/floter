# P11-aq 规格 · 输出面板的标题带要占自己的位置，不要压在正文上

## 1 意图

捕获输出面板的标题/状态带是**绝对定位**浮在正文上的（`internal/launcher/output.go:479`，
`ui.Column(c).Absolute().Top(edge)...`），而正文只给这条带子留了 `edge + t.Space(4)`
（= 60 + 16 = **76u**）的顶内边距（`output.go:452` / `:463`）。这条带子的自然高度是
**42u**（标题行 22 + 状态行 20），于是它的下半段（y=76…102）与正文的前 26u 落在同一块像素上：
**两段字叠着画**，并且这段带宽里的指针全被带子吃掉——看得见的字选不动。

一句话：把标题带放回正常流里（参考侧就是这么排的），让版面自己为它留位置。

为什么现在做：本轮选题顺序 = 用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地
条目（§6 只剩 `⌘K`＝拍板）→ **上一轮「仍待」第 1 条**——P11-ap 明写「`output.go` 的输出面板
标题带按同一判据也该 `PassThrough`，本轮没做：要量『底下那行文字能不能 drag-select』得先有
输出面板的夹具」。夹具现在有了（`runListOutput` + `auditLauncherFrame`），量出来的不是
「带子挡了正文」，而是**正文根本没有被让出位置**。

## 2 证据（动手前实测，720×488，输出面板）

夹具：`runListOutput`（`internal/launcher/output_pagination_test.go:28`），三行分组 JSON
（`Group A / Alpha / Beta`、`Group B / Gamma`）。

| 元素 | 实测矩形（`Tester.Find`） |
| --- | --- |
| 标题 `tool` | `x=12 y=60 w=696 h=22` |
| 状态 `Finished  ·  Enter copies · Esc closes` | `x=12 y=82 w=696 h=20` |
| 分组头 `Group A` | `x=54 **y=82** w=58 h=22` |

两个矩形 `y` 都在 82，`x` 一个 12…708、一个 54…112：**状态行整段盖在分组头上**。逐像素
复核（把 `view.Status` 置空再画一帧，比较 y=82…103、x=40…130 的墨）：22 行里有 **13 行**
变了——被盖住的那块像素两段字都在。

文字页同样：正文顶内边距 `edge+header` = 76，状态行占到 102，第一行正文（行高 21）落在
76…97，与标题行下沿（60…82）与状态行（82…102）都相交。

指针（`Tester.Move` + `Tester.Cursor`，`CursorText` = 可选文本）：

```
x=30/300/600, y=64…100  → CursorDefault   （带子里，选不动）
x=30/300/600, y=110/130/200 → CursorText  （带子下，可选）
```

即：**看得见的正文（76…102）同时处于「选不动」的区间**。

参考侧的形状（`main:src/plugin-window/DetachedPluginApp.tsx:296-329`）：
`<header className="plugin-window__bar">` 与 `<div className="plugin-window__body">` 是
**正常流里的兄弟**，`body` 在 `header` 之下滚动，没有任何绝对定位叠层。

## 3 逐条落地项

位置全部在 `internal/launcher/output.go` 的 `outputBody`：

1. **标题带从绝对定位改为列的第一个孩子**（`output.go:479-521`）。
   - 现状：`ui.Column(c).Absolute().Top(edge).Left(t.Space(3)).Right(t.Space(3))`。
   - 应改成：`ui.Column(c).FillWidth().Padding(edge, t.Space(3), 0, t.Space(3))`——同样的
     上边距 `edge`、同样的左右 12u，但**高度由内容决定**，不再是浮层。
2. **正文排在标题带之下、吃掉剩下的高度**（`output.go:449-478`）。
   - 现状：`ui.Column(c).Fill()` 里只有正文，正文顶内边距 `edge+header`。
   - 应改成：外层 `Column` 里先标题带、再 `ui.Column(c).Grow(1).FillWidth().MinHeight(0)`，
     正文自己的顶内边距从 `edge+header` 降到 `header`（`t.Space(4)` = 16u 的呼吸），
     因为 `edge` 这一段已经由标题带的 `Padding` 顶掉了。
3. **列表页与文字页都走同一条带**：`Padding(...)` 的顶值两处都改（`output.go:452`、`:463`）。

不改的东西：标题带的墨与字阶、按钮几何（`Space(7)`）、`AtEnd()` 分页触发、`outputFooter`、
`a.Output` 的滚动状态、`leaveOutput`/`OutputFor` 的会话管理、窗口高度链
（`Geometry.Height()` / `outputRows`）。

## 4 行为契约零改动红线

- **高度链**：`outputBody` 只在面板内部重排，`Geometry.Height()` 与
  `TestTheOutputViewPricesItsOwnContent` / `TestTheOutputListWindowIsCappedAtTheBudget` 的
  窗口高度一个像素都不许变。
- **键盘模型**：Enter/↑↓/Esc/数字键的语义不动（本轮不碰 `moveOutputSelection`、
  `runOutputRow`、`outputNumbers`）。
- **会话管理**：`a.output` 的生命周期、分页 `loadMoreOutputPage` 不动。
- **设置写回 / i18n 文案 / 命令接线**：零改动。
- 视觉轮只改墨与面：本轮**只动位置**，不动任何颜色、字阶、圆角。

## 5 门槛与基线

基线（本轮实测，改动前）：**2377 PASS / 0 FAIL / 28 SKIP / 1943 测试函数**。

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → ≥ 2378（只许涨）
三平台 go build ./...                                  → 过
```

新增 pin（两条）：

- `TestTheOutputPanelPaintsNoTextOverText`（`internal/launcher/outputframe_test.go`）：
  用既有的 `auditLauncherFrame` 审**输出面板**的文字页与列表页（浅/深两外观）。修前必须红
  （`"Finished  ·  …" is painted over "Group A"`），修后绿。
- `TestTheOutputBodyStartsBelowTheStrip`：正文的第一个元素盒子（列表页的第一行/文字页的
  文字块）与标题带的盒子**不相交**，且指针在这段带宽里对正文是 `CursorText`（列表页则是
  行被悬停）。

## 6 mutation 计划（实跑结果，全部已还原文）

1. **把标题带改回浮层**（`Column(c).Absolute().Top(edge).Left(..).Right(..)`）→ 两条 pin 变红：
   `light output list: "Beta" {X:54 Y:97 …} is painted over "Finished  ·  ↑↓ choose · …"
   {X:12 Y:82 …} (overlap 650.0x5.0)`、`the body starts at y=16.0, the strip ends at y=102.0`。
2. **给标题带钉一个小于它内容的固定高度**（`Column(c).FillWidth().Height(t.Space(6))`，24u 裹
   42u 的带子）→ 两条 pin 变红：`"Finished  ·  …" {X:12 Y:82 …} is painted over "Group A"
   {X:54 Y:82 …} (overlap 58.0x20.0)`、`the body starts at y=76.0, the strip ends at y=102.0`。

试过但**没**变红的两条（记下来，别重领）：

- 把标题带的上边距从 `edge` 改成 `edge-t.Space(6)`（带子往上飞 24u）：两条 pin 都绿——
  不变式是单向的（正文不许高于带子下沿），留个缝隙不是这轮的错误；写进报告 §4 的
  「pin 的边界」。
- 去掉正文列的 `Grow(1)`：也绿（`Fill()` 自己就能撑满）；这说明 pin 不负责「正文占满
  剩余高度」，`outputheight` / `output_pagination` 那组负责。
