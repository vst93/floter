# P11-bm 报告 · 看不见的呼吸点也一直在要帧

## 1. 根因（本轮实测，file:line 为修前）

终端页浮条（`internal/terminalui/terminalui.go` 的 `titleBar`）在**未揭示**时把整条身份行
画成零不透明度：

- `:1389-1390` `_, _, hovered := bar.PointerPosition()` / `revealed := hovered || bar.FocusWithin()`
- `:1424-1426` `if !revealed { identity.Opacity(0) }`
- `:1430` `a.statusDot(c, inks)` —— **无条件**调用
- `:1529-1536` `statusDot` 里
  `motion.Value(c, func() float32 { return dot.Loop("terminal.dot", 2400*time.Millisecond, ui.Bounce(ui.EaseInOut)) })`，
  然后 `dot.Opacity(1-0.5*breath)` / `dot.Scale(1-0.12*breath, …)`

`Element.Loop`（`mygo v0.7.0 ui/widgets.go:69-84`）每帧调 `e.c.AnimationFrame()`；
`ui/context.go:395` 把它翻成 `rt.animating = true`，`ui/runtime.go:744-748` 对它的处理是
**下一帧重建整棵视图**（不是重绘）。于是一个**零不透明度**的 1.75u 圆点，让整个终端视图
按显示器刷新率无限重建：120 Hz 面板上每秒 120 次，只为一个谁也看不见的动画。

`motion.Value` 全仓只有三处，另外两处都不是常驻的：启动器分页环只在 `footerLoading` 时建
（`internal/launcher/output.go:284-288`）、设置页动作转圈只在 `busy` 时建
（`internal/settingsui/rows.go:564`）。**只有终端这一处是常驻的。**

## 2. 落地

- `internal/terminalui/terminalui.go`
  - `statusDot(c *ui.Context, inks barInks)` → `statusDot(c *ui.Context, inks barInks, drawn bool)`；
    调用点传 `revealed`。
  - 函数体：`exited` 早退不变；其后 `if !drawn { return }` **再**读循环。揭示时的周期
    （2400ms）、缓动（`Bounce(EaseInOut)`）、两个 keyframe（`1-0.5*breath` /
    `1-0.12*breath`）一个字节没动，`motion.Value` 的 reduce-motion 语义也没动。
- `internal/terminalui/idleframes_test.go`（新）
  - `TestTheAtRestStatusDotAsksForNoFrames`：三臂。
    ① 指针停在画布上（`Move(450,400)`）→ `FramePending()` 必须 **false**；
    ② 指针移到浮条上（`Move(150,14)`，点被画出来）→ 必须 **true**（证明第一臂不是
       靠「循环被整个删掉」换来的绿）；
    ③ 同一揭示态 + `ReduceMotion` → 必须 **false**（证明前两臂量到的就是那个环境循环，
       不是别的过渡）。
  - `TestDumpTheAtRestTerminalPNGs`（opt-in，`FLOTER_PNG=1`）：本轮像素证据的来源。

## 3. V / B / S 台账

- **V = 0**：没有任何像素值/几何常数被重定基线。未揭示时父级 `Opacity(0)` 早已把点藏住，
  修前修后的静止帧**逐字节相同**（§5）。
- **B = 0**：既有测试零改动全绿。
- **S = 0**：没有结构变化（元素树、几何链、动效参数都不动；只多了一个布尔参数与一道守卫）。

## 4. 门槛（本轮实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `GOOS=windows go vet ./...` | 净 |
| `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，FAIL 0 |
| `--- PASS` 计数 | **2452 → 2453** |
| `--- SKIP` 计数 | 49 → **50**（+1，是 opt-in 的截图 dump） |
| 测试函数 | 2001 → **2003**（+1 pin、+1 dump） |
| 三平台 `go build ./...` | 全过 |

## 5. 变异（实跑，逐条变红后还原）

- **M1**：删掉 `if !drawn { return }`（回到无条件读循环）
  ```
  --- FAIL: TestTheAtRestStatusDotAsksForNoFrames (0.03s)
      idleframes_test.go:42: the at-rest terminal page asked for another frame: the invisible status dot still reads its loop
  ```
- **M2**：调用点把 `revealed` 写死成 `true`（守卫还在，但永远为真）
  ```
  --- FAIL: TestTheAtRestStatusDotAsksForNoFrames (0.03s)
      idleframes_test.go:42: the at-rest terminal page asked for another frame: the invisible status dot still reads its loop
  ```
- 还原：`md5sum -c` → `internal/terminalui/terminalui.go: 成功`；`touch` 后复跑同一条 pin 绿
  （避开测试缓存）。

## 6. 视觉证据（我实测看到了什么）

`/tmp/floter-P11-bm-rest-light-{before,after}.png`、`/tmp/floter-P11-bm-rest-dark-{before,after}.png`
（900×620，指针停在画布上）：

- 修前/修后**同一外观逐字节相同**（`cmp` 无差异，light 与 dark 都 identical）。这正是不变量：
  本轮拿掉的是「看不见的动」的代价，不是任何一块墨。
- 图上看到的东西：浮条未揭示，画布自上而下是 shell 的两行提示与空行，状态点、会话名、
  四个按钮**一个都不可见**——也就是说修前那 120 帧/秒确实只是在重画这张静止画面。
- 揭示态的图仍在 `TestDumpTerminalPagePNGs` 产出的 `/tmp/floter-p8e-terminal-*.png` 里：
  指针在浮条上时点照旧呼吸（第二臂的 `FramePending()==true` 是它的机器版）。

## 7. 仍待（P11-bm）

1. 老拍板项原样有效（见 `.pending-bugs.md` 的「需用户拍板」表）：搜索框 caret / 默认浅色
   3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 便条投影 /
   P11-bd 未揭示时的窗口拖拽手柄 / P11-be `Permissions` 行用词 / P11-bf `configDiscardBar`
   12u / P11-bg 顶边失败带覆盖画布顶部两行。
2. **同族下一步（不需拍板，下一轮可领）**：**揭示态**下点仍在以「重建整棵视图」的代价呼吸。
   `mygo v0.7.0` 为此提供了 `Painter.AnimationFrameRate(fps)`（`ui/paint.go:126-145`：
   「按这个帧率重画**而不重建**」），把点从 `Element.Opacity/Scale` 换成
   `Draw(func(p, r))` 里的圆 + `p.AnimationFrameRate(30)` 即可，120 次重建/秒 → 30 次重绘/秒。
   本轮没做是因为它要改点的画法（墨与面的像素 pin 会动，属 V 类重定基线），一个表面一条
   根因，留给单独一轮。
3. 侦察结论：`motion.Value` 的另外两处（启动器分页环、设置页动作转圈）都只在对应工作
   **进行中**才建，静止时本来就不注册循环——**别把它们当新 bug 重领**。
