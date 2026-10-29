# P11-bd 报告 · 终端页上没有动作的浮层不该是命中目标

选题顺序：用户实测反馈（`.pending-bugs.md` 顶部 F1/F2/F3 已由 P11-y/z/aa 收完，用户新报的
`Super+N` 编号渲染已由 P11-bb 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K` 动作面＝
拍板）→ 上一轮「仍待」（P11-bc 第 1 条是老拍板项、第 2 条正是本轮这一条）→
**界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。规格 `docs/rounds/P11-bd-spec.md`。

## 1. 为什么选这一处

P11-bc 的「仍待」第 2 条自己写着：`feedbackNote`（动作失败那句提示）**仍是浮卡，本轮故意
没动**——理由是不想动它的几何（做成整宽带会跟浮条 `Space(7)` 的面互压）。那条理由只挡住了
**几何**那一半，挡住了不该挡的那一半：这条提示从头到尾**没有 `PassThrough`**，而参考给它的
`pointer-events: none` 就写在 `main:src/styles/terminal.css:414`，紧邻 P11-bc 引用的
`:303`（常驻便条）。用户 F3 的原话是「不要 floating/overlay 观感」，而浮层在**输入侧**的
后果——吃掉它下面的指针——正是 P11-af（浮条）与 P11-bc（常驻便条）连着修了两轮的同一件事。

顺着这条线量下去，本轮的真正收获比预想的大：**P11-bc 与 P11-af 那两处的修法都只修了一半**。

根因一句话：**`mygo` 的 `PassThrough()` 不沿子树传播**。框架的命中表逐个元素记
（`ui/runtime.go:1208-1218`），只把 `flagInvisible`/`flagInert` 的元素排除在外
（同文件 `:1196`）——**不看透明度**，也不看父元素是否 `PassThrough`。而一句 `ui.Text`
自己就是可选中面（`flagSelectable`），于是「把外层标成透明」在这三处都静默地漏掉了字。

## 2. 落地

`internal/terminalui`：

| 位置 | 修前 | 修后 |
|---|---|---|
| `terminalui.go:1243` `feedbackNote` | 行没有任何 `PassThrough`（参考 `:414` 有 `pointer-events: none`） | 行 + `ui.Icon` + `ui.Text` 各一次 |
| `terminalui.go:1191` `residentNote`（P11-bc 标的） | 只有**行**是 `PassThrough`，两句 `Text` 仍是命中目标 | 两句 `Text` 各一次 |
| `terminalui.go:1324` `barButton` | 签名没有「揭示」这一档；只靠 `:1449` 的 `actions.Opacity(0).PassThrough()` | 新增 `live bool`：未揭示时按钮与字形各 `PassThrough()`，且**不挂 tooltip** |
| `terminalui.go:1464-1476` `titleBar` 的五个调用 | — | 传 `revealed` |
| `drawer.go:102` 抽屉自己的关闭钮 | — | 传 `true`（抽屉里的钮永远是活的） |

`Focusable()` 一个没动：参考的 `:focus-within` 揭示靠的正是「键盘还能进得去」，而指针在
未揭示时穿过去——两半都在。

**零改动**：几何、面、墨、字号、文案、出现条件、时钟（4.5s）、画布高度与 PTY 行数、
键盘模型、命令接线。本轮改的是**命中标志**，一个像素都不动（§5 有逐字节证据）。

### 为什么 `barButton` 的 `live` 要单独说清

`revealed` 每帧从 `bar.PointerPosition()` 读（P11-af 定下的），所以指针一进浮条它就为真。
也就是说：一个**先悬停、后点击**的用户拿到的是活按钮（与参考一致，`:hover` 揭示后按钮
就可点）。`pointer-events: none` 挡的是**指针刚到达的那一帧**——点击事件与揭示落在同一帧里
时（`ui.Tester.ClickAt` 就是这个形状：一次 `PointerDown`/`PointerUp`，没有先行的悬停帧），
用户点在一块**一个像素都没画**的 28u 方块上。修前它把终端关了（实测 `closed=1`）。

## 3. 台账（V / B / S）

- **V = 0**：零像素改动。证据是 §5 的 PNG 逐字节比对（修前/修后 md5 相同）。
- **B = 0（产品行为零改动的地方）**：所有既有 pin 的**断言**一个字没改；5 处夹具补了
  「先悬停再点」这一帧——它们原本断言的是「点一个没被画出来的控件能生效」，而那正是本轮
  删掉的行为。这与 P11-ab 的「取样点修正、断言不放宽」是同一类处置。
- **S = 1 类**：`barButton` 的签名多一档（`live`），`titleBar` 五个调用点与 `drawer` 一个
  调用点跟着改；`TestTheCanvasOverlaysGoQuietWhenHidden` 是**新**的源码契约，钉住
  「`PassThrough` 要逐层标」这条根因（只标外层必红）。

函数 **1972 → 1977**（+4 pin，+1 opt-in dump）；PASS **2431 → 2435**；SKIP **39 → 40**；FAIL 0。

## 4. 门槛（实测）

```
gofmt -l cmd internal                          → 空
go vet ./...                                   → 净
GOOS=windows go vet ./...                      → 净
GOOS=darwin  go vet ./...                      → 净
go test -count=1 ./...                         → 全绿（42 个 ok 包，0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2435（基线 2431，P11-bc）
GOOS=linux/windows/darwin go build ./...       → 三个都过
```

## 5. 视觉证据：本轮**零像素改动**（这是结论，不是省略）

`FLOTER_DUMP_P11BD=1 go test -run TestDumpP11BDPointerFrames ./internal/terminalui/`
导出同一帧（会话 + 常驻便条 + 失败提示，浅/深各一张）到
`/tmp/floter-P11-bd-notes-<light|dark>.png`；把两个产品文件 `git checkout` 回修前，同一张
dump 写到 `/tmp/before/`，逐字节比对：

```
3c20cd5dbf87c5c42d3487133662cbba  /tmp/before/floter-P11-bd-notes-dark.png
3c20cd5dbf87c5c42d3487133662cbba  /tmp/floter-P11-bd-notes-dark.png
839cef6676394e47a1bd955f565a9ae8  /tmp/before/floter-P11-bd-notes-light.png
839cef6676394e47a1bd955f565a9ae8  /tmp/floter-P11-bd-notes-light.png
```

四张 md5 全等 —— 修前修后**同一帧**。所以「我实测看到了什么」的答案是：**图上什么都没变**，
变的是同一块像素**属于谁**：修前指针落在提示的字上、常驻便条的句子上、右上角那个看不见的
方格里，全归浮层；修后它们穿到画布（§1 表里的「画布有没有拿到键盘」那一列）。

## 6. 变异（3 条实跑变红 + 还原 + `touch` 复跑绿）

| # | 注入 | 红在哪（原始报错串） |
|---|------|----------------------|
| M1 | `git checkout` 两个产品文件，整体回到修前 | `a click on the hidden close control closed the terminal (1 times)`；`a click on the notice's words did not reach the canvas (box {46 20 87 22})` + `a click on the notice's padding did not reach the canvas`；`a click on the resident note's words did not reach the canvas (box {179 571 287 22})`；`barButton no longer takes the reveal state: a hidden control cannot be told from a live one` |
| M2 | 只删 `feedbackNote` 的三处 `PassThrough()` | `a click on the notice's words did not reach the canvas (box {46 20 87 22})` + `feedbackNote marks 1 of its 3 layers PassThrough: the rest still take the pointer` |
| M3 | 只删 `residentNote` 两句 `Text` 的 `PassThrough()` | `a click on the resident note's words did not reach the canvas (box {179 571 287 22})` + `residentNote marks 1 of its 3 layers PassThrough: the rest still take the pointer` |

