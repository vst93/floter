# P13-ew · 滑块行的三对，与压在软边上的那行读数

轮号 `P13-ew`｜表面 = **设置页的滑块行一族**（`a.slider` + 组末的 `a.effectiveOpacity`）
合约新增 **`docs/liquid-glass.md` §6.118**｜规格 `docs/rounds/P13-ew-spec.md`

## 这轮为什么让它更轻、更有空间感

轴词（「Transparent / Solid」）此前离自己的轨道 **13-14px**、离下面那句原因只有 **10px**——
两个词读成下一段的开头，滑块整块因此读成「上面那句话的一部分」。现在它们是这一行里
**离轨道最近的一件**（11 < 13 < 15）：轨道、滑块、两端的刻度第一次是一个对象，
一句话（标题+提示）与一件面（轨道组）在距离上分开。同一张卡的末尾，那一行读数不再压在
卡片下沿的软边上（盒底离卡沿 **2.0 → 5.7px**），组与卡片之间重新有了空气。

## 证据（HEAD `4dd0e2c2`，`inkBands`，深浅两套逐字节相同）

| 探针 | 对 | 旧墨缝 | 新墨缝 |
|---|---|---|---|
| 合成卡（禁用滑块，五件齐全，640×340） | 标题 → 提示 | 8 | **6** |
| 同上 | 提示（末行）→ 轨道 | 12 | **15** |
| 同上 | 轨道 → 轴词 | **13** | **11** |
| 同上 | 轴词 → 原因句 | **10** | **13** |
| 真页面 720×2600 · Font size 行 | 标题 → 提示 | 13 | **10** |
| 真页面 · 透明度滑块 | 轨道 → 轴词 | 14 | **11** |
| 合成卡 · 读数行 | 盒底 → 卡沿 | **2.0** | **5.7** |
| 合成卡 · 一行自己 | 盒底 → 卡沿 | 5.7 | 5.7（对照臂） |

旧账里最刺眼的一条：**「能按的面」（轨道）离它的刻度 13px，而一句陈述（原因）离刻度只有
10px**——§4.3 的规矩是「间距属于它两侧那一对」，这里三对读成一个数。

## 落地

1. `internal/settingsui/settingsui.go:4780` `a.slider`：内列 `Gap(Space(1))` 拆成两个
   `Gap(a.rowLineGap())` 的内列（标题+提示 / 轨道+轴词），外列 `Gap(Space(1))` →
   `Gap(Space(2))`（「那句话→轨道」「轨道组→原因句」两对）。每个滑块行净 −1.6 … +2.0px。
2. `internal/settingsui/settingsui.go:4856` `a.effectiveOpacity`：
   `Padding(Space(1), inset)` → `Padding(Space(1), inset, Space(2.25), inset)`。
   上边仍紧（它是这一组的答案），下边补齐卡片自己的行边距。
3. 墨 / 盒 / 控件 / 分隔线 / 读数 / 行计数 / `row.apply` 的触发条件：一个字节不动。

## pin

`internal/settingsui/slidergap_test.go`

- `TestASlidersAxisWordsHugTheirTrack`：轴词→轨道 比「提示→轨道」与「轴词→原因」各小 ≥2px。
- `TestASlidersTitleAndHintShareTheRowsOwnSeam`：标题→提示的**盒**缝 = `rowLineGap()`
  （±0.5），合成卡与真页面 Font size 行两臂同时成立（用盒缝是因为墨缝会随字形降部漂 4px：
  同一个 1px 盒缝在「Font size」下读 10px、在「App transparency」下读 6px）。
- `TestTheGroupsReadoutClearsTheCardsEdge`：读数行下沿 ≥5px，且与同卡一行自己的下沿
  （5.7px）相差 ≤2px。
- opt-in `TestDumpP13EWSliderPNGs`（`FLOTER_P13EW_DIR` / `FLOTER_P13EW_TAG`）。

## 台账

- **V 0**：没有重定任何视觉数值基线（缝是新的一层，不覆盖旧值）。
- **B 0**：`internal/settingsui` 全套零改动全绿。
- **S 1**：`TestTheSliderReadoutAndReasonAreOneBranchEach`（源码文本 pin）——标题行挪进
  `Gap(a.rowLineGap())` 的内列后缩进深一级，断言**内容**不变，只改缩进，并在测试里写明
  `P13-ew S 台账`。

## 变异（4 条实跑，原始报错串）

- **M1** 轨道+轴词的内列缝回 `t.Space(1)`：
  `light: the axis words read 13px under their track and 14px above the row's own statement (+1)`
  → 红，还原。
- **M2** 外列缝回 `t.Space(1)`：
  `light: the axis words read 11px under the track, the track 11px under the hint (+0)` +
  `the axis words read 11px under their track and 10px above the row's own statement (-1)`
  → 红，还原。
- **M3** 读数行下边距回 `t.Space(1)`：
  `light: the group's readout line leaves 2.0px under itself, against the card's edge — the card's own rows leave 5.7px`
  → 红，还原（这条同时复现了「before」的 2.0px）。
