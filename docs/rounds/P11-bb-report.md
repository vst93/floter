# P11-bb 报告 · 编号家族只落在「读得全的那一段」+ 按住修饰键时底部常驻终端提示

选题顺序：用户实测反馈（`.pending-bugs.md` 顶部，2026-10-28）→ 合约未落地条目（§6 全表
`已落地`）→ 上一轮「仍待」→ 界面质感与交互。本轮取**用户新报的那一条**（最优先）：
搜索列表的 `Super+1…0` 徽章在滚动时的渲染。规格 `docs/rounds/P11-bb-spec.md`。

## 1. 根因（两处，都不是「徽章画错了」）

### 1.1 判可见的尺子只答「一个像素」

`assignNumbers`（`internal/launcher/launcher.go`）用 `a.List.Visible()` 当可见区间，
而框架的定义是**任何**一个像素还在框里（`mygo/ui/list.go` 的
`y+ht(i) > top && y < bottom`）。于是一行被裁掉九成也照领 `1`；它的徽章在行中线，
早被裁掉——用户看到的第一枚徽章是 `2`，按 `Super+1` 跑的却是屏幕外那一行。

### 1.2 列表的视口盒子**含**搜索框盖住的那条带（真正让用户看见的那一半）

搜索框是 `Absolute()` 的浮层（`launcher.go` 的 `fieldRow`），列表的顶内边距是
「字段行 + 呼吸」（`listEdge = fieldRow + Space(breathUnits)`），所以行是**从字段底下
滚过去**的。列表自己的盒子从面板顶边算起（实测 `{X:0 Y:0 W:720 H:443}`），于是
「滚到字段底下」的行在几何上仍然「在视口里」，而它整行都在不透明的字段带后面。
实测（`/tmp/floter-P11-bb-numbers-light.png`，720×488，30 个结果滚半步）：
行 0 的盒子 `{X:8 Y:36 W:704 H:40}`，字段带 0…60，徽章列的第一块键帽在 **y=86**，
即第一行**读得全**的行。修前行 0 拿 `1`。

参考实现读两件事，本仓移植时两件都丢了：`visibleRowRange`
（`main:src/launcher/result-budget.ts:768-810`）① 跳过被**上**边缘裁掉的行；
② 底边只露一部分的行仍算（`end` 是「最后一个有任何一部分在框里的行」）。

## 2. 落地

1. **几何来源**：`rowBody` 建行时把 `ui.Row(...)` 的 `Element.Bounds()` 写进
   `App.rowRects[i]`（框架给的是**上一帧**的矩形，`mygo/ui/api_gen.go` 的 `Bounds`
   注释如此）；`View` 里 `ui.List(...)` 的 `Bounds()` 写进 `App.listRect`，
   **并把顶边抬到 `listEdge`**（`rect.Y += listEdge; rect.H -= listEdge`）——那才是
   用户读得到的那条带。`rowRects` 在列表开始建之前 `clear`，离开列表的行不留过期盒子。
2. **判据**：`numberedViewport` 用「行盒 ∩ 视口盒」的面积比。`first` = 第一行
   ≥ `numberVisibleShare`（0.7，用户的 30% 阈值）；`last` = 最后一个 share > 0 的行
   （底边只露一部分的行仍领号，`0` 落在它上面，与参考一致）。
3. **首帧回落**：还没有几何（第一遍、`Bounds()` 还是空盒）时回落到旧的
   `List.Visible()`，列表打开时不会一个徽章都没有。
4. **带子的第三个句子**：`heldHint()`（`modifierHeld && output == nil && !pluginModeActive()`）
   接进 `statusBand` / `Geometry.Feedback` / `FeedbackVisible`，`feedbackRow` 在
   `pendingPower`、`toast` 都不在场时画 `heldHintRow`：`Super+Enter` 两块键帽 +
   `HeldTerminalHint`（zh「快捷进入终端」/ en「Open a terminal session」）。
   优先级 `问句 > toast > 按住提示`（P11-az 的「一条带一个句子」不变）。
5. **i18n**：`i18n.Launcher.HeldTerminalHint`（en/zh 各一句）。

## 3. 台账（V/B/S）

- **V = 1**：`TestResultShortcutsRunTheNumberedRow`（`view_test.go`）——它不是数值 pin，
  是**家族跟着视口走**之后按键帧与读帧不再同一张表：按下数字会把选中行滚进视野，
  那一滚是动画，滚的每一帧编号挪一行。改法：按 `0` 之前 `Advance(500ms)` + 两帧等它停稳，
  再读「带 0 的那一行」；`0` 那一臂从 `a.Results()[last]` 改成「带 0 的行」。
  断言没有放宽：仍是「这个数字跑的就是带这个数字的那一行」。
- **B = 0 / S = 0**：高度链、键盘模型、会话、设置写回、命令接线零改动。

## 4. 变异（3 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 红在哪（原始报错串） |
|---|------|----------------------|
| M1 | `numberVisibleShare = 0.7` → `0.0` | `row 0 carries 1 while only 0.40 of it is on screen` / `row 0 is numbered 1 while only 0.40 of it is on screen` / `the mostly clipped row 0 kept a number (map[0:1 1:2 … 9:0])` |
| M2 | `heldHint()` 恒 `false` | `the band is not charged while the modifier is held` / `the band does not say "Open a terminal session"` / `holding the modifier grew the window by 0, want the 30u status band` |
| M3 | 视口盒去掉 `listEdge` 顶抬 | `the family's viewport starts at y=0, want the field's band 60` |

每条都 `cp` 还原后 `md5sum -c /tmp/p11bb.md5` 逐字节核对，`touch` 测试文件再跑一次绿。

## 5. 视觉证据（我自己复核了像素，不是只看测试数字）

`/tmp/floter-P11-bb-numbers-{light,dark}.png`、`/tmp/floter-P11-bb-heldhint-{light,dark}.png`
（`FLOTER_PNG=1 go test -run TestNumberBandDump ./internal/launcher/`）。

**我在图上量到的**：

- `numbers-light`：徽章列的墨迹从 **y=86** 起（40u 一行：86…425，共 10 块），字段带
  0…60 里一块都没有——滚到字段底下的那一行不再领号。编号 `map[1:1 … 9:9 10:0]`，
  即 `1` 落在第一行读得全的行上，`0` 落在底边只露一半的第十行上。
- `heldhint-light`：底部 y≈463…482 一条，x=16…67 是两块键帽（`Super` / `Enter`），
  其后是句子 `Open a terminal session`；带子之上没有任何面板/投影（只有字与键帽）。
  深色两张同位置、同一句话（`23,23,25` 底）。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2429（基线 2427）| SKIP 38（37 + 1 条 opt-in dump）| 测试函数 1969（1966 + 3）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 仍待（P11-bb）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的终端便条投影 / P11-ba 的终端「进程已退出」便条改 inline。
2. **新侦察结论（本轮量过，别当 TODO 重领）**：编号家族读的是**上一帧**的几何
   （`Bounds()` 的语义），所以快速滚动时编号比行慢一帧；30% 阈值就是为这个滞后留的余量。
   改成同帧需要框架暴露行矩形，属框架改动，不做。
3. **新侦察结论**：`frameHeight(maxResultSlots)` 这间「十行的房」实际只放得下 9 行半
   （字段带 60 + 9.5×40 = 443 = 列表底边），所以 `0` 只出现在「底边只露一半的第十行」上；
   这是窗口高度链（红线）的既有形状，不是本轮引入的。