每条还原后 `md5sum -c /tmp/p11bd.md5` 逐字节核对（`895ed283…` / `cc0c0b86…`，M1 之后
`drawer.go` 那一行参数按记录重放），`touch` 后复跑全包绿。

## 7. 侦察结论（本轮量过，别当 TODO 重领）

1. **浮条里那行「会话标题」仍是窗口拖拽手柄，且未揭示时它也在**（`terminalui.go` 的
   `identity.Children` 里 `.DragWindow()`）。这不是漏标：P11-af 定的就是「手柄收窄成标题的
   字」，而参考的 identity 是**永远** `pointer-events: none`、拖拽靠整条 `.terminal-bar`
   （`cursor: grab`）。要不要把「未揭示时标题也不接指针（于是没地方拖窗口）」改掉，是
   交互决定 → 见「仍待」。
2. **`barButton` 的 tooltip 在未揭示时读不到**：`revealed = hovered || bar.FocusWithin()`，
   两者任一为真都已经揭示，所以 `if live` 那半是**防御性**的、不是本轮观察到的缺陷。留它
   是因为「隐藏就是隐藏」这条判据，别把它当成一个独立 bug 去复现。
3. **`titleBar` 的 `actions.Opacity(0).PassThrough()` 保留**：它管的是行自己的盒子
   （4u 的缝与行尾余量），按钮由 `live` 各自管；两者不是重复。

## 8. 仍待（P11-bd）

1. **需拍板**：浮条的「窗口拖拽手柄」要不要在未揭示时也让位？现状是标题的字（透明但可拖）
   横在画布第一行的左半边。改它 = 未揭示时窗口没地方拖（参考是整条 bar 可拖，本仓按
   P11-af 收窄过），属交互决定。
2. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的终端便条投影 / P11-ba 的终端「进程已退出」便条改 inline
   （**改的是「占布局位置」那一半：那会 resize PTY，仍属交互决定**）。
3. **同族另一半仍故意没动**：`feedbackNote` 的**几何**（`Absolute().Top(12).Left(12)` 的
   圆角浮卡 + `Float` 面 + 投影）。做成贴边带会落在 `titleBar` 揭示时画的 `Space(7)` 面
   底下，两个面互相压 —— 要先决定它去底边还是让浮条让位。
