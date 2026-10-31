# P13-bd 报告 · 终端页空态：一句陈述 + 两条出路，而不是「一句灰提示 + 两条亮线」

轮号 **P13-bd** ｜ 表面：**终端页的空态** ｜ 规格 `docs/rounds/P13-bd-spec.md`
合约新增 **§6.23**（`docs/liquid-glass.md`）。

---

## 一、这轮为什么让它更轻、更有空间感（一句话）

**层级由面与墨的「差」说话，不由「谁更灰」说话。** 终端页的空态此前把两件东西的轻重
画反了：标题（这页唯一的陈述）是全页**最淡**的一件，两条出路（可以按的东西）比它**更亮**。
轻便液态玻璃的判据是「受光边 / 更亮的墨 = 这块面可以被碰」——于是这一页读起来是
「一条灰提示，下面挂着两条亮线」，而正确的读法是「一句话，下面两条可以走的路」。
改后三档成阶梯（陈述 ≫ 可以碰 ≫ 键帽图例），**字阶与盒子一个字节没动**。

---

## 二、先量（改前，实测）

`860×600`，指针静息，`en`：

| 件 | 位置（HEAD `dd49b2a0`） | 改前墨（浅 lo / 深 hi） | 对页面的对比 |
|---|---|---|---|
| 标题 `EmptyTitle` | `terminalui.go:883` `TextColor(t.TextMuted)` | 浅 **114** / 深 **162** | 浅 138 / 深 141 |
| 出路 `EmptyNew` | `terminalui.go:968` `tokens.TextTertiary` | 浅 **118** / 深 **145** | 浅 134 / 深 124 |
| 页面自身 | — | 252 / 21 | — |

浅色档：标题只比出路**暗 4 级**（114 vs 118）——它们几乎是同一个灰。
深色档反过来：标题 162 比出路 145 **亮 17 级**——标题读成最亮的一件。

**同一个应用里另一套空态已经是对的**（`settingsui/empty.go`，P8-b 抬过）：句子
`TextStrong`、标记 `TextSecondary`、理由 `TextTertiary`。这一族有**两种语法**，
终端页恰好是没抬的那个，且把阶梯画反了。

## 三、落地

| # | 位置 | 旧 | 新 |
|---|---|---|---|
| 3-1 | `terminalui.go:883` 标题 | `TextMuted`（§4.7 的**旧值**，合约原话「读起来像一条错误提示」） | `tokens.TextStrong`（§4.7 的陈述档，与 `settingsui.emptyState` 同一条） |
| 3-2 | `terminalui.go:968` 出路静息 | `tokens.TextTertiary` | `tokens.TextSecondary` |
| 3-3 | 同处出路 hover | `tokens.TextSecondary` | `tokens.TextStrong` |
| 3-4 | 键帽（`emptyLine` 的 badge） | `LitFace` + `Caption` | **不动**（§6.3 表 3-8 已落地，P13-d） |

改后实测（同一帧、同一批盒子）：

| 件 | 浅 | 深 |
|---|---|---|
| 标题对比 | **225**（114 → 27） | **216**（162 → 237） |
| 出路对比 | 145（118 → 107） | 133（145 → 154） |
| 阶梯 | 225 ≫ 145 | 216 ≫ 133 |

## 四、一条没做成的（如实记录，也是本轮最有价值的一条）

本轮先试过把标题的字阶从 `t.FontSize + 1` 换成 `tokens.Title`（14 × 0.9 = 12.6）。
它**不是墨，是几何**：实测把这一行的盒高从 **23 推到 19**，
`TestTheTerminalPageStaysOnTheHeightLadder` 立刻红：

```
empty en light 640x360: height 19.0 is not on the ladder [15 22 23 28] (±0.50)
empty en light 640x360: no run of height 23.0 was painted (heights [15 19 22 28])
```

