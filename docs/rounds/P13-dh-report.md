# P13-dh · 报告：抽屉的顶缝没丢，丢的是 pin 的取行方式

**一句话**：`docs/liquid-glass.md` §6.76 把这一格记成「设计间距下顶缝丢失」，
本轮只量不改，量出的结论相反 —— **每一帧的缝都在它该在的地方，pin 读了画布那一行**。

## 1. 怎么证的（先量）

抽屉自己就是那个 `ui.Scroll`：`internal/terminalui/drawer.go:112-115`
`BorderWidth(1, 0, 0, 0).BorderColor(tokens.LensRim)`，它的盒子就是 pin 里的 `strip`
（`Label(scopy.GroupTerminal)` 挂在这个节点上，所以 `Find` 回来的矩形是抽屉自己，
`W=整宽 H=抽屉高`）。设计尺下这个盒子的顶是小数，而 pin 用 `int(strip.Y)` 截断：

| 窗口 | `strip.Y` | 发丝实际行 | `int()` 截断读到的行 | 判 |
|---|---|---|---|---|
| 640×360 | 50.80002 | 51（缝 `[236 236 236]` / 深 `[58 58 59]`） | 50 = 画布白 `[255 255 255]` / 深 `[24 24 27]` | 假红 |
| 720×360 | 109.20001 | 109 | 109 | 碰巧对 |
| 720×600 | — | 缝在 | 缝在 | 碰巧对 |
| 900×600 | 384.40000 | 384 | 384 | 碰巧对 |
| 1200×800 | 616.80000 | 617 | 616 = 画布 | 假红 |

painter 在画盒子前 `snap` 到整数设备像素，取整用 `math.Round`
（`/home/tar/go/pkg/mod/github.com/egoist/mygo@v0.7.3/ui/paint.go:163-170`）。
所以 pin 该用 `math.Round` 取行，不是 `int()`。**产品零改动**。

## 2. 落地项

| # | 位置 | 现状 → 改成 |
|---|---|---|
| 1 | `internal/terminalui/view_test.go`（`render` :22） | 新增 `renderThemed`：`c.SetTheme(tokensOf(a, c).Theme)` + `ReduceMotion`。本轮只给缝两条 pin 用；`render` 一个字节不动（其余 pin 的库存尺是 `P13-dg-t` 的债） |
| 2 | `internal/terminalui/drawerseam_test.go:40` 的 `TestTheDrawerSeamIsOnItsTopEdge` | `render` → `renderThemed`；`top := int(strip.Y)` → `top := drawerSeamRow(strip)` |
| 3 | 同文件 `:117` 的 `TestDumpP11AVDrawerSeamPNGs` | `render` → `renderThemed`：证据图第一次是**用户看到的那把尺子** |
| 4 | `internal/terminalui/litedge_test.go:131-132 / 164-166` `TestTheDockedDrawerWearsTheE1LitEdge` | 同一条缝、同一个 `int(strip.Y)`；900×600 上 `384.4` 碰巧对，是**潜伏的同一条错** → `drawerSeamRow(strip)`，一个函数两处共用 |
| 5 | `internal/terminalui/drawerseam_test.go` 新增 `drawerSeamRow` | 注释里写死上面那张实测表 + `paint.go:163-170` 的取整依据 |

**产品文件零改动**：`git diff --stat` 只有 `internal/terminalui/*_test.go` 与 `docs/`。

## 3. 台账（V / B / S）

- **V 0、B 0、S 0**：没有一条断言被放宽、加容差或删掉。变的是**读哪一行**，
  而读哪一行是 painter 已经定好的（`math.Round`），不是设计数值的重定基线。
- 覆盖**是涨的**：`TestTheDrawerSeamIsOnItsTopEdge` 此前只在库存尺（`14px/4.0`）上跑过，
  本轮起跑在设计尺（`10.80/3.6`）。抽屉盒的顶在这个尺上才是小数 —— 也就是**从这一轮起，
  这条 pin 才真的量到生产页面**。

