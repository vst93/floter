# P13-ef 报告 · 最后 9 处「自己搭取景」的夹具换到出货尺

**这轮为什么让它更轻/更有空间感**：本轮不动墨，动的是**尺子**——这一包最后一次把
「装主题」抄了 9 遍的夹具收成一个定义点（`themedView`）。之前这些 pin 量的是框架库存
主题（`Spacing 4.0` / 行盒 40 / 帧高 488）画的帧，用户在窗口里看到的是 3.6 / 36 / 439
的那一张。空间感的所有后续判断（面差、发丝行位、行块左沟）都以「量的是哪张卡」为前提，
这轮把最后一张假卡换掉。

## 落地（逐条对应 spec）

1. **取景只有一个定义点**：新增 `themedView(a)`（`internal/launcher/view_test.go`），
   `render` / `renderShipped` / `renderIn` / `renderAt` 与 9 处直接开窗的测试全部调用它。
   装主题的两行从此只出现一次（`grep -c "c.SetTheme(a.tokens(c).Theme)" view_test.go` = 1）。
2. **9 处直接开窗换尺**（`bandheight_dump` / `clickdraw` / `deadlinewake`×2 / `footerink` /
   `feedbackpredicate` / `searchruns` / `spinnerframes`×2 / `outputfold` 出图）：
   帧高 488 → **439**；计数的三处保留自己的闭包，只把 `a.View(c)` 换成 `draw(c)`。
3. **`clippedRow` 按像素判「切」**：`share > 0 && share < 1`（float32 除法）→
   `top < clipBottom && bottom > clipBottom`（整数像素）。
   **这是本轮最有价值的一条**：出货尺上行盒 `y=100.4 h=36` 在 318.6 高的视口里算出的
   share 是 **0.99999997**，于是「整行都在视口里」的第一行被判成「被折边切了一条缝」——
   pin 量的是第一行的头带（fold=1），两条读数一样，报的是「折边没打开」。这条判据在
   库存尺的整数坐标下永远碰不到，换尺才露出来。
4. **折边量尺那一族跟尺**：`inkAt(frameHeight…)` → `themedFrameHeight(…)`；
   出货尺上标题落在折边上的实测窗口是 `275`(f=0.12) / `280`(f=0.32) / `295`(f=0.89) /
   `320`(f=0.47)，输出页 `350`(f=0.29) / `360`(f=0.68) / `404`(f=1.00)。
   重钉：单档 300 → **280**；两档 408/300 → **295 / 275**；输出页 404 → **350**
   （360 只切到 17.6/26，墨 79/116 过不了「不到一半」那条线，不是合格的取景）。
5. **新 pin** `TestTheDirectTestersDrawTheShippedFrame`：直接开窗的帧高 439、发丝 49、
   第十行整行在帧里（库存尺把这三个数写成 488 / 55 / `y=506` 溢出 439）。
   配 opt-in 出图 `TestDumpP13EFFixturePNGs`。

## V / B / S 台账

- **V（视觉数值重钉）3 条**：
  - 状态带 `30` → `tenths(30 * designUnit())` = **270 tenths = 27px**（`30u × 0.9`；
    旧的 30 只在库存 `Spacing 4.0` 下成立，那正是「把参考的 u 写成像素」的病）。
  - 输出页折边取景 `404` → **350**（实测 fold 0.292）。
  - 启动器标题折边取景 `300`/`408` → **280** / **295 vs 275**（实测窗口见上）。
- **S（结构真变了）1 条**：`clippedRow` 的判据从比例改成像素（旧判据在 float32 噪声下
  误判整行为「被切」）。
- **B（行为/接线）0 条**：产品代码 **0 行**，高度链 / 键盘模型 / 会话管理 / 设置写回 /
  i18n / 命令接线一律未碰。

## 门槛（实测）

```
gofmt -l cmd internal      # 空
go vet ./...               # 净
GOOS=windows go vet ./...  # 净
GOOS=darwin  go vet ./...  # 净
go test -count=1 ./...     # 全绿（0 FAIL）
--- PASS 计数              # 3333（基线 3332）
--- SKIP 计数              # 165（基线 164，新增 opt-in 出图一条）
go build ×3                # 过
测试函数                   # +2（1 条 pin + 1 条 opt-in 出图），无删减
```

## mutation（3 条实跑，全部变红并逐字节还原）

- **M1** `themedView` 删掉 `c.SetTheme(a.tokens(c).Theme)`：
  `TestTheDirectTestersDrawTheShippedFrame` 红 —
  `the field block's hairline is at y=55, want 49 on the shipped ruler`、
  `the tenth row ends at y=506, past the 439px frame: the fixture is a row short`；
  `TestTheDefaultFixtureDrawsTheShippedFrame` 红 — `clip y=60` / `hairline is at y=55, want 49`。
