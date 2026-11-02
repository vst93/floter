# P13-dv 报告：捕获输出页的五条前提破（出货尺重钉）

方向一句话：**把捕获输出页的夹具从「库存尺 + 字面量」搬到「出货尺 + 页面自己的几何」，
五处前提重新成立——这是启动器/输出页夹具债的 S 类收口。**

**为什么让它更轻/更有空间感**：条子下面那条呼吸带第一次成为**页面自己的量**
（裁剪上沿 − `Space(4)`），两条分支用同一条推导；空气不再是库存尺遗留的 16px 魔法数，
深浅两套、两个分支的带完全一致。

## 先量（出货尺：Spacing 3.6，帧 720×439）

- 编号族：库存尺 9 条 → 出货尺 **10** 条（`1..9`，第十条键帽 `0`）。
- 列表分支：条末墨 **y=83**，裁剪上沿 **y=100**，带 **86..99**（14 行 = `round(4×3.6)`），
  滚一行后带内墨 **0**（浅/深）。
- 正文分支：裁剪上沿 **y=100**（本轮新记账），带 `86..99` 墨 **0**；
  此前正文分支没有裁剪盒，夹具只能读库存字面量 118。
- 视口下沿：**466 → 419.2**（帧高 488 → 439）。
- 首行整行：`visibleShare` = **0.999999769**（`7.2 + 705.6` 的 float32 加减丢 1 ulp），
  盒 `x=7.2..712.8 y=100.4..136.4` 完全在视口 `x=0..720 y=100.4..439` 内；
  静息墨 **219**（库存尺旧注释记 228）。

## 落地

1. `internal/launcher/output.go:761`（产品，只补记账）：正文分支 `ui.Scroll(...).Bounds()`
   记入 `a.outputListRect`；`assignOutputNumbers` 对正文早退，编号族零影响。
2. `internal/launcher/outputedges_test.go:56` `outputAirBand(clipTop)`：下沿 = 裁剪上沿，
   高 = `round(4 × Spacing)`；删掉 118 与 16 两个字面量。
3. 同文件 :66 / :105 两条带夹具改 `listOutputShipped`，先读 `a.outputListRect.Y`，
   再从带的上沿往回找条末墨；带与条墨必须不相交（夹具自检）。
4. 同文件 :138 视口下沿 `466 → 419.2`（`tenths` 读 + 旧值→新值注释）。
5. `internal/launcher/outputfold_test.go:161` 首行前提由比例改成**几何谓词**（四条边）；
   阈值 150 不动，实测 219（旧值 228 是库存尺的读数）。
6. `internal/launcher/headingfold_test.go:246` 编号族 `9 → 10`（S：旧结构 → 新结构），
   标题盒仍不进编号族。
7. 新增 opt-in dump `TestDumpP13DVOutputBandPNGs`（`:269`），四张图。

## 台账

**V 3 / B 0 / S 2**（+1 opt-in dump）：

- V：视口下沿 419.2；两条分支的带定义（16 字面量 → `Space(4)` 推导）。
- S：编号族 9 → 10 条；首行前提「比例 == 1」→ 几何谓词。
- B 0：产品行为零改动（只有记账 + 注释）；旧断言强度不降。

## 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | **空** |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | **净** |
| `GOOS=linux/windows/darwin go build ./...` | **全过** |
| `go test -count=1 ./...` | **全绿** |
| PASS / FAIL / SKIP | **3325 / 0 / 159**（基线 3325 / 0 / 158，+1 opt-in dump） |
| 测试函数 | **2492**（基线 2491，+1） |

## Mutation（3 条实跑，红 → 还原 → md5 逐字节 → touch 复跑）

- **M1 产品记账**：`a.outputListRect = scroll.Bounds()` → `ui.Rect{W: …}`（丢 Y）。
  红：`outputedges_test.go:119: dark=false: the status strip drew no ink above the clip (clip top y=0)`。
- **M2 带的锚点**：`clipTop - h` → `clipTop`。
  红：列表 `the band under the strip (y=100..113) carries ink 53`（深 50）；
  正文 `the text scrolled into the strip's breathing band (y=100..113) with ink 216`（深 219）。