## 4. 变异（实跑，逐条贴原始报错串）

**M1**：`drawerSeamRow` 改回截断 → 缝 pin 红（8 条，2 尺寸 × 2 外观 × 2 断言）：

```
--- FAIL: TestTheDrawerSeamIsOnItsTopEdge (0.11s)
    drawerseam_test.go:97: light 640x360: the row above the strip (y=49) is the seam colour [255 255 255] — the hairline is painted outside the drawer, on the canvas
    drawerseam_test.go:102: light 640x360: the window's bottom row is [230 230 230], not the drawer's face [236 236 236] — …
    drawerseam_test.go:97: dark 1200x800: the row above the strip (y=615) is the seam colour [24 24 27] — …
```

**M2**：`drawer.go:114` 的 `BorderWidth(1, 0, 0, 0)` 回退成 `(0, 0, 1, 0)`（P11-av 那条 bug）→ 缝 pin 红：

```
--- FAIL: TestTheDrawerSeamIsOnItsTopEdge (0.11s)
    drawerseam_test.go:84: light 640x360: the strip's first row [230 230 230] is its own face — the drawer and the canvas above it meet with no seam
    drawerseam_test.go:102: light 640x360: the window's bottom row is [236 236 236], not the drawer's face [230 230 230] — …
```

**M3**：`BorderColor(tokens.LensRim)` → `tokens.HairlineSoft` → E1 受光边 pin 红：

```
--- FAIL: TestTheDockedDrawerWearsTheE1LitEdge/light (0.03s)
    litedge_test.go:139: [light] the drawer's seam {217 217 217 255 …} is not brighter than its own face {230 230 230 255 …}: the E1 lit edge is missing
    litedge_test.go:154: [light] the drawer's seam {217 217 217 …} is nearer the old weak hairline ({217 217 217 …}) than the E1 lit edge ({236 236 236 …})
```

三条都已还原，`md5sum -c` 逐字节核对（`drawerseam_test.go` / `drawer.go` 均「成功」），
`touch` 后复跑 `TestTheDrawerSeamIsOnItsTopEdge` 得 `ok`（避免测试缓存复用）。

## 5. 图（`FLOTER_PNG=1`，900×600，设计尺）

- `/tmp/floter-P11-av-drawer-seam-light.png`
- `/tmp/floter-P11-av-drawer-seam-dark.png`

**复核方式（照实写）**：我这一侧没有肉眼，所以「看」是用**整列 + 整行的像素剖面**做的，
剖面就是这张图上真实发生的事：

```
light  y=0..383 (255,255,255) | y=384 (236,236,236) | y=385..599 (230,230,230)
dark   y=0..383 (24,24,27)    | y=384 (58,58,59)    | y=385..599 (39,39,40)
```

**这张图上看到的一处变化**：同一份 dump（P11-av 那两张，库存尺）里抽屉顶在 **y=339**、
画布只到 338；换成生产尺之后顶缝落在 **y=384**，画布多出 **45px**，抽屉从 261px 收到 216px
（60u × 3.6）。也就是说这条缝此前量的根本不是一个尺子下的页面 —— 本轮它第一次站在生产尺上。
浅色那一行：缝 `236` 压在面 `230` 上是 **+6**（受光边），而它对面外的画布 `255` 是 **−19**
（所以它读作「抽屉的上沿」而不是「画布的下沿」）；深色：缝 `58` 对 面 `39` 是 **+19**。
两种外观都满足 §1 的不变量（受光边严格亮于面）。

抽屉内容没有被压扁：顶缝下 14 行是抽屉自己的内边距（`t.Space(2.5)` 起），第 15 行起出现
控件墨，900×600 上 `DrawerScroll.MaxY == 0`（整块放得下），216 行里 174~176 行带多色墨。

## 6. 偷了什么（qiaomu-design）

