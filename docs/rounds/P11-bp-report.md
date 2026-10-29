# P11-bp 报告 · 分页环在画的那一帧里转，不再每帧重建整棵视图

## 1. 根因（file:line 为修前）

`internal/launcher/output.go:314-330` 的 `footerSpinner`：

```go
ring.Draw(func(p *ui.Painter, r ui.Rect) { …整圆 + Trim(0, 0.25) 的引头… })
ring.Rotate(footerSpinnerTurn * motion.Value(c, func() float32 {
    return ring.Loop("launcher.footer.spinner", footerSpinnerPerTurn, ui.Linear)
}))
```

`Element.Loop`（`mygo v0.7.0 ui/widgets.go:69-84`）每次都调 `e.c.AnimationFrame()`，
`ui/runtime.go:744-748` 把它翻成「下一帧**重建**整棵视图」；而 `Box.Rotate` 不是
`kindIcon`，它把旋转 `Then` 进 `e.transform`（`ui/svg.go:96-105`），所以角度只能在
**构建时**定下来。后果：输出正在翻页的那几百毫秒里，整个启动器视图按显示器刷新率重建
（120 Hz = 每秒 120 次）为一个 12u 的环。实测（新增 pin，修前同一条判据）：
**一次完整转动（700ms）里视图重建 42 次**。

## 2. 落地

`internal/launcher/output.go` 的 `footerSpinner`：

- 相位搬进 `Draw` 回调：`motion.Value(c, func() float32 { … p.Now() … })`，起点落在新的
  `App.footerSpin`（元素状态随重建而丢，绘制不该丢——与 `terminalui.App.dotBreath` 同一条规则）；
- 旋转改成 `p.Transform(ui.Rotation(angle), cx, cy, func(){ 画整圆 + 引头 })`：画在元素
  自己的坐标系里，圆的描边在旋转下宽度不变，还是 1.5px；
- 要帧改成 `p.AnimationFrameRate(footerSpinnerFPS)`（60，一圈 0.7s 需要平滑角度），
  减少动效时一个都不注册。

`internal/launcher/launcher.go` 新增 `App.footerSpin time.Time`（带文档注释）。

### 2.1 同族的另一半故意没做（有实测依据，见 §6）

`internal/settingsui/rows.go` 的 `actionSpinner` 也是构建期 `Loop` + `icon.Rotate`。它
**不能**照搬这套手法，原因是框架的绘制分层：

- `p.draw`（`ui/repaint.go:38-58`）只把 **Draw 回调产出的 op** 记成可重画的 part
  （`rt.drawn`），`repaintParts`（`repaint.go:89-131`）重跑的是这些回调；
  **图标自己的那笔 op 不在回调里**，它在 `paint.go:322-328` 的 `kindIcon` 分支，
  属于静态场景，重画时原样回放。
- 也就是说「在 Draw 里写 `icon.Rotate(angle)`」改的只是下一**次构建**的字段，重画那一趟
  仍画旧角度。本轮实测：启动器那条 pin 同款判据下，标记的墨在 900ms 里
  **逐字节不变**（38 px / 2220 mass 两帧相同），角度也没动。
- 唯一在回调里能画的旋转是 `p.Transform` + `p.Icon`，但它是把**已光栅化的蒙版**用仿射
  矩阵转。实测同一枚 10u 图标同一角度（30°）两种画法：
  **元素 `Rotate` → 38 px / 4221 mass；painter 变换 → 49 px / 4070 mass**
  （px +29%、墨 −3.6%）：弧被bilinear 摊薄了，肉眼就是「变虚」。为了省下几百毫秒的
  重建把一个 in-flight 标记画虚，不是这一档该做的取舍，所以本轮**不动它**，
  两条出路记进「仍待」（要框架给「绘制期旋转图标」或把弧当路径画，后者等于给一枚
  图标再造一份几何，需要拍板）。

## 3. V / B / S 台账

- **V = 0**：没有一条像素数值需要重定基线。环的 12u 盒、`footerSpinnerStroke = 1.5`、
  `Trim(0, 0.25)` 的引头、`Hairline` + `TextMuted` 两档墨、`700ms` 一圈：一个字节没动。
  证据见 §5：深浅两外观的 **rest 与 turn 四张图与修前逐字节相同**。
