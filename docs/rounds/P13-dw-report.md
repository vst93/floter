# P13-dw 报告：同一张卡的两张列表，两半此前各跑一把尺子（混排收口）

方向一句话：**两条「两处相等」的像素 pin 一直是绿的，但两处各画在一把不同的尺子上——本轮把
两半装到同一面调色板的出货帧上，并把「相等」锚到参考算术与字阶上。**

**为什么让它更轻/更有空间感**：这两条 pin 管的是同一张卡的两张列表的**横向节奏**
（16u 图标列 + 28u + 10u gap = 54u）与**标题字阶**。它们此前量的是库存尺的 54.0 / 22.0，
而 app 真画的是 48.6 / 17.0 —— 也就是说，卡片里那条文字列与标题档在测试里从来没有被这样量过；
量准了，才能保证两种列表在**同一列**上起笔、小节标题落在**同一档**上。

## 先量（同一台建机，改前 / 改后）

| 读数 | 库存尺（pin 今天量的帧） | 出货尺（app 真会画的帧） |
|---|---|---|
| 帧 | 列表 `720×817`（`WindowHeight("small")+500`）/ 输出 `720×488` | **`720×439`**（`themedFrameHeight(maxResultSlots)`） |
| 列表行标题 X / 行盒高 | `54.0000` / `22.0000` | **`48.6000`** / **`19.0000`** |
| 输出行标题 X / 行盒高 | `54.0000` / `22.0000` | **`48.6000`** / **`17.0000`** |
| 列表小节标题 X / 行盒高 | `54.0000` / `22.0000` | **`48.6000`** / **`17.0000`** |
| 输出分组标题 X / 宽 / 行盒高 | `54.0000` / `33.0000` / `22.0000` | **`48.6000`** / **`25.0000`** / **`17.0000`** |
| 输出行盒宽 | `650.0` | **`656.9999`** |

**假绿的成因**：§6.86 里那条红串（`list title x=54.0, output panel's at 48.6`、
`heading is 17 tall, launcher's 22`）是**两半不同尺**时才会现形的差；等两条 pin 各自稳定在
一把尺上（一条全库存、一条全…）之后，它们又都绿了 —— 只是绿在一个 app 不会画出来的帧上。

## 落地

1. `internal/launcher/textcolumn_test.go:33`（列表半）：裸 `ui.NewTester(…, WindowHeight("small")+200)`
   → `renderShipped(t, a, false)`（出货尺：`c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight`）。
   原注释「a taller window so the block is not clipped」是库存尺帧高不足的产物，出货尺装得下。
2. `textcolumn_test.go:43`（输出半）：`runListOutput` → `listOutputShipped(t, b, rows, false)`。
3. `textcolumn_test.go:57` **新增参考算术锚**：`tenths(listTitle.X) == tenths(54 × designUnit())`
   —— 「两张相等」看不出「一起漂走」，这条锚把列钉回 `16u + 28u + 10u = 54u`。
4. `textcolumn_test.go:80/92`（opt-in dump）：两半同换出货尺，出图与 pin 量的必须是同一帧。
5. `internal/launcher/outputheading_test.go:22`（列表半）：裸 `ui.NewTester(… +500)` → `renderShipped`。
6. `outputheading_test.go:32`（输出半）：`runListOutput` → `listOutputShipped(t, b, stdout, false)`。
7. `outputheading_test.go:44` **新增字阶锚**：分组标题行盒高 == `designMetrics()` 的 `SubtitleLine`（17）。
   「两处相等」之外再问一句「落在参考命名的哪一档上」。
8. `internal/launcher/headingcolumn_test.go:94`：`sameColumn` 的 `±0.5` 容差带 →
   `tenths(a) == tenths(b)`（一次收紧 4 条同族 pin：`headingcolumn_test.go:43/57/74`、
   `textcolumn_test.go:47/56`）。

产品代码**零改动**（`git status` 只有 `internal/launcher/*_test.go` 与 `docs/`）。

## 台账

**V 6 / B 0 / S 0**，测试函数 2492 → **2492**（只加强既有断言，不新增夹具）：

- V：两处取景改为出货尺（读数 `54.0 → 48.6`、`22 → 17`）；`sameColumn` 收紧影响的 6 条比较
  （其中 2 条是 `±0.5` 带内的宽判据，4 条是同一比较器的其他调用点）。
- B 0：两条 pin 断言的仍是「两张列表/两个标题同列同档」，语义零改动；红了两处都是取景，
  不是产品回归（M1/M2 实测红串即 §6.86 的历史红串）。
- S 0：没有结构变更。

## 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | **空** |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | **净** |
| `GOOS=linux/windows/darwin go build ./...` | **全过** |
| `go test -count=1 ./...` | **全绿** |
| PASS / FAIL / SKIP | **3325 / 0 / 159**（基线 3325 / 0 / 159） |
| 测试函数 | **2492**（基线 2492） |

