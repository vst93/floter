# P13-bh 报告 · 全仓最后两处斜体，都压在中文上

轮号 **P13-bh**｜分支 `mygo-rewrite`｜规格 `docs/rounds/P13-bh-spec.md`｜
合约新增 **§6.27**（7 行：6 `已落地` / `已落地（复核）` + 1 `故意不做`）

## 一句话

生产代码（`cmd/` + `internal/`，不含测试）里 `.Italic()` 只剩 **2 个调用点**，两处都在
设置页扩展卡的「最近一次输出」凹槽里，而且**两处压的都是中文**：空输出占位
`（无输出）`（`settingsui.go:6899`）与截断尾 `……输出已截断（上限 64KB）`（`:6926`）。
中文没有斜体字形，这两行每一帧都在让引擎**伪造**一个——11px 的 CJK 笔画被重采样一次再
横向拉伸约 15°。本轮把它清零，并把「这不是程序输出」交给**字体寄存器**与**文案自带形状**。

## 这轮为什么让它更轻、更有空间感

用户说现在的界面「平、糊、旧」。「糊」这个字，在这一处是**字面**的：

- 中文（苹方 / 思源 / 微软雅黑）只有正体。`font-style: italic` 落到引擎的伪合成斜切
  （pango `pangoStyleItalic`、DirectWrite `fontStyleItalic`、CoreText `syntheticItalic`），
  **不是设计过的斜体字形，是把字形重采样一遍再切**。11px 下这一刀等于把笔画糊开。
- 所以这两行既**不清晰**（伪斜切），又**没有层级**（同字号同墨，只靠一个假的倾斜说
  「这是旁白」）。这是「细节打磨」里最不该留的一种：一个**不存在的字形**被当成了设计语言。

改后：两行是**正体**，字面干净；「旁白」的语义由两件真东西承担——
①**字体寄存器**（真输出是 `runOutputFont` 等宽栈，这两行是系统字体）；
②**文案自带形状**（占位被全角圆括号包住、尾行以省略号开头；两种形状都不会出现在程序的
输出里，而且都是参考自己的词）。

## 先量（本轮开工实测）

**伪斜切到底改了什么**（460×140、真扩展页地面、`zh`）：

| 度量 | 正体 | 伪斜体 | 差 |
|---|---|---|---|
| `（无输出）` 归一化字形掩码 | 37×11 | 37×11 | **77 / 407 格不同** |
| `……输出已截断（上限 64KB）` | 127×11 | **128×11** | 整行**宽出 1px**（斜切把首字的左缘推出去） |
| 逐行最优对齐位移（顶→底） | — | −1 → +1 | 2px 的斜切，在 11px 行盒上 |

**两行离凹槽面的距离**（`runOutputEdgeFace` 的不透明合成面，`far` 级）：

| 行 | 墨 | 浅 | 深 |
|---|---|---|---|
| 占位 `（无输出）` | `t.TextMuted` | **96** | **91** |
| 截断尾 | `TextTertiary` | **89** | **75** |
| （对照：真输出） | `TextStrong` | 201 | 199 |

浅色那 **7** 级确实薄。**记账而不抬档**：把占位抬到 secondary 会让这块**空**凹槽的声音
（115 级）比它自己的标题（`最近一次输出`，600 字重的 tertiary）还响——一个空块不该比它的
标题更响。参考 CSS 自己的两档就是 muted / tertiary（`extensions.css:1259-1267` /
`:1268-1280`）。

## 落地（`internal/settingsui/settingsui.go`）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `:6899` 空输出占位 | `ui.Text(...).FontSize(Caption).TextColor(t.TextMuted).Italic()` | `a.runOutputAside(c, tokens, copy.RunNoOutput, false)` |
| 2 | `:6926` 截断尾 | 同上 + `TextTertiary` | `a.runOutputAside(c, tokens, copy.RunOutputTruncatedTail, true)` |
| 3 | 新增 `runOutputAside` | — | 两行**一个家**：`ui.Text(c, line).FontSize(tokens.Caption).TextColor(runOutputAsideInk(tokens, tail))` |
| 4 | 新增 `runOutputAsideInk` | — | 墨阶的**唯一出处**：`tail` → `TextTertiary`，否则 `t.TextMuted` |