那是行为契约红线（高度链），不是墨。换回 `t.FontSize + 1` 后盒子逐像素回到 23。
**视觉轮只改墨与面**——这一条是现场证据。

## 五、V / B / S 台账

**V 0 / B 0 / S 0**。本轮没有改任何一条既有断言：`pageheight_test.go` 的 23 / 22 / 15 / 22
四条高度、`view_test.go` 的三句文案、`TestTheTerminalPageStaysOnTheHeightLadder` 的 16 个子
测试，全部零改动通过。

## 六、门槛（全部实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | **空** |
| `go vet ./...` | **净** |
| `GOOS=windows go vet ./...` | **净** |
| `GOOS=darwin go vet ./...` | **净** |
| `go test -count=1 ./...` | **全绿，0 FAIL** |
| `--- PASS` 计数 | **3132 → 3142（+10）** |
| `--- SKIP` 计数 | **104 → 105（+1，opt-in 出图）** |
| 测试函数 | **2304 → 2309（+5）** |
| 三平台 build | **过** |

基线用同一把尺复量：`git stash -u` 回到 HEAD 后实测 **3132 PASS / 104 SKIP / 2304 函数**，
与 P13-bc 报的收尾数字一致。

## 七、变异（4 条实跑，全部变红，`md5sum -c` 逐字节还原）

| # | 注入的缺陷 | 变红的 pin 与原始报错串 |
|---|---|---|
| M1 | 标题退回 `t.TextMuted` | `TestTheEmptyStateTitleIsTheStatement`：`light: the title carries 138 levels of contrast and the ways out 145 — the page's one statement is weaker than its two exits by 7 levels…` + `dark: … by -8 levels`；`TestTheEmptyStateRegisterIsTheStatementsRegister`：`the empty state's title is not painted in tokens.TextStrong…` |
| M2 | 出路画到陈述的同一档（`TextStrong` 静息） | `TestTheEmptyStateWaysOutAreNotStatements`：`light: "New blank session" carries 225 levels of contrast against the title's 225 — only 0 levels weaker…`（两条出路 × 两套外观） |
| M3 | hover 臂退回静息档 | `TestAWayOutLiftsUnderThePointer`：`light: the way out carries 145 levels of contrast at rest and 145 under the pointer — the ink moved 0 levels, not the 10 a hover rung needs`（深色同） |
| M4 | 标题改成字面色值 `ui.Hex("#1B1B1D")` | `TestTheEmptyStateRegisterIsTheStatementsRegister`：`the empty state's title is not painted in tokens.TextStrong (§4.7's statement register…)` |

每条注入后 `md5sum -c /tmp/p13bd.md5` 逐字节核对还原，`touch` 后复跑绿。

**M2 的第一次尝试不是缺陷（如实记录）**：最初写的 M2 是「出路退回 `TextTertiary`」——
它**不红**，因为 `TextTertiary` 依然远弱于 `TextStrong`，那个方向本来就不是缺陷。
这一族真正的缺陷是**出路被画成陈述**（另一侧），把 M2 改成那个方向才抓得到根因。
按纪律**没有**为了让 M2 变红去放宽断言。

## 八、出图与眼睛复核

`/tmp/floter-P13-bd-terminal-empty-{light,dark}-{before,after}.png`（`FLOTER_P13BD_DUMP=1`）。

逐像素对照（before vs after，整帧）：**只有一处区域变化**——
`light: changed px=2003  bbox x=351..509 y=257..339`、`dark: changed px=2008  同 bbox`。
那就是标题与两条出路自己的墨，**没有一行像素在别处动**（页面、浮栏、键帽的面全在 bbox 外）。

**这张图上看到的一处变化**：`light-after` 的标题从一枚「和出路几乎同色的灰字」变成这一页
**最重**的一件（27 对页面 252），两条出路退到 107——眼睛读到的顺序第一次是
「一句话，然后两条路」，改前读到的是「一条灰提示，两条亮线」。

