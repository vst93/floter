# P13-ey 报告 · 设置面板的「说明句」一族：一条竖线、一个寄存器

规格 `docs/rounds/P13-ey-spec.md`，合约新增 `docs/liquid-glass.md` §6.120。
产品代码 **2 处**（`internal/settingsui/rows.go` 的 `extensionHint` 与其签名、
`settingsui.go` 两个调用点），新 pin 3 条 + opt-in 出图 1 条。

## 一句话：这轮为什么让它更轻、更有空间感

一页上的两句解释不再一大一小：它们现在**同一条字号、同一条左竖线**，于是「分组标题
→ 它的说明句 → 列表」读成一条竖直的脊，而不是两组各自排一遍。层级仍由**墨**说
（说明句比标题低一档），不再靠偷偷改字号——字号是**尺度**，不是层级工具。

## 落地

`internal/settingsui/rows.go:295` 的 `extensionHint` 从两条支合成一条：

```go
ui.Row(c).FillWidth().Padding(0, t.Space(1)).Children(func() {
    ui.Text(c, hint).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextTertiary)
})
```

三个决定各有出处：

- **`tokens.Body`（10.8）而不是 caption（9.9）**：正文档是这一族里的多数寄存器
  （`sectionHint`、`cardProse`、deep-link 的 inline hint、Base plugins 那句都是它），
  Detected 是唯一例外；而且 caption 档正是 preflight D2「正文 ≥ 14px」那条**需拍板**
  争议里的最小档（第 71 份证据：9.8px 的字描边够不到自己的 token 墨）。把唯一一个
  「说明句穿 caption」的位置收上来，是把争议面**缩小**，不是扩大。
- **`t.Space(1)`（4px）**：这是 `extensionSection` 标题自己的缩进（`rows.go` 的
  `extensionSection`，标题行 `Padding(0, t.Space(1))`）。说明句的第一个字形因此落在
  它解释的那个标题的第一字形**正下方**。
- **墨不动（`tokens.TextTertiary`）**：面板的标题是 `--text-muted`，说明句低一档才成立。
  写成「全仓统一色号」会把两句都抬到和标题同墨，那是**加噪**，不是打磨。合约里因此把
  这一族的规则写成「说明句 = 标题墨低一档」，并列出两个页面的落点。

`caption bool` 参数随之消失（它只剩「把这句话画小一档」一个作用），两个调用点
（`settingsui.go:1917`、`:6170`）各少一个实参。这是本轮唯一的**结构**改动。

## 实测（760×1400 真页面，深浅各一遍）

| 量 | 改前 | 改后 |
|---|---|---|
| `BasePluginsHint` X（标题 X=190.8） | **188.8**（外凸 2.0） | **190.8**（0.0） |
| `DetectedHint` X（标题 X=190.8） | 190.8（0.0） | 190.8（0.0） |
| `BasePluginsHint` 行盒 / 行数 | 17 / 2 | 17 / 2 |
| `DetectedHint` 行盒 / 行数 | **15** / 1 | **17** / 1 |
| 两句的墨重（浅 / 深） | 104 / 95 | 104 / 95（未动） |
| 设置卡片页 `ShortcutsAppHint` 行盒 | 17 | 17 |

`DetectedHint` 在 760 宽下**没有回绕**（改后 1 行，盒高 15 → 17），所以面板只长 2px，
列表与它的点击目标一个像素都没挪。

## 出图

`/tmp/p13ey/{before,after}/floter-P13-ey-integrations-{light,dark}.png`
（`before` 用 `git worktree` 在 HEAD `75e90b18` 渲，同一个测试文件拷过去跑）：

| 帧 | diff | bbox |
|---|---|---|
| light | **24433 px** | `(187,195)-(743,815)` |
| dark | **24391 px** | `(187,195)-(743,815)` |

bbox 的上沿 `y=195` 正是 Base plugins 那句所在的带；下沿 `y=815` 是 Detected 那句变高
2px 之后它下面整段跟着下移 2px 的范围。**这张图上看到的一处变化**：Detected 那句
「Programs floter can see but nothing has connected yet.」与它上面 Base plugins 那句
现在同一个字号，且两句的左边缘与各自标题的第一个字对齐成一条竖线。

## V / B / S 台账

**V 0 / B 0 / S 1**。S = `extensionHint` 的签名（去掉 `caption bool`）与两个调用点，
断言内容不变；`internal/settingsui` 全套零改动全绿（含
`TestBasePluginHintRidesAboveTheList`、`TestDetectedHintRidesAboveTheList`、
`TestExtensionSectionTitleWearsTheReferenceRegister`）。

## mutation（实跑，`md5sum -c` 逐字节还原 + `touch` 复跑）

- **M1** 缩进 `t.Space(1)` → `t.Space(0.5)`：
  `light: the base explanation starts -2.0px off its title's line (hint 188.8, title 190.8)`
  / `light: the detected explanation starts -2.0px off its title's line …` —— 第①条红，
  深浅各两行。
