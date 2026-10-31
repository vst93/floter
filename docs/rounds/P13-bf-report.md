# P13-bf 报告 · 设置页禁用滑块：全行唯一一句「为什么」比它的提示还轻

轮号 **P13-bf**｜分支 `mygo-rewrite`｜规格 `docs/rounds/P13-bf-spec.md`｜
合约新增 **§6.25**（4 行：2 `已落地` + 2 `已落地（复核）`）

## 一句话

透明度组的两条滑块在「这个窗口拿不到背板材质」时是**禁用行**，而这一行里
**说明「为什么滑不动」的那句是全行最淡的一件**（比解释功能的提示还轻 14 / 23 级），
而**那个不会动的读数**却和行名只差 10 级——一行四件东西的阶梯是**反**的。
本轮把它掰回三档：行名 ≫ 读数＝原因 ≫ 提示＝刻度。

## 这轮为什么让它更轻、更有空间感

层级只由**差**说话。这个「禁用槽」此前只退了一半：轨道退了（21 → 11 级），
**读数一格没退**（193 / 188 对行名 203 / 198），而它上面那句「为什么」反倒掉到
提示之下（88 / 71 对 102 / 94）。于是这一行读起来是「一个很重的名字 + 一个同样重的
数字 + 一片更亮的灰」——数字和名字挤在同一个 z 平面上，真正需要被读的那句话沉到
最底下。§1 对「槽」的判据是「整块退一半、内线各留一半」：退必须是**整块**的，
一半退一半不退，就是用户说的「糊」。

## 落地（`internal/settingsui/settingsui.go` 的 `slider`）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | 读数（`row.format(v)` 那行） | 恒 `tokens.TextStrong` | `row.disabled != ""` 时 `tokens.TextSecondary`；可用行**不动** |
| 2 | 原因句（`row.disabled` 那行） | `Caption` / 500 / `tokens.TextTertiary` | `Caption` / **580** / `tokens.TextSecondary` |
| 3 | 行名 | `TextStrong` | 不动（这一行不是 `Disabled` 子树） |
| 4 | 功能提示、轴两端 | `TextMuted` | 不动 |

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → **净**；`GOOS=windows go vet ./...` → **净**；`GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **0 FAIL**
- PASS **3147 → 3154**（**+7**）｜FAIL **0**｜SKIP **106 → 107**（+1，opt-in 出图）｜
  测试函数 **2315 → 2319**（+4）
- `GOOS=linux/windows/darwin go build ./...` → **三平台全过**
- **V 0 / B 0 / S 0**（没有任何旧 pin 变红；本轮只加 pin，没改任何既有断言）

## 先量（实测，640×320、真设置页地面；地面浅 247 / 深 14，卡面浅 230 / 深 39）

重量＝墨与**卡面**的亮度差，两套外观同序。

| 位置 | 改前（浅 / 深） | 改后（浅 / 深） |
|---|---|---|
| 行名 | 203 / 198 | 203 / 198（不动） |
| 读数（可用） | 193 / 188 | 193 / 188（不动） |
| 读数（禁用） | **193 / 188** | **117 / 109** |
| 原因句 | **88 / 71** | **123 / 115** |
| 功能提示 | 102 / 94 | 102 / 94（不动） |
| 轴两端 | 102 / 94 | 102 / 94（不动） |

原因压过提示 **21 / 21 级**；禁用读数与原因同档（差 6 / 6）；可用读数与行名差 10 / 10。

## 一行几何变化：**没有**

字阶一个字节不动（原因仍是 `tokens.Caption`，只换墨与字重）。两帧的差异**全部**落在
墨上（见下面的逐像素对照）。

## 变异（4 条实跑，4 红，`md5sum -c` 逐字节还原后 `touch` 复跑绿）

| # | 注入 | 变红的 pin 与原始报错串 |
|---|---|---|
| M1 | 原因退回 `Caption`/500/`TextTertiary` | `TestTheDisabledSlidersReasonOutranksItsHint`：`the reason reads 105 against its hint's 119: a statement that explains a state is lighter than the sentence describing the feature` / `the reason is only -14 above its hint (105 vs 119): one rung, not a register`（深色 `96 vs 119`、`-23`） |
| M2 | 禁用读数仍 `TextStrong`（去掉分支） | `TestADisabledSlidersReadoutStepsBackAndALiveOneDoesNot`：`the disabled readout still reads 210 against the live one's 210: it has not stepped back` / `the disabled readout reads 210 and the reason 140: the two facts of one disabled row are on different rungs`（深色 213 / 140） |
| M3 | 可用读数也跟着退到 `TextSecondary` | 同一条 pin：`the live readout reads 134 against its name's 220: it used to be one step under the name` / `the disabled readout still reads 134 against the live one's 134`（深色 134 / 223） |
| M4 | 原因字重退回 500 | `TestTheSliderReadoutAndReasonAreOneBranchEach`：`the slider is missing "FontWeight(580).TextColor(tokens.TextSecondary)"` |

还原：`git checkout` 后 `git apply` 原补丁，`md5sum -c /tmp/p13bf.md5` → `settingsui.go: 成功`。

## 出图（眼睛复核过）

