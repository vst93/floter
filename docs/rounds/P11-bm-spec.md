# P11-bm 规格 · 看不见的呼吸点也一直在要帧

## 意图（一句话）

终端页在**指针不在浮条上**（点根本不可见）时仍然每帧向窗口要一次重绘——一整棵终端视图
按显示器刷新率重建（120 Hz 面板 = 每秒 120 次），只为一个没有人看得见的动画；本轮让它
**只在画得出来的时候**才要帧。

为什么现在做：用户实测反馈 F1/F2/F3/F4 已由 P11-y/z/aa/bb/bj/bk 收完，合约 §6 全表
`已落地`（只剩 `⌘K`＝拍板），上一轮「仍待」全是拍板项。按用户 2026-10-28 定的优先档
（界面质感与交互 → 基础功能 → 三平台一致性 → 像素细节），F4 的原话还带着「**流畅性**」
三个字，而这是全仓唯一一处「不动也在烧帧」的地方。

## 证据（本轮实测）

- `internal/terminalui/terminalui.go:1389` `_, _, hovered := bar.PointerPosition()` /
  `:1390` `revealed := hovered || bar.FocusWithin()`。
- `:1424-1426`：`if !revealed { identity.Opacity(0) }` —— 未揭示时**整条身份行（含状态点）
  画成零不透明度**。
- `:1430` `a.statusDot(c, inks)` 无条件调用；`:1529-1536` 里面用
  `motion.Value(c, func() float32 { return dot.Loop("terminal.dot", 2400ms, Bounce) })`。
- `Element.Loop`（`ui/widgets.go:69-84`）每帧 `e.c.AnimationFrame()`；`context.go:395`
  把它翻成 `rt.animating = true`，`runtime.go:744-748` 的意思是**下一帧重建整棵视图**
  （不是重绘）。所以：终端页一开着、指针又不在浮条上，视图就按刷新率无限重建。
- 全仓 `motion.Value` 只有三处：终端状态点（本条）、启动器分页环（只在
  `footerLoading` 时建，见 `internal/launcher/output.go:284-288`）、设置页动作转圈（只在
  `busy` 时建）。**只有终端这一处是常驻的**。

## 落地项

1. `internal/terminalui/terminalui.go`
   - `statusDot(c *ui.Context, inks barInks)` → `statusDot(c *ui.Context, inks barInks, drawn bool)`，
     调用点传 `revealed`。
   - 函数里：`exited` 的早退不变；其后 `if !drawn { return }` **再**读 `motion.Value`。
     即：点画不出来时不注册循环，画得出来时行为一字不改（周期 2400ms、`Bounce(EaseInOut)`、
     `1-0.5*breath` / `1-0.12*breath` 全部零改动）。
   - 注释写清根因（`Element.Loop` 要的是**重建**，不是重绘；零不透明度不会让循环停下来）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节都不动**。
墨与面也不动：未揭示时父级 `Opacity(0)` 已经把点藏住，本轮只是不再为一个藏起来的元素
要帧；揭示时的像素与修前逐字节相同（下面 §验证 第 3 条）。

## 验证

1. `go test -count=1 -run TestTheAtRestStatusDotAsksForNoFrames ./internal/terminalui/`
   —— 修前红、修后绿。
   - 第一臂：指针停在画布上（`Move(450,400)`），`Frame()` 之后
     `tt.FramePending()` 必须为 false（「这一帧之后没有任何东西还要下一帧」）。
   - 第二臂：指针移到浮条上（`Move(150,14)`，点被画出来），同一断言必须为 **true** ——
     证明第一臂的绿不是「循环被整个删掉」换来的。
2. 门槛全套（gofmt / vet ×3 / test 全量 / PASS 计数 / build ×3）。
3. 视觉证据：修前/修后两张图**逐像素相同**（`/tmp/floter-P11-bm-rest-{before,after}-{dark,light}.png`
   + 一张揭示态的图证明点还在呼吸），并写「我实测看到了什么」。

## mutation 计划（≥2 条，逐条实跑）

- **M1**：把 `if !drawn { return }` 删掉（回到无条件读循环）→ `TestTheAtRestStatusDotAsksForNoFrames`
  第一臂变红（`FramePending()` 为 true）。
- **M2**：把 `drawn` 恒传 `true`（调用点写死）→ 同一条 pin 第一臂变红，红在同一句。
- **M3**（可选）：把 `exited` 早退挪到 `drawn` 判断之后 —— 不该红（守护既有行为边界，
  证明本轮没有动退出态）。

每条改完 `md5sum -c` 逐字节还原、`touch` 后复跑。
