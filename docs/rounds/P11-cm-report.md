# P11-cm 报告：底部那条带的「有没有」只许有一个说法

规格 `docs/rounds/P11-cm-spec.md`。轮号 P11-cm。方向按用户定的优先级
「界面质感与交互（含用户报的 bug）→ 基础功能 → …」自选：F1–F4 已收尾、合约 §6 全
`已落地`、上一轮「仍待」第 1 条整张是拍板项，于是本轮在交互里挑一处**实测能看见**的
不一致。

## §1 病灶：一个判据三份拷贝，其中一份漂了

三处都声称是「同一个判据」，逐字抄下来（HEAD `f090500f`）：

| 位置 | 写法 | 含 `pendingPower` |
|---|---|---|
| `launcher.go:1254`（`View` → `Geometry.Feedback`） | `a.toast != "" \|\| a.pendingPower != "" \|\| a.heldHint()` | 是 |
| `launcher.go:3164`（`statusBand`，列表让位） | `a.toast == "" && a.pendingPower == "" && !a.heldHint()` 取反 | 是 |
| `launcher.go:3187`（`FeedbackVisible()`，**shell 量窗口读它**） | `a.toast != "" \|\| a.heldHint()` | **不是** |

`statusBand` 的注释写着「It is the *same* predicate as `Geometry.Feedback`」，
`FeedbackVisible` 的注释写着「for the shell's window sizing」——两句话都在说「同一个」，
而第三份确实不是。这就是「三份手写拷贝」的代价：改的时候漏了一份，而且漏的那一份
正是**没有测试直接读**的那一份（`FeedbackVisible` 只在 shell 与 pin 里被间接用到）。

### 用户看见的是什么

实测（`720×488` 帧、十二条最近启动、`restart` 查询，`runPower(PowerRestart)`）：

```
pending="restart"   shell 量 532   画面量 562   （差正好是 30u 的 launcherStatusUnits）
窗口 532：带子 y=506..528，最低一行 "Recent 09" 在 y=506..530  →  整行压在带子底下
窗口 562：带子 y=536..558，最低一行 506..530                    →  干净
```

这正是 P11-az 修过的那个症状（「列表最后一行永远在带子底下，看不见、点不到」），
换了一条路径回来：P11-az 让**列表**给带子让位（`statusBand`，它是对的），
但**窗口**没长那 30u（`FeedbackVisible`，它漏了 `pendingPower`）。于是列表让出的位置
被带子占了，可列表的最后一行仍然在窗口最底下 —— 窗口比它自己画的东西矮一个带子。

shell 的两条路径读它：`launcherHeight`/`targetSize`（进表面、设置写回）与
`resize()` 都问 `FeedbackVisible()`；而 `walkLauncherHeight` 的稳态判据是
`lastH == launcherTarget`，`launcherTarget` 由 `View` 的 `ResizeTo` 写入（含
`pendingPower`）。两份数一矮一高，**走不走全看哪一份先落地** —— 也就是「有时对、
有时错」，最坏的一种。

## §2 改法：把判据收成一个函数

`launcher.go`：

1. 新增 `feedbackVisible()`（`:3211`）：`a.toast != "" || a.pendingPower != "" || a.heldHint()`
   —— 就是 `statusBand` 今天取反的那一条，原样抽出来。
2. `View` 的 `Geometry.Feedback`（`:1254`）→ `a.feedbackVisible()`。
3. `statusBand`（`:3171`）→ `if !a.feedbackVisible() { return 0 }`。
4. `FeedbackVisible()`（`:3197`）→ `return a.feedbackVisible()`，注释改成它现在真的
   是同一个判据。

**产品代码净改动 = 一个 3 行的函数 + 三处读它。** 高度链（`height.go` 的
`feedbackBand` = 7.5 × spacing）、带子的几何（`Absolute().Bottom(a.footerBand)` +
`Height(t.Space(7.5))`）、墨与面（`IconSurface` / `RaisedWarm` / `Warning` 环）、
键盘模型、会话管理、设置写回、i18n 文案、命令接线 —— 一个字节没动。

### 故意不动

- `power.go:220` 的 `cancelPower` 与 `launcher.go:1687` 的 Escape 分支都只清
  `pendingPower`，语义一致（`cancelPower` 唯一的生产调用点是 `statusband_test.go`，
  另一支是键路径）。这是记账，不是缺陷。
- `feedbackRow` 的三句优先级（问句 > toast > 按住修饰键的提示）不变：本轮只统一
  「带子有没有」，不碰「带子上说哪一句」。

## §3 台账 V / B / S

- **V = 0**：没有任何视觉数值被重定基线。
- **B = 0**：既有行为/度量/接线 pin **零改动全绿**。
- **S = 0**：没有结构变更（判据抽成函数，树、几何、行序不变）。
- 新增：`internal/launcher/feedbackpredicate_test.go`（2 个函数 + 4 个子测试 = 6 PASS）
  与 `internal/launcher/bandheight_dump_test.go`（1 个 opt-in dump，SKIP 计数不变）。

