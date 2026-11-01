# P13-cf 报告：启动器结果列表的满态收口——先量，量下来它已经读同一条规则（但空气只有 4px）

规格 `docs/rounds/P13-cf-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.49**｜
起点 `5f6ceb73`（P13-ce）

## 这轮为什么让它更轻、更有空间感

P13-ce 把「页头下面的呼吸属于这一页，不属于内容」立成判据之后，只剩一个问号：启动器的
**结果列表**读不读它。量完的答案是**读**（P11-bj 起就把空气放在流里），但读出来的数很小：
字段块的发丝线在 **y=55**，内容裁剪在 **y=60**——**4px**。同一张设计语言里，捕获输出页
的同一段空气是 **16px**（§6.48）。**同一条规则、两个数**，这才是这一页真正的问题，
而不是「内容把空气吃掉了」（那件事在这一页结构上不可能发生：裁剪边就是空气的下沿）。

4 这个数在高度链里（`breathUnits`），抬它会让窗口长 12px —— 红线，所以这一轮**只量不改**，
把它作为需拍板项交出去，同时把三条判据钉成 pin。

## 第一次量错了（记在这里，因为它值一条判据）

按 P13-ce 的办法量「字段带最后一行墨」，得到 **19px**（y=40 → 60），和输出页的数一模一样，
差点就写成「两张卡同值，收工」。错在 `contrastIn` 的判据：它把「框内最偏离面色的像素」
当墨，而字段块的最后一笔是一条**整行同色**的发丝线（`launcher.go` 的
`ui.Box(...).Top(fieldRow-1).Height(1)` + `HairlineSoft`）——整行都是 239，它自己就是
那一行的「面色」，偏差算出 **0**。改读卡片左侧 gutter（x=5，除了块自己的填色就只有那条线）
才看见它：**y=55**。于是空气从 19px 修正为 **4px**。

**判据：量一条线要读 gutter 剖面，不要读整块对比度。**

## 改了什么

| 文件 | 改动 |
|---|---|
| `internal/launcher/resultsedge_test.go` | 新 pin 两条 + 一条 opt-in 出图（带内零墨 + 归属 / 两端折边按比例） |
| `docs/liquid-glass.md` | 新增 §6.49（含「收口缺陷的形状是页头的墨在裁剪内」与「contrastIn 看不见整行同色的线」两条反直觉） |

**产品代码零改动**：`launcher.go` / `height.go` / `rowBody` / `foldAlpha` 一个字节不改；
高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部不动。

## 实测（720×488 帧，30 个应用、查询 `app`）

| | 数 |
|---|---|
| 字段块发丝线（块的最后 1px） | **y=55**（`fieldRow-1`，深浅同值） |
| 列表裁剪盒 / 首行盒 | `{0,60,720,383}` / `Y=60`——**重合**（列表没有上 padding） |
| 发丝线 → 裁剪上沿 | **4px**（56..59），滚 0/10/20 时带内墨 **0/0/0**，深浅同值 |
| 顶边折边（露出 0.50） | **117/226**（理论 113）；深 **107/214**（理论 107） |
| 下沿折边（露出 0.57） | **130/226**（理论 130）；深 **123/214**（理论 123） |
| 捕获输出页的同一段空气 | **16px**（`output.go` 的 `header := t.Space(4)`） |

图上看到的一处变化（`/tmp/floter-P13-cf-results-{light,dark}-{rest,scrolled}.png`）：
滚 20px 之后，被折边切一半的那一行**墨只剩一半**（226 → 117），而它的**版面没动**——
它在 y=60 处被裁断，断口正好是那条 4px 空气的下沿；两张图的差分包围盒
`8,60..711,442`（深 `46653 px`、浅 `58015 px`），**41..59 那条带一个像素没变**，
即空气不跟着滚。

## 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 3243
go test -count=1 -v ./... | grep -c -- "--- SKIP"  → 127
GOOS=linux/windows/darwin go build ./...  → 三平台过
```

基线 `5f6ceb73`：PASS **3241** / FAIL 0 / SKIP **126** / 测试函数 **2397**。
本轮：PASS **3243**（+2）/ FAIL **0** / SKIP **127**（+1，opt-in 出图）/ 测试函数 **2400**（+3）。

## V / B / S 台账

