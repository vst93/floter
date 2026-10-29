# P11-bq 报告 · 到点的句子要一次「到点唤醒」，不是每帧重建整棵视图

## 1. 为什么是这一件（选题依据）

上一轮（P11-bp）报告「仍待」第 3 条原话：剩下的 `c.AnimationFrame()` 调用点**未逐条核对**，
「下一轮先量『谁在一直要帧』再动手」。用户实测反馈 F1–F4 已收完、合约 §6 全 `已落地`
（只剩拍板项），所以本轮接这一条：先把九个调用点量一遍，再按量到的结果动手。

## 2. 量出来的分类（修前，`grep -rn "c\.AnimationFrame()" internal/ | grep -v _test`）

九个点里**只有两个真的在动**：

| 位置 | 是什么 | 期间有运动 | 处置 |
|---|---|---|---|
| `internal/shell/shell.go:4305` | 面板入场 `entrance < 1` | 有（0→1 的 ease） | 不动 |
| `internal/shell/notices.go:396` | toast 入场 `progress < 1` | 有（spring） | 不动 |
| `internal/launcher/fieldkeys.go:281` | 剪贴板删除的确认窗口 | 没有 | `deadline.Wake` |
| `internal/launcher/power.go:103` | 关机/重启问句窗口 | 没有 | `deadline.Wake` |
| `internal/launcher/launcher.go:3053` | 底部 toast 的 4.5s | 没有 | `deadline.Wake` |
| `internal/settingsui/debounce.go:105` | 设置写盘防抖窗口 | 没有 | `deadline.Wake` |
| `internal/settingsui/savealert.go:55` | 写失败便条的 5s | 入场由 `AnimateTo` 驱动 | `deadline.Wake` |
| `internal/settingsui/settingsui.go:6067` | 三个内联确认窗口 | 没有 | `deadline.Wake`（取最早） |
| `internal/settingsui/settingsui.go:6124` | 会话关闭问句窗口 | 没有 | `deadline.Wake` |
| `internal/terminalui/terminalui.go:1265` | 终端失败提示的 4.5s | 没有（P11-bg 已删入场） | `deadline.Wake` |

**根因不是「忘了省」，是用错了工具**：`Context.AnimationFrame` 是「下一帧**重建整棵视图**」
（mygo v0.7.0 `ui/runtime.go:744-748`），这些线要的是「到点醒一次」。而本仓早就有正确答案：
设置页的「已复制」确认用 `c.After(until.Sub(now))`（`settingsui.go:3917`、`:5086`）。
同一个问题两个写法，本轮收成一个。

## 3. 落地

### 3.1 `internal/deadline`（新包，只有 `Wake`）

```go
func Wake(c *ui.Context, wait time.Duration) {
	if c == nil || wait <= 0 { return }
	c.After(wait + wakeSettle)   // wakeSettle = 1ms
}
```

`+1ms` 不是容差，是**正确性**：七处判据都是严格大于（`now.Sub(at) > window`、
`now.After(until)`）。在**恰好等于** deadline 的那一帧醒，判据仍为假 → 再排一个
`wait == 0` → `Wake` 什么都不做 → 这条线**永远不再醒**。M3 实跑证明（-1ms 时 terminalui 与
launcher 两条 pin 立刻变红：「the note/toast outlived its window」）。生产里定时器只会晚到，
测试器（`ui.Tester.Advance`）按精确算术推进，这一毫秒是必须的。

### 3.2 每处用各自那一支时钟算剩余时长

launcher / settingsui 的 `a.now()` 是可注入的（测试手动推进），`c.Now()` 是另一个时钟域；
混用会把剩余时长算错。所以 `Wake` 收 `time.Duration`，每处自己减。

### 3.3 一处被这次改动照出来的真问题（值得单记）

`settingsui` 的写失败便条**入场动画是被旧的每帧重建带着走的**：`banner.AnimateTo(key, 0, …)`
在第一帧「定住起点 0」，而 `AnimateTo` 在 `made` 那条路上不请帧（`ui/motion.go:305-320`），
所以**第二帧**才是运动的第一帧 —— 以前是 `expireSaveFailed` 的每帧重建免费提供了它。
换成到点唤醒后，标尺立刻红：「the entrance did not settle: progress 0」（便条以 0 不透明度
停住 = 看不见）。修法：第一帧那次 `AnimateTo(key, 0, …)` 之后**自己请一帧**
（`savealert.go`，注释写明「a value starts at its first target, so the motion begins on the
frame after this one」）；运动本身的帧仍是 `AnimateTo` 自己请（`ui/motion.go:340`），
落位后的五秒才归 deadline。

### 3.4 「busy 里的确认」不排醒

`expireConfirmations` 里答案在飞的 band（`disableBusy`/`removeBusy`）即使过了窗口也留着，
它的 deadline 已经过去 → `Wake` 不排（`wait <= 0`）。它由 shell 回报
（`finishDisableConfirmation` → `Actions.Invalidate`）唤醒；以前那种「每帧自转」是白烧。

