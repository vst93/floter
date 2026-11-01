# P13-bz 报告 · 设置页的侧栏与内容此前是同一个 z 平面

规格 `docs/rounds/P13-bz-spec.md`｜合约新增 **§6.43**｜选题 = `.pending-bugs.md` P13-by「仍待」第 1 条。

## 这轮为什么让它更轻、更有空间感

一句话：**这一页此前只有一级 z。** 侧栏和内容都戴 `Sunken`——一层 0.17 的浅色薄罩，
压在窗口地上合成后只差 **1 级**（dark `14→15`、light `247→246`）——两栏之间唯一的分界
是中间那根 1px 竖发丝线。这正是用户 2026-10-30 说的「所有东西挤在同一个 z 平面上，
靠 1px 描边分家」在设置页上的算术来源。现在铬是一块**面**（+25 / −17 级），内容是**底**，
那根线退成两个材质交界上的一条接缝——**同一个像素都不多画，两栏之间多了一整级 z。**

## 先量（改前，本轮实测）

`theme.ForPreferences(settings.Default(), dark, ui.Preferences{})`，默认 47% 框：

| | 地 | `FaceOver(地, Sunken)` | 差 |
|---|---|---|---|
| dark | `14,14,16` | `15,15,17` | **+1 级** |
| light | `247,247,248` | `246,246,248` | **+1 级** |

整页渲染（720×900，`PageGeneral`）横扫选中行中线：侧栏 `x=10..140` 与内容 `x=400..719`
在行外读到的是同一对像素。两栏之间只有 `x=159` 那一根线（dark 27 / light 233）。

## 落地（一处材质、两条顺带）

| 位置 | 改前 | 改后 |
|---|---|---|
| 两栏的父行 `ui.Row(...)`（`settingsui.go`） | 无填充，露出窗口地 14 / 247 | `Background(tokens.Control)`：**39 / 230**（+25 / −17 级） |
| 148u 侧栏列 | `Background(Sunken)`（与内容同一层薄罩） | 去掉自己的填充，继承父行的面 |
| 内容列 | `Background(Sunken)`（薄罩压在面上会合成成 35） | `Background(FaceOver(地, Sunken))`：**像素逐字节不变**（15,15,17 / 246,246,248） |
| 深链值的代码片 `codeRow` | `Background(Sunken)`：与卡片只差 4 级 | `FaceOver(地, Sunken)`：**15 / 246**（−24 / +16 级） |

**为什么面戴在父行而不是侧栏列上**：列间距是 11u + 1px + 11u，只给 148u 的列上色，
11u 的列间距会露出窗口地——那根线就漂在两块地中间，成为一条没有落在任何东西上的线。
戴在父行上，铬是一整块面，线正好落在材质交界（`x=159`）——macOS 设置面板的形状。

**为什么侧栏不戴冠线**：E1 是「落在一个底上的块」，有上边也有下边。侧栏是一列贴着窗口边的
通高容器（上接页头发丝、下接窗口下沿），冠线在这里就是给面板画框（§0 硬禁令 3）。
层级由面差表达（§1 第一条手段）。

**为什么内容页的填充要不透明**：它现在压在父行的**面**上，一层 0.17 薄罩会合成成 35
（比今天的 15 亮 20 级，倒退）。`FaceOver(地, Sunken)` 与今天「地 + 薄罩」的合成值**逐像素相同**，
只是把它算成了不透明色——与 `sheetedge.go` 的 `panelFace` 同一个函数、同一个理由。

## 改后实测

| | 铬面 | 内容页 | 选中行 | 竖发丝线 | 面差 / 接缝 |
|---|---|---|---|---|---|
| dark | **39,39,40** | 15,15,17（不变） | 58（面 +19） | 51（+12） | **24 / 12** |
| light | **230,230,230** | 246,246,248（不变） | 216（面 −14） | 217（−13） | **18 / 13** |

## pin

`internal/settingsui/sidebarface_test.go`：`TestTheSidebarIsAFaceNotAVeil`（四条判据 × 深/浅）
+ 一条回归钉（内容页必须仍等于 `FaceOver(地, Sunken)`，±2）+ `TestDumpSidebarFace`（opt-in 出图）。

## V / B / S 台账

**V 3 / B 0 / S 0。**

- **V**：`raycast_test.go` 的竖发丝线 pin——线现在压在**面**上，期望值 dark `27 → 51`、
  light `233 → 217`，扫描起点 `x=100 → x=148`（100 起会把侧栏行块 58 当成线）。
- **V**：`baseplugincard_test.go` 的 `cardBox`——采样点从窗口地 `x=2` 改成按 token 算的
  `FaceOver(地, Sunken)`（`x=2` 现在是铬面，不再是空地）。
- **V（没有改断言）**：`deeplinkcopy_test.go`——delta ≥ 12 的断言一个字没动，改的是产品
  （代码片的填充 4 级 → 24 级）。它此前是**假绿**：那张 pin 跑在混色帧里
  （app token 深色 / tester 帧主题浅色），chip 与卡在那张帧里差 34 级。本轮把内容页算成
  不透明色后混色帧消失，假绿才露出来。**这一条是本轮顺带量到的，不是本轮引入的回归**
  （P12-a 把 `Control` 抬到 0.10 时它就已经只差 4 级了）。

## 门槛（实测）