## Mutation（3 条实跑，红 → 还原 → md5 逐字节 → touch 复跑）

- **M1 列表半退回库存尺**（`renderShipped` → `render`）：
  `textcolumn_test.go:46: the launcher list's title starts at x=54.0, the output panel's at 48.6:
  two lists in one card share one text column`（另有 :55、:64 两条同帧红）。
- **M2 输出半退回库存尺**（`listOutputShipped` → `runListOutput`）：
  `outputheading_test.go:41: the output group heading is 22 tall, the launcher's section title is 17`
  （历史红串的镜像：这次错的是输出那一半）；字阶锚同时红（`22` 对 `SubtitleLine` 17）。
- **M3 产品行 gap `2.5u → 2.6u`**（列移 0.36px）：
  新判据红 —— `the launcher list's title starts at x=48.6, the output panel's at 49.0`；
  把 `sameColumn` 换回 `±0.5` 后**同一条缺陷转绿** —— 这就是容差带会吞掉 0.4px 的实证。

三条都还原后 `md5sum` 逐字节核对（`9faeab7d…` / `345e5ba5…` / `0dd01290…`），
`touch` 后四条同族 pin 重跑全绿（不吃测试缓存）。

## 出图（肉眼复核）

`FLOTER_PNG=1` 跑 `TestDumpP11ADOutputPanelPNGs`：
`/tmp/floter-P11-ad-{list,output}-{light,dark}.png`，四张全 **720×439**（此前列表那两张是 817 高）。
逐像素找每张图里「文字带的最左墨列」：

| 图 | 文字带（y 区间） | 最左墨列 |
|---|---|---|
| `list-light` | 92..102（行标题） | **49** |
| `list-dark` | 92..102 | **49** |
| `output-light` | 140..149（首行） | **49** |
| `output-dark` | 139..149 | **49** |

**这张图上看到的一处变化**：列表那张图从 817 高变成 439 高（帧换成 app 真画的那一帧），
而两张图的文字带起点从「列表 54 / 输出 49」变成**同为设备列 49** —— `round(48.6)`。

## skill 用法（qiaomu-design，打磨车道）

读了 `SKILL.md` → `references/user-preferences.md`（P-5 可读性优先、P-38 无左侧竖线、
P-14 无常驻系统自言自语）→ `craft-loop.md` / `preflight.md`。方向已定（轻便液态玻璃），
走「已有方向，只要打磨」车道，不触发 Phase 2 预览、不出 HTML。

**偷了什么（工艺密度菜单，逐条落数）**：

- 第 6 项「编辑细节 / 精确几何」：把「同一列」写成可复核的算术 `54 × designUnit() = 48.6`，
  并落到 painter 的十分位（`tenths`），删掉 `±0.5` 的模糊带 —— 这是本轮的主改动。
- 第 9 项「一致性检查」：同一张卡的两张列表必须**同尺同列同档**；本轮把「同尺」这一步
  显式钉住（`renderShipped` / `listOutputShipped`），此前只是「同列」的断言在一张混排帧上通过。
- 第 12 项「材质细节」的克制：本轮不新增任何墨、边、影 —— 只把已有的两个数（列、字阶）量准。

**preflight 逐条**：

- **A 反套路**：无渐变 / 发光 / 斜体 / 眉批式小标题；本轮改动全在夹具。
- **B 设计读取一致性**：设计读取一句话 =「启动器的两种列表，同一张卡、同一条文字列（54u）、
  同一档小节标题（副标题档 17），用轻便液态玻璃的面差表达层级」；两条 pin 现在都在这句话上。
- **C 动效抽查**：无动效改动；dump 里深色那一臂仍走 `SetDark + Advance(500ms)` 的既有路径。
- **D2 中文页**：**仍未过** —— `tokens.Caption` 9.9px / 正文 10.8px 对 preflight「正文 ≥ 14px」。
  第 **48** 份证据，等用户拍板（不擅自改字号）。

## 仍待（P13-dw）

1. **下一格（首选，独立一轮，产品墨）**：`P13-dg-l-4` —— `outputsubtitle` 选中行副标题
   浅 119 / 深 112，对启动器选中行 125 / 118，差 6 级；两支要收到同一条公式上
   （`TextMuted` over 选中面带 → 第二条面档？先量再定）。
2. 其后：`dg-l` 最后一格 = **阶梯（V）1 条**（`rungs [15 18 22 24 25 28]` 实画 `[15 17 19 25 25.2]`），
   与 §6.86 的「键帽盒 `7 × 3.6`」同族。
3. 其后：`P13-dg-t` / `P13-dg-s` 两个画面包的夹具取景收口，每包一格。
4. **需拍板（沿用 + 第 48 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 对 D2「≥ 14px」。
5. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
6. 抢救 stash 核对（第五十次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