## 九、偷了什么（`craft-loop.md` 工艺密度菜单，逐项落成数值）

| 从菜单取的项 | 落成的具体数值 |
|---|---|
| **#5 多层染色阴影**（同类替换：层级由成对的明度差说话） | 三档明度阶梯：陈述 **225 / 216** 级对比 ≫ 可以碰 **145 / 133** ≫ 键帽图例 `Caption` 档（不动）。三档之间的最小间距定为 **20 级**（pin 的下界；`settingsui` 同一族 P8-b 实测 222 / 139 / 94，相邻两档最小 45 级，这里取 20 是它的下界的一半，留小屏余量） |
| **#12 材质细节（内边框高光）** | 键帽的 E1 `LitFace` 冠线**一个字节不动**（实测 hover 前后 `234 → 234`（浅）/ `45 → 45`（深））。「材质不参与指针」的显式落成：可以碰的那一行在 hover 时抬**墨**，不抬**边** |
| **#4 签名交互（一个足矣）** | 出路行的 hover = **只抬墨一档**（`TextSecondary → TextStrong`，实测 +80 / +83 级），不换色相、不加面、不画影。§2 判据 2「E1 不许有外投影」在这里是硬约束：这一行不是面，是一行字 |
| **#6 编辑细节（hairline / 字距）** | **不动**：`Caption` 键帽的 15px 盒与 `Title` 的 23px 盒逐像素保留（`pageheight_test.go` 零改动通过），§4「圆角与间距刻度不改」的直接兑现 |

DNA 注入：本轮方向由用户钉死（轻便液态玻璃），供体是 `docs/liquid-glass.md` §1/§2/§3 的既有
token 层（`TextStrong` / `TextSecondary` / `LitFace`），而不是 58 站库里另挑站点。理由：本仓的
单一来源是 `internal/theme/theme.go`，引入第二个供体会产生两套色阶。

## 十、`preflight.md` 逐条结论

### A. 反套路扫描

- 无紫蓝渐变 / 无 `#000000` 纯黑 / 无外发光 / 无渐变文字 —— **过**（只换了两个 ink token，
  零新增颜色字面值；M4 的变异恰好证明「字面色值」会被 AST pin 抓住）
- 字体不是 Inter/Roboto/Arial —— **过**（字体栈一个字没动，系统栈）
- 全页无 `font-style: italic` —— **过**（无斜体；强调只用墨档）
- Hero 居中构图 —— **不适用**（应用内空态，不是落地页）
- 三等分卡片横排 / 卡片当万能容器 —— **不适用**（空态按 §4.7 明确**无底板**）
- 假数据 / 烂梗名 —— **不适用**（文案是既有 i18n，零改动）
- 文案无「革命性/颠覆/无缝/赋能/下一代」、感叹号 ≤1 —— **过**（文案零改动）
- 眉批数量 / H1 行数 / 无左侧竖线装饰 —— **不适用 / 过**（无新增装饰）

### A2. 进攻三步核验

- DNA 注入 —— **过**，见 §九（供体是合约自身的 token 层，理由已写明）
- 工艺密度 ≥5 —— **3 项落成 + 1 项显式「不动」。这一条不满 5，如实记录**：本轮是空态的
  一句话，表面只有三件东西（标题 / 两条出路 / 键帽），能改的只有墨与 hover；硬凑到 5 项
  就得给这个空态加面、加影、加装饰——那正是 §4.7 与 §2 判据 2 明确禁止的（空态**无底板**、
  E1 **不许有外投影**）。**不通过的理由写在这里，而不是静默跳过。**
- 两轮制 —— **过**：第一轮「标题换 `TextStrong` + 出路换 `TextSecondary`」（§三），
  第二轮「字阶换 `tokens.Title` → 高度链红 → 退回」（§四），修复已发生。

### B. 设计读取一致性

