# P13-ed 报告：外观取景（`renderIn`）换到出货尺

选题：`P13-ec`「仍待」第 1 条 = `liquid-glass.md` §6.86 的 73-5 尾巴。表面 = **测试夹具**
（不是界面）。规格 `docs/rounds/P13-ed-spec.md`，合约新增 **§6.99**。

## 这轮为什么让它更轻、更有空间感

这一族 40 个调用点此前把「面」画在一张 app 不画的卡上（帧 488 而不是 439，卡面 253 而不是
246），量的沟是 8 而不是 7。换尺之后，**这些 pin 读的墨与面差就是出货尺的那一套**：间距节奏
（8u = 7.20）、层级（卡面 246 / 深 15）、受光边都按 3.6 的步进成立 —— 后面每一轮「细节打磨」
的读数不再建在一张虚构的卡上。

## 先量

把 `renderIn` 换成 `SetTheme(a.tokens(c).Theme)` + `themedFrameHeight` 之后，**6 个测试函数**
变红（40 个调用点里绝大多数读的是相对量）：

| 红 | 真实原因 |
|---|---|
| `TestTheActionBarTakesTheSelectionOnACommandLine` / `TestTheListIsTheCentredReferenceColumn` / `TestTheFooterIsABlockWithTheWeakRule` | `gutter` 读的是 `ui.LightTheme().Space(2)` = **8**，出货尺画 **7**（8u × 0.9 = 7.20） |
| `TestTheHoveredFieldControlIsAFaceNotARing` | 悬停小格盒子在 `615.6`、宽 `25.2`，painter 画 `[616, 641)`；pin 用 `int()` 截断读 **615** = 地面 → 假的「画了一圈环」 |
| `TestThePowerGlyphsWearTheSystemIconInk` | 图标板取样点 `title.X − 38` = **10.6**，落在 28u 板的左沿**外一格**（板从 11 起），读的是面板 |
| `TestTheFieldTailIsStrongerThanTheChipsAside` | 阈值 `+50` 是库存尺的读数（当时尾部读的是**框架**的 `TextMuted`、旁注读的是**本仓**的 `TextTertiary`，跨调色板差 84）；出货尺两支同调色板，差 **41 / 69** |

## 落地

1. `appearance_test.go:19` `renderIn` 装主题 + `themedFrameHeight(maxResultSlots)`（与
   `renderShipped` 同一个寄存器）。
2. 五处 `gutter` 换 `int(math.Round(8 * designUnit()))`（`actionbarfooter_test.go` × 4、
   `raycast_test.go:319`，与 `raycast_test.go:194` 早已在用的写法同一个）。
   **耦合**：`TestTheActionBarSquaresItsTopCornersOverAList` 在中间态（沟换了、尺没换）红过
   —— 沟与尺必须同一轮动。
3. `plateface_test.go` 两侧列按 painter 的取整读（`round(x)` / `round(x+w)−1`），与 §6.77
   `readFace` 的 `int()` → `math.Round` 同一条教训。
4. `selface_test.go` 板面取样移进格子（`int(cell.X)+3`）；墨的判据换成 `coverageOver`
   —— **反解覆盖率**而不是要求精确等于 token 色：出货尺 10.8px 的描边没有全覆像素
   （实测浅/深都是 **0.90**），三通道必须同意同一个 alpha，且 ≥ 0.8；`Warning` 反解失败
   （红通道 alpha 1.44 / 绿通道越界）—— 判据仍然只认「哪一种墨」，不是「有没有墨」。
5. `fieldtail_test.go` 阈值 `50 → 30`（实测 41 / 69）。
6. 新 pin `TestTheAppearanceFixtureDrawsTheShippedFrame`（帧高 439 / 发丝 49 / 行块左沟 7，
   两套外观）+ opt-in 出图 `TestDumpP13EDAppearancePNGs`。

