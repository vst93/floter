# P13-cr 报告 · 命令板的第三行墨：一个乘法在两套外观上落在两个梯级

**这轮为什么让它更轻、更有空间感**：扩展页命令板上一块板五行字，此前只有第三行
（`Summon with: …`）**不是阶梯给的**——它是 `TextMuted` 再乘一把 0.85（参考 CSS 的
`__triggers { opacity: .85 }` 叠在 `span { color: var(--text-muted) }` 上，两个旋钮一道算）。
同一个乘法在两套外观上落在两个位置：**浅色恰好撞上阶梯的第三档**（141 对 142，1 级——白算），
**深色落在两档之间的无人区**（122，比 `TextMuted` 133 低 11、比 `TextTertiary` 110 高 12）。
一块板上于是有两种「第三档」：浅色的读起来是「比描述轻一点点（1 级，看不见）」，
深色的读起来是「和别的事实不一样，但不是任何一档」。交给阶梯之后，这块板在**两套外观上
都是三级**：名字 → 事实 → 召唤词，每一级都是 token 表上的一格，不是一次算术。

## 1. 证据（先量后改，产品代码未动时读到的原始像素）

`renderIn` + `actionRungProbe`（`720×900`，扩展页命令列表，`commandListIntegration`），
静息帧：

| 外观 | 板面 | 名字 `TextSecondary` | 描述 `TextMuted` | Summon（改前） | `TextMuted×0.85` over 板 | `TextTertiary` over 板 |
|---|---|---|---|---|---|---|
| 浅 | `{213,213,214}` | `{107,107,112}` | `{128,128,135}` | `{141,141,147}` | `{141,141,147}` | `{142,142,148}` |
| 深 | `{61,61,63}` | `{154,154,160}` | `{133,133,141}` | `{122,122,129}` | `{122,122,129}` | `{110,110,118}` |

读出来的三件事：① 浅色的乘法与 `TextTertiary` 只差 **1 级**——那一档在这个外观上本来就是
阶梯的第三档，乘法绕了一圈回到原处；② 深色的同一个乘法落在 **122**，两档都不是；
③ `settingsui.go:3313`（描述）、`:3348`（别名 label）、`:3362`（taken 注）**三行同族事实**
都是 `TextMuted` 一档到底，整块板只有这一行多乘了一把 0.85——它不是层级，是一次算术。

## 2. 改法（只改一个墨 token）

`internal/settingsui/settingsui.go` `commandIdentity`：

```
-	ui.Text(c, copy.IntegrationsCommandsTriggers(strings.Join(words, ", "))).
-		FontSize(tokens.Caption).TextColor(t.TextMuted).Opacity(0.85).
-		SingleLine().Ellipsis("\u2026")
+	ui.Text(c, copy.IntegrationsCommandsTriggers(strings.Join(words, ", "))).
+		FontSize(tokens.Caption).TextColor(tokens.TextTertiary).
+		SingleLine().Ellipsis("\u2026")
```

- **为什么不删成 `TextMuted`**：那会让这一行与它上面的描述**逐像素相同**（实测浅色
  `{128,128,135}` / `{128,128,135}`，0 级）——两块事实合并成一句，板上少说一句话。
- **为什么不跳回 `TextSecondary`**：那是名字的档，实测深色会与名字**同值**（97 / 97），
  阶梯倒挂（详见变异 M3）。
- **为什么是 `TextTertiary`**：它是阶梯自己的下一格，且在浅色上**几乎不动**（141 → 142）——
  这一轮在浅色上是「把一次算术换成一次查表」，在深色上才是那 12 级。
- 排版盒、字阶（`tokens.Caption`，全行一致）、`SingleLine`/`Ellipsis`、可点区域、
  `switchControl` 的 `Label()`、`SetCommandEnabled` 接线**一个字节没动**。

## 3. 台账

**V 0 / B 0 / S 0**

- **V 0**：这一行此前没有任何 pin 量它的墨（`commandlist_test.go` 量的是板面与开关通断）。
  本轮是**新增** pin，不是改旧断言。
- **B 0**：`commandlist_test.go` 三条、`TestOnlyTheCommandSwitchTogglesTheCommand`、
  别名编辑器与 debounce 全部零改动通过。
- **S 0**：结构没变（同一个 `if`、同一个 `ui.Text`）。
- 合约里登记为 `docs/liquid-glass.md` §6.61（差距表新增一行，状态 `已落地`）。