## §4 门槛（实测，HEAD 上跑）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2770
go test -count=1 -v ./... | grep -c -- "--- FAIL"      → 0
go test -count=1 -v ./... | grep -c -- "--- SKIP"      → 55
grep -rn "func Test" --include=*_test.go internal/ | wc -l → 2055
GOOS=linux/windows/darwin go build ./...               → 三个都过
```

基线（我在 HEAD `f090500f` 上重跑的实测值，不是旧报告的数字）：**PASS 2764 /
FAIL 0 / SKIP 55 / 2053 函数**。本轮 **2764 → 2770**（+6），函数 **2053 → 2055**（+2），
SKIP 55 不变（新 dump 是 opt-in）。

## §5 mutation：3 条实跑，2 红 1 绿

每条都先 `cp` 备份，跑完 `md5sum -c` 逐字节核对还原，再 `touch` 复跑一次（避开测试缓存）。
`launcher.go` 的还原基线：`857a431cdd5ae057389261fa40ad09cf`。

### M1（注入缺陷：`View` 的判据退回旧写法，丢掉 `pendingPower`）

`Feedback: a.feedbackVisible(),` → `Feedback: a.toast != "" || a.heldHint(),`

**红 2 处**，原串抄下来：

```
feedbackpredicate_test.go:83: the frame asked for 486, a band over the base 486 is 0 (want the 30u band)
    --- FAIL: TestTheBandHasOnePredicate/a_power_confirmation
feedbackpredicate_test.go:144: "Recent 09" ends at 530, under the band's 506 (window 532):
    the row is drawn and cannot be clicked
--- FAIL: TestTheChargedHeightLeavesNoRowUnderTheBand
```

两条 pin 各自独立报红：一条报「窗口没长那 30u」，一条报「最低一行压在带子底下」。
第二条是**用户看得见**的那一半，数字与 §1 的实测一致（530 / 506 / 532）。

### M2（注入缺陷：`statusBand` 改回手写判据但漏掉 `pendingPower`）

`if !a.feedbackVisible() { return 0 }` → `if a.toast == "" && !a.heldHint() { return 0 }`

**红 1 处**：

```
feedbackpredicate_test.go:74: the list reserves the band = false, want true
    --- FAIL: TestTheBandHasOnePredicate/a_power_confirmation
```

而 `TestTheChargedHeightLeavesNoRowUnderTheBand` 在这一条 mutation 下**仍然绿** ——
窗口高度是对的、列表让位是对的、行确实停在带子上沿；坏的是带子会**盖住**行。
这说明两条 pin 读的是两个不同的半边（窗口的账 / 列表的让位），一个缺陷只能打中一个。
这正是为什么要两条，而不是一条「总数相等」。

### M3（负对照，预期绿）

`feedbackVisible` 的三项换序：`a.heldHint() || a.pendingPower != "" || a.toast != ""`

**绿**（`ok floter/internal/launcher`）。pin 钉的是「答案一致」，不是求值顺序 ——
这一条是故意留着证明 pin 没有顺手钉住一个不该钉的实现细节。

三条全部 `md5sum -c` 逐字节还原成功，`touch` 后复跑绿。

## §6 视觉证据（4 张图，我自己看过）

`/tmp/floter-P11-cm-band-{light,dark}-{before,after}.png`（`FLOTER_PNG=1
go test -run TestDumpP11CMBandPNGs ./internal/launcher/`）。

dump 的日志（同一次运行）：

```
light before: window 532, band y=506..528, lowest row "Recent 09" ends at 530 (UNDER the band)
light after:  window 562, band y=536..558, lowest row "Recent 09" ends at 530 (above the band)
dark  before: window 532, band y=506..528, lowest row "Recent 09" ends at 530 (UNDER the band)
dark  after:  window 562, band y=536..558, lowest row "Recent 09" ends at 530 (above the band)
```

**我在图上看到的那一处变化**（逐像素读出来的，不是只看日志）：在
`floter-P11-cm-band-light-before.png` 里，`x=60` 这一列从 `y=502` 起就是琥珀带的面
`rgb(248,235,227)`，而 `Recent 09` 的标题墨在 `y=513`（`rgb(121,108,100)`）——
**字画在琥珀面上**。同一列在 `-after.png` 里 `y=502..521` 仍是面板的
`rgb(253,253,254)`，`y=513` 的墨是 `rgb(107,107,108)` 落在**面板面**上，琥珀带退到
`y=532` 才开始。深色两张同构：`before` 的 `y=502` 是 `rgb(49,40,37)`（暖面），
`after` 的 `y=502` 是 `rgb(23,23,25)`（面板面）。浅深两套都成立。

## §7 仍待（P11-cm）

1. **老拍板项原样有效**（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **`Geometry.Feedback` 是 `bool`，不是计数**：带子只有一条，三个句子互相让位
   （`feedbackRow` 的优先级）。这一条 P11-az 已经写明是红线（改成计数要动高度链），
   本轮只是记账，**别拿它当 TODO 重领**。
3. **同一族的下一个可疑处**（本轮没动、也没实测）：`launcher.FeedbackVisible()` 现在是
   `feedbackVisible` 的别名，而 `terminalui` 有一个**同名不同义**的 `FeedbackVisible()`
   （`terminalui.go:653`，那是终端自己的通知条）。两个表面各自的带子，别当重复删掉一个。
4. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
