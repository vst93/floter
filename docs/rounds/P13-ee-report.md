# P13-ee 报告：显式开窗那一族（`renderAt`）换到出货主题

选题：`P13-ed`「仍待」第 1 条 = `liquid-glass.md` §6.86 的 **73-5** 尾巴。表面 = **测试夹具**
（不是界面）。规格 `docs/rounds/P13-ee-spec.md`，合约新增 **§6.100**。

## 这轮为什么让它更轻、更有空间感

这一族 13 个调用点此前把「面」画在一张 **488px** 高的卡上，墨和间距是框架库存主题
（`Spacing 4.0` / `FontSize 14`），而 app 出货的是 **439px / 3.6 / 10.8**。换尺之后，
这些 pin 读的发丝（`y=55 → 49`）、行块左沟（`8 → 7`）、卡面（`253 → 246` 浅 / `23 → 15` 深）
全部落在出货尺的那一套上 —— 也就是说，此后每一轮「层级更清楚了 / 面差更明显了」的读数
不再建在一张虚构的卡上。这轮本身不改界面，它修的是**读界面的尺子**。

## 先量

把 `renderAt` 换成 `SetTheme(a.tokens(c).Theme)`、13 个调用点换成 `themedFrameHeight` 之后，
`go test ./internal/launcher/` **一次全绿，0 红**。

这本身是这轮最硬的发现：这一族的 pin 全是**相对读数**（墨带 runs、文字重叠审计、墨跑数、
帧高=调用点给的），对尺子不敏感。所以「换尺」在这族里是**无声的** —— 换对和换错都绿。
因此本轮真正的交付是那条能分辨两把尺子的 pin，而不是若干条重钉（**V 0 条**，因为一条都不红）。

补 pin 的过程中量到两件事：

1. **剪贴板页在静止态没有选中行**：整行卡面一路铺到 x=0（`246,246,248` 连续 22 列），
   所以「行块左沟」这个探针在模式页上读不到东西（第一版 pin 读到 **14**，那是 chips 带的边，
   不是行块）。沟的探针只能落在**普通页的选中行**上 —— 已按此拆成两条路径。
2. 模式页的 `listRect.Y` 是 **72**（普通页 54）：chips 带把列表整体推下去 18px，两档窗口
   高度下都一样 —— 这是这一族把「窗口尺寸」当参数的由来。

## 落地

1. `modetextframe_test.go:135` `renderAt` 回调先 `c.SetTheme(a.tokens(c).Theme)`。
2. 13 个 `renderAt(... frameHeight(maxResultSlots) ...)` → `themedFrameHeight(maxResultSlots)`
   （`filesempty` ×3、`filesframe` ×1、`filesmarks` ×2、`modetextframe` ×5、`quietclip` ×1）。
3. 两张 `sizes` 表收成一个包级寄存器 `fixtureWindowSizes`（放在 pin 旁边），
   `filesframe` / `modetextframe` 都写 `sizes := fixtureWindowSizes`。
4. `quietclip_test.go:111` 的墨带高度换 `themedFrameHeight`。
5. 新 pin `TestTheExplicitWindowFixtureDrawsTheShippedInks`：两档窗口（439 / 360）各钉
   帧高、发丝 49、选中行块左沟 7，外加寄存器臂。
6. opt-in 出图 `TestDumpP13EEWindowFixturePNGs`。

台账 **V 0 / B 0 / S 1**（S：取景换尺 + `sizes` 表收成一个寄存器）。
**产品代码 0 行**。测试函数 2500 → **2502**，PASS 3331 → **3332**，SKIP 163 → **164**。

## 变异（3 条实跑变红并还原，`md5sum -c` 逐字节）

- **M1** `renderAt` 退回不装主题 →
  `720x439: the field block's hairline is at y=55, want 49 on the shipped ruler (the stock theme puts it at 55)` +
  `720x439: the selected row's block starts at x=8, want 7 (8u × 0.9 = 7.20 on the shipped ruler; the stock theme paints 8)`（两档窗口各两条，共 4 条）。
- **M2** `fixtureWindowSizes[0][1]` 退回 `frameHeight` →
  `the explicit-window audits draw at 488px, want the shipped frame 439px (the stock ruler charges 488px)`。
- **M3** 发丝的 `want` 反向写成 55 →
  `720x439: the field block's hairline is at y=49, want 55 …`（两档窗口），证明这条 pin
  不是「两边都是 49」里空过的。

三次还原后 `md5sum -c`：`modetextframe_test.go` 3ff48be6… 成功、`fixtureruler_test.go`
8a9f31e2… 成功（M3 前 `touch` 过一次，避免测试缓存复用）。

## 出图（眼睛复核）

`/tmp/floter-P13-ee-{stock,shipped}-{light,dark}.png`，剪贴板页，720 宽：

