# P13-bk 报告 · 字段行尾部那一格，在三页上说错了话

## 这轮为什么让它更轻、更有空间感

字段行是这张卡的**第一条带**，它的右端此前挂着一句 278px 宽的三段式指令
（「Enter runs it · Tab completes · Esc cancels」），而这句话在剪贴板页与浏览器页上
**两个分句都不成立**，且与同一帧下方 chips 行的「Tab to switch」**直接矛盾**——同一个窗口
对同一个键给出两个相反的说法。改后这条带在插件模式里只剩 94px 的一句
「Type to search」，宽度少 184px（66%），带子安静下来，页面的陈述句（居中的那句）
第一次是**唯一**在说话的那一件。命令模式的空态上，这句话此前被画了两遍（页面一遍大、
字段一遍小），现在只说一遍——而且是**大的那一遍**。

## 先量（HEAD `f7d6a688`，`ui.Tester` 出图后逐像素读峰值对比，地面 浅 253 / 深 23）

| 页 | chips 行的提示 | 字段行尾部的提示 |
|---|---|---|
| 剪贴板（空） | `Tab to switch · Ctrl+D to favorite` rect `{325 60 379 20}` ink **144 / 112**（对比 328 / 267） | `Enter runs it · Tab completes · Esc cancels` rect `{390 17 278 22}` ink **116 / 164**（对比 412 / 421） |
| 浏览器（空） | `Tab to switch` rect `{248 60 456 20}` ink **144 / 112** | 同上 rect `{390 17 278 22}` ink **116 / 164** |
| 命令模式（无匹配） | — | `tt.Texts()` 里这句话出现 **2 次**（页面陈述 `Emphasis`+`TextStrong` + 字段尾部 `FontSize`+`TextMuted`） |
| 计算器 / 文件 / 普通页 | — | `Type to search` rect `{574 17 94 22}` ink **116 / 164** |

`internal/i18n/i18n.go:117-119` 自己的契约写着「CommandModeHint is the field's hint
**while an extension command's arguments are typed**」。剪贴板页没有命令、也没有参数。

## 落地

| # | 位置 | 改前 → 改后 |
|---|---|---|
| 21-1 | `launcher.go:1569` 字段行的 `switch` | `case a.mode != nil \|\| a.clipboard \|\| a.browser:` → **`case a.mode != nil:`**。剪贴板与浏览器落进 `default`，与计算器/文件页同一答案（空查询 → `copy.Hint`；有查询 → ✕ 清除键） |
| 21-2 | 同文件命令模式臂 | 加 `if !quietPage` 守卫：页面已经把这句话当陈述画过（`emptyMessage` 在 `a.mode != nil` 返回同一条 `CommandModeHint`，`:877`），尾部不再重复 |
| 21-3 | 同文件 `:1360` 空态臂 | `} else if len(results) == 0 {` → `} else if quietPage {`；`quietPage := a.output == nil && !a.configOpen() && len(results) == 0` 写在 `actionBarSplit` 之后，**两处共用一个判据** |

**一个如实的副作用**：剪贴板页与浏览器页在**有查询**时，字段尾部从「那句假话」变成
**✕ 清除键**——与计算器/文件页同一个控件（同一行、同一个 `fieldSettingsButton`）。不是新功能，
是同一族的第五页终于和另外三页长得一样。

## 几何

零变化。字段行是 `Absolute().Height(fieldRow)`（56u），尾部是它的一个 `Shrink(0)` 孩子，
字段 `Grow(1)` 吃掉腾出的宽度；整帧像素差 bbox **`(391,22)-(667,37)`**（277×16，就是尾部那一格），
chips 行、页面陈述、列表、窗口高度一个像素都没动。**V 2 / B 0 / S 0**（两条 V 见下）。