**为什么把墨收进 `runOutputAsideInk`**：pin 要量的是**页面真的画出来的那个数**，不是测试
自己按 token 拼出来的重建。第一次变异（M2）就是踩在这上面——测试里 `measure(line, tokens.TextTertiary)`
是测试自己传的颜色，产品侧改错也照样绿。收进 helper 之后，pin 与产品读同一个函数。

**几何零变化，只换字形**：`Italic` 不改行盒，`FontSize` / `MaxHeight(45u)` / `LineHeight` /
凹槽的面与两道滚动边（§6.17）/ `runOutputFont` / i18n 文案**一个字节不动**。

## 顺带修的一处 HEAD 红（本轮发现，不是本轮引入）

§6.26 的 26-1 行写着 `` `run.TimedOut \|\| !run.Success` ``，而
`internal/ci/liquidglass_contract_test.go` 按 `|` 切单元格——那个**转义竖线**把行切成 6 段，
状态列读到 `\` 而不是 `已落地`。`TestTheLiquidGlassContractSupersedesTheRaycastOne`
在 HEAD `b33023ad` 上**本来就是红的**（实测 203 行 / 202 带状态）。本轮把它写成
「`run.TimedOut` 或 `!run.Success`」，全仓 `\|` 归 0（`grep -c '\\|' docs/liquid-glass.md` → 0）。

这是 **V**（视觉合约的表述），不是 B：改的是文档措辞，不动任何产品行为。

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → **净**；`GOOS=windows go vet ./...` → **净**；`GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **0 FAIL**（全绿）
- PASS **3157 → 3160**（**+3**）｜FAIL **0**｜SKIP **108 → 109**（+1，opt-in 出图）｜
  测试函数 **2323 → 2327**（**+4**）
- `GOOS=linux/windows/darwin go build ./...` → **三平台全过**
- **V 1 / B 0 / S 0**（V 是上面那处 §6.26 的表格表述；本轮没有任何旧 pin 因设计变化变红）

## 变异（4 条实跑，全红，逐字节还原）

| # | 注入 | 红在哪（原始报错串） |
|---|---|---|
| M1 | `runOutputAside` 的 `ui.Text(...)` 后加回 `.Italic()` | `TestNoSurfaceInTheAppSlantsItsChinese`：`production code still asks for an italic face at 1 site(s)`；`TestTheCJKPlaceholderIsNotSheared`：`"（无输出）": the page draws a shape 77 glyph cells away from upright (the sheared frame is 77 away): the line is still italic` |
| M2 | `runOutputAsideInk` 的占位退回 `TextTertiary` | `TestTheEmptyCaptureStaysAheadOfTheTruncationAside`：`[light] the empty placeholder stands 83 levels off the block's face, the truncation aside 89: the block's own voice must not be quieter than its aside`（深色同：70 对 75） |
| M3 | `runOutputAside` 改用 `runOutputFont`（等宽栈） | 同一条的源码臂：`runOutputAside draws its line in the capture's monospace register: it is an aside, not the program's output` |
| M4 | 在 `internal/terminalui/terminalui.go:883` 注入一处 `.Italic()` | `TestNoSurfaceInTheAppSlantsItsChinese`：`production code still asks for an italic face at 1 site(s): ../../internal/terminalui/terminalui.go:883` |

`md5sum -c` 逐字节核对（`settingsui.go` / `noitalic_test.go` 两枚），全部 `成功`。

**M2 的第一版没红，这条要记下来**：pin 当时自己按 token 拼颜色，产品改错也绿。把墨收进
`runOutputAsideInk` 之后才红。这是「pin 必须量页面画的、不是量测试拼的」的实例。

## 出图（自己用眼睛复核过）

`/tmp/floter-P13-bh-asides-{light,dark}-{empty,truncated}-{before,after}.png`
（`FLOTER_P13BH_DUMP=1 FLOTER_P13BH_TAG=<before|after>`）。整帧逐像素对照：

| 帧 | 变化像素 | bbox |
|---|---|---|
| light empty | 256 | (210,515)-(247,524) |
| light truncated | 748 | (205,578)-(332,589) |
| dark empty | 257 | (210,515)-(247,524) |
| dark truncated | 745 | (205,578)-(332,589) |

