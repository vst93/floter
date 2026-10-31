# P13-an 报告：About 页的只读事实——同一条线上，名字是标题、值是内容

## 意图

`keyValue`（`internal/settingsui/settingsui.go:4106`）是全仓最后一族**只读事实行**：
About 卡片的 `框架` / `链接协议` / `设置文件`（`:3993-3995`）与工具编辑器里不可变的 `集成 ID`
（`:4890`）。它借用了设置行的墨语义 —— 名字拿 emphasis、值塞在控制槽里用最轻的灰。
这轮把墨对调，盒子一个像素不动。

## 这轮为什么让它更轻 / 更有空间感

一行里「谁说了算」是反的：小字说「它是什么」，大字说「这是什么」。实测（`inkSpan` = 盒子内
luma 跨度，760×1400，深浅两套）：

| | 名字（旧） | 值（旧） | 名字（新） | 值（新） |
|---|---|---|---|---|
| light | 202 | 101 | **122** | **202** |
| dark | 198 | 94 | **115** | **198** |

换过之后这张卡片从「设置列表的续页」读成一张**身份表**：11px 的标题说明这是哪一项，
15px 的事实是这一行真正的内容，`/home/…/settings.json` 这一行终于成了全卡片最重的东西
—— 而它正是用户打开这一页唯一想找的字符串。

## 落地

- `settingsui.go` `keyValue`：`a.row(...)` → `a.cardRow(c, ui.Center, nil, rowFace{}, 值, 名字)`。
  名字 `tokens.Caption` / 500 / `TextSecondary` / `SingleLine` / `Ellipsis("…")`；
  值 `t.FontSize+1` / 580 / `TextStrong` / `Selectable()`（**保留可选中**：用户要复制路径）。
  `cardRow` 的框架（44px 地板、分隔线、1px 行距、`rowInset`、左右两端）逐字沿用。
- `internal/settingsui/keyvalueink_test.go`（新）：值比名字重 ≥60 级 / 名字 ≤150 级 /
  两段墨的竖带重叠 ≥ 短者 60%（实测行 1 **8/9**、行 2 **9/9**、行 3 **12/12**）/ 值在名字右侧 /
  三行仍按 **45px** 递进，× 深浅两套；附 opt-in 出图。
- `docs/liquid-glass.md` **§6.11**（14-1..14-8，两条 `故意不做`：不叠成两行、不用 `ui.Baseline`；
  一条 `需拍板`：控制槽 `Shrink(0)` 的框架账）。
- i18n、几何、行为：零改动（文案一个字节没动；行距 45px、卡片高度、可选中、长路径折行都不变）。

### 关于 `ui.Baseline`（量过才放弃的）

11px 的标题和 15px 的值按理该共基线。实测 `AlignItems(ui.Baseline)` 会把这**一对**
从行中心推到行的上内边距（pair 上移 **4.9px**：上 9px / 下 17px）—— 本框架的
`alignBaselines` 只对齐基线、不重新居中（`ui/form.go:284-318`）。留在 `ui.Center` 后，
两段墨的竖带重叠是 8/9、9/9、12/12，即已经是同一条光轴；所以这一条记 `故意不做（框架对齐给的账）`。

### 关于窄窗（量过才收的尾）

值从 288px 长到 **339px**（+15%），640 宽的窗里标签列从 **112px** 掉到 **61px**，
11px 的 `Settings file`（约 66px）会被硬切。所以名字加 `Ellipsis("…")` —— 宁可省略号，
不要半个字，与 `rowValue` 对「程序起的名字」的截断是同一种（`rows.go` `rowValue` 注释）。
**结构性的那一半**（`cardRow` 的控制槽是 `Shrink(0)`，长值先吃标签列）是框架布局账，
写进 `.pending-bugs.md` 的「需拍板」。

## 门槛（最终字节，实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → 全绿，**FAIL 0**
- `go test -count=1 -v ./... | grep -c -- "--- PASS"` → **3050**（基线 3049）
- SKIP **95**（基线 94，+1 = 本轮 opt-in 出图）；测试函数 **2257**（基线 2255，+2）
- `GOOS=linux/windows/darwin go build ./...` → 三平台过
- **V 1 / B 0 / S 0**（V = 本轮新增 pin；没有旧 pin 被改写，没有任何测试被删/放宽）

## 变异（实跑，逐条红、md5 逐字节还原）

- **M1** 值换回旧墨（`t.FontSize` / 400 / `t.TextMuted`）：
  `keyvalueink_test.go:108: light: "Framework" resolves 122 levels and its value "mygo 0.9" only 101 — the fact must out-ink the name of the fact by a whole rung (§6.11)`（三行 × 深浅共 6 条）。红。
- **M2** 名字换回 emphasis（`+1pt` / 580 / `TextStrong`）：
  `keyvalueink_test.go:113: dark: the fact's name "Link scheme" resolves 198 levels — on a fact sheet the name is the caption over its value…`。红。
- **M3a** 值字号 +4（`t.FontSize + 5`）：**不红，如实记**——行高由 44px 地板决定，
  值 17→22px 仍在盒内，45px 行距不变（这正是「盒子没动」的证明方向）。