## 门槛（实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows go vet ./...      → 净
GOOS=darwin  go vet ./...      → 净
go test -count=1 ./...         → 全绿
--- PASS 计数                  → 3168 → 3189（+21）
--- SKIP 计数                  → 111 → 112（+1，本轮 opt-in 出图）
测试函数                       → 2337 → 2343（+6）
三平台 go build ./...          → 过
```

## V / B / S 台账

- **V 2**：`internal/launcher/view_test.go` 的 `TestClipboardModeWithoutAStore:552` 与
  `TestBrowserModeWithoutASearch:882` 拿「Enter runs it」当「模式开了」的证据。这两条钉的是
  **旧设计值**，改断言到新值（字段尾部 `copy.Hint` + 本模式自己的空态句），注释写
  `P13-bk 重定基线：旧值 → 新值`。测试的**意图**（模式开了、页面说得出自己在哪）没变，
  只是证据从一句假话换成这一页真说的两句。
- **B 0**：其余 3187 条零改动全绿。
- **S 0**。

## 变异（4 条实跑，全部先红后逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `case a.mode != nil:` → `case a.mode != nil \|\| a.clipboard \|\| a.browser:` | `--- FAIL: TestTheClipboardAndBrowserPagesNeverContradictTheirChipsRow` + `TestTheFieldTailIsGuardedByTheQuietPage` |
| M2 | 删掉 21-2 的 `if !quietPage` 守卫 | `--- FAIL: TestTheCommandModeSaysItsHintOnce`（「the command-mode hint is drawn 2 times, want 1」）+ 结构 pin |
| M3 | 尾部墨 `t.TextMuted` → `tokens.TextTertiary` | `--- FAIL: TestTheFieldTailIsStrongerThanTheChipsAside/light`：`the field's own instruction is 328 from the ground, the chips row's aside is 328: one register for two roles` |
| M4 | `quietPage` 里去掉 `!a.configOpen()` | `--- FAIL: TestTheFieldTailIsGuardedByTheQuietPage` |

`md5sum -c /tmp/p13bk.md5` 逐字节核对 `internal/launcher/launcher.go`（`98aeaee9…`）**成功**，
`touch` 后复跑绿。

## 出图

`/tmp/floter-P13-bk-{clipboard,browser,commandmiss}-{light,dark}-{before,after}.png`
（`FLOTER_P13BK_DUMP=1 FLOTER_P13BK_TAG=<before|after>`）。三页两套外观的差异 bbox 全部落在
**同一个矩形 `(391,22)-(667,37)`**（1534 / 1534 / 1275 像素），即字段行右端那一格。
**这张图上看到的一处变化**：浅色剪贴板页的字段行右端，那串灰字从「Enter runs it · Tab
completes · Esc cancels」变成短句「Type to search」，句子左边缘不动（同一个 8u 尾距），
右边缘回收 184px，字段的查询文字于是多出 184px 的呼吸——而它下面 chips 行的
「Tab to switch · ⌘D to favorite」第一次是全窗口唯一提到 Tab 的那句话。

## 偷了什么（craft-loop 工艺密度菜单，逐条对号）

| 菜单项 | 落成的具体数值 |
|---|---|
| **第 6 项 · 编辑细节**（页边注 / hairline / **一个界面里的说法一致性**） | 同一族的五个插件模式：尾部文案**一条**（`copy.Hint`，94px）、墨**一档**（`TextMuted`，峰值对比 412 / 421）、行盒**一个**（22px）。跨页逐像素相等（`TestTheFourPluginModesWearOneFieldTail`，容差 2 级） |
| **第 3 项 · 状态/层级细节**（**一句话只说一遍**，同级不许互斥） | 命令模式空态：这句话从 2 次 → **1 次**，且留下的是**大**的那一遍（页面陈述 `Emphasis` + `TextStrong`），不是小的那一遍。判据：`tt.Texts()` 计数 + 存活那一份的 `y ≥ 56`（不在字段行里） |
| **第 12 项 · 材质细节**（内边框高光 / 墨阶） | 尾部（`TextMuted`，412 / 421）必须明显重于 chips 行的旁注（`TextTertiary`，328 / 267）——下界 +50 级。两条都是「关于这一行的提示」，但一条是输入面、一条是它下面的带子 |

**没有取 1（氛围层）/ 2（::selection）/ 4（签名交互）/ 5（多层染色阴影）/ 7（展示排版）/
8（流体字号）/ 9（滚动条）/ 10（彩蛋）/ 11（图形资产）**——本轮改的是**一行已有的提示**，
不是新页面。craft-loop 的「每页 ≥ 5 项」是**按页**的密度门槛，启动器那五项已在 §6.1 /
§6.7 / §6.16~§6.20 / §6.26 / §6.29 落过；给 22px 高的一格硬塞彩蛋是密度表演。

**DNA 供体**：**没有**从 58 站库取供体。方向被用户钉死（轻便液态玻璃），而本仓已有单一来源
合约；本轮偷的是**同一份合约里的既有判据**（§6.24 的「从属行不许与结论平起平坐」、§6.26 的
「同一档墨必须指同一个亮度」）——把「一句话在一个窗口里说两遍」判成缺陷，比引入第二个供体更值钱。

## preflight 逐条结论

**A · 反套路扫描**：无渐变、无纯黑、无外发光（自绘，墨是实色 token）；**无斜体**
（`internal/` 生产代码 `.Italic()` 调用点为 0，P13-bh 收的）；无三等分卡片、无眉批、无假数据。
**本轮新增的「反套路」判据**：**同一窗口里两句互相矛盾的话**（「Tab completes」vs「Tab to switch」）
就是 AI 味最重的一种——它是把两段模板文案各自搬过来、没人读一遍的产物。**过**。

**B · 设计读取一致性**：这一页面向「刚打开剪贴板模式、还没复制过东西」的用户，语言是
「安静、一句」：一句陈述（居中）+ 一句输入面的指示（尾部）+ 一条带子的旁注（chips）。
记忆点：**五个插件模式只有一个说法**。动效气质一致（本轮零新增动效，颜色过渡仍是
`theme.ColorStep` 120ms）。**过**。

**C · 动效抽查**：本轮**没有新增动效**，只换了一条文案与它的守卫。既有过渡（字段行的
颜色过渡、✕ 的 hover 抬档）未动，`ReduceMotion` 兜底仍在。**过（无新增）**。

**D2 · 中文页追加项**：文案全部走 i18n（`输入命令或应用名称` 的短句版是
`copy.Hint` = 「输入搜索」），全角标点，中英不混排；**⚠️ 一条不过**：尾部是 13px
（`t.FontSize`），低于 preflight D2 的「正文 ≥ 14px」。**不能过，且本轮不许改**：改字阶 =
改行盒 = 改高度链（§6.24 / §6.25 / §6.26 / §6.28 / §6.29 已六次撞红）。这是
`tokens.Caption` = 11px 那条**需拍板**项的**第七份证据**。

## 仍待（P13-bk）

1. **需拍板（沿用，本轮第七份证据）**：13px / `tokens.Caption` 11px 与 preflight D2
   「正文 ≥ 14px」的张力。**改字阶 = 改高度链**。
2. **需拍板（新，本轮量到）**：插件模式里字段的 **placeholder** 恒为 `copy.Placeholder`
   （「输入命令或应用名称」），剪贴板页上它说的是「应用名称」——那页打不出应用。要改就得
   **新增一条 i18n 文案**（按模式分），踩 i18n 红线，等拍板。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
4. **下一格候选**：①`settingsui.go:5714` 的 `metadataCell` 标签与 `:5683` / `:6847` 的同类
   从属标签（先量它与主体的关系）；②启动器 `fieldSettingsButton` 那一簇尾键在**三种页面**
   （普通/插件模式/命令模式）上的 hover 与禁用阶梯；③`shell/pinned.go` 便条正文只有一档字阶
   （它是一段快照，可能本来就对，先量）。
5. **抢救 stash（本轮复核）**：`stash@{0}` = `afc38b247c`（`metaSeparator`）已在 HEAD；
   `stash@{1}` = `114b7d1585` 只改 `.pending-bugs.md`、文字已在文件里。**两条都无需再试合。**