- **B = 0**：既有测试零改动全绿（`TestOutputFooterLoadingDrawsTheRing`、
  `TestOutputFooterStates`、`internal/motion` 的两条）。
- **S = 0**：结构没变（还是「一个 12u 的 Box，画圆 + 引头」）。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
GOOS=windows go vet ./...                          → 净
GOOS=darwin  go vet ./...                          → 净
go test -count=1 ./...                             → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 2457（基线 2455）
... | grep -c -- "--- SKIP"                        → 52（基线 51，+1 为本轮 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2009（基线 2006，+3）
GOOS=linux/windows/darwin go build ./...           → 三份过
```

新 pin：`TestTheFootersRingTurnsWhilePaintingInsteadOfRebuilding`（三件事：一整圈里
**构建 0 次** / 仍在要帧 / 引头四分之一周期转过 45°–135°）与
`TestTheFootersRingRestsUnderReduceMotion`（不建、不要帧、不动）。
实测角度：**46.4° → 131.9°，转了 85.5°**（四分之一周期 = 90°，差 4.5° 是一帧的滞后）。

## 5. 视觉证据

`/tmp/floter-P11-bp-footer-{light,dark}-{rest,turn}.png` 与同名的 `-before.png`
（修前代码同一时刻的两帧）。`cmp` 结果：

```
light-rest: identical   light-turn: identical
dark-rest:  identical   dark-turn:  identical
```

四张图**逐字节相同**——两条代码路径都把转动的起点放在第一个被画出来的帧上，所以同一
时刻的角度相同。这张图上看到的一处变化：**没有变化**，本轮拿掉的是一次次整棵视图的
重建，不是任何一块墨。

## 6. 变异（三条实跑变红 + `md5sum -c` 还原 + `touch` 复跑）

| # | 注入的缺陷 | 红在哪 | 原始报错 |
|---|---|---|---|
| M1 | 相位读回构建期的 `ring.Loop(…)` | 新 pin 臂 A | `the paging ring built the view 42 times over one turn: the frames are builds, not paints` |
| M2 | 删掉 `p.AnimationFrameRate` | 新 pin 的「要帧」 | `the paging ring asked for no frame at all: nothing is driving it` |
| M3 | 角度读出来但固定成 0 | 新 pin 的角度臂 | `the ring's quarter went from 46.43164° to 46.43164° over a quarter of its period (0° of the circle)` |

三条都 `cp` 原文件还原后 `md5sum -c` 成功（`internal/launcher/output.go: 成功`），
再 `touch internal/launcher/*.go` 复跑全绿。

## 7. 仍待（P11-bp）

1. 老拍板项原样有效（`.pending-bugs.md`「需用户拍板」表）：搜索框 caret / 默认浅色
   3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 便条投影 /
   P11-bd 未揭示时的窗口拖拽手柄 / P11-be `Permissions` 行用词 / P11-bf
   `configDiscardBar` 12u / P11-bg 顶边失败带覆盖画布顶部两行。
2. **同族剩下的最后一处**：设置页 `actionSpinner`（`internal/settingsui/rows.go:556-566`）
   仍是构建期 `Loop` + `icon.Rotate`。**不要照搬本轮手法**——实测无效（重画回放的是
   图标那笔静态 op，见 §2.1）。两条出路：
   (a) 框架给「绘制期读角度的图标」（`Painter.Icon` 加 rotate，或让 `kindIcon` 的 op
   进 part）；(b) 把 LoaderCircle 画成矢量弧（等于给这枚图标再造一份几何，需拍板）。
3. 侦察结论（**未逐条核对，下一轮先量再动手**）：`internal/` 里剩下的
   `c.AnimationFrame()` 调用点（`launcher/fieldkeys.go:281`、`power.go:103`、
   `launcher.go:3053`、`settingsui/debounce.go:105`、`savealert.go:55`、
   `settingsui.go:6074,6131`、`shell/notices.go:396`、`shell/shell.go:4305`、
   `terminalui.go:1265`）看起来多是**一次性**的「下一帧再做」或输入光标闪烁，
   不是「构建期读环境动效相位」。要动它们得先量「谁在一直在要帧、要了多少次」。
