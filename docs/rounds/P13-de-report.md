# P13-de 报告：启动器的字号也是「阶梯上的档」——4 处落地，5 处撞在取景上

## 一句话

把启动器里 `t.FontSize ± N` 的 9 处偏移量搬到字阶档位：**4 处落地**（过滤行提示、
作用域标记的名字、两步删除的确认句、分页脚注的词），**5 处不许动**——它们不是排版问题，而是**测试取景**
问题（P13-dd 的 `73-5`），量出来的红是「pin 在量一个不存在的界面」，不是产品回归。

## 先量（`ui.Tester`，**画出来的**盒子，不是 `MeasureText` 的数）

P13-dd 的尺子在这一个包上**不能用**：`MeasureText(0, …)` 走的是画笔默认字体，页面画出来
的字是主题那一族，同一句话两把尺子差 2px（实测：`body-1` 在 tiny 档上 `MeasureText` 说 15、
画出来是 13）。所以本轮把尺子也画进同一套主题里：期望值是**同一个字符串在同一个主题下按
两个寄存器各画一次的盒子**（`p13deProbeBox`）。tiny（0.8）档实测：

| 寄存器（画出来的行盒） | 盒子 | 出货档 0.9 |
|---|---|---|
| `tokens.Caption` | **14** | 15 |
| `t.FontSize - 1` | **13** | 15 |
| `tokens.Title` | **17** | 19 |
| `t.FontSize + 2` | **18** | 19 |

出货档上两两重合（这就是这些偏移量一直没被发现的原因）；tiny 档上分家 1px，pin 才有眼睛。

## 改了什么（4 处，全部是 `caption`）

| # | 位置 | 旧 | 新 |
|---|---|---|---|
| de-1 | `internal/launcher/filters.go:235` | `t.FontSize - 1` | `tokens.Caption` |
| de-4 | `internal/launcher/launcher.go:1535` | `t.FontSize - 1` | `tk.Caption` |
| de-7 | `internal/launcher/launcher.go:2743` | `t.FontSize - 1` | `tokens.Caption` |
| de-8 | `internal/launcher/output.go:345` | `t.FontSize - 1` | `tokens.Caption` |

行盒、字重、墨、间距、i18n 文案、高度链**零改动**：出货档上这四个盒子逐像素不变
（15 = 15），变的只有 0.1pt 的字宽——真正变的是**它们从此跟着界面步长缩放**。

## 没改的 5 处：一条一条撞在取景上（本轮最重要的发现）

`internal/launcher` 的测试夹具用 `render(t, a)`，它**不** `c.SetTheme(a.tokens(c).Theme)`；
而生产里 shell 在画面板前就装好了设计主题。于是同一个函数里 `t := c.Theme()` 是框架库存的
**12px**，`a.tokens(c)` 是设计阶梯的 **10.8 / 9.9 / 12.6**——夹具量的是两套字号混排的界面。

| # | 位置 | 改法 | 红在哪（实跑原话） |
|---|---|---|---|
| de-2 | `launcher.go:1303` 计费的行标题行盒 | `rowTitleLineSize(tk.Theme)` 具名助手 | `view_test.go:2781: the title wrapped: {… H:24}, one line is 22.0`（`TestLongRowTextIsCutNotWrapped`） |
| de-3 | `launcher.go:1306` 副标题行盒 | `tk.Body` | 同上一条同源（计费与绘制的字号在这一帧里本来就不是同一个） |
| de-5 + de-11 | `launcher.go:1633` / `triggerhint.go:153` 助记提示的绘制与量算 | `tokens.Caption` | `triggerhint_test.go:146: the nudge's box = 127, want the row's room 134`（`TestTriggerHintFits`） |
| de-6 | `launcher.go:2646` 结果行标题 | `tokens.Title` | `textheight_test.go:67: light en match: height 19.0 is not on the ladder [15 18 22 24 25 28]`（18 / 24 两档消失） |
| de-9 | `output.go:695` 捕获输出页状态条 | `tokens.Caption` | `outputedges_test.go:74: the band under the strip (y=118..133) carries ink 228`（`TestTheOutputListNeverScrollsIntoTheHeaderBand`） |
| de-10 | `output.go:980` 便条行副标题 | `tokens.Caption` | `outputsubtitle_test.go:58: the chosen row's subtitle ink is -9 levels from the selected result row's (122 vs 131)` |

**这不是「改不动」，这是「尺子必须先换」**：这 5 处每一条 pin 都在量一个生产上不存在的帧
（库存 12px 的主题）。顺序只能是 **先收口夹具取景（`73-5`）、再搬这 5 处**，否则就是拿
「夹具的字变小了」当成「产品变了」去重钉基线。所以它们仍然 `未开始`，理由写进合约 §6.74。

## 台账

- **V 0 / B 0 / S 0**：一条既有 pin 都没动。
- 新增 2 条像素 pin（`internal/launcher/fontladder_test.go`）：
  `TestTheFilterRowsHintIsACaption`、`TestTheArmedDeletesConfirmIsACaption`——两条都在 tiny 档
  上量，并在开头先断言「这个档位上两个寄存器真的画得不一样」，pin 不会因为两档重合而空转。
