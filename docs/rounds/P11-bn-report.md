# P11-bn 报告 · 活着的状态点按 30 帧「重画」，不再按刷新率「重建整棵视图」

## 1. 根因（file:line 为修前）

终端页浮条的身份行里那个 1.75u 状态点，**揭示**（指针在浮条上）时这样呼吸
（`internal/terminalui/terminalui.go:1543-1547`）：

```go
breath := motion.Value(c, func() float32 {
    return dot.Loop("terminal.dot", 2400*time.Millisecond, ui.Bounce(ui.EaseInOut))
})
dot.Opacity(1 - 0.5*breath)
dot.Scale(1-0.12*breath, 1-0.12*breath)
```

`Element.Loop`（`ui/widgets.go:69-84`）要的是 `Context.AnimationFrame`
（`ui/context.go:395`），而 `runtime.go:744-748` 把它翻成 **`host.requestFrame()` 且
`rt.animating`** —— 也就是「下一帧**重建整棵视图**」。P11-bm 只给**未揭示**（零不透明度）
那一臂加了 `if !drawn { return }` 守卫；**揭示**时仍然每次刷新重建整个终端视图
（120 Hz 面板 = 每秒 120 次），为一个半透明小圆点的淡入淡出。

框架为这件事准备了另一档：`Painter.AnimationFrameRate(fps)`（`ui/paint.go:126-145`）
请求的是**只重画**（`rt.repainting` → `repaintFrame`：「paints the elements of the last
frame again … the view is neither built nor laid out」，`runtime.go:729-737`、`:878-905`），
并且按请求的帧率来（`pace`）。它只能在**画**的时候读时间（`Painter.Now()`，`paint.go:100`），
于是相位必须从「构建期」搬到「绘制期」。

## 2. 落地

- `internal/terminalui/terminalui.go`
  - `statusDot`：构建期不再画那个不透明的 `dot.Background`，改成
    `dot.Draw(func(p *ui.Painter, r ui.Rect))` —— 里面按 `p.Now()` 算同一个 `breath`，
    `p.Fill` 一个直径 `min(r.W,r.H)*(1-0.12*breath)`、alpha `1-0.5*breath` 的圆，
    并 `p.AnimationFrameRate(dotBreathFPS)`（=30）。
  - 相位起点落到新字段 `App.dotBreath time.Time`：**第一个被画出来的帧**记下它，
    之后 `(now-start) % 2400ms`。放 App 而不是元素状态，是因为元素状态随重建而丢，
    绘制不该丢（放元素上就只能靠重建来续命）。
  - 常量 `dotBreathPeriod = 2400ms` / `dotBreathFPS = 30` 放在函数后，注释写清它们
    是旧 build 的 2.4 s 循环，不是新设计数。
  - 减少动效仍走 `internal/motion.Value`：它返回 `0`（0% keyframe）且**不读相位**，
    所以那一帧既不记起点、也不注册 `AnimationFrameRate` —— 静止的桌面依旧拿不到任何帧。
  - 关键帧一个字节没动：周期 2400ms、缓动 `ui.Bounce(ui.EaseInOut)`、
    `opacity = 1-0.5b`、`scale = 1-0.12b`。点的 1.75u、浮条 3u 内缩、墨
    （`canvasInks` 的 `Tertiary`）、`PassThrough`、exited 态的 0.7 静态灰点、
    浮条高度链：全部未动。
- `internal/terminalui/idleframes_test.go`
  - 新 pin `TestTheRevealedDotsBreathPaintsAgainInsteadOfBuilding`：三件事一次量。
    ① `Tester.Advance(1200ms)` 期间**视图构建次数必须为 0**（`Advance` 的帧走
    `surfaceFrame` → `repaintFrame`，只重画）；② 同一时刻 `FramePending()` 仍为 `true`
    （不是靠「什么都不要帧」换来的绿）；③ 同一次 Advance 里点的墨量必须变（相位确实
    在**画**的时候读）。最后附一条：50% keyframe 的中心像素 == `--text-tertiary` @0.5。
  - `TestDumpTheDotBreathPNGs`（opt-in，`FLOTER_PNG=1`）：本轮像素证据。

## 3. V / B / S 台账

- **V = 0**：没有一条既有 pin 需要改。0% keyframe（静止/减少动效）**逐字节不变**（§6）。
  50% keyframe 的两个数（0.5 / 0.88）由新断言正面钉住。
- **B = 0**：既有测试零改动全绿（`go test -count=1 ./...`）。
- **S = 0**：结构没变（元素树、几何、`PassThrough`、动效参数），变的是「谁在什么时候读相位」。

唯一可观察的差别：**呼吸的起点从「第一次构建」移到「第一帧被画出来」**——这正是
P11-bm 的语义（画不出来就不走）。实测见 §6 的 mid-breath 两张图。