**图上看到的一处变化**：浅色 `truncated` 帧里，`……输出已截断（上限 64KB）` 从一行**向右
歪着**的灰字变成一行正的灰字——before 帧上「输出已截断」四个字的竖笔明显向右倾（伪斜切），
after 帧上它们和上面 `one/two/three` 的等宽基线一样是垂直的。上面 `最近一次输出` 那行标题、
两道滚动边的淡、凹槽的面**一个像素没动**（bbox 只覆盖那一行字）。

## 偷了什么（qiaomu-design `craft-loop.md` 工艺密度菜单，逐项）

本轮读的是 `SKILL.md` → `references/user-preferences.md`（P-1 就是本轮的依据）→
`craft-loop.md` + `preflight.md` + `chinese-typography.md`（中文页那条路径）。

- **`craft-loop.md` 第 6 项「编辑细节」的近亲——「文字的**字形**也是细节」** → 落成
  「`.Italic()` 的生产调用点为 0」这条 census。工艺密度菜单把细节算在「hairline / 章节编号 /
  大写字距标签」上；这一轮把同一格推到**字形层**：字被引擎改过，比字距错 0.5px 严重得多。
- **第 12 项「材质细节 / 内边框高光」的判据「同一面只说一句话」** → 落成「旁白不用假倾斜说」：
  倾斜在中文里不是一种「说法」，是一个**伪造的字形**；说法交给字体寄存器与文案形状。
- **`chinese-typography.md` §三「中文禁斜体（无斜体字形设计，强制倾斜=笔画变形）」** →
  这条就是本轮的判据原文，落成 census 的硬断言（0 个调用点）。
- **`user-preferences.md` P-1（硬禁令，界面排版禁用斜体，中文尤其）** → 本轮是它在本仓的
  最后一处违反，清零后写进 §6.27 作为常驻账。
- **preflight A（反 AI 套路）**：本轮**没有**加任何新装饰——反而删掉了一个伪造的字形效果；
  无渐变、无发光、无装饰性强调。
- **preflight B（设计读取一致性）**：这一页只有一个方向（「旁白轻于捕获」），与
  `liquid-glass.md` §1「层级靠差说话」同一条；两行的墨阶**沿用参考自己的两档**，不新造。
- **preflight C（动效抽查）**：本轮**零动效**（只换字形），没有过渡、没有 hover、没有入场。
- **preflight D2（中文页）**：**过了，而且是本轮的主项**——D2 的「中文禁斜体」正是这轮清掉的；
  `zh` 文案一个字节不改（`（无输出）` / `……输出已截断（上限 64KB）` 原样，全角标点与省略号
  都是 `i18n` 原文自带）。
  **不能过的项**：D2 的「正文 ≥ 14px」——这两行是 `tokens.Caption`（11px），与
  `tokens.Caption` 那一族的张力同源，**改字阶 = 改高度链**（P13-bd / P13-bf / P13-bg 已三次
  实测撞红），仍记在 `.pending-bugs.md` 的「需拍板」表。

## 仍待（P13-bh）

1. **需拍板（沿用，本轮第四份证据）**：`tokens.Caption` = 11px 与 preflight D2「正文 ≥ 14px」
   的张力。**改字阶 = 改高度链**。本轮两行旁白的墨阶可以随便换，字阶一动行盒就动。
2. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
3. **下一格候选**（通用队列「状态反馈 / 空态满态」剩下的两处，先量再改）：
   ①`settingsui.go:5714` `label`（`Caption` + `TextTertiary`）与 `:5683` / `:6847` 的同类
   从属标签——P13-bf 的教训是**从属行不该一律抬**，要先量它们与主体的关系；
   ②`internal/shell/detach.go` 插件窗口里同一个运行**没有**状态行（P13-bg 仍待 2），
   属「跨表面统一」，按纪律不跨表面做。
4. **抢救 stash**：`stash@{0}`（`afc38b247c`，`settingsui.go` 的 `metaSeparator`）复核后确认
   已在 HEAD（`metaFact` / `integrationMeta` 都在，`git apply -3` 冲突是因为后续轮次已改过
   同一段）；`stash@{1}`（`114b7d1585`）只改 `.pending-bugs.md`，文字已在文件里。
   **两条都无需再试合。**