- **M3b** 值字号 +13（27px）：`keyvalueink_test.go:139: dark: row 1 starts 52.19998 below the row above it, not the card's own 45px pitch`。红（说明行距那条 pin 有牙）。
每条跑完 `cp` 回原文件并 `md5sum -c`：`4a617c950cc0b5348dec557b8d87293d` **成功**，再 `touch` 复跑。

## 出图

`/tmp/floter-P13-an-about-{light,dark}-{before,after}.png`（760×1400，四帧）。
整帧 diff：light **3533 px**、dark **3533 px**，bbox `(200,147)-(732,252)` ——
**恰好是三行文字的那块矩形**（x 从文字列起点 200 到值的最右端 732），卡片边框、行分隔线、
`Project` 链接行、下方两张卡片一个像素没动。

图上看到的一处变化：第三行 `/home/tar/.config/floter/settings.json` 现在是自己那一行里
**最重**的东西（值 202 / 名字 122），而在 before 帧里它是这一行最淡的东西（101 / 202）——
同一行上轻重关系整个翻了过来，位置一个像素没挪。

## 偷了什么（`qiaomu-design` / `craft-loop.md`）

- **DNA 注入（`design-systems/linear.app/DESIGN.md`）**，取 3 个具体值：
  1. **Micro 11px / 510** 的极小标签档（`DESIGN.md:95`）→ 名字落在本仓既有的
     `tokens.Caption` = 11px / 500，而不是新造一个 12px 档；
  2. **「夹在中间的重量」510/590**（`DESIGN.md:7`）→ 本仓已有的 500 / 580 就是这一手，
     这轮把 580 从名字挪到值上，字阶本身一个数没造新的；
  3. **元数据用三级/四级灰、值用主墨**（Tertiary `#8a8f98` / Quaternary `#62666d`，`DESIGN.md:34-35`）
     → `TextSecondary`（名字）↔ `TextStrong`（值）。
- **工艺密度菜单**：
  - 取 **#6 编辑细节（小写字距标签）**：名字 = 11px / 500 / `TextSecondary`，与值 15px 的
    大小比 **0.73**，落成「眉批 + 内容」的编辑式读法；
  - **#5 多层染色阴影**按反面对执行：§2 判据 2「E1 不许外投影」，只读行不加阴影、不加面；
  - **#12 材质细节（内边框高光）**不适用：本轮不换材质，只换墨（材质账在 §6 早几轮已收）。
- **两轮制**：第一轮换墨 + 量（六条墨账）→ 四视角评审挑出两处（见下）→ 第二轮只修这两处
  （`ui.Baseline` 的取舍、窄窗 `Ellipsis`），没有再扩大范围。
  - 艺术总监：值变重后，About 页头带（build 身份）与卡片里的值会不会抢？→ 实测头带仍是
    唯一的大字号，卡片最高 15px，层级没乱，保留。
  - 工程师：窄窗标签列被吃（61px）→ 加了 `Ellipsis`，框架那一半留给拍板。

## preflight（`qiaomu-design/references/preflight.md`）

- **A 反 AI 套路**：过。没有渐变字、没有外发光、没有玻璃叠玻璃；这轮只动两个已存在文字元素的墨，
  没有新增任何装饰元素，也没有新增颜色（用的全是 `theme.Tokens` 里已有的档）。
- **B 设计读取一致性**：过。单一来源是 `docs/liquid-glass.md` §6.10（P13-ak）立的
  「名字退一档、事实占 emphasis」；本轮是它在**同一行两端**的写法（§6.11 14-5 写明为什么不叠两行）。
- **C 动效抽查**：不适用/过。无新增动效；行本身仍走框架 120ms 的颜色过渡，`reduce-motion` 下语义不变。
- **D2 中文页追加项**：**部分不过，如实记**：
  - 中文用系统字体栈 / 中西文间距 / 全角标点：过（文案零改动）。
  - 「正文 ≥ 14px」：值 15px **过**；名字 11px 属眉批不是正文，但 11px 与 D2 的下限确有张力
    —— 这一条 P13-ak「仍待 1」已经记过（`tokens.Caption` = 11px 是既有字阶，要动就是动整条字阶）。
  - 「字重只用 400/500/600」：名字 500 **过**；值 580 是**本仓既有的 emphasis 档**
    （`rowTitle` 一直用它，P13-ak 也是 580）。为了只读行这一族不在仓库内部分叉，这轮沿用 580，
    没有为了对齐 D2 单独改成 600（改就得改整族）。**这一条是已知的、有意的偏离。**

## 仍待（P13-an）

1. **需拍板（§6.11 14-8 的后半）**：`cardRow` 的控制槽是 `Shrink(0)`，长值先吃标签列。
   本轮只给了省略号（局部），根治要改框架的收缩规则 —— 那会影响全仓每一行（B 风险），
   要拍板。
2. **需拍板（沿用 P13-ak 仍待 1）**：`tokens.Caption` = 11px 与 preflight D2「正文 ≥ 14px」的张力；
   本轮把这档用在**名字**上（P13-ak 也用了），所以这条更该有人拍一次。
3. 下一个候选：`terminalui.statusDot` 的墨在两套外观下的级数；`ContentAlpha()` 沿透明度滑块的
   两端（任务书「先量再改的两个数」之一，仍未量）。
4. 抢救 stash 本轮复核未变（`114b7d15` / `afc38b24` 内容已在 HEAD），按纪律没有静默 drop，
   也没有再试合。
