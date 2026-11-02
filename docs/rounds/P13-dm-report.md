# P13-dm 报告 · 工具表单的「工具链那一行」：一句话从控制槽回到它解释的选择器下面

规格 `docs/rounds/P13-dm-spec.md`，合约新增 **`docs/liquid-glass.md` §6.82**。
表面 = **设置页 → Integrations → Add a local tool → 脚本模式** 的语言选择器那一块。

## 这轮为什么让它更轻、更有空间感

一句话被挂在「Language」这一行的**控制槽**里，等于把它判给了那条行**最右的那一格**
（720 帧：x=628.8，卡片文字列右沿 457.8）—— 它跟自己描述的控件隔了 470px，
读者要在两处之间来回看；而这一行自己还占着一条 44u 地板的设置行、把 "Language" 念了第二遍。
现在它是选择器自己的**第二行**：与提示同列（x=201.8）、同一格呼吸、按句折行，
那条 44u 地板行整条消失（`y` 从 406.6 上提到 400.1，行盒仍在同一块里）。
层级从「和别的设置项平级」退到「在选择器之下」—— 正是 §2 说的：靠**位置与列**表达层级。

## 先量（改前，720×1600，设计主题，深浅同值）

| 判词 | 改前盒子 | 改前墨 | 改后盒子 | 改后墨 |
|------|----------|--------|----------|--------|
| `ready`（`node v20.11.0`） | x=**628.8** w=61.0 y=406.6 h=15 | 628..688 | x=**201.8** w=67.0 y=400.1 | 201..267 |
| `missing`（`Not found on PATH: node…`） | x=358.8 w=**331.0** | 359..687 = **329** 列 | x=201.8 w=340.0 | 201..540 = **340** 列 |
| `checking`（`Checking the local toolchain…`） | x=548.8 w=145.0 | 548..687 | x=201.8 w=145.0 | 201..344 |

- 选择器提示的列：**x=201.8 / w=256**（改前改后不变）。
- `missing` 那句的自然墨宽（同尺子、无约束实测）：**340**；控制槽只给 **331** ——
  **11px 被省略号吃掉**，尾端的 `n.` 与句号从未被画出来。这行是「装好工具链再打开这个表单」，
  被吃掉的正是**出路**那半句（与 §6.81 的同族：披露的后半段没人读得到）。
- `tt.Texts()` 里 "Language" 出现三次（行标签 / 选择器 a11y 标签 / 这条运行时行的标签）。

## 落地

1. `internal/settingsui/settingsui.go:5590` `a.choose(...)` 加变参 `below ...func()`，
   直接转发给 `a.row`（`cardRow` 的 `below` 列，已按 `rowInset` 对齐，§6.81 同一机制）。
2. `scriptRows` 删掉整块 `a.row(c, copy.LocalToolLanguage, "", func(){ ui.Text(...) })`，
   改为把句子作为 `below` 传给选择器。
3. 新增 `scriptRuntimeNote`（`settingsui.go:5712`）：`FillWidth` + `Space(2.25)` 上呼吸、
   `tokens.Caption` / 500 / `TextMuted` —— 与 `cardProse`（§6.81）同一形状，
   字阶取 **caption**（本仓给机器事实的那一档，`.session-manager__meta`），
   比它上面的静态提示（body 档）低一档，所以两句话不是同一个音。
4. 文案、判词、探针缓存/epoch 语义、`row`/`cardRow` 的任何度量、选择器宽度：**一个字没动**。

## 台账（V / B / S）

- **V 0**：没有任何现有 pin 钉过这一行的几何（`go test ./internal/settingsui/` 改前改后全绿，
  227s 全包实跑）。旧几何此前**没有墨账**，本轮新增两条 pin。
- **B 0**：`scriptRuntimeLine` 的三种判词、探针缓存、epoch 丢弃、改名后的 `choose` 变参
  —— 相关测试零改动全绿（`pluginsheet_test.go` 的 8 条运行时/缓存测试）。
- **S 0**：没有结构改名/搬移，只把「一条设置行」降级成「一条 setting 的子行」。

## 新增 pin（`internal/settingsui/runtimeline_test.go`，3 个函数）

- `TestTheToolchainLineSitsUnderItsPicker`：三种判词 × 深浅 →
  句子左沿 == 选择器提示左沿、墨从盒子第一列起、在提示之下、在脚本编辑器之上。
- `TestTheToolchainLineIsNotCutWhenTheVerdictIsLong`：卡片里的墨宽 == 同句无约束墨宽
  （340 == 340）。**这条 pin 我第一版写错了**：原来写的是「720 帧与 1200 帧墨宽相等」，
  M2 变异时它是**绿的** —— 因为设置卡片有最大宽度，1200 帧根本没给句子更多地方，
  这条断言在旧布局下也成立（假 pin）。改成「与无约束自然墨宽相等」后变异如实变红。
- `TestDumpToolchainLineForP13dm`（opt-in `FLOTER_DUMP=1`）：三判词 × 深浅出 6 张图。

## mutation（2 条实跑变红并还原）

- **M1** `scriptRuntimeNote` 的行加 `.Justify(ui.End)`（把句子推回右沿）：
  `TestTheToolchainLineSitsUnderItsPicker` 6 条红，原始报错
  `[dark/ready] the toolchain line starts at x=621.8, its picker's hint at 201.8: a row's second line
  starts on the row's text origin, not in the control slot`（浅同值）。
  `NotCut` 仍绿 —— 右对齐只挪列、不减宽，符合预期。
