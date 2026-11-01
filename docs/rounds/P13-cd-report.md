# P13-cd 报告：同一张卡的两个空态，一句话、一处决定

规格 `docs/rounds/P13-cd-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.47**｜
HEAD 起点 `dafdd455`（P13-cc）

## 这轮为什么让它更轻、更有空间感

空态是「这一页什么都没有」这句话的**重量**。捕获输出页此前把这句话画成**正文**：
12px、`t.Text`、可选中、贴在正文第一行——和页头那行命令行**同档墨**（`darkest=25`），
和「命令真的打印了这句话」在界面上无法区分。也就是说，一个空页面上最重的一档墨
属于「没有内容」这句话本身，而页头条子里那句结局（`TextSecondary`）比它还轻。
现在它退回 `Emphasis` + `TextStrong`、居中在正文带的中线上：**空态在 z 上退后一步，
正文才是这一页的前景**。结果区那一半零像素改动——这是「抽出共享函数」的正向证据。

## 改了什么

| 文件 | 改动 |
|---|---|
| `internal/launcher/launcher.go` | 结果区空态那两行抽成 `App.quietStatement(c, message)`（`Emphasis` + `TextStrong` + `SingleLine`）；结果区改调它，几何/`listEdge`/`Grow(1)`/`Center()` 一行不动 |
| `internal/launcher/output.go` | `OutputView` 加 `Empty bool`（只在「这一页没有任何输出」那一支置真）；正文臂加一支 `Column.Fill().Center()` + `a.quietStatement`，**不建 `Scroll`**。`view.Text` 内容一字不改（高度链 `pluginTextRows`、复制 `PinText`、`outputText()` 全部照旧） |
| `internal/launcher/outputempty_test.go` | 新 pin 4 条（像素 5 账 × 深浅两套 / 结果区未挪位 / AST 普查 / opt-in 出图） |
| `docs/liquid-glass.md` | 新增 §6.47 |

**为什么用标志而不是比较字符串**：一条命令真的打印 `The command wrote nothing`，那是输出，
必须仍然是正文（可选中、可复制）。字符串比较会把那一帧也画成空态。**故意不做**。

## 实测（720×488 帧，深浅同值）

| | 改前 | 改后 |
|---|---|---|
| 结果区空态墨盒 | `{X:335 Y:263 W:51 H:10}`，心 x=360.5 | **不变**（帧心 360） |
| 捕获输出页空态墨盒 | `{X:12 Y:123 W:188 H:16}`，心 x=106，正文第一行 | `{X:281 Y:290 W:157 H:13}`，心 x=359.5 |
| 行盒（寄存器） | 22px（正文 12px） | **18px**（`Emphasis`） |
| 竖直中线 | 131（带 `100..484`，心 292） | 296.5 |
| 图上差分 | — | **1724 px**，包围盒 `12,123..437,302`——页头、条子、面板一个像素没动 |

## 偷了什么（qiaomu-design · craft-loop.md）

**DNA 注入（`references/design-systems/apple/DESIGN.md`，3 个具体值）**

1. `Micro 12px (0.75rem) / 400 / 1.33`——12px 在那一套阶梯里是**fine print / 脚注**的寄存器。
   这正是改前那句话穿的衣服：把一句陈述写在脚注的寄存器里，用户读到的是「备注」而不是
   「这一页的状态」。落地：这句话离开 12px，走 `Emphasis`（行盒 18px）。
2. `Body 17px` 与 `Body Emphasis 17px/600`——强调是**字重/墨**，不是字号。落地：我没有加字号，
   只把墨从 `t.Text` 换成 `TextStrong`（同一阶字号的另一档墨），并**没有**新造一个字号。
3. `Cinematic breathing room`（`:197`，「whitespace 不是空的，它是场景之间的停顿」）——
   落地：空态居中在那条正文带的中线上（实测 296.5 对带心 292，±12 内），而不是骑在带的第一行。

**工艺密度（craft-loop.md 第二节菜单，逐项指认）**

- **第 12 项「材质细节」→ 反向落地**：空态**不拿材质**——无底板、无受光边、无阴影，
  只有正文有面。数值：这句话的行盒 18px、墨 `TextStrong`、与带的距离 4.5px 偏置。
- **第 7 项「展示排版动作」→ 落地**：居中即排版动作，墨盒心 359.5 对帧心 360（±0.5px）。
- **第 2 项 `::selection` 定制 → 反向落地**：这句话**退出**可选中集合——`Selectable()` 只属于
  真输出。一行能被选中/复制、却永远复制不出别的东西的「状态文字」是把选中语义送给了空态。
- **第 9 项「自定义滚动条」→ 反向落地**：一行字的页面**不建 `Scroll`**（`launcher.output` 这个
  键在这一支上不存在），没有滚动条要设计，也没有可滚的东西。
- **第 5 项「多层染色阴影」→ 故意不做**：空态不进 E2（§2：阴影只给浮层）。给一句陈述加投影
  就是拿材质冒充层级，正是用户说的「类玻璃边」。

密度只落地 3 项（全部是减法/反向），2 项故意不做。这不是凑不满，是**一个空态就是一句话**——
收口轮的上限由「少」决定，不由「多」决定；把它写成 5 项就等于开始装饰它。

## preflight.md 逐条

- **A（反 AI 套路）**：无渐变、无外发光、无纯黑、无斜体、无三等分卡、无左侧竖线、无假数据、
  无 AI 空词——**过**（本轮只改一句陈述的墨与位置，没新增任何装饰）。
- **A2（进攻三步）**：DNA 3 值 / 工艺密度 3 落地 2 不做 / 两轮制——**过**。
  第一轮＝改后帧（见出图），四视角评审各挑一处：
  ① **艺术总监**：这一页原本最平庸的是「页头一句名字 + 下面一行假装是输出的灰话」，
  两处墨几乎同档，页面没有前景；现在唯一的前景是正文。
  ② **工程师**：`view.Empty` 分支里不建 `Scroll`，但 `a.Output` 的滚动状态仍被 `showOutput`
  重置——空页 → 有输出的页之间不会带着旧偏移（`TestAFreshOutputStartsAtItsBeginning` 全绿）。
  ③ **甲方**：三秒看完记住的是「这条命令什么都没说」——改前记不住，因为那行字长得像输出。
  ④ **读者**：改前 12px 左对齐短句在 720px 宽的正文里孤零零贴左边，读起来像半句话被截断；
  现在居中，眼睛不用回左边找。
  第二轮（只修 3-5 处）：M2 逼出的第 ⑤ 条账（寄存器：行盒 ≤ 20px）——**只量位置量不出字号**，
  这一条是第二轮补的，不是第一轮想到的。
- **B（设计读取一致性）**：一页一个明暗主题（深浅各出一套帧，不混）——**过**。
  记忆点一句话：**空态退后一步，正文才是前景**。动效气质：本轮零动效，不涉及。
- **C（动效抽查）**：本轮**零动效改动**（无 transition/animation/keyframe，无 hover 位移）。
  「按压有反馈」「popover origin」「prefers-reduced-motion」等条目在本轮 diff 上没有对象，
  按 **N/A** 记，不冒充已过。
- **D2（中文页）**：中文文案「命令没有输出」**一字未改**；全角标点：该句无标点，N/A；
  中西文空格：纯中文短句，N/A；字重只用 400（`TextStrong` 是墨不是字重）——**过**。
  唯一不过项沿用第 9 份证据：`tokens.Caption` 11px 与「正文 ≥ 14px」冲突，**需拍板**；
  本轮没有新增任何正文级文本（空态是 `Emphasis`，行盒 18px）。

## 门槛（全部实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows go vet ./...      → 净
GOOS=darwin  go vet ./...      → 净
go test -count=1 ./...         → 全绿（无 FAIL）
--- PASS 计数                  → 3232 → 3237（+5：像素 pin 父+深浅 2 子、结果区 pin、普查 pin）
--- SKIP 计数                  → 124 → 125（+1：opt-in 出图）
测试函数数                      → 2388 → 2392（+4）
三平台 build                    → 过
```