## 4. 门槛（实测）

```
gofmt -l cmd internal                     空
go vet ./...                              净
GOOS=windows go vet ./...                 净
GOOS=darwin  go vet ./...                 净
go test -count=1 ./...                    全绿（exit 0，0 FAIL）
PASS 3286（基线 3283）· FAIL 0 · SKIP 136（基线 135，新增一条 opt-in 出图）
测试函数 2432（基线 2430，+2：一条 pin 一条 dump）
GOOS=linux/windows/darwin go build ./...   全过
```

## 5. 变异（三条实跑，全部变红并逐字节还原）

| # | 注入 | 结果（原始报错串） |
|---|---|---|
| M1 | 把 `.Opacity(0.85)` 放回去 | 深色红：`the summon line's ink is {122 122 129}, want the TextTertiary rung over its plate ({110 110 118})`；**浅色不跑也知道绿**——这正是本轮量到的「乘法只在深色上错位」 |
| M2 | 只删乘法、不换档（`TextMuted`） | 浅色三红：`{128 128 135}, want … ({142 142 148})` / `the summon line {128 128 135} and the description {128 128 135} stand 0 levels apart: the two facts collapsed onto one rung` / `the plate's inks run 106 (name) / 85 (description) / 85 (summon) off their plate: the ladder is out of order` |
| M3 | 跳两档到 `TextSecondary` | 深色红：`{154 154 160}, want … ({110 110 118})` / `the plate's inks run 97 (name) / 78 (description) / 97 (summon) off their plate: the ladder is out of order` |

每条之后 `cp` 回原文件 + `md5sum -c`（`89ab84b2deeecab125aee98410ffaf93`）逐字节核对，
最后 `touch` 两个文件再复跑一次整条 pin（绿，`166.5s`）。

## 6. 出图（深 / 浅，改前 / 改后）

`/tmp/floter-P13-cr-commands-<light|dark>-<before|after>.png`（720×900，真扩展页命令列表）。
逐像素差分（>8 级才算动）：

| 外观 | 差分 px | bbox |
|---|---|---|
| 浅 | **0** | —（142 与 141 的差落在抗锯齿噪声里） |
| 深 | **721** | `(201,514)-(373,652)` |

**眼睛复核的一处**：`...-dark-after.png` 上 `Summon with: jv, JSON, jview, viewer` 这行
比 `...-dark-before.png` **明显退了一档**——改前它和上面的描述（`View JSON`）几乎同墨
（122 对 133，差 11），改后它成了板子上最轻的一行（110，比描述低 23），而浅色那张两张图
逐像素相同。也就是说：这一行此前在深色上「看着像描述」，在浅色上「看着像第三档」——
同一个界面两种读法，现在两套外观都读成第三档。

## 7. 偷了什么（qiaomu-design / craft-loop.md）

本轮读了 `SKILL.md` → `references/user-preferences.md`（用了 P-5「可读性优先于气质」与
P-1/P-3 的字体侧约束）→ `references/craft-loop.md` → `references/preflight.md`。走的是
「已有方向，只要打磨」那条车道（`craft-loop.md` + `preflight.md`），**没有**出四方向预览页
（用户已把方向定死），也没有等点选。逐条落成：

1. **craft-loop §一（DNA 注入）**：供体不是外部站点，而是**参考实现自己的 CSS**
   （`main:src/styles/extensions.css` 的 `.extension-command-list__triggers`）。偷的不是数值，
   是把它**翻译**过来这条纪律：网页上 `opacity` 与自己的底色复合，本仓的墨是面板上的洗色，
   所以同一个 0.85 在浅/深两套外观上是两个位置（浅 1 级 / 深 12 级）——这一段 RAW→本仓的
   翻译，落成「墨只能来自 token 阶梯」。
2. **工艺密度菜单 #6（编辑细节：hairline / 字距）与 #12（材质细节：内边框高光）故意不做**：
   这块板的线（`Hairline` 外框 + `LitFace` 的一对 1px 内线，P13-m 已落地）与字阶（全行
   `Caption`）都已定稿，本轮的正确动作是**减少一个旋钮**，不是再加一层工艺。
   **记进「故意不做」，不是漏掉。**
3. **工艺密度菜单 #5（多层染色阴影）不适用**：这一行是 E1 面上的一行字，§2 判据 2 明写
   「E1 不许有外投影」。
