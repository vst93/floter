# P13-ce 规格：捕获输出页的满态收口——折叠是语法，条子下的呼吸不跟着滚

上一轮（P13-cd）「仍待」第 1 条。起点 `15df3d51`（P13-cd）。

## 意图

**这一页的满态两端收口，先量再改**：P13-cd 的「仍待」写着「下沿是 `foldAlpha`（行按
可见比例退后），顶边却是行滚到条子下面被硬切，两端不成对」——这句里的**两个前提都要先量**，
量完可能一个都不成立。量完发现：

1. **「顶边是硬切」不成立**。列表分支两端都走 `foldAlpha`：滚一行后顶边那一行的墨是
   **92**（整行 228 的 **40%**，正是它露出的 16/40），下沿那一行是 **160**（**70%**，
   28/40）。两端按**同一个比例规则**退后，成对。
2. **真正不成对的是「条子下面那 4u 呼吸」**。它是列表自己的 `Padding`（`output.go` 的
   `list := ui.List(...).Fill().Padding(header, …)`），也就是**内容**的一部分，于是滚一行
   就被内容吃掉：实测顶边那行被折边切成两半的字尾落在 **y=102**，而条子的最后一行墨在
   **y=100**——中间只隔 **2px**。正文分支更糟：它连折边都没有（正文不是行，`foldAlpha`
   没有可量的盒子），滚起来第一行字是**满墨 228** 贴在条子上。

所以这一轮不改折叠，改**呼吸的归属**：页头下面那 16px 属于这一页，不属于内容。

## 为什么是「折叠」而不是 `ScrollEdge`（写进 §6.48 的判据）

设置页的滚动体两端是 `glass.ScrollEdge`（§6.16 / §6.17：把不透明的面重放成 0.85→0 的
渐变）。这一页**不能用**：启动器卡片是**窗口层的半透明面**（§1：真透明度只给窗口层），
它下面没有一块不透明的面可供重放——`ScrollEdge` 的 `Background` 只能是一枚假的不透明色，
那就是 §0 明令禁止的「拿背景色回放的渐变遮罩冒充玻璃」，也正是 `foldAlpha` 的注释里早就
写下的那条禁令。**卡片上的收口只能是内容侧的**：行按可见比例退后（`foldAlpha`），
呼吸留在裁剪之外。这两条合起来就是这一页的合法收口语法。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `output.go` 列表分支（`list := ui.List(...)`） | `Padding(header, Space(2), 0, Space(2))`：4u 呼吸在**裁剪之内**，滚一行就被吃掉（顶边字尾落在 y=102，条子墨底 y=100） | 呼吸移到裁剪之外：`ui.Column(...).Grow(1).FillWidth().MinHeight(0).Padding(header, 0, 0, 0)` 包住列表，列表自己的 `Padding(0, Space(2), 0, Space(2))`。裁剪边固定在 **118**，视口下沿仍在 **466** |
| 2 | `output.go` 正文分支（`ui.Scroll(c.Key("launcher.output"))`） | `Padding(header, Space(3), Space(1), Space(3))`：同一笔呼吸在裁剪之内，滚起来第一行**满墨**贴在条子上 | 同一形状：外层 `ui.Column(c).Fill().Padding(header, 0, 0, 0)`，`Scroll` 自己 `Padding(0, Space(3), Space(1), Space(3))` |
| 3 | `output.go` `a.outputListRect` | `list.Bounds()` = `{0,102,720,364}`（含 4u 呼吸） | `{0,118,720,348}`：视口盒就是裁剪盒，与 `foldAlpha` 的语义对齐（**行盒与视口盒同源**） |
| 4 | 高度链 / 键盘 / 会话 / 复制 / i18n | — | **零改动**：`pluginTextRows`、`OutputList`、`PinText`、`outputText()`、`outputSpans` 一个字节不改；只改「这 16px 算谁的」 |

## 门槛与 PASS 基线（实测）

- 起点：**3237 PASS / 0 FAIL / 125 SKIP / 2392 测试函数**（P13-cd 实测）。
- 本轮：新增 4 条 pin + 1 条 opt-in dump → 预期 **3241 PASS / 0 FAIL / 126 SKIP / 2397 函数**。
- `gofmt -l cmd internal` 空；三平台 `go vet` 净；三平台 `go build` 过。

## mutation 计划（每条说明会让哪个 pin 变红）

- **M1** 列表分支退回旧形状（外层 padding 去掉、列表 `Padding(header, …)`）→
  `TestTheOutputListNeverScrollsIntoTheHeaderBand`（带内墨 0 → **92**）与
  `TestTheOutputViewportStartsOnItsFirstRow`（102 ≠ 118）双红。
- **M2** 折边只在**下沿**生效（`fold < opacity && box.Y >= viewport.Y`）→
  `TestTheOutputFoldRecedesTheSameAtBothEnds` 红（顶边 0.85 可见却带满墨 **228**，应 193）。
- **M3** 正文分支退回旧形状 → `TestTheOutputTextNeverScrollsIntoTheHeaderBand` 红
  （带内墨 0 → **228 / 221**）。

## 出图

`FLOTER_PNG=1 go test -run TestDumpP13CEOutputEdgesPNGs ./internal/launcher/`
→ `/tmp/floter-P13-ce-{list,text}-{light,dark}-{before,after}.png`。
**静止帧四张 md5 逐字节相同**（`3c575bcc…` / `8a401b18…` / `53ed8aef…` / `5d537ad9…`）——
收口轮的正向证据：少掉的 16px 高度与多出来的 16px 空气是同一笔。滚动帧才是本轮的变化。
