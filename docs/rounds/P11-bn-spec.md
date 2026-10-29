# P11-bn 规格 · 活着的状态点按 30 帧「重画」，不再按刷新率「重建整棵视图」

## 1. 意图与证据

`internal/terminalui` 浮条上的状态点在**揭示**态下仍然走 `Element.Loop`
（`terminalui.go:1543-1547`）→ `Context.AnimationFrame` → `runtime.go:744-748`
「下一帧**重建**整棵视图」。P11-bm 只给**未揭示**（零不透明度）那一臂加了守卫，
揭示态每帧仍是整棵树重建：120 Hz 面板上每秒 120 次，端点只是一个 1.75u 的呼吸圆点。

`mygo v0.7.0` 为这件事提供了两个东西（P11-bm 报告 §7 第 2 条已点名、本轮落地）：

- `Painter.AnimationFrameRate(fps)`（`ui/paint.go:126-145`）：请求的帧是**只重画**，
  「the window paints it again as often, rather than at each of the display's refreshes …
  the next frame paints the elements of this one again **without building the view**」。
- `Painter.Now()`（`ui/paint.go:100`）：重画帧的时间，`Draw(func(p *ui.Painter, r ui.Rect))`
  回调里读，值随帧动而树不动（同包 `footerSpinner`/`internal/launcher/output.go:316`
  已是这个手法，只是它用 `Rotate` + `Loop`，仍在构建期）。

## 2. 逐条落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go` `statusDot` 尾部 | 构建期 `dot.Background(inks.Tertiary)` + `motion.Value(c, dot.Loop("terminal.dot", 2400ms, Bounce(EaseInOut)))` → `Opacity(1-0.5b)` + `Scale(1-0.12b)` | 构建期不再画不透明的点；`dot.Draw(func(p, r))` 里按 `p.Now()` 算同一个 `b`，`p.Fill` 一个直径 `min(r.W,r.H)*(1-0.12b)`、alpha `1-0.5b` 的圆，并 `p.AnimationFrameRate(30)` |
| 相位起点 | 元素状态里的 `Loop` 起点（元素 id 稳定，跨帧保留） | App 字段 `dotBreath time.Time`：第一个**被画出来**的帧记下起点，之后 `(now-start) % 2400ms`；未揭示/减少动效时不记 |
| 减少动效 | `motion.Value` 返回 0（不读相位、不注册循环） | 不变：仍走 `internal/motion.Value`，`0` 时画 0% keyframe，且**不**注册 `AnimationFrameRate` |

关键帧一个字节不动：周期 `2400ms`、缓动 `ui.Bounce(ui.EaseInOut)`、
`opacity = 1-0.5b`、`scale = 1-0.12b`（0% 与 50% 两个 keyframe 与旧 build 一致）。

**不做的事**：不改点的大小（1.75u）、位置（浮条 3u 内缩）、墨（`canvasInks` 的
`Tertiary`）、`PassThrough`、exited 态（`0.7` 静态灰点）、浮条的高度链。

## 3. 行为契约零改动红线

高度链（浮条 7u / 身份行）、键盘模型、会话管理、设置写回、i18n 文案、命令接线
一律不动。本轮只动「这个点在哪一帧被画、由谁驱动」。

## 4. 门槛命令 + PASS 基线（P11-bm 收尾实测值）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 0 FAIL
--- PASS 计数                             # 2453（基线）
--- SKIP 计数                             # 50
测试函数                                  # 2003
三平台 go build ./...                     # 过
```

## 5. mutation 计划

- **M1**：把 `p.AnimationFrameRate(dotBreathFPS)` 换成 `p.AnimationFrame()`（回到
  「重建整棵视图」那一档）→ 新 pin `TestTheRevealedDotPaintsAgainInsteadOfBuilding`
  必须红（`tt.Advance` 期间视图被重建）。
- **M2**：删掉 `if !drawn { return }` 之前的……不适用；改为把 `p.AnimationFrameRate`
  整句删掉 → 同一条 pin 的「还在要帧」那一臂必须红。
- **M3**：Alpha 换成 `1-0.12*breath`（缩放和透明互换）→ 呼吸的墨量 pin
  （`TestTheStatusDotIsNeutralAndStillBreathes`）必须红。

## 6. 视觉证据

`FLOTER_PNG=1` dump：`/tmp/floter-P11-bn-dot-{light,dark}-{rest,breath}.png`
（0% 与 50% 两帧，900×40 的浮条），报告写「我实测看到了什么」。
