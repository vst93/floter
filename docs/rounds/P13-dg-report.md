# P13-dg 报告：夹具的尺子和生产的尺子不是同一个比例

**轮号**：P13-dg ｜ 规格 `docs/rounds/P13-dg-spec.md` ｜ 合约 `docs/liquid-glass.md` §6.76
**改动面**：`docs/` + `.pending-bugs.md`（**产品与测试代码零改动**，`git status` 可证）

---

## 1. 这轮做了什么

P13-df 证明了「夹具量的是框架库存主题，生产量的是设计主题」，并把它记成三行账
（启动器 21 / 终端页 8 / 设置页四十余条）。本轮把那三行账**从「数字」变成「判据」**：
两套主题差的是不是一个统一比例？如果是，收口就是重钉；如果不是，先得修帧高。

**答案：不是统一比例。间距差 0.90，字与行盒差 0.77。** 于是上一轮记的「21 条红」
不是 21 个数字，而是**三类手续**，并且其中 6 条不是重钉而是前提破。
这就是本轮把一格拆成三格（每包一格）的依据。

## 2. 实测（本轮自己跑出来的）

| 量 | 库存（夹具） | 设计（生产） | 比例 |
|---|---|---|---|
| `Spacing` | 4.0 | 3.6 | 0.90 |
| `FontSize` | 14.00 | 10.80 | 0.77 |
| `MeasureText(0, span "Ag", 正文)` | 22.0 | 17.0 | 0.77 |

装主题的位置：生产 `internal/shell/shell.go:4262`；夹具三个 `render()`
（`launcher/view_test.go:56`、`terminalui/view_test.go:22`、`settingsui/view_test.go:34`）都没装。

**为什么 0.90 ≠ 0.77 是这轮最值钱的一条**：一行的盒子在设计尺上同时受两条腿约束 ——
`Geometry.Height` 里 `floor = rowUnits * spacing`（0.90）与 `content = TitleLine`（实测行盒，0.77），
取 `max`。启动器实跑时行盒掉到 17、地板 36，**地板赢**，所以抖动在行上，
可在**页头/小节**这些只按行盒算的带上，0.90 与 0.77 的差就直接落在帧高里：
`TestTheHeadingBoxesStayOutOfTheNumberedFamily` 量到编号族从 9 条变 10 条（实测
`numbers=map[… 10:0]`），也就是**帧高在设计尺上多出近一行**。

## 3. 21 条红的分类（启动器，实跑失败串节选）

| 类 | 判据 | 例（实测串） | 手续 |
|---|---|---|---|
| **数字漂**（V） | 断言写着参考的 `Nu`，换算成库存 px（N×4） | `the field's caret starts at x=48, want the rows' title column 54`；`grew the window by 27, want the 30u status band`；`the field block's floor is at y=49, want y=55`；`the pin's resting box is 18.0 wide, want the reference's 20u`；`the pin key's box is 25.2 wide, want the strip's own 28u` | 钉到新值，注释写 `P13-dg 取景收口：54 → 48.6`。**14 条** |
| **前提破**（S） | 夹具开窗是库存尺算出来的，设计尺下「被裁的行」不再是同一行 | `the premise failed: the first row is only 1.000 on screen`；`one of the two frames has no ink in the row at all: -1, -1`；`the family has 10 entries, want 9`；`the viewport's bottom edge moved to y=419.2, want 466` | **换窗口**（帧高/滚动位置），不加容差、不放宽断言。**6 条** |
| **混排**（B 类，先跑 B） | 同一个测试里一半节点读设计阶梯、一半读库存主题 | `the launcher list's title starts at x=54.0, the output panel's at x=48.6: two lists in one card share one text column`；`the output group heading is 17 tall, the launcher's section title is 22` | **先把那一半也装上主题**，再看它是不是产品回归。**这条正是 P13-df 记的第三类**，本轮把它量成了具体两条串。 |

> 注：`x=54.0` 对 `x=48.6` 这一条同时说明「混排」有多危险 —— 像素断言**照样全绿的那一半**
> 量的不是这个设计，而这一条之所以红，是因为两个包各自读了自己的尺子。

## 4. 终端页（`render()` 单点装主题，实跑）

失败串（节选，逐字抄）：

- `en 640x360: the drawer reports no scroll range (MaxY=0.0) — the strip is 309.2u tall and its content does not fit`
- `light 640x360: the row above the strip (y=49) is the seam colour [255 255 255] — the hairline is painted outside the drawer, on the canvas`
  `light 640x360: the window's bottom row is [230 230 230], not the drawer's face [236 236 236] — the hairline belongs on the drawer's top edge, not on the window's bottom edge`
  （同一串在 `1200x800`、深色两套外观上**逐条重现**：`dark 640x360: … [24 24 27]` / `[39 39 40]` 对 `[58 58 59]`）
- `pageheight_test.go:129: light: "Process exited · code 3" (the resident note's first line) is 17.0 tall, not 22.0`
- 抽屉阶梯：`rungs [17 20 22 28 30]` 实画 `[15 17 25.2 27]`（20→15、22→17、28→25.2、30→27）
- 终端页阶梯：`rungs [15 22 23 28]` 实画 `[15 17 18 25.2]`