`/tmp/floter-P13-bf-slider-{light,dark}-{on,off}-{before,after}.png`
（`FLOTER_P13BF_DUMP=1 FLOTER_P13BF_TAG=<before|after>`）。

整帧逐像素对照（before → after，640×340）：

| 帧 | 差异 bbox | 变化像素 |
|---|---|---|
| light **on**（可用行） | **None** | **0** |
| light off | `(37,38)-(522,162)` | 4797 |
| dark **on**（可用行） | **None** | **0** |
| dark off | `(37,38)-(522,162)` | 4797 |

**这张图上看到的一处变化**：在 `light-off` 那一帧里，**右下角那句长原因**
从「几乎看不见的浅灰」（墨 142）变成了「读得出来的深灰」（107），而**右上角那个
`47%`** 从「和行名一样黑」（37）退到「一档之下」（113）——同一张图上，最该被读的
那句话变重了，那个不会动的数字变轻了。而 `light-on` / `dark-on` 两帧**一个像素都没动**，
这正是「可用行不动」的证据。

## 「偷了什么」（qiaomu-design / craft-loop 工艺密度菜单）

逐条对账（本轮读的 reference：`user-preferences.md`（全量）→ `craft-loop.md`（全量）
→ `preflight.md` A / A2 / D2 节）：

| 菜单项 | 取的具体值 | 落在哪 |
|---|---|---|
| **#12 材质细节**（「内边框高光、glass 折射边」的反面用法：**只在一处亮**） | 唯一允许亮起来的那条线是 `LensRim`（§1 新上限 0.09）；本轮的判据是「**陈述**是这一行唯一被点亮的东西」——把 `TextTertiary`（88 / 71）抬到 `TextSecondary`（123 / 115），字重 500 → **580** | `settingsui.go` 原因句那一行 |
| **#6 编辑细节**（「章节编号、hairline、大写字距标签」→ 移植成「一档一档的字重」） | 一行里三个**字重**角色：580（名字 / 陈述）、默认（读数 / 散文）、500（刻度）。本轮把「陈述」从 500 提到 **580**，让它与「散文」在**同一个墨档**里也分得开 | 同上 |
| **#5 多层染色阴影**（「2-3 层、染底色色调，拒绝单层灰阴影」→ 移植成「**一个槽的退必须是整块的**」） | 轨道已退（21 → 11 级），读数没退 → 现在读数退 **76 / 79 级**。退是**整块**的，不是「一半退一半不退」 | `slider` 读数分支 |
| **#4 签名交互**（「hover 揭示」→ 移植成「**只有禁用那一档才换墨**」） | 读数在可用态**逐像素不动**（整帧 0 变化）；墨的变化只发生在「这个控件真的不能动」的时候——状态反馈不该是装饰 | 出图 light-on / dark-on 两帧 |

## preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：本轮不新增任何颜色 / 渐变 / 发光 / 斜体 / 装饰（P-1 斜体禁令：零命中）；
  文案零改动（沿用 `i18n` 既有 `TransparencyUnavailable`）；无感叹号、无空词。
- **B 设计读取一致性**：本仓的单一来源是 `docs/liquid-glass.md` + `internal/theme/theme.go`；
  本轮两处墨全部走 `theme.Tokens`（`TextStrong` / `TextSecondary`），**没有一处字面色值**
  （结构 pin ③ 就在防这个）。
- **C 动效抽查**：本轮不动时长、不动曲线（`ContentTransition{Numeric:true}` 保留）；
  `theme.ColorStep` 120ms 那条账（`internal/ci/colorstep_contract_test.go`）零改动通过。
- **D2 中文页**：`tokens.Caption` = 11px 与 preflight「正文 ≥ 14px」的张力**仍然存在**，
  本轮**没有改字阶**（改字阶＝改高度链，P13-bd 已实测撞红）。**这一条按「不能过」如实记下**，
  已进 `.pending-bugs.md` 的「需用户拍板」（沿用 P13-bd / P13-be 的同一条）。

## 台账

- **V 0**：没有任何视觉 pin 因为新值变红。
- **B 0**：没有任何行为 pin 变红（高度链、键盘、写回判据 `chosen && row.disabled == ""` 全绿）。
- **S 0**：没有结构 pin 需要改（本轮新增的结构 pin 是**新**断言，不是改旧断言）。

## 仍待（P13-bf）

1. **需拍板（沿用）**：`tokens.Caption` = 11px 与 preflight D2「正文 ≥ 14px」的张力。
   **改字阶 = 改高度链**（P13-bd 已实测撞红，本轮又给了一条证据：原因句的墨档可以随便换，
   字阶一动行盒就动）。必须用户拍板。
2. **下一格候选**：`settingsui.go:5714` 的 `label`（`Caption` + `TextTertiary`）与
   `:5683` / `:6847` 的同类从属标签——这一族里还剩几处「从属标签用 tertiary」，
   但**先量它们与主体的关系**再决定要不要动（P13-bf 的教训是：从属行不该一律抬）。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
4. **抢救 stash（沿用 P13-be 的结论，本轮未再动）**：`stash@{0}`..`stash@{12}` 全部不需再试合。
