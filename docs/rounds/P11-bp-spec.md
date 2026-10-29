# P11-bp 规格 · 转圈在画的那一帧里转，不再每帧重建整棵视图

## 0. 意图

同一个族（`internal/motion` 的三个环境动效）里的最后两处仍在**构建期**读相位：
启动器分页环 `footerSpinner` 与设置页的 in-flight 标记 `actionSpinner`。它们各自
`motion.Value(c, func() { e.Loop(key, …) })`，而 `Loop` 要的是
`Context.AnimationFrame`——`runtime.go:744-748` 把它翻成「下一帧**重建**整棵视图」。
于是「输出正在翻页」「扩展动作正在跑」这两种状态期间，整个启动器视图 / 整个设置页
按显示器刷新率重建（120 Hz 面板 = 每秒 120 次）。

为什么现在做：P11-bo 仍待第 3 条（同族 perf 项）点名这两处，并写明「下一轮可领」；
P11-bn 已经把同族的第三个动效（终端状态点）搬成「绘制期读相位」，本轮把剩下的两处
按同一手法收口，族内不再留第三份答案。用户 2026-10-28 定的优先档里 F4 原话含
「流畅性」，这一条正落在上面。

## 1. 逐条落地项

### 1.1 启动器分页环 `internal/launcher/output.go:314-330`

- **现状**：`ring := ui.Box(...)`；`ring.Draw(func(p, r){ 画整圆 + Trim(0,0.25) 的引头 })`；
  `ring.Rotate(footerSpinnerTurn * motion.Value(c, func() float32 { return ring.Loop(
  "launcher.footer.spinner", footerSpinnerPerTurn, ui.Linear) }))`。
  `Box.Rotate` 不是 `kindIcon`，它把旋转 `Then` 进 `e.transform`（`ui/svg.go:96-105`），
  所以旋转量必须在**构建时**定下来；而定它的 `Loop` 每帧要一次全新视图。
- **应改成**：相位在 `draw` 回调里按 `p.Now()` 读，起点落在新的 `App.footerSpin`
  （元素状态随重建而丢，绘制不该丢——与 `terminalui.App.dotBreath` 同一条规则）；
  旋转用 `p.Transform(ui.Rotation(angle), cx, cy, func(){ … })`（画在元素自己的
  坐标系里，圆的描边宽度不受影响）；并用 `p.AnimationFrameRate(footerSpinnerFPS)`
  要的是**只重画**的帧。

### 1.2 设置页 in-flight 标记 `internal/settingsui/rows.go:556-566` —— **本轮未动（实测否掉）**

- **现状**：`icon := ui.Icon(c, mustGlyph("loader-circle"))…`；
  `icon.Rotate(actionSpinnerTurn * motion.Value(c, func() float32 {
  return icon.Loop(key, actionSpinnerPerTurn, ui.Linear) }))`。
- **本想改成**：`Icon` 是 `kindIcon`，看起来它的 `rotate` 是绘制期读的
  （`ui/paint.go:322-328` 在 `e.paintFn` 之后读 `e.rotate`），于是把
  `icon.Rotate(angle)` 放进 `icon.Draw` 回调。
- **实测：这样不行**。`p.draw`（`ui/repaint.go:38-58`）只把 **Draw 回调产出的 op**
  记成可重画的 part，`repaintParts` 重跑的是这些回调；图标自己那笔 op 在
  `paint.go:322-328` 的 `kindIcon` 分支里，属于静态场景，重画时原样回放。
  实测：900ms 里标记的墨逐字节不变（38 px / 2220 mass）。
- **也否掉了替代方案**：回调里 `p.Transform` + `p.Icon` 是转一个已光栅化的蒙版。
  实测同一枚 10u 图标同 30°：元素 `Rotate` 38 px / 4221 mass，painter 变换
  49 px / 4070 mass（px +29%、墨 −3.6%），弧明显变虚。
- 结论写进报告 §2.1 与 §7 仍待；要动它需框架支持或给这枚图标再造一份矢量弧（拍板）。

## 2. 行为契约零改动红线

- **墨**：环的 `footerSpinnerStroke = 1.5`、`footerSpinnerTurn = 360`、
  `footerSpinnerPerTurn = 700ms`；引头 `Trim(0, 0.25)`；`Hairline` + `TextMuted`
  两档墨；`t.Space(3)` 的 12u 盒。设置页 `actionSpinnerTurn = 360`、
  `actionSpinnerPerTurn = 900ms`、`t.Space(2.5)` 的 10u 盒、`ink`。
- **减少动效**：仍经 `internal/motion`：`ReduceMotion` 时返回 0（0% keyframe）、
  **不读相位**、**不注册帧**。`motion.TestValueRestsUnderReduceMotion` 零改动。
- **布局**：两个元素的外框尺寸、`Shrink(0)`、gap、行高一个字节不改。
- **行为**：分页状态机（`outputFooterState`）、busy 状态（`repairBusy`/`uninstallBusy`）、
  键鼠模型、i18n 文案、命令接线：不碰。

## 3. 门槛命令 + PASS 基线（本轮实测，动手前）

```
gofmt -l cmd internal           → 空
go vet ./...                    → 净
GOOS=windows go vet ./...       → 净
GOOS=darwin  go vet ./...       → 净
go test -count=1 ./...          → 全绿，PASS 2455 / SKIP 51 / FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 2455
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2006
GOOS=linux/windows/darwin go build ./... → 三份过
```

## 4. 新 pin（两条）+ 一条 opt-in dump

1. `internal/launcher/spinnerframes_test.go`
   `TestTheFootersRingTurnsWhilePaintingInsteadOfRebuilding`
   - 臂 A：`Advance(footerSpinnerPerTurn)` 期间视图**构建 0 次**，且
     `FramePending()` 为真（帧仍在要，只是不重建）；
   - 臂 B：环比 —— 相位 0 与 1/4 周期时，环的引头相对圆心转过约 90°
     （`45°..135°` 的带子），证明「转」是真在读相位，不是画了个静态环。
     实测 46.4° → 131.9°（85.5°）。
2. `TestTheFootersRingRestsUnderReduceMotion`：`ReduceMotion` 下 `FramePending()`
   为假、构建 0 次、环仍画在 0% 且不动。
3. `TestDumpTheFooterSpinnerPNGs`（`FLOTER_PNG=1`）写
   `/tmp/floter-P11-bp-footer-<dark|light>-{rest,turn}.png`。
   设置页那一半本轮不动，因此没有它的 pin（理由见 §1.2）。

## 5. Mutation 计划（每条都会让哪个 pin 变红）—— 实跑结果见报告 §6

- **M1**：把 `footerSpinner` 的相位读回构建期的 `ring.Loop(...)` →
  `TestTheFootersRingTurnsWhilePaintingInsteadOfRebuilding` 臂 A 红
  （实测 `built the view 42 times over one turn`）。
- **M2**：删掉 `p.AnimationFrameRate` → 同一 pin 的「要帧」红
  （实测 `asked for no frame at all`）。
- **M3**：把环的旋转固定成 `0`（读相位但不使用）→ 角度臂红
  （实测 46.43° → 46.43°，`0° of the circle`）。

## 6. 台账预期

V = 0（没有既有像素 pin 钉过这两处理的角度；环/标记的形状、尺寸、墨不变）。
B = 0（既有 34 条相关 pin 零改动）。S = 0。