- **M2** `scriptRows` 还原成旧的 `a.row(...)`：两条 pin 全红，`NotCut` 的原始报错
  `the verdict inks 329px in the card but 340px unconstrained: the sentence is cut`，
  `SitsUnder` 6 条红（`x=628.8 / 358.8 / 548.8` 对 `201.8`）。
- 两次还原后 `md5sum internal/settingsui/settingsui.go` 均为
  `4c5fd575042881d8174deb7ff97fe318`（与改后逐字节一致），`touch` 后复跑 pin 两条 `ok`
  （不吃测试缓存）。

## 门槛（实测）

```
gofmt -l cmd internal                             → 空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...  → 净
go test -count=1 ./...                            → 全绿（0 FAIL）
--- PASS                                          → 3320（基线 3318，+2 本轮 pin）
--- SKIP                                          → 153（基线 152，+1 本轮 opt-in dump）
测试函数                                          → 2481（基线 2478，+3）
GOOS=linux/windows/darwin go build ./...          → 过
```

## 出图（深/浅两套，改前改后）

`/tmp/floter-P13-dm-runline-<dark|light>-<ready|missing|checking>-<before|after>.png`（12 张）。
**这张图上看到的一处变化**：`dark-missing-before` 里那句「还没装」的判词是右半边一块
**到右沿就断**的文字（墨止于 687，卡片右沿 689.8，尾端没有句号），
`dark-missing-after` 里同一句话从左边 x=201.8 起笔、端点落在 540，
**句号和 `again.` 都在**（墨宽 329 → 340）；`light-missing` 同形。

## preflight（逐条，`references/preflight.md`）

- **A 反 AI 套路**：无渐变字、无发光、无紫蓝二手氛围层、无居中 hero、无 emoji 图标、
  无 `transition: all`（自绘无 CSS 过渡）；本轮只挪一行的列。**过**。
- **B 设计读取一致性**：「设置页 → 集成 → 本地工具编辑器，面向要接一个自己写的脚本的用户，
  用克制的终端工具语言，倾向本仓 `liquid-glass.md` 的 elevation 阶梯」；
  本轮改动没有引入第二种列原点（201.8 与全表的行文字原点一致）。**过**。
- **C 动效抽查**：本轮零动效；判词切换（checking → ready/missing）本来就是重绘，不加过渡。**过（不适用）**。
- **D2 中文页**：中文界面下这条句子同样是 caption 档 **9.9px**，
  与 preflight「正文 ≥ 14px」照旧冲突 —— **仍未过**（第 38 份证据，需用户拍板）。
- **A2.1 执行者路由**：默认由当前 pi 直接执行，未调用 K3。**过**。

## 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落数）

这一轮只动一行的**归属**，菜单里与「面/材质/氛围」有关的项不适用；取到的是这三项：

1. **第 6 项「编辑细节（hairline / 大写字距标签 / 页边注）」** → 落成**少念一遍标签**：
   "Language" 从 3 次降到 2 次（行标签 + 选择器 a11y 标签），
   这一行不再有自己的 emphasis 槽，那条 44u 地板行连同它的发丝线一起消失。
2. **第 5 项「多层染色阴影」的本仓等价物：同一平面上的列对齐**（§2 的机制）→ 落成
   **x=201.8**：句子的左沿 == 它选择器提示的左沿（旧 628.8 / 358.8 / 548.8 三种，
   同一个句子随判词长短换三个起点）。一个原点，三种判词都落在同一列。
3. **第 10 项「收尾彩蛋 / 只在特定时刻出现的细节」** → 落成**被吃掉的那半句**：
   `…Install it, then reopen this form to check again.` 的尾端 `again.` 此前 11px 被省略号取代，
   现在三种判词都读得完 —— 这一块唯一给出**出路**的那句话，第一次完整出现在版面上。

不适用并写明理由：第 1 项氛围层（桌面自绘面板，纹理上限 3%，本轮不碰面）、
第 2 项 `::selection`（mygo 自绘）、第 3 项品牌 focus 环（不在这一行）、
第 4 项签名交互（这一行没有交互）、第 7/8 项展示排版与流体字号（一句说明不需要展示级排版）、
第 9 项滚动条、第 11 项图形资产、第 12 项材质细节（字没有面）。

**两轮制**：第一轮 = `below` + `scriptRuntimeNote` 并出前后图 → 看图时发现
`NotCut` 的 720/1200 对照在旧布局下**也是绿的**（卡片有最大宽度，1200 帧没给更多地方），
第二轮只改这一处：把它换成「与同句无约束墨宽相等」，随后 M2 变异如实变红。

## 仍待（P13-dm）

1. **下一格（细节打磨 · 空态/满态）**：扩展页空态「只有一行结论没有出路」是**需拍板**项
   （不动）；不需要拍板的是工具表单里**还没量过盒子的两处** ——
   `toolContentActions` 的 Copy/Export 动作行（`settingsui.go:5407` 一带）与
   `paramempty`「The script runs fine without parameters.」那行空态（`paramempty_test.go` 只量过
   发丝线，没量过它的列与行盒）。先量再改。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口
   （§6.75/§6.76/§6.77），本轮一字未动，仍在队里。
3. **需拍板（沿用 + 第 38 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