## 4. 门槛（本轮实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `GOOS=windows go vet ./...` | 净 |
| `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，FAIL 0 |
| `--- PASS` 计数 | **2453 → 2454** |
| `--- SKIP` 计数 | 50 → **51**（+1，opt-in 截图 dump） |
| 测试函数 | 2003 → **2005**（+1 pin、+1 dump） |
| 三平台 `go build ./...` | 全过 |

## 5. 变异（实跑，逐条变红后还原）

- **M1**：`p.AnimationFrameRate(dotBreathFPS)` → `c.AnimationFrame()`（回到「重建」那一档）
  ```
  --- FAIL: TestTheRevealedDotsBreathPaintsAgainInsteadOfBuilding (0.04s)
      idleframes_test.go:149: the raised bar's breath built the view 72 times over one half period: the frames are builds, not paints
  ```
  （72 = 1200ms 里 tester 的 60 Hz 帧数：修前揭示态就是这么多整树重建。）
- **M2**：整句 `if !c.Preferences().ReduceMotion { p.AnimationFrameRate(...) }` 删掉
  ```
  --- FAIL: TestTheAtRestStatusDotAsksForNoFrames (0.05s)
      idleframes_test.go:56: the revealed bar's breathing dot asked for no frames at all: the loop is gone, not just gated
  --- FAIL: TestTheRevealedDotsBreathPaintsAgainInsteadOfBuilding (0.00s)
      idleframes_test.go:153: the raised bar's breathing dot asked for no frame at all: nothing is driving it
  ```
- **M3**：墨漂一档（`ink := inks.Tertiary` → `inks.Muted`）
  ```
  --- FAIL: TestTheStatusDotIsNeutralAndStillBreathes (0.03s)
      accentguard_test.go:86: the resting dot is {128 128 135 255 [0 0 0]}, want the tertiary step at full opacity ({142 142 148 255 [0 0 0]})
  --- FAIL: TestTheTerminalBarDotDropsItsHaloAndWearsTertiary (0.00s)
      barchrome_test.go:178: the live dot's centre = {128 128 135 255 [0 0 0]}, want the tertiary step ({142 142 148 255 [0 0 0]})
  ```
- **M4**：起点每帧重置（`if a.dotBreath.IsZero() { … }` → 无条件 `a.dotBreath = p.Now()`）
  ```
  --- FAIL: TestTheStatusDotIsNeutralAndStillBreathes (0.04s)
      accentguard_test.go:97: the live dot's ink mass is 14816 at the 0% keyframe and 14816 at the 50% one (a difference of 0): the breath is not running, so a neutral dot says nothing at all
  --- FAIL: TestTheRevealedDotsBreathPaintsAgainInsteadOfBuilding (0.00s)
      idleframes_test.go:157: the dot's ink mass is 14816 at the 0% keyframe and 14816 at the 50% one (a difference of 0): the phase is not read while painting
  ```
- 还原：`md5sum -c` → `internal/terminalui/terminalui.go: 成功`，`gofmt -l` 空，
  `touch` 后复跑同四条 pin 全绿（避开测试缓存）。

## 6. 视觉证据（我实测看到了什么）

`FLOTER_PNG=1` 的 dump 在修前（`git show HEAD:` 的原文件）与修后各跑一次，
两张一对（900×40 的浮条，指针停在浮条上）：

- `/tmp/floter-P11-bn-dot-{light,dark}-{rest,breath}-{before,after}.png`
- **rest（0% keyframe）：`cmp` 逐字节相同**——浅色与深色都 identical。这正是本轮的不变量：
  墨、圆的大小、AA 边缘、位置一个像素没动。
- **breath（`Advance(1200ms)` 那一帧）：两张不同**，原因写在这里而不是含糊过去：修前的
  相位从「第一次构建」起算（tester 构造时就建过一次，那时指针还在 0,0、浮条没揭示），
  所以 1200ms 已经越过 50% keyframe 一点点；修后从「第一帧被画出来」起算，正好落在 50%。
  实测点区（x 0..22, y 0..40）的墨量和：before **5732** / after **5356**，两者都比 rest
  的 **14816** 淡得多——即两张都是呼吸中的帧，差的是相位起点，不是关键帧数值。
  50% keyframe 本身由新断言正面钉住（中心像素 == `--text-tertiary` @0.5）。
- 我看图看到的：呼吸到半程时那个点明显变小变淡（7u → 约 6u、透明度减半），
  周围浮条与标题墨一个像素没变。

## 7. 仍待（P11-bn）

1. 老拍板项原样有效（`.pending-bugs.md` 的「需用户拍板」表）：搜索框 caret /
   默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 便条投影 /
   P11-bd 未揭示时的窗口拖拽手柄 / P11-be `Permissions` 行用词 / P11-bf `configDiscardBar`
   12u / P11-bg 顶边失败带覆盖画布顶部两行。
2. **同族下一步（不需拍板，下一轮可领）**：`footerSpinner`
   （`internal/launcher/output.go:310-326`）也还是构建期 `Rotate` + `Loop`——
   分页转圈只在加载中出现，代价比点缀小，但**同一个手法**（`Draw` 里 `p.Now()` +
   `AnimationFrameRate`）可以直接搬；`localSpinner`/设置页动作转圈
   （`internal/settingsui/rows.go:564`）同理。搬的时候注意它的 `Rotate` 是元素变换
   （绘制期也算变换），改成 `p.Transform` 或路径旋转，像素会动（V 类）。
3. 侦察结论：`Tester` 的帧永远按 60 Hz 出（`headless.go:335-339`，`due()` 不读
   `paceEvery`），所以「30 fps」这个**数字**在测试里量不出来，本轮量的是它的**性质**
   （视图零构建）。别为了量 30 去改 tester。