- **M3 编号族 pin**：`want 10` 退回 `9`。
  红：`the numbered family has 10 entries, want 9 (numbers=map[1:1 … 10:0])`。

三条都 `md5sum -c` 成功（`05bab2e4… / 10578d96… / c32762cf… / f5dab880…`），
`touch` 后五条夹具重跑全绿（不吃测试缓存）。

## 出图（肉眼复核）

`/tmp/floter-P13-dv-{list,text}-{light,dark}.png`，四张全 **720×439**。
逐像素抽读带区（x=360）：`list-light` y=86/92/99 全是 `(246,246,248)`，
`list-dark` 全是 `(15,15,16)`；正文两张同值——**这就是这轮看到的变化**：
条子下面的 14 行在任何一套外观上都是同一种「空」，没有墨、没有影、没有渐变。

## skill 用法（qiaomu-design，打磨车道）

读了 `SKILL.md` → `references/user-preferences.md`（P-38 无左侧竖线、P-1 无斜体、
P-14 无常驻系统自言自语、P-5 可读性优先）→ `craft-loop.md` / `preflight.md`。
方向已由用户定死（轻便液态玻璃），走「已有方向，只要打磨」车道，不触发 Phase 2 预览。

**偷了什么（工艺密度菜单，逐条落数）**：

- 第 6 项「编辑细节 / 精确几何」：把「条子下面的空气」写成三个可复核的数——
  条末墨 83、裁剪上沿 100、带 86..99（14 行 = `round(4×3.6)`），替换库存字面量 118/16。
- 第 5 项「多层染色阴影」的反面纪律：呼吸带是**不带材质**的空气——实测带内
  `bandInk = 0`（两条分支、深浅四帧），浮层影不越界。
- 第 12 项「材质细节」的克制：本轮不新增受光边/高光；空的带就是空的，
  按 §0 硬禁令「不许用渐变白边模拟高光」。

**preflight 逐条**：

- **A 反套路**：无渐变/发光/斜体/编号眉批；本轮不新增任何墨（只把带量准）。
- **B 设计读取一致性**：两条分支现在报同一个裁剪原点（y=100）与同一条带规则；
  设计读取一句话 =「捕获输出页，面向读运行结果的人，用轻便液态玻璃语言，
  倾向近黑底 + 面差 + 一条受光边」。
- **C 动效抽查**：无动效改动；唯一的「动」是夹具里 `ScrollTo(4)` 与 `Output.Y=300`，
  两条分支在滚动后带内仍是 0 墨。
- **D2 中文页**：**仍未过**——`tokens.Caption` 9.9px / 正文 10.8px < preflight「正文 ≥ 14px」。
  第 **47** 份证据，等用户拍板（不擅自改字号）。

## 仍待（P13-dv）

1. **下一格（首选）**：`P13-dg-l-3` = 2 条混排 B：`textcolumn_test.go:40`、
   `outputheading_test.go` 的裸 `ui.NewTester` 要装上这一面的调色板（与 `render` 同一份）。
2. 其后：`P13-dg-l-4`（产品墨，独立一轮）：`outputsubtitle` 选中行副标题浅 119 / 深 112，
   对启动器选中行 125 / 118，差 6 级，两支要收到同一条公式上。
3. 其后：`P13-dg-t` / `P13-dg-s` 两个画面包的夹具取景收口，每包一格。
4. **需拍板（沿用 + 第 47 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 对 D2「≥ 14px」。
5. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
6. 抢救 stash 核对（第四十九次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。

## 后续遗留（同一表面、不在本轮）

`TestTheFurtherTheOutputRowIsCutTheMoreItRecedes` 与 `TestTheOutputFoldRecedesTheSameAtBothEnds`
仍跑在 `runListOutput`（库存尺）上——它们属于 §6.86 表里的「阶梯（V）」与混排族，
留给 `P13-dg-l-3` 之后的轮次，本轮不顺手大改。