```
gofmt -l cmd internal                空
go vet ./...                         净
GOOS=windows go vet ./...            净
GOOS=darwin  go vet ./...            净
go test -count=1 ./...               全绿（0 FAIL）
--- PASS 计数                       3222 → 3225（+3）
--- SKIP 计数                       122 → 123（+1 = opt-in 出图）
测试函数                             2380 → 2382
GOOS=linux/windows/darwin go build   过
```

## mutation（3 条实跑，`md5sum -c` 逐字节还原后 `touch` 复跑绿）

- **M1** 父行不再戴面（回到「两列同一 z 平面」）→ **红**：
  `[light] the sidebar face {247 247 248} sits 0 level(s) over the window ground {247 247 248}, under 8`
  + `[dark] … sits 0 level(s) over the window ground {14 14 16}, under 8`
  + 回归钉 `[light] the content pane = {229 229 230}, want FaceOver(ground, Sunken) = {246 246 248} (18 levels apart)`。
- **M2** 选中行不再有自己的填充（与脚下的面同值）→ **红**：
  `[light] the chosen page {230 230 230} sits 0 level(s) over the sidebar face {230 230 230}, under 8: the selection stopped standing up`（深色档同一条）。
- **M3** 竖发丝线 1px → 4px → **红，但原因与规格里写的不一样**：加宽把内容推右了 3px，
  探针落到铬面上，报的是 `the sidebar face {230 230 230} and the content pane {230 230 230} are 0 level(s) apart`。
  也就是说 M3 证明的是「这条 pin 的几何探针是承重的」，**没有**证明「④ 钉的是面差不是线」。
  ④ 的那半条结论目前只有 M1 的反向证据（把面拿掉，④ 会跟着红）。**这一点如实记在这里，
  不改规格的说法去凑。** 还原用 `git checkout -- rows.go`——顺带说明：这条命令把本轮
  尚未提交的 `codeRow` 改动一起回退了，已按同一份文本重新落回并复跑
  `TestTheDeepLinkValueIsTheReferenceCodeChip` 绿。

## 出图（自己复核）

`/tmp/floter-P13-bz-settings-{dark,light}-{before,after}.png`（720×900，`PageGeneral`；
before 由 `git worktree` 在 HEAD 上跑同一份 dump 得到）。

**这张图上看到的一处变化**：dark 的 after 图里，左边 159px 是一条中灰带（39），
右边内容仍是近黑（15），那根竖发丝线（51）正好落在灰带结束的那一列上；
before 图里这条灰带和内容是同一种近黑，两者之间唯一的差别就是那根线本身。
light 图上方向相反但同样清楚：灰带 230、纸 246，线落在两者之间。

## 偷了什么（qiaomu-design / craft-loop.md 工艺密度菜单）

本轮读的是 `SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性优先 /
P-38 禁左侧竖线装饰选中态 / P-39 并列工作区满高隔离）→ `references/craft-loop.md` →
`references/preflight.md`。走的是「已有方向，只要打磨」那条路径，**没有**出四方向预览页
（方向已由用户定死：轻便液态玻璃 + 空间感）。

- **第 12 项「材质细节」→ 取了「材质分档」这一半**：`Control` 面档落成 **dark 0.10 白 /
  light 0.07 黑**，压在 47% 框上合成 **39 / 230**；面与底差 **25 / 17 级**（判据 ≥ 8）。
- **第 6 项「编辑细节 → hairline」→ 取了「线要落在接缝上」**：竖发丝的位置不动（`x=159`），
  但它的**角色**从「唯一分界」退成「材质交界上的接缝」——线自己 12 / 13 级 < 面差 24 / 18 级，
  这条不等式就是 pin 的判据 ④。
- **第 5 项「多层染色阴影」→ 明确拒绝偷**：§2 判据 2「E1 不许有外投影」，侧栏不是浮层；
  给它加影就是把「贴着」画成「浮着」，与用户要的 elevation 阶梯相反。
- **第 3 项「品牌 focus 环」/ 第 4 项「签名交互」→ 不适用**：本轮零交互改动（键盘模型、
  列表焦点环、行的 120ms 颜色过渡一个字节没动）。
- **第 1 项「氛围层（噪点/雾，<6%）」→ 明确拒绝偷**：自绘没有 blur，加噪点正是 §0 硬禁令 1
  要挡的「拿贴图冒充玻璃」。

## preflight 逐条结论

- **A 反 AI 套路**：过。无渐变、无外发光、无斜体（P13-bh 已清）、无紫蓝渐变；本轮**减少**了
  一根线的承重，没有新增装饰线。`italic`/`gradient` 在本轮 diff 里零出现。
- **B 设计读取一致性**：过。声明「功能性设置面板 · 轻便液态玻璃 · 空间感靠面差」→ 产出是
  两级 z（25 / 17 级）+ 一条落在交界上的接缝；记忆点：**灰铬 | 1px 线 | 白纸**。
- **C 动效抽查**：过（本轮零动效改动）。没有 `transition: all`、没有 ease-in、没有 scale(0)；
  行的 `Transition{Colors: true, Duration: theme.ColorStep}` 原样保留；按下/悬停三档未动。
- **D2 中文页**：**部分不过，且是本轮之前就不过的**——`tokens.Caption` 是 11px，低于
  「正文 ≥ 14px」。这一轮没有改任何文案或字号，该张力仍在 `.pending-bugs.md` 的「需拍板」
  第 1 条（第 7 份证据）。本轮新增的注释与文档全在代码/文档里，不进用户界面。
