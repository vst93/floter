# P13-ct 报告 · 降级面的灯：一条关着的命令，冠线既不是亮的，也不是 62% 亮的

方向一句话：**扩展页的命令块按状态分材质**——开关开着的戴 `LitFace`（灯亮），关着的戴
`DimFace`（灯熄、暗线留、面还在），那道 `.Opacity(0.62)` 保留。从此「关着」在两套外观上
是同一句话。

## 1. 为什么是这一格

任务书「下一格从『状态反馈』的另一半（**禁用 / 只读面**）里挑，先量再改」；§6 差距表已无
`未开始` 行。禁用面一族的其它成员（`disabledActionButton` / `busyActionButton` /
`sessionKillConfirm` / `extensionIconButton` / `lockedControl` / `disabledSwitch` / 卸载整卡）
都已按 §6.2b-6 与 §6.15 落地过。剩下的**唯一没量过**的是**不是 `Disabled`、只是降级**的那
一类——`commandPlate`（`.extension-command-list__row--off`）：它的开关与别名框仍然是活的，
所以不能挂框架的禁用位，也就没人替它回答「降级面的灯怎么办」。

## 2. 先量（平铺 harness，`litSum` 三通道和，1 亮度级 = 3）

| 状态 | 外观 | 地面 | 面 | 冠线行 − 面 | 底对侧 − 面 |
|---|---|---|---|---|---|
| 开着的命令 | 浅 | 742 | 690 | **+18（6 级）** | 63（21 级） |
| 开着的命令 | 深 | 44 | 118 | **+57（19 级）** | 24（8 级） |
| 关着的命令（改前） | 浅 | 742 | 709 | **+9（3 级）** | 39（13 级） |
| 关着的命令（改前） | 深 | 44 | 89 | **+39（13 级）** | 12（4 级） |

改前的两行来自变异 M1 的报错串（同一个 harness、同一条读数）：
`the off command's plate still wears a lit edge: its crown row is 718 over a 709 face (+3 levels)`
与 `… 128 over a 89 face (+13 levels)`。

**结论**：不是「淡了一点」，是「没有决定过」。`LitFace` 无条件先戴上，0.62 再把整棵子树洗一
遍，于是冠线成了 62% 的灯：浅色档一条本来只有 6 级的线上剩 3 级（读出来是一道洗白的边），
深色档 19 → 13 级（还明明白白亮着）。同一句「这条命令是关的」在两套外观上是两个说法。

## 3. 落地

- `internal/settingsui/settingsui.go` `commandPlate`：
  `if command.Enabled { tokens.LitFace(plate) } else { tokens.DimFace(plate); plate.Opacity(0.62) }`。
  注释补 P13-ct 一段：为什么换 `DimFace`（§3「受光边是这块面可以被碰的记号」+ §6.15 18-1
  的同一半），以及为什么**不加** `Disabled`（开关与别名框仍然活着，参考那边是 `--off`
  不是 `:disabled`）。
- 新增 `internal/settingsui/platecrown_test.go`：① 开着的板冠线 ≥ 3 级 ② 关着的板冠线 ≤ 2
  （`litSum` 单位，与 `assertDisabledFace` 同一个容差）③ 关着的板底对侧留着（浅 ≥ 5 级 /
  深 ≥ 2 级）④ 关着的板的面仍离地面 ≥ 6 级（浅在下、深在上）；深/浅各一遍。板画在干净的
  地面上，框用 `Bind`/`Resolve` 取，pin 里没有任何像素坐标。
- opt-in 出图 `TestDumpP13CTCommandPlate`（`FLOTER_P13CT_DUMP=1`，`FLOTER_P13CT_TAG=before/after`）。

## 4. 前后对比图

`/tmp/floter-P13-ct-commandplate-light-before.png` / `-after.png`、
`/tmp/floter-P13-ct-commandplate-dark-before.png` / `-after.png`（整页 720×900）。

差分：浅色 **531 px**、深色 **533 px**，bbox `(189,597)-(702,712)` = **关着的那块板**
（`Build`），开着的 `JSON Viewer` 一个像素都不在里面。逐行直方图：

```
light  y=597 → 507 px（整条冠线，板的宽度）… y=598…603、711…712 各 2~8 px（圆角弧上的抗锯齿）
dark   y=597 → 507 px，其余同上
```

**这张图上看到的一处变化**：关着的那块板只剩**顶行那一条线**变了——它从「比面亮 3 级
（深色 13 级）」变成「与面同高」，而它下面那道 0.62 的淡、底对侧那条暗线、板上的字一个像素
没动（行 597 之外的差异全在圆角弧上，每个 2~8 px）。也就是说这块面从「点了 62% 的灯」变成
「灯熄了、仍站得住」。

## 5. 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落到数值）