本轮读了 `SKILL.md` 的 Router（「已有方向，只要打磨」那条路径）、`references/user-preferences.md`
的 P-1 / P-2 / P-5 / P-17、`craft-loop.md` 全篇、`preflight.md` 全篇。
**不走** Phase 2 四方向预览（方向由用户钉死，自动车道没人点选）。

- 从 `craft-loop.md` **工艺密度菜单第 12 项「材质细节」**（内边框高光 / glass 折射边）：
  落成的具体数值 = 浅色 `LensRim` 在抽屉面上的合成 `236` 对 面 `230`（**+6 级**）、
  深色 `58` 对 `39`（**+19 级**），并且首次在**生产尺**上复核（此前只在校验尺上被量过）。
- 从**工艺密度菜单第 6 项「编辑细节 / hairline」**：落成缝的**全宽**断言 ——
  900×600 上 `x=0..899` 一个颜色（`drawerseam_test.go` 的逐像素环）。
- 从 `craft-loop.md` 的**「两轮制」里那句『1× 看整体、再放大找没落在任何东西上的线』**：
  落成对本轮的处置 —— 报告里先把「缝到底在哪一行」逐窗口量出来（四窗口 × 两外观的
  剖面 + `strip.Y` 的小数位），再动测试；**先量再改**，结论是产品不用改。
- 该文件其余工艺项（氛围层 / 字体 / focus 环 / 签名交互）在「一个 1px 缝」上没有适用面，
  不做样子。

## 7. preflight 逐项

- **第 0 步（偏好账本）**：已读。P-1（无斜体）不适用（无文字排版改动）；
  P-2（先看后选）按任务书**豁免**（方向已由用户给死，不出预览页、不停）；
  P-5（可读性优先）见 D2；P-17（自发现缺陷必须沉淀）→ 本轮把「pin 取行要 `math.Round`」
  写进 `liquid-glass.md` §6.77，并注明后三格（dg-t/l/s）的硬约束，这就是沉淀。
- **A（反 AI 套路）**：本轮**无新像素** → N/A。既有四条禁令（blur 冒充玻璃 / glass 叠 glass /
  渐变白边 / 组件层自造透明）`git diff` 里一条都没被触碰（本轮没有生产文件改动）。
- **B（设计读取一致性）**：**局部通过、整包未通过**。局部：缝在设计尺下仍在、位置正确、
  受光边仍严格亮于面。整包：`internal/terminalui` 的其余 pin 仍骑在库存尺上（`render` 未装主题），
  也就是「同一张卡上两个比例」这条读取不一致**还在**，它是 `P13-dg-t` 的活。本轮没有用
  「绿了」冒充「一致」。
- **C（动效抽查）**：N/A（无动效改动；新 dump 装 `ReduceMotion`）。
- **D2（中文页正文 ≥ 14px）**：**仍未通过** —— `tokens.Caption` 9.9px、正文 10.8px，
  与第 33 份证据同源。改字号会动整页高度链（行为契约），**需拍板**，不擅自改。

## 8. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：全绿，**PASS 3311 / FAIL 0 / SKIP 148 / 2467 测试函数**
  （与基线逐项相同；本轮只改断言的**读法**，不动测试函数条数）
- `GOOS=linux/windows/darwin go build ./...`：过

## 9. 仍待（下一轮自己挑）

1. **`P13-dg-t`（下一格）**：`internal/terminalui` 整包收口（阶梯 4 条 V + 抽屉 2 条 S）。
   开工先按 §6.77 查「从矩形取像素行」的 pin 有没有 `int()` 截断 —— 那是这一族在 `3.6`
   尺子上最容易出的假红。
2. **`P13-dg-l` / `P13-dg-s`**：见 §6.76。
3. **捕获输出页在页头的呼吸带里起笔（仍未证实）**：`strip's last ink y=117` 对
   `viewport top y=100.4`，归 `dg-l` 第一步用单点装主题复现。
4. **需拍板（沿用 + 第 33 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2。
5. **抢救 stash 核对（第三十五次）**：顶部三条仍判定与 HEAD 无未接续内容，未试合、未静默丢弃。