- 新增 1 条 opt-in 出图 `TestDumpP13DEFontLadderPNGs`（`FLOTER_P13DE_DUMP=1`）。

## 门槛（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                 # 全绿（无 FAIL 行）
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 3311（任务书基线 3300）
测试函数数                                             # 2467（基线 2453）
GOOS=linux/windows/darwin go build ./...                # 过
```

`--- SKIP` 137（opt-in 出图各一条，未变）。

## 变异（3 条，实跑，全部还原并 `md5sum` 逐字节核对 + `touch` 复跑）

1. M1 `filters.go` 的提示改回 `t.FontSize - 1` →
   `fontladder_test.go:130: the hint's line box = 13, want the caption rung's 14 (body-1 draws 13)`。
2. M2 删除确认句改回 `t.FontSize - 1` →
   `fontladder_test.go:98: the confirmation's line box = 13, want the caption rung's 14 (body-1 draws 13)`。
3. M3 删除确认句**过冲一档**到 `tokens.Emphasis` →
   `fontladder_test.go:98: the confirmation's line box = 15, want the caption rung's 14 (body-1 draws 13)`。
   （第一次写的是 `tk.Emphasis`，那个作用域里没有 `tk` → **编译错**，不算变红；换成 `tokens.Emphasis` 重跑。）

三次还原后 `md5 ok`，`touch` 再跑两条 pin 全绿。

## 出图（两套外观，tiny 档，画面上有那句被改的确认句）

`/tmp/floter-P13-de-launcher-{light,dark}-{before,after}.png`。
逐像素差分（自写 `/tmp/pngdiff.go`，stdlib）：浅 **368 px**、深 **370 px**，
bbox `(570,83)-(657,92)`——只有行尾那句 `Delete…` 的墨。

**这张图上看到的一处变化**：浅色 before 里那句确认句比行内其余事实**矮 1px、且右端更靠右**
（13 高、11pt 的旧档），after 里它与同行的 caption 事实同高、右端退回 1px——因为它的行盒
现在由阶梯的 caption 管，不再由「body 减一格」这个数字管。

## 偷了什么（qiaomu-design）

- 读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md`（本轮走「已有方向，
  只要打磨」车道，不走近四方向预览、不出 HTML）。`preflight.md` 的结论见下。
- `craft-loop.md` §一 DNA 注入：从参考的 `settings.css` / `extensions.css` 偷了**一个动作**——
  小字一律写成 `var(--text-caption)` **这个名字**，不写 `body − 1px`（`extensions.css:960-970`）。
  落成：`tokens.Caption` 是名字，`t.FontSize - 1` 是数字；数目上它们相等，步长上它们不等。
- `craft-loop.md` 菜单**第 6 项「编辑细节」**：阶梯就是编辑刻度——一页只有五档字，
  非档位的偏移量等于在刻度之间画了半格。落成 de-1 / de-4 / de-7 三处换成档名，
  并把「为什么另外 5 处不换」写进合约（§6.74）而不是静默留下。
- `craft-loop.md` 菜单**第 5 项「多层染色阴影」**：本轮**不取**——这一族改的是字，不是材质；
  在字号轮里加阴影就是拿装饰冒充进步（§1 的材质不变量也不许在 E1 上长影子）。
- `craft-loop.md` §三「两轮制」：第一轮改 4 处 → 出图（before/after × 深浅）→ 逐像素复核
  （368 / 370 px，bbox 只在那一行）→ 第二轮只做「把撞红的 5 处写成结论」这一件事。
- `preflight.md`：**A（反 AI 套路）过**——没有玻璃/渐变/光晕/噪点，本轮一个材质都没动；
  **B（设计读取一致性）不能过**——夹具与生产不是同一套字号（见上 6 条），这是本轮记下的
  唯一的 B 项缺陷，靠 `render()` 取景收口才能过；**C（动效抽查）不适用**（无动效）；
  **D2（中文正文 ≥14px）不能过**——本仓 caption 9.9 / 正文 10.8，这是那份长期证据的
  第 31 份，仍需用户拍板。

## 仍待（P13-de）

1. **下一格（不需拍板，首选）**：`internal/launcher` 的夹具取景收口——`render()` 装上设计
   主题（或加 `renderThemed`，与 P13-dd 在 `settingsui` 的做法同一条路），逐条重钉受影响的
   pin；之后本轮剩下的 5 处（de-2/3/5/6/9/10 + de-11）才能搬，`internal/ci` 的字阶账也
   才能扩到 `internal/launcher`（现在扩过去只能靠「给调用点开例外」，那是把账写软，不做）。
2. **需拍板（沿用 + 第 31 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2
   「正文 ≥ 14px」。
3. **需拍板（沿用）**：会话行 56 地板 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行
   结论没有出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow`
   控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 终端弧 S1b / S1c / S2 尾。