- **M4** 等价写法（`a.rowLineGap()` → 字面量 `1`）→ 全绿（三条 pin 都过）。
  每条还原后 `md5sum -c` 逐字节核对（`d1bb4d94a75ebe0a818cb1724156254a`）并 `touch` 复跑。

## 门槛（实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → 全绿（0 FAIL）
- PASS **3368**（基线 3365）｜SKIP **180**（基线 179，+1 = 本轮 opt-in 出图）｜
  测试函数 **2542**（基线 2538，+4）
- `GOOS=linux/windows/darwin go build ./...` → 过

## 出图

`/tmp/p13ew/{before,after}/floter-P13-ew-slider-{light,dark}.png`（860×1800 的 General 页，
crop `(170,990)-(860,1660)`：Terminal appearance 的两个滑块 + 透明度组）。
逐像素对比：浅色 diff **49854 px**、深色 **49357 px**，bbox `(17,45)-(673,663)`。
`before` 用 `git worktree` 在 HEAD `4dd0e2c2` 上渲（源码与今天一致，只差本轮两条改动）。

**图上看到的一处变化**：透明度组那张卡的下沿往下长了 6px——读数行（「Effective opacity /
≈ 47%」）与卡沿之间第一次看得见一条与卡内其他行同宽的空气，此前它几乎贴住卡沿。

## 偷了什么（`craft-loop.md` 工艺密度菜单逐项）

- **第 6 项「编辑细节」**（hairline / 大写字距标签）→ 落成三个具体数值：
  - 标题→提示 **1.0px 盒缝**（墨 6px），与 trailing / `stackedRow` 逐像素相同（§6.117 已定）；
  - 轨道→轴词 **1.0px 盒缝**（墨 11px）——刻度属于轨道，取这一行最紧的一档；
  - 那句话→轨道 / 轨道组→原因句 **7.2px 盒缝**（`Space(2)`，墨 15 / 13px）。
  数值不是我拍的：6px 是这一族自己的读数；15 / 13px 落在 §4.3 已经量过的「一件可以按的面」
  那一档（`stackedRow` 的 9/11px 与空态的 12px 之间）。
- **第 12 项「材质细节」不取**，附理由：受光边 / 内边框高光这一轮一个字节都没碰墨与面
  （视觉轮只改缝，见红线）；「玻璃」的层级这一支已在 §6.1-§6.110 落完。
- 其余不取：第 1/5 项（氛围层、多层染色阴影）——自绘桌面应用没有 `backdrop-filter` 的等价
  物，且本轮不动面；第 2/3/4 项（`::selection`、focus 环、签名交互）属于键盘模型（红线）；
  第 7/8/10 项是展示排版动作，设置面板不适用；第 9 项（自定义滚动条）不在这张面上；
  第 11 项（图形资产）本轮没有新图标。
- 没有走 Phase 2 四方向预览（用户已把方向定死），也没有出 HTML 预览页。

## preflight 逐条

- **A（反 AI 套路）**：无渐变 / 无外发光 / 无纯黑 / 无斜体（本仓 `Text` 没有斜体 API）；
  无三等分卡片横排；文案无「革命性/颠覆/赋能」一类词，本轮没有新增任何文案；
  眉批 ≤ ⌈section/3⌉（设置页只有 section 标题，没有编号 meta-label）。
  **通过**（其中字体族、Hero 构图、Bento 等项是 web 专用，本仓无对应面，不适用）。
- **B（设计读取一致性）**：一页一个明暗主题（pin 分深浅两臂）；记忆点一句话——「刻度第一次
  属于它自己的轨道」；动效气质不变（本轮零动效改动）。**通过**。
- **C（动效抽查）**：本轮没有新增 transition / keyframes；`ContentTransition{Numeric:true}`
  与按压反馈一个字节未动；`ReduceMotion` 在探针里打开。**通过**。
- **D2（中文页）**：中西文与数字之间空格、全角标点在既有文案里成立；**「正文 ≥ 14px」不过**
  ——`tokens.Caption` 9.9 / 正文 10.8，这是 `.pending-bugs.md` 里第 71 份证据的**需拍板**项
  （改字阶 = 改高度链），本轮不擅自改。

## 仍待

1. **需拍板（沿用，未动）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2（第 71 份证据）/
   启动器两行结果行 36px 盒装 27 行墨 / §6.111 浮层动作失败句宽度上限 / §6.113 的 113-5 /
   §6.114 的 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 / 终端会话抽屉失败只加字
   不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素
   不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
2. **下一格（通用队列 · 空态满态）**：设置页与启动器的**空态**还没按 §4.3 的缝量过
   （`emptyState` 一族在 P8-b / P13-aj 量过，但「满态」——一页填满时最后一行的下沿——
   只在 §6.113 量过浮层）。先量再改。
3. 抢救 stash（第七十六次）：`2cf8436498`（2026-11-03 11:46）已判定被 HEAD 完全覆盖
   （`git diff stash@{0} HEAD -- internal/` 只剩 HEAD 多出的 `readonlyhover_test.go`，
   12 行「差异」全是 HEAD 改写过的更好版本），无需试合；其余五条仍判定与 HEAD 无未接续内容，
   本轮未试合、未静默丢弃。