两条重要后果：

1. **抽屉的顶缝在设计间距下丢了**（`y=strip.Y` 是画布白、`y=strip.Y+1` 才是抽屉面，那道
   发丝线跑到了窗口底边）。这条在 4 个窗口 × 2 套外观上**逐帧重现**，而且它整个都发生在
   装好主题的 `render()` 里 —— **可信度高于启动器那批**（那批有混排）。登记为 **P13-dh**，
   独立一轮，属「空间感」：抽屉与画布之间那道缝就是层级那一层。
2. **抽屉在 640×360 上报不出滚动范围**（`MaxY=0.0`，条带 309.2u）：设计尺下内容不再溢出，
   夹具的前提（「最后几行被裁、要能滚到」）消失。这属于 **S**：加行、不是改断言。

## 5. 拆轮（`§6.75` 一格的处置）

| 格 | 表面 | 形状 | 现状 |
|---|---|---|---|
| `P13-dg-l` | `internal/launcher` | 21 条：14 V + 6 S + 1 混排（+ 取景必修的 `frameHeight`） | 未开始 |
| `P13-dg-t` | `internal/terminalui` | ≥8 条：阶梯 4 条 V + 抽屉 2 条 S + 缝 2 条（缝转 P13-dh） | 未开始 |
| `P13-dg-s` | `internal/settingsui` | 四十余条（P13-dd 已记），`renderThemed` 已有，只差整体收口 | 未开始 |

**顺序**（写死给下一轮）：`P13-dh`（抽屉顶缝，产品缺陷，有图可看）→ `P13-dg-t` 收口
（它顺带把 dh 的 pin 钉在装好主题的取景上）→ `P13-dg-l` → `P13-dg-s`。
理由：dx 是**产品回归**，而 dg-* 是**夹具债**；先把用户看得见的那条修掉。

## 6. 偷了什么（qiaomu-design）

本轮读了 `SKILL.md` 的 Router 表与 `references/user-preferences.md` 的 P-1 / P-2 / P-4 / P-5。
走的是「已有方向，只要打磨」那条路径（`craft-loop.md` + `preflight.md`），**不走**四方向预览。

- 从 `craft-loop.md` 的**工艺密度**里取的是「**一把尺子**」这一条（间距、字号、行盒必须同源缩放），
  落成判定：`Spacing 0.90` 对 `FontSize 0.77` ⇒ 帧高不能只按间距算，必须把实测行盒喂进
  `Geometry{Font, TitleLine, SubtitleLine}`（`internal/launcher/height.go` 的 `Height()` 早就支持，
  P13-dg-l 的落地项就是它）。
- 该条同时给出了 21 条红的**分类判据**（同源 = V；不同源且前提消失 = S），这就是报告 §3 的表。
- 本轮不产像素（零代码改动），所以 `craft-loop` 的其余工艺项（光学对齐、字重配对、留白节奏）
  与 `motion-craft` 对本轮**无适用面**，不做样子。

## 7. preflight 逐项

- **A（反 AI 套路）**：本轮无新像素 → **N/A**。既有禁令（§1 四条：blur 冒充玻璃 / glass 叠 glass /
  渐变白边 / 组件层自造透明）本轮**未被触碰**。
- **B（设计读取一致性）**：**本轮就是 B 项本身**，结论是**不一致**：夹具 `14.00/4.0` 对生产
  `10.80/3.6`，同一张卡上 `54.0` 与 `48.6` 两个文本列。**未通过**，处置 = 拆三格（§5）。
- **C（动效抽查）**：N/A（无动效改动）。
- **D2（中文页正文 ≥14px）**：**仍未通过**，与沿用第 33 份证据同源：`tokens.Caption` 9.9px、
  正文 10.8px。**需拍板**，不擅自改（改字号会动整页高度链）。

## 8. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows` / `GOOS=darwin`：净
- `go test -count=1 ./...`：全绿；**PASS 3311 / FAIL 0 / SKIP 148 / 2467 测试函数**（与基线逐项相同）
- 三平台 `go build ./...`：过
- **变异：不适用**（本轮无代码可注入缺陷；照实写，不用跑绿冒充）

## 9. 仍待

1. **P13-dh（下一格，独立一轮）**：终端抽屉的顶缝在设计间距下丢失 —— 复现串见 §4，
   `y=strip.Y` 是画布白、`y=strip.Y+1` 才是抽屉面，发丝线跑到窗口底边；4 窗口 × 2 外观重现。
2. **P13-dg-t / -l / -s**：见 §5，顺序 `dh → dg-t → dg-l → dg-s`。
3. **捕获输出页在页头的呼吸带里起笔**（本轮附带量到，未证实）：设计尺下
   `strip's last ink y=117, viewport top y=100.4`，列表起点高出条带末墨 16.6px；
   它出现在混排取景里，要先用装好主题的单点夹具复现再定，登记给 `P13-dg-l`。
4. **需拍板（沿用 + 第 33 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
5. **抢救 stash 核对（第三十四次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