**V 0 / B 0 / S 0**——旧 pin 零改动，新增两条 pin 全部是「新账」，不是重定基线。

## mutation（3 条实跑，全部变红并逐字节还原）

- **M1 · 把那 4px 挪进列表的 padding**（复现 P13-ce 的缺陷类）：`Height(listEdge)` →
  `Height(fieldRow)` + 列表 `Padding(t.Space(breathUnits), …)`。红两条臂：
  `0px of air between the field's hairline (y=55) and the clip (y=56), want the height chain's 4px (breathUnits)`
  与 `the first row starts at y=60, the clip at y=56 — the list carries a top padding of its own`。
  （**带内零墨那条臂没红，且它不该红**：裁剪边就是带的下沿，行进不了那条带——这条写进 §6.49 了。）
- **M2 · 折边在顶边失效**：`rowBody` 里把传给 `foldAlpha` 的视口上沿抬到 0。红：
  `dark=false: the row at the top fold shows 0.50 of its box but draws ink 226 of its whole 226 (want 113) — it is cut, not receded`
  （下沿那条臂仍绿，说明两条臂量的是两端、不是同一处）。
- **M3 · 折边比例只对一端成立**：`foldAlpha` 的 `visible >= box.H` → `visible >= box.H*0.5`。
  两端同时红：顶 `226 of its whole 226 (want 113)`、下 `226 of its whole 226 (want 130)`。

每条：注入 → 抄下报错串 → `git checkout` → `md5sum -c` 逐字节核对（成功）→ `touch` 复跑绿。

## 偷了什么（qiaomu-design · craft-loop.md + preflight.md）

**从 `craft-loop.md` 的工艺密度菜单取的项，落成的具体数值：**

1. **「把判据钉在像素账上，不钉在感觉上」**（`craft-loop.md` 的 audit→polish 顺序）——
   落成 §6.49 的三条判据与两条 pin：`4px`（`breathUnits × 4`）、`±12.5%`（折边比例容差）、
   `y=55`（发丝线位置）。三个数都可复算，不依赖谁的审美。
2. **「先读现物，再动手」**（打磨模式第 1 条）——落成「第一次量错」那段：先量到 19px、
   复核发现是 `contrastIn` 的判据漏了整行同色的线，改成 gutter 剖面后是 4px。
   **这条救了一轮假交付**：按 19px 写下去，报告会说「两张卡同值」，而真相是两个数。
3. **`user-preferences.md` P-5（可读性优先于气质）**——落成「4px 是否够」这一问不靠感觉
   回答：量了被切行的墨（117/226、130/226）确认折边仍然可读，所以 4px 这一轮**不算缺陷**，
   只算「与输出页不一致」，交拍板而不是擅自改高度链。

**preflight 抽查（A / B / C / D2）**：

- **A（反 AI 套路）**：过。本轮零装饰性产物——没有渐变白边、没有 glow、没有为了好看加的
  阴影；唯一的新增物是测试与文档。
- **B（设计读取一致性）**：过。改动只读 `docs/liquid-glass.md` §1（材质三档）与 §6.48 的
  判据；4px 与 16px 的差异被明确登记为**同一规则的两个数**，而不是「两张卡各自为政」。
- **C（动效抽查）**：过（本轮无动效改动）。折边是**比例**不是动画：`foldAlpha` 读上一帧盒子，
  滚动时行按露出比例退后，没有新的 transition、没有 tween。
- **D2（中文页正文 ≥ 14px）**：**不适用且仍是未决项**——本轮没有新增任何中文文案，
  也没动字号；`tokens.Caption` 11px 与 D2 的冲突沿用第 9 份证据，仍在 `.pending-bugs.md`
  的「需用户拍板」里，不静默跳过。

## 仍待（写进 `.pending-bugs.md`）

1. **需拍板（新）**：启动器的 `breathUnits`（4px）对捕获输出页的 `Space(4)`（16px）——
   同一条规则两个数。抬 `breathUnits` 到 4 单位 = 窗口长 12px，高度链是红线。
2. 需拍板（沿用）：`tokens.Caption` 11px / `ui/tooltip.go:133` / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 按下档那一对线 / 终端外观抽屉行要不要面 /
   终端弧 S1b / S1c / S2 尾。
3. 下一格（通用队列）：**状态反馈的另一半——禁用 / 只读面**。先量再改。