4. **craft-loop §三（两轮制）**：第一轮改完后读三档（§1/§3 表），第二轮只修一处——
   把 pin 的 ② 从「两行不许相同（`d > 0`）」收紧成「**一整档**（`d ≥ 8`）」，因为 `d > 0`
   会被 1 级的抗锯齿差蒙过去，而这一轮的核心断言正是「它是**另一档**，不是另一行」。

## 8. preflight 逐条

- **A（反 AI 套路）**：过。无渐变字/紫蓝光晕/纯黑底（深色是 `#0E0E10`）、无斜体（P-1）、
  无 emoji 图标、无三等分卡片、无左侧竖线装饰（P-38）、文案无空词、感叹号 0 个。
  **不适用两条并写明**：① 「字体不是 Inter/Roboto/系统默认」——本仓字体栈是
  `docs/raycast-design.md` §4 钉死的行为契约；② 眉批/Hero 规则属于网页，本仓是桌面自绘窗口。
- **A2.1（K3 门）**：过（不适用）——当前任务没有点名 K3，按 P-46 由本代理直接执行。
- **A2.2（创意提示词门）**：不适用（不是开放方向）。
- **B（设计读取一致性）**：过。一页一个主题；本轮改动只在这一行的墨，与 §1「同一色相、
  token 唯一来源」和 `theme.Tokens` 的四档阶梯是同一句话；记忆点一句话：
  **「这一行是第三档，不是第二档乘 0.85。」**
- **C（动效抽查）**：过（不适用）。本轮零动效改动。**一处已量到的说明**：这一行的墨是
  **硬切**，而同页的 `sidebarRow` / `cardRow` / `headerButton` 都挂了
  `Transition{Colors: true, Duration: theme.ColorStep}`。查了 `mygo@v0.7.3`
  的 `ui/transition.go:805-847` `animateColors`：它只动 **`e.bg` 与 `e.borderC`**（背景与
  边框），**不动文字墨**（`node.TextColor` 写的是 `e.ts.color`，`ui/element.go:1196`）——
  所以给 `actionWithGlyph` 补一条 `Transition{Colors: true}` 对墨是**空操作**。
  这条结论接替了 P13-cq「仍待」第 1 条（那条的提议本身是无效的，记在新「仍待」里）。
- **D（工程抽查）**：过。可点区域与 `aria` 名称未变（`switchControl(...).Label(...)` 原样），
  焦点、禁用语义（板子那道 0.62）未被触及；超长 summon 词仍走 `SingleLine`+`Ellipsis`。
- **D2（中文页）**：过（不适用）。本轮没有改任何字号/行高/字重/i18n 文案；页面文案仍是英文；
  `tokens.Caption` 11px 与「正文 ≥ 14px」的冲突仍是**需拍板**项（第 19 份证据）。
- **E（三秒测试）**：过。这一页视线里仍然只有一个焦点（命令板列表本身）；本轮删掉的是一个
  旋钮，不是加了一个争夺注意力的元素。

## 9. 仍待（P13-cr）

1. **P13-cq 的「仍待」第 1 条作废并改写**：给 `actionWithGlyph` 补
   `Transition{Colors: true}` **不是**解（`mygo` 的颜色过渡只覆盖背景与边框，
   见上 §8 C）。要做「墨的过渡」只能自己插值文字色，属另一片（且要动帧），**先不碰**。
2. **下一格（不需拍板，先量）**：同族的第二处「token × ad-hoc alpha」——
   `internal/launcher/output.go:794` 的 `tokens.TextTertiary.Alpha(0.7)`（捕获输出页图例的
   分隔点）。先量它在深/浅两套外观上离它两侧那行字多少级，再决定是留在原地还是交给阶梯。
3. **需拍板（沿用 + 第 19 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路（`settingsui.go:1838`）；会话页
   空态说错了**对象**（「持久化会话」vs 进程内会话）；终端页会话抽屉 / 标签对失败只加字
   不换墨；`launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带 chip 的按下档；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
5. **需拍板（第 5 份证据，§6.32-4）**：同一张卡上的两种「这一层的名字」。
6. **故意不做（本轮记）**：命令板 `Opacity(0.62)`（关掉的命令整板淡）——那是面的淡，
   不是墨的淡（见 §6.61 尾注）。
7. **抢救 stash 核对（第二十次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