| | stock | shipped |
|---|---|---|
| 帧高 | 488 | **439** |
| 发丝 | `y=55`（浅 239 / 深 36） | `y=49`（浅 232 / 深 28） |
| 卡面 | 253 / 23 | **246 / 15** |

共同区域 720×439 上逐像素 diff 浅 **316019** / 深 **316041**，最大通道差 **247 / 230**。

**这张图上看到的一处变化**（在 `x=360` 逐行读的）：浅色 shipped 图在 `y=49` 有一条
`(232,232,234)` 的发丝，上下都是卡面 `(246,246,248)`；浅色 stock 图的同一条发丝在 `y=55`
且是 `(239,239,240)`，上下都是 `(253,253,254)`。也就是说 stock 图上整块内容比出货卡**低 6px**
（发丝以下的 chips 带与列表跟着低 6px），而卡本身还高 49px —— 底部多出整整 49px 的地面。

## 门槛（实测）

```
gofmt -l cmd internal      # 空
go vet ./...               # 净
GOOS=windows go vet ./...  # 净
GOOS=darwin  go vet ./...  # 净
go test -count=1 ./...     # 全绿（0 FAIL）
--- PASS 计数              # 3332（基线 3331）
SKIP 计数                  # 164（基线 163，新增出图一条）
go build ×3                # 过
```

## 用了 qiaomu-design（`craft-loop.md` + `preflight.md`，已有方向只要打磨那条路）

**偷了什么**（逐条：菜单项 → 落成的具体数值）：

1. **`craft-loop.md` 工艺密度第 6 项「编辑细节：hairline / 大写字距标签」** → 发丝的**行位**
   从库存尺的 `y=55` 落到出货尺的 `y=49`（`round(14u × 3.6 − 1)`，14u 是字段块的盒高）。
   一个夹具引用这条线时，读的必须是这一行。
2. **第 5 项「多层染色阴影 / 面档」的度量前提** → 卡自己的面 **253 → 246**（浅）、
   **23 → 15**（深）。面档只有两级差 7 / 8 个亮度时，任何「层级读不出来」的判断都必须
   先确认量的是哪张卡；这轮把那张卡换成出货的。
3. **第 6 项的「间距节奏」一半** → 行块左沟 `8 → 7`（`8u × 0.9 = 7.20`，painter 从 7 画）。
   间距刻度只有落在 0.9 的步进上才和「4u / 8u / 14u」这些参考数字对得上。
4. **`craft-loop.md` 三·两轮制（渲染 → 评审 → 只修 3-5 处）** → 第一轮换尺（0 红），
   评审发现「pin 分辨不出尺子」这个比换尺更严重的问题，第二轮只补这一条 pin。

**preflight 逐条结论**：

- **A（反 AI 套路）**：不适用/通过 —— 本轮不产界面；没有渐变、没有 glow、没有斜体。
  与本仓合约一致的那条硬禁令（§0「不许用渐变白边模拟高光」）没有被这轮触碰。
- **B（设计读取一致性）**：通过 —— 一个仓库只有一个调色板来源（`internal/theme`），
  本轮把最后一个绕过它的夹具收回 `a.tokens(c).Theme`。此前 `renderAt` 读的是框架库存主题，
  属「设计读取不一致」的实体案例。
- **C（动效抽查）**：不适用 —— 本轮不动动效；但保留了一条纪律：`renderAt` 的 `SetDark`
  之后**没有**加 `Advance`，因为这族 pin 读的是墨带几何而不是颜色值；若以后有 pin 在这族里
  读绝对颜色，必须补 `Advance`（P9-c 教训）。
- **D2（中文页）**：**未过（沿用，第 54 份证据）** —— `tokens.Caption` 9.9px / 正文 10.8px
  对「正文 ≥ 14px」；本轮没有动字号，仍在 `.pending-bugs.md` 的「需用户拍板」里。
  另：本轮量到 10.8px 的字描边覆盖率只有 **0.90**（§6.99 的 `coverageOver`），
  即「9.8px 的字够不到自己的墨」这条更硬的证据仍成立。

## 仍待（P13-ee）

1. **下一格（首选）**：8 处**直接** `ui.NewTester(..., frameHeight(maxResultSlots))` 的库存尺
   （`clickdraw_test.go:34`、`deadlinewake_test.go:37/107`、`footerink_test.go:75`、
   `headingfold_test.go:104`、`searchruns_test.go:38`、`spinnerframes_test.go:107/164`、
   `bandheight_dump_test.go:33`、`outputfold_test.go:235`）—— 同一把库存尺，另占一格。
2. **需拍板（沿用）**：便条行 rank 1.28 / `tokens.Caption` 9.9px 对 preflight D2「≥14px」/
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash 核对（第五十八次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