- **M2** `clippedRow` 的像素判据退回「第一行也算」：
  `TestTheFurtherTheOutputRowIsCutTheMoreItRecedes` 红 —
  `height 439: row 0 box={X:7.2 Y:100.4 W:705.6 H:36} share=1.000`（两次），
  `the premise failed: the fold did not open (1.000 → 1.000)`。
- **M3** 状态带断言退回 `30`：`TestTheBandHasOnePredicate` 红 —
  `the frame asked for 418, a band over the base 391 is 27 (want the 30u band, 3.0px)`。
- 还原后 `md5sum -c` 三个文件全部「成功」。

## 出图（自己复核过）

`/tmp/floter-P13-ef-light.png`、`/tmp/floter-P13-ef-dark.png`，各 720×439。

**这张图上看到的一处变化**：两档在 `x=360` 的竖扫都只有一处变色——浅色 `y=49`
是 `(232,232,234)` 的发丝，上下都是卡面 `(246,246,248)`；深色 `y=49` 是 `(28,28,29)`，
上下是 `(15,15,16)`。库存尺那张图上同一条发丝在 `y=55`，且帧高 488、卡面 253/23 ——
也就是说本轮换掉的正是「用户看不到的那张卡」。

## 用了 qiaomu-design（`craft-loop.md` + `preflight.md`，已有方向只要打磨那条路）

**偷了什么**（逐条：菜单项 → 落成的具体数值）：

1. **`craft-loop.md` 工艺密度第 6 项「编辑细节：hairline」** → 发丝行位从库存尺的
   `y=55` 落到出货尺的 `y=49`（`round(14u × 3.6 − 1)`），9 处夹具引用的就是这一行。
2. **第 5 项「多层染色阴影 / 面档」的度量前提** → 卡面 `253 → 246`（浅）/ `23 → 15`（深）。
   面档只有 7-8 个亮度差时，任何「层级读不出来」的判断都必须先确认量的是哪张卡。
3. **第 12 项「材质细节：内边框高光」的度量前提** → 折边墨量必须按**同一段像素**在两个
   折边下比（`headBand` 的 8px 头带），本轮把两档都钉到出货尺的 439/459，读数
   0.406 → 0.989，才能说「退后」而不是「被切」。
4. **`craft-loop.md` 三·两轮制（渲染 → 评审 → 只修 3-5 处）** → 第一轮只换尺，
   一次跑出 3 个红；评审发现其中一个红**不是**取景问题而是 `clippedRow` 的判据在
   float32 下误判，第二轮只修这一处 + 两条取景，没有顺手大改。

**preflight 逐条结论**：

- **A（反 AI 套路）**：通过/不适用 —— 本轮不产界面；没有渐变、glow、斜体，也没有用
  「渐变白边冒充玻璃」（`liquid-glass.md` §0 的硬禁令未被触碰）。
- **B（设计读取一致性）**：通过 —— 一个仓库一个调色板来源（`internal/theme`），本轮把
  最后 9 个绕过它的夹具收回 `a.tokens(c).Theme`；`grep -c` 证明装主题只剩一个定义点。
- **C（动效抽查）**：不适用 —— 不动动效；`renderAt` / `renderIn` 的 `SetDark` 之后仍不
  加 `Advance`，因为这族 pin 读的是几何而不是颜色值（P9-c 教训保留）。
- **D2（中文页）**：**未过（沿用，第 55 份证据）** —— `tokens.Caption` 9.9px / 正文 10.8px
  对「正文 ≥ 14px」；本轮没动字号，仍在 `.pending-bugs.md` 的「需用户拍板」里。

## 仍待（P13-ef）

1. **下一格（首选）**：同一族的**另一半**——这一包还有约 15 处 `ui.NewTester` 自己搭取景
   且**没装主题**（`actionbarfooter:915`、`bench:29/41`、`dimplate:36`（故意用自己的地面，
   需判）、`filters:117/1118`、`fontladder:45/63/152`、`headingcolumn:37/52/124`、
   `noteplate:135`、`outputdot:108`、`plateface:67/275`、`plate_lens:24`、`tipglass:284`、
   `triggerhintdump:29`、`triggerhint:124/166`、`uiscale:84`、`view:1029`）。
   本轮只收了「帧高」那一族（9 处），这一族是**墨**，另占一格；先量再改。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px 对 preflight D2 / 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash 核对（第五十九次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