- **M2** 字号 `tokens.Body` → `tokens.Caption`：
  `light: the detected explanation's line box is 15.0 — still the caption step the round removed`
  / `the panel's explanation (15.0) and the cards' own group hint (17.0) are -2.0px apart:
  two registers again` —— 第②③条红。
- **M3** 等价改写 `tokens.Body` → `a.tokens(c).Body`：**全绿**（证明三条 pin 钉的是读出来的
  行盒与位置，不是某个写法）。
- 还原：`md5sum -c` 逐字节对上 `11fdeeef25958268606195faea653304`，`touch` 后整包复跑
  `ok … 243.339s`。

## 用 skill 的账（`qiaomu-design`）

开工前读了 `SKILL.md` → `references/user-preferences.md`（335 行）→ `craft-loop.md` →
`chinese-typography.md` → `preflight.md`；按本轮表面（一段中文/西文混排的说明句）只读
这五份，没有全库通读。

**偷了什么**（逐条落成具体数值）：

1. **`craft-loop.md` 工艺密度菜单第 6 项「编辑细节 · 大写字距标签 / hairline」的同一条原则
   ——「同一角色的元素必须落在同一条基线/竖线上」**：落成 `t.Space(1)` = 4px 的左缩进，
   让说明句的第一个字形与分组标题的第一个字形共用一条竖线（改前 188.8 vs 190.8）。
2. **`chinese-typography.md` 三·排版参数「层级主要靠字号 + 颜色，字重只做辅助」的反向读法**
   —— 既然层级靠**颜色**，就不该用字号去表达层级：说明句与标题的差由墨（tertiary vs
   muted）承担，字号回到同一档。落成 `tokens.Body` + 保留 `TextTertiary`。
3. **`craft-loop.md` 三·两轮制**：第一轮改完出 4 张帧 + 四视角评审（见下），第二轮只做
   一件事——确认 Detected 那句**没有回绕**（1 行 17px，不是 2 行 34px），因为一旦回绕，
   面板会长 17px，那是布局改动而不是墨的改动。

**四视角评审**（各挑一处）：

- **艺术总监**：这一族此前最弱的是 Detected 那句——它比旁边那句小一档，读起来像脚注，
  而它其实是「这些程序我看得见但没人连」这条**结论**。收成同档后，它才配得上它的标题。
- **工程师**：`caption` 布尔值去掉后，`extensionHint` 只有一个行为；两个调用点少一个实参，
  编译期就不会再出现「第三个实参忘了传 true」这类分叉。
- **甲方**：三秒扫过 Integrations 页，现在两组是「标题 / 一句解释 / 列表」同一个节奏，
  不用在第二组重新校准字号。
- **读者**：眯眼看，两句的左边缘与标题对齐成一条脊；此前 Base plugins 那句比标题左探
  2px，眼睛会在那一列上「磕」一下。

## preflight 逐条结论

- **A 反套路**：无渐变白边、无外发光、无模糊贴图；本轮只动字号档与左缩进。**过**。
- **A2 进攻三步**：DNA/工艺密度见上「偷了什么」3 条；两轮制已走。**过**。
- **B 设计读取一致性**：一句话记忆点 =「说明句是标题的下一行，不是下一个字号」；明暗
  两套各跑一遍，`tokens.Body` / `TextTertiary` 都读当前主题，没有混主题。**过**。
- **C 动效抽查**：本轮零动效（没有 `transition`、没有入场）。**不适用**（不是静默跳过：
  这一族本来就没有动画）。
- **D2 中文页**：本轮不新增任何可见文案；中文串 `DetectedHint`
  （「Floter 能看到但还没连接的程序。连接时会走和其他工具一样的权限确认。」）随字号
  10.8 一起变大，行高仍由框架的 1.5 倍给出（17/10.8 ≈ 1.57）。**中西文空格**：这条串是
  i18n 既有文案（「Floter 能看到…」已带空格），本轮不改文案。**字号 ≥ 14px 仍不过**——
  沿用需拍板项（`tokens.Caption` 9.9 / 正文 10.8），本轮不动，也不假装过。

## 仍待 / 下一格

1. **下一格（通用队列 · 文案密度，第二条）**：设置页的**读数句**（`a.effectiveOpacity`
   那类「当前 47%」的答案行）与**行内提示句**（`row.description`）是否同一档密度——
   滑块行那族已在 §6.118 量过缝，但没量过**字号档**；先量再改。
2. **需拍板（沿用，未动）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2（第 71 份
   证据）/ 启动器两行结果行 36px 盒装 27 行墨 / §6.111 / §6.113-5 / §6.114-4 / `104-2` /
   便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. **本轮证伪的一条**：没有把两句的**墨**也统一到同一个色号。理由不是「忘了」——面板的
   标题是 `--text-muted`、设置卡片的标题是 `--text-strong`，说明句的墨必须是「标题墨低
   一档」；统一色号会让面板里说明句与标题同墨，层级反而塌掉。合约 §6.120 把这条写成规则，
   而不是留一个待办。