台账 **V 5 / S 2 / B 0**（V：沟 ×2 函数、尾部阈值、小格取整、板面取样；S：取景换尺、
墨判据换 `coverageOver`）。**产品代码 0 行**。测试函数 2498 → **2500**，PASS 3328 → **3331**，
SKIP 162 → **163**（新增出图）。

## 变异（4 条实跑变红并还原，`md5sum -c` 逐字节）

- **M1** `renderIn` 退回库存尺 → 5 红：
  `the appearance fixture draws a 488px frame, want the shipped ruler's 439px` /
  `the field block's hairline is at y=55, want 49` / `the row block starts at x=8, want 7`。
- **M2** 尾部墨 `t.TextMuted` → `tk.TextTertiary` →
  `the field's own instruction is 308 from the ground, the chips row's aside is 308`（深 292/292）
  —— 证实这条 pin 只靠**墨差**变红，不靠字号。
- **M3** 图标墨 → `tokens.Theme.Warning` →
  `the power glyph is {165 52 45}, want the amber system-icon ink {162 77 21} … (recovered coverage 0.00)`。
- **M4** 小格读回 `int(clear.X)` 截断 →
  `the hovered control's left column at mid height is {246 246 248} but its face is {220 220 222} — a ring is drawn`。

## 出图

`/tmp/floter-P13-ed-{stock,shipped}-{light,dark}.png`（剪贴板页，两把尺子各两套外观）。
共同区域 720×439 上逐像素 diff 浅 **316049** / 深 **316059**，最大通道差 247 / 230；
**卡自己的面 253 → 246（浅）/ 23 → 15（深）**，帧高 488 → 439。
图上看到的一处变化：卡面从 `253` 掉到 `246`，发丝从 `y=55` 搬到 `y=49` —— 夹具此前画的是
一张比 app 更亮、更高的卡。

## 用 skill（qiaomu-design，打磨车道）

读了 `SKILL.md` → `references/user-preferences.md`（P-1 无斜体 / P-5 可读性优先 /
P-17 自发现问题必须沉淀）→ `craft-loop.md` + `preflight.md`。

**偷了什么**（craft-loop 工艺密度菜单，逐条落到数值）：

- 第 **5** 项「多层染色阴影」→ 判据的**反证**：E1 面不许有外投影，`plateface` 的两侧列读法
  正是「面不是环」那条；本轮把它的读数修到 painter 的格子（616/640），使 E1 与 E2 的分家在
  1x 屏上成立。
- 第 **12** 项「材质细节：内边框高光」→ 受光边是**一条 1px 实色线**（禁渐变白边）；本轮
  没有新增任何高光，只是让 pin 量到 app 真画的那条。
- 其余 10 项（氛围层 / 品牌 focus 环 / 流体字号 / 自定义滚动条 / 彩蛋…）与「面」无关，
  **不硬凑** —— 这一轮是取景轮，不是造型轮。

**preflight 结论**：A（反 AI 套路）过 —— 无新增装饰，禁令 1/3 仍由 `elevation_contract_test.go`
与 pin 守着；B（设计读取一致性）**变好** —— 这一族此前读框架库存主题、现在读 `a.tokens(c)`
同一套墨；C（动效抽查）过 —— 无动效改动，`Advance(500ms)` 仍在两套外观里等过渡落地；
**D2（中文页）未过** —— `tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」，第 53 份证据，
仍在「需拍板」。

## 仍待（P13-ed）

1. **下一格（首选）**：`renderAt` 的 10 个调用点仍配 `frameHeight(maxResultSlots)`（库存尺）
   —— 显式开窗那一族的「满列表帧」那一半。`filesframe` / `modetextframe` 的 `size[0], size[1]`
   调用点是**窗口本身当被测对象**，另判。
2. **需拍板（沿用）**：便条行的 rank 1.28 / `tokens.Caption` 9.9px / 终端会话抽屉失败只加字
   不换墨 / 扩展页空态只有一行结论 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹
   提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
3. 抢救 stash 核对（第五十七次）：顶部三条仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