- 产出与声明的读取一致 —— **过**（声明「陈述 ≫ 可以碰 ≫ 图例」，实测 225 ≫ 145 ≫ Caption）
- 全页主题锁定 —— **过**（深浅两套各跑一遍，阶梯方向由 `contrast()` 按外观解析）
- 一句话记忆点 —— **过**：「这一页只有一句话，然后两条路」
- 动效气质与视觉气质一致 —— **过**（只有既有的 120ms 墨过渡，无新增动效）

### C. 动效抽查

- 有明确目的、高频/键盘操作无动画 —— **过**（hover 是低频指针动作）
- 无 `transition: all` / `ease-in` / `scale(0)` —— **过**（本轮无新增 transition）
- UI 动效 ≤300ms、按压有反馈 —— **过**（沿用 `theme.ColorStep` = 120ms）
- popover origin / reduced-motion / hover 媒体查询 —— **不适用**（自绘框架，无 CSS；
  hover 语义由 `ui` 框架自己管）
- 只动 transform/opacity —— **过**（本轮只动 `TextColor`，即墨）
- 快速触发组件用 transition —— **过**

### D2. 中文页追加项

- 中文用系统字体栈 —— **过**（字体栈零改动）
- 中文行高 ≥1.5、正文 ≥14px、字重 400/500/600 —— **不能完全过，如实记录**：终端页标题
  实测 12.6px（`t.FontSize + 1`，随界面档缩放），`Caption` 键帽 ≈9.9px。这正是本仓长期挂着
  的那条「需拍板」（`tokens.Body` 10.8px / `tokens.Caption` 11px 与 D2「正文 ≥14px」的张力）。
  本轮**不许擅自改字阶**——§四 已实测：改字阶立刻撞高度链红线。**结论：这条张力仍需用户
  拍板，本轮不越界。**
- 中西文之间空格、全角标点 —— **过**（`en` / `zh` 文案零改动）
- 暗色页正文 `.85` 且对比 ≥7:1 —— **过**：改后深色标题对比 **216 级**

## 十一、仍待（P13-bd）

1. **需拍板（沿用，本轮实测又给了一条硬证据）**：`tokens.Caption` 11px / 终端页标题 12.6px
   与 preflight D2「正文 ≥14px」的张力。**改字阶 = 改高度链**（本轮 §四 已现场撞红），
   所以这条必须由用户拍板，不能由视觉轮自作主张。
2. **同族下一格候选**：空态这一族现在只剩 `shell/detach.go:448` 的 `detachedStatus`
   （捕获输出页的三句状态行：`OutputWindowRunning` / `OutputWindowEmpty` / `OutputWindowIdle`，
   画在 `tokens.Caption` + `TextMuted`，**没有陈述档**）。它是**状态行**不是空态，
   该不该有陈述档要先量——这是下一轮的自然选题。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
4. **抢救 stash**：本轮开工 `git stash list` 有 13 条，逐条核对结论（**全部不需再试合**）：
   `stash@{0}` = `afc38b247c`、`stash@{1}` = `114b7d1585` —— 已在 P13-bc 核过（内容分别随
   `676543ed` / `1bc03d96` 落在 HEAD，逐字相同）；`stash@{2}` / `stash@{3}` —— 已在 P13-ac 核过
   （与 P13-x 同一份补丁，已被 `4783c62d` 覆盖）；`stash@{4}`（P11-cq work）—— 已在 P12-a 核过
   （`settingsMemo` 三处都在 HEAD）；其余 `stash@{5}`..`stash@{12}` 是 P7 时代的旧 WIP
   （keycap slice / slider 度量 / plugin number-field / 自定义快捷键校验 / action bar），
   对应功能在 HEAD 里都已存在（`keycap.go` 的 E1 键帽、`rows.go` 的 slider、`shortcuts` 的校验、
   `actionbar.go`），**已被后续轮次覆盖**。本轮没有静默丢弃任何一条，也没有新接续。