## 4. V / B / S 台账

- **V = 0**：没有一个像素数值需要重定基线。窗口长度、判据、墨、几何、文案、i18n 全没动。
- **B = 0**：既有测试**零改动**全绿。到点该发生的事一件没少（见 §6 的 pin 与 §5 的图）。
- **S = 0**：没有测试因为结构变化而改断言。`expireConfirmations` 内部把 `live bool` 换成
  「最早的到点时刻」（`next time.Time` + `consider`），是私有实现。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
GOOS=windows go vet ./...                          → 净
GOOS=darwin  go vet ./...                          → 净
go test -count=1 ./...                             → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2460（基线 2457，+3 pin）
... | grep -c -- "--- SKIP"                        → 54（基线 52，+2 为本轮 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2014（基线 2009，+5）
GOOS=linux / windows / darwin go build ./...       → 三份过
```

## 6. pin（三条，修前逐条红，报错串照抄）

1. `internal/launcher/deadlinewake_test.go` · `TestTheTimedToastWakesAtItsDeadline`
   - 修前：`the toast built the view 60 times in a second: the frames are rebuilds of a line
     that does not change`
   - 修后：绿。判据三条：**同样一秒里 toast 一次都不额外构建**（基线取同页未聚焦时的一秒，
     实测 0 —— 光标闪烁只在窗口有焦点时跑，这是量出来的，不是假设）、窗口内句子仍在、
     到点后句子自己消失。
2. `internal/terminalui/deadlinewake_test.go` · `TestTheTimedFeedbackWakesAtItsDeadline`
   - 修前：`the note built the terminal view 60 times in a second: …`
   - 修后：绿（一秒 0 次构建，4.5s 后 `FeedbackVisible()` 为假，且到点那一秒只多一次构建）。
3. `internal/settingsui/deadlinewake_test.go` · `TestTheSaveFailureBannerWakesAtItsDeadline`
   - 修前：`the banner built the settings view 60 times in a second: …`
   - 修后：绿（入场先落位，判据从落位后开始数；一秒 0 次构建，到点那一秒恰好一次，
     5s 后 `SaveFailedVisible()` 为假）。

## 7. mutation（3 条实跑，注入 → 红 → 还原 → `md5sum -c` → `touch` 复跑绿）

| # | 注入 | 实测红 |
|---|---|---|
| M1 | `launcher.go` 的 `deadline.Wake(c, a.toastUntil.Sub(now))` → `deadline.Wake(c, time.Millisecond)`（等价于每帧都要，即旧行为） | `the toast built the view 500 times in a second, 500 more than the page's own 0` |
| M2 | `terminalui.go:1265` 同款注入 | `the note built the terminal view 500 times in a second: …` |
| M3 | 去掉 `deadline.Wake` 里的 `wakeSettle`（在 deadline 恰好那刻醒） | `the note outlived its window` / `the toast outlived its window: ["No results" … "Saved to the clipboard"]`；**settingsui 那条仍绿**——它的页面在窗口里还有别的帧活动，恰好相等那一下被别的帧越过了，所以这条判据只在两个纯静页上对边界敏感。照实记，不改判据。 |

三条都 `md5sum -c` 逐字节还原，`touch` 后复跑绿。

## 8. 视觉证据（墨没有动）

- `/tmp/floter-P11-bq-note-{light,dark}-{before,after}.png`（900×620，终端页的失败提示带）：
  `cmp` **逐字节相同**。我在这张图上看到的：标题条下沿 y=27…55 是一条整宽的
  `(242,242,242)` 带子（`Control` 面），x=60 / y=36…42 是那枚警示字形的红 `(173,72,67)`
  —— 修前修后这四个像素值一个没变。
- `/tmp/floter-P11-bq-toast-{light,dark}-{before,after}.png`（720×488，启动器的底部 toast 带）：
  `cmp` **逐字节相同**。**诚实说明**：这张 488 高的视图里 toast 带不在可见区内（唯一非面板色
  的像素是 y=55 那条字段带的下沿发丝），所以它只证明「帧没变」，不证明「看到了那条带子」；
  真看图请看上面 terminalui 那对。

## 9. 仍待（P11-bq）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行与参考的差异）。
2. 设置页 `actionSpinner` 仍在构建期转（`internal/settingsui/rows.go:556-566`）：要动它得先有
   「绘制期读角度的图标」（`Painter.Icon` 带 rotate，或让 `kindIcon` 的 op 进 part），
   或把 LoaderCircle 画成矢量弧。**别照搬 P11-bp 的手法**，实测无效（P11-bp §2.1）。
3. shell 两处入场（`shell.go:4305`、`notices.go:396`）是**真运动**，不动；如果以后要省，
   该走的是 `Painter` 的绘制期动画，不是把入场删掉。
4. `deadline.Wake` 现在只服务七处纯时钟；**新增「到点才变」的线时用它，不要再用
   `c.AnimationFrame`**（本轮把这条写进了包注释）。
