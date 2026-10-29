# P11-cm 规格：底部那条带的「有没有」只许有一个说法

## 意图

**一句话**：`View` 用来给窗口量高的判据、`statusBand` 用来给列表让位的判据、以及
`FeedbackVisible()` 交给 shell 量窗口的判据，三处写的是同一个事实的三份拷贝，其中
**shell 读的那一份少了 `pendingPower`** —— 于是电源确认在场时，shell 量的窗口比
画面自己量的矮 30u。

为什么现在做：用户定的优先级是「界面质感与交互（含用户报的 bug）→ 基础功能 → …」。
用户实测反馈 F1–F4 已收尾，合约 §6 全 `已落地`，上一轮「仍待」第 1 条整张是拍板项。
本轮在交互里挑一处**实测能看见的**不一致：同一个判据三个写法，其中一个已经漂了。

## 证据（本轮实测，HEAD = f090500f）

三处判据，逐字抄下来：

| 位置 | 写法 | 含 `pendingPower` |
|---|---|---|
| `internal/launcher/launcher.go:1254`（`View` → `Geometry.Feedback`） | `a.toast != "" \|\| a.pendingPower != "" \|\| a.heldHint()` | 是 |
| `internal/launcher/launcher.go:3164`（`statusBand`，列表让位） | `a.toast == "" && a.pendingPower == "" && !a.heldHint()` 取反 | 是 |
| `internal/launcher/launcher.go:3187`（`FeedbackVisible()`，**shell 量窗口读它**） | `a.toast != "" \|\| a.heldHint()` | **不是** |

`shell.go:4744` 的 `launcherHeight` 把它塞进 `Geometry.Feedback`，`targetSize` 读它，
`resize()`（进表面 / 设置写回）用它定窗口；`walkLauncherHeight` 的稳态判据是
`a.lastH == a.launcherTarget`，而 `launcherTarget` 由 `View` 的 `ResizeTo` 写入（含
`pendingPower`）——两份数一矮一高，走与不走全看哪一份先落地。

实测（`720×488` 帧、五条最近启动、`restart` 查询）：

- `internal/launcher` 单测：`pendingPower="restart"` 时 shell 量 **532**、画面量 **562**
  （差正好是 30u 的 `launcherStatusUnits`）。
- 在 shell 量的 532 上画那一帧：确认带 `y=506..528`，而最低一行 `Recent 09`
  是 `y=506..530` —— **整行压在带子底下**，看不见也点不到（P11-az 修过的那个症状，
  换了一条路径回来）。
- 在画面量的 562 上画：带子 `y=536..558`，最低行 `506..530`，干净。

`View` 那条注释（`launcher.go:3156`）自己写着「It is the *same* predicate as
`Geometry.Feedback`」，`FeedbackVisible` 的注释写着「for the shell's window sizing」——
三处都声称是同一个，实际不是。这是本轮要收口的那个事实。

## 落地项

1. **`internal/launcher/launcher.go`：新增一个未导出的 `feedbackVisible()`**
   - 现状：判据三份手写拷贝（上表三行）。
   - 应改成：`func (a *App) feedbackVisible() bool { return a.toast != "" || a.pendingPower != "" || a.heldHint() }`
     —— 就是 `statusBand` 今天取反的那一条，原样抽出来。
2. **`View` 的 `Geometry.Feedback`（`:1254`）** → `a.feedbackVisible()`。
3. **`statusBand`（`:3164`）** → `if !a.feedbackVisible() { return 0 }`。
4. **`FeedbackVisible()`（`:3187`）** → `return a.feedbackVisible()`，并改注释：它现在
   真的是同一个判据，shell 读它与画面量的是同一个数。
5. **`power.go:220` 的 `cancelPower` 与 `launcher.go:1687` 的 Escape 分支**：本轮**不动**。
   两者都只清 `pendingPower`，语义一致（`cancelPower` 唯一的生产调用点是
   `statusband_test.go`，另一支是键路径）——这是记账，不是缺陷，写进报告「故意不动」。

**零改动红线（逐条核对，一个字节不动）**：高度链（`height.go` 的 `feedbackBand` = 7.5×spacing）、
带子的几何（`Absolute().Bottom(a.footerBand)` + `Height(t.Space(7.5))`）、墨与面
（`tokens.IconSurface` / `tokens.RaisedWarm` / `Warning` 环）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线。本轮改的是**谁回答「有没有」**，不是那个回答是什么。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（HEAD `f090500f` 实测，不是旧报告的数字）：**PASS 2764 / FAIL 0 / SKIP 55 /
2053 测试函数**。目标：PASS ≥ 2765，函数 ≥ 2054，FAIL 0，SKIP 55 不变。

## pin

1. `internal/launcher/feedbackpredicate_test.go` 的
   `TestTheBandHasOnePredicate`：三个来源（toast / 电源确认 / 按住修饰键）逐个置上，
   每次断言 `a.FeedbackVisible() == (Geometry{...Feedback: a.FeedbackVisible()}.Height() >
   Geometry{...Feedback: false}.Height())` **且** `statusBand(c) > 0` **且** `View` 报给
   `ResizeTo` 的高度差正好是 30u。三种来源、三条读数，一个答案。
2. `TestTheChargedHeightLeavesNoRowUnderTheBand`：电源确认在场时，在
   `FeedbackVisible()` 定的高度上画那一帧，逐行断言 `r.Y+r.H <= band.Y`。
   **这条是「改前红、改后绿」的那一条**：把 `feedbackVisible()` 退回旧写法（去掉
   `pendingPower`）它立刻红，报 `"Recent 09" ends at 530, under the band's 506`。

## mutation 计划（≥2 条，实跑）

- **M1**：`feedbackVisible()` 退回旧写法（`a.toast != "" || a.heldHint()`，丢掉
  `pendingPower`）。预期：`TestTheChargedHeightLeavesNoRowUnderTheBand` 红（行压带子），
  `TestTheBandHasOnePredicate` 的电源那一臂红（shell 量 532、画面量 562）。
- **M2**：`statusBand` 改回手写判据但**漏掉 `pendingPower`**（`a.toast == "" && !a.heldHint()`）。
  预期：`TestTheBandHasOnePredicate` 的电源臂红（带子不让位），而 1 的「同一个判据」
  也红——证明这条 pin 读的是两个来源的一致性，不只是某一个数。
- **M3**（负对照，预期绿）：把 `feedbackVisible` 的三项换序。预期全绿——pin 钉的是
  「答案一致」，不是求值顺序。