## V / B / S 台账

**V 0 / B 0 / S 0**：没有一条既有 pin 需要改（`launcher` 50s 全绿，`settingsui` 108s 全绿）。
本轮加的是新账，不是重定基线。`OutputView` 加一个字段属于内部结构，没有既有断言读到它。

## mutation（3 条实跑，逐字节还原）

- **M1** `output.go`：`} else if view.Empty {` → `} else if false && view.Empty {`（空态退回走
  `Scroll` + 正文墨）→ **红 10 臂**（5 条账 × 深浅）。原始报错串：
  `the empty state is not centred: ink centre x=106, want 360 (ink box {X:12 Y:123 W:188 H:16})`
  ／`the empty state still sits on the body's left inset: ink x=12 of a 720 frame`
  ／`the empty state still rides the body's first row: ink y=123 of a 488 frame`
  ／`the empty state is centred at y=131, want 292 (band 100..484)`
  ／`the empty state wears the body's size: its line box is 22px tall, want the emphasis step's 18`
- **M2** `launcher.go` `quietStatement`：`FontSize(tk.Emphasis)` → `FontSize(c.Theme().FontSize)`
  → **红 2 臂**：`the empty state wears the body's size: its line box is 22px tall, want the emphasis step's 18`。
  （第一次写 `tk.FontSize` 直接编译失败——`theme.Tokens` 没有 `FontSize`，改为 `c.Theme().FontSize` 重跑。）
- **M3** `output.go`：把 `a.quietStatement(c, view.Text)` 换回内联
  `ui.Text(...).FontSize(a.tokens(c).Emphasis).TextColor(a.tokens(c).TextStrong).SingleLine()`
  → **红 2 臂**：`output.go: outputBody builds its own Emphasis + TextStrong sentence; an empty state
  has one decision point (App.quietStatement, §6.47)` ／
  `the empty-state sentence is called from map[launcher.go:1], want exactly launcher.go and output.go`。

三条都用 `cp` 备份 + `md5sum -c` 逐字节核对还原（`/tmp/m1.md5` / `/tmp/m2.md5` / `/tmp/m3.md5`
全部「成功」），还原后 `touch` 再复跑一次绿（避免测试缓存复用）。

## 出图

`/tmp/floter-P13-cd-empty-output-{light,dark}-{before,after}.png`（`FLOTER_PNG=1`，
`FLOTER_PNG_TAG` 选 before/after 两侧）。图上看到的一处变化：**那句话从左下角的第一行
挪到了正文带的正中**——差分包围盒 `12,123..437,302`，页头、状态条子、面板边缘
一个像素没动（差分只覆盖文字本身与它经过的路径）。

## 仍待（本轮自己定的下一步）

1. **下一格**：捕获输出页的**满态**这一半还没量。它的下沿用的是 `foldAlpha`（行按可见比例
   退后，§6.2b 第 1-10 行），而不是设置页那条 `glass.ScrollEdge`（§6.16 / §6.17）——
   **两端不成对**：底边是软边（退后），顶边是行滚到条子下面被硬切。先量「顶边最后一行的
   墨展开度」，再决定是补一道顶边还是承认折叠就是这一页的收口语法。
2. **需拍板（沿用 + 第 9 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第六次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