| 取第几项 | 菜单里的签名动作 | 本仓落成什么 |
|---|---|---|
| **#12** | 材质细节（内边框高光 / glass 折射边） | 这条是它的**反面**用法：一个「高光」只有在**该亮的面**上才亮。落成 `commandPlate` 的两个材质臂 —— 开着 `LensRim` 冠线（浅 +6 级 / 深 +19 级），关着 **0 级**，且**底对侧不许跟着熄**（浅 13 级 / 深 4 级） |
| **#5** | 多层染色阴影（拒绝单层灰） | 本仓的等价物是 E1 的**一对**线（顶受光 / 底暗），不是一条。落成 pin 的四条判据里除了冠线那两条，另两条专门钉「底对侧还在」——一条线是阴影，一对线才是面 |
| **#6** | 编辑细节（hairline） | 板的外圈 `Hairline`（画框）与内圈的冠线（材质）是**两条**、两个角色：本轮画框一个字节没动（`Border(1, tokens.Hairline)` 原样），只换了内圈那条。这正是「收的是墨与面，不是框」 |

不取：`#1` 氛围层、`#8` 流体字号、`#9` 滚动条、`#11` 图形资产——本仓是 Go 自绘桌面应用，
没有 DOM 层，这些在本轮这一块面上没有对应物（强凑就是 §0 的「假装有 blur」）。

## 6. preflight 逐条结论

- **A 反套路**：无渐变、无发光、无斜体（本轮没加任何装饰；反而**减**了一条亮度）。
  HTML 专属项（字体族 / Hero 构图 / 眉批 / H1 行数）不适用——这里没有 HTML。
  唯一相关的一条 P-38「不许用左侧竖线做强调」：本轮没加竖线，命令块仍然靠面与一对线说话。
  **通过。**
- **B 设计读取一致性**：读取为「桌面效率工具，设置页的扩展命令列表，用轻便液态玻璃语言，
  倾向 elevation 阶梯（面 + 受光边 + 阴影）」；记忆点 = **「一张列表里只有活着的那块点灯」**。
  产出与读取一致：两套外观给出同一个结论（0 级 / 0 级），而不是两个百分比。**通过。**
- **C 动效抽查**：本轮**零动效改动**（没有新增 transition / 缩放 / 时长）。相关项都仍成立：
  0.62 是静态不透明度（不是动画）；板的 hover/press 不在这块面上。**通过（无适用项）。**
- **D2 中文页**：本轮**不改任何文案、字阶、字重、行高**。已知未决项照旧挂在拍板上：
  `tokens.Caption` 11px 与 D2「正文 ≥ 14px」的张力（**第 21 份证据**，本轮不改）。**通过（无新增违规）。**

## 7. V / B / S 台账

- **V 0**（没有任何断言钉在旧材质上：这条面此前一个像素 pin 都没有）
- **B 0**（`TestTheCommandListDrawsTheReferencePlates` 的「the off command's plate is not dimmed」
  与「plate 有自己的面」两条**零改动**通过——证明那道 0.62 与布局没被动）
- **S 0**

## 8. 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿
--- PASS 3290（基线 3289，+1）
--- FAIL 0
--- SKIP 139（基线 138，+1 = 本轮新增的 opt-in 出图）
测试函数 2439（基线 2437，+2 = pin + dump）
GOOS=linux/windows/darwin go build ./...               → 过
```

## 9. 变异（3 条实跑，全部变红，`md5sum -c` 逐字节还原）

- **M1 · 关着的板重新戴 `LitFace`**（回到改前两行）
  → `platecrown_test.go:128` 两红：
  `dark=false: the off command's plate still wears a lit edge: its crown row is 718 over a 709 face (+3 levels). A command whose switch is off is drawn with the light out (`DimFace`), not with a softer light`
  `dark=true: … crown row is 128 over a 89 face (+13 levels) …`
- **M2 · 删掉那道 0.62**
  → `commandlist_test.go:143`：
  `the off command's plate is not dimmed: {213 213 214 255} == {213 213 214 255}`
- **M3 · 关着的板什么材质都不戴**
  → `platecrown_test.go:140`：
  `dark=true: the off command's plate lost its counter-side too: 89 under a 89 face (−0 levels). `DimFace` keeps the shade and drops the light, not both`

每条还原后 `md5sum -c` 均 `成功`，`touch` 后复跑绿。

## 10. 仍待（P13-ct）

1. **下一格（不需拍板，先量）**：`actionWithGlyph` 的 hover 墨是硬切——`settingsui/rows.go` 的
   `row := ui.Row(c)…` 没有 `Transition{Colors: true}`。**但先量后发现方向不对**：框架的
   `Transition.Colors` 只动元素自己的 `bg` / `borderC`
   （`mygo@v0.7.4/ui/transition.go:810-851` 的 `animateColors`），这条动作没有面、墨写在子
   `Text`/`Icon` 上，补 transition **一个像素都不会变**。要真正让墨跟手，得先有「墨也能过渡」
   的机制——那是框架层，属另一片，先记在这里，不要静默补一行无用代码。
2. **需拍板（沿用 + 第 21 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路；会话页空态说错了**对象**；
   终端页会话抽屉 / 标签对失败只加字不换墨；终端外观抽屉行要不要面；
   `launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带 chip 的按下档；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **需拍板（第 5 份证据，§6.32-4）**：同一张卡上的两种「这一层的名字」。
5. **记一笔（不改）**：`rowFace.Muted`（`.settings-row--muted` 的 0.62）**当前零调用点**。
   它和本轮是同一个形状（不是 `Disabled` 的降级面）；将来接上时答案照 §6.63-4。
6. **抢救 stash 核对（第二十二次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
