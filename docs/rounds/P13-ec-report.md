# P13-ec 报告 · 默认取景换到出货尺（73-5 收尾）

轮号 `P13-ec`｜表面 = **测试夹具（不是界面）**｜规格 `docs/rounds/P13-ec-spec.md`｜
合约新增 `docs/liquid-glass.md` **§6.98**

## 1. 选题与意图

`P13-eb`「仍待」第 2 条 = `liquid-glass.md` §6.86 的 **73-5**（`internal/launcher` 的
`render()` 四十多条 pin 在量库存字 14）。`dg-t`（terminalui，P13-dp）与 `dg-l`
（launcher 的像素断言，P13-dq…P13-eb）都已收口，只剩这一包**默认的那把尺子**还画着
框架库存主题（`Spacing 4.0` / `FontSize 14`）。

**一句话意图**：让 `render()` 的 180 个调用点量的都是用户真看到的帧，并把 5 条变红的
断言按 V / S 逐条重钉，一个容差都不加。

## 2. 先量（动手前）

把 `render()`（`view_test.go:147`）临时换成 `SetTheme` + `themedFrameHeight`：
**5 个测试函数红**（比 §6.75 估的「四十多条」少得多——因为 `dg-l` 那几轮已经把
像素断言搬到 `renderShipped` 上了，剩下的是还在用默认取景的那些）：

| # | 测试 | 失败串（原文） | 类 |
|---|---|---|---|
| 1 | `TestTheSearchRowLeadsWithANeutralMark` | `65 ink pixels in the 10u gap (x=44..53)` | V |
| 2 | `TestBrowserRowsDropTheIconPlate` | `the browser row's icon cell is plated ({232 232 233 255} over the row's own bare fill {207 207 208 255})` | S |
| 3 | `TestTheResultListNeverScrollsIntoTheFieldBand` | `the field block's hairline is at y=49, want 55` | V |
| 4 | `TestANoteRowsTitleOutweighsItsSubtitle` | `light note: title ink 8083, subtitle ink 6322 (ratio 1.28)` | V |
| 5 | `TestTheOutputStripControlsAreE1Faces` | `the pin key's box is 25.2 wide, want the strip's own 28u` | V |

**两条要单独说的量**（本轮真正的发现）：

- **#4 不是容差问题：便条行在生产上真的只有一条腿。** 标题画在 `t.FontSize`
  （`output.go:969`），副标题画在 `tokens.Body`（`output.go:996`）——出货档两者**都是
  10.8**，rank 只剩字重（580 对 400），实测 **1.28**。库存取景里标题是 14、副标题是
  10.8，才有 **1.41**：那个帧生产上不存在。**#4 的 1.35 门槛从来没在出货尺上量过。**
- **#2 的图标格没有板。** 逐像素扫过浏览器行整条带（`row0 = {7.2 72 705.6 36}`，
  `y=81`）：`x=8..232` 全是行的填充 `232`，`x=680` 是 `207`——那是行尾那个元素
  （`y=85..95` 读到 `220`）的边缘。夹具的参照点 `InputWindowWidth-40` 从来就不是
  「行自己的面」，库存尺下只是恰好撞对。
- **#5 还有第二层**：宽度断言过了之后，`readFace` 用 `int()` 截断读盒（§6.77 的禁令），
  出货尺下盒顶是小数，读到的「顶内线」是相邻一行——浅色读成 `246`（亮）、深色读成
  `15`（比面还暗）。改 `math.Round` 后两条一起绿。

## 3. 落地（产品 0 行）

| 文件 | 改动 |
|---|---|
| `internal/launcher/view_test.go:147` | `render()` 装主题 + `themedFrameHeight`；注释写清这是默认取景，`renderIn`/`renderAt` 仍是债 |
| `internal/launcher/actionbarfooter_test.go:300-330` | 列 `16/44/54`、行 `<55` → 按 `designUnit()` 换算（格 `14..39`、空 `40..48`、行到发丝 49）；断言仍是**精确 0** |
| `internal/launcher/builtinglyphs_test.go:169` | 参照点 `InputWindowWidth-40` → 标题盒右沿 + 20px（实测 `232`，行自己的面） |
| `internal/launcher/resultsedge_test.go:66-78` | 发丝 `fieldRowUnits*4-1` → `round(14u×spacing−1)` = 49；气 → `round(54)−(49+1)` = 4（推导，不是容差） |
| `internal/launcher/stripfaces_test.go:89,46` | `27.5..28.5` 的容差带 → `tenths(key.W) == tenths(28u)` = 25.2；`readFace` 的 `int()` → `math.Round` |
| `internal/launcher/rowweight_test.go:39-45` | 门槛 1.35 → **1.25**（实测 1.28），注释写清 1.41 是库存尺的帧 |
| `internal/launcher/fixtureruler_test.go`（新） | 一条 pin：默认取景的帧高 439 / 发丝行 49，且两把尺子的帧高必须不相等；一个 opt-in 出图（4 张） |

## 4. 台账

**V 5 / B 0 / S 2**（V：搜索行三列、发丝行、气、28u 与 `readFace` 取整、便条门槛；
S：`render()` 换尺、浏览器行的参照点）。产品代码 **0 行**（`internal/launcher/*.go` 的
非测试文件一个字节没动）。

测试函数 **2496 → 2498**（+2：守卫 pin、出图），PASS **3327 → 3328**，SKIP **161 → 162**。

## 5. 门槛（全部实测）

```
gofmt -l cmd internal          → 空
go vet ./...                   → 净
GOOS=windows go vet ./...      → 净
GOOS=darwin  go vet ./...      → 净
go test -count=1 ./...         → 全绿（0 FAIL）
--- PASS 3328 | --- FAIL 0 | --- SKIP 162
测试函数 2498
GOOS=linux|windows|darwin go build ./... → 三平台过
```

## 6. 变异（4 条实跑变红并还原，md5 逐字节核对 + `touch` 复跑全绿）

- **M1a（意外，如实记）** `renderShipped()` 退回库存尺（我的补丁第一次替换到的是文件里
  先出现的 `renderShipped`）→ **12 个测试函数红**，红串例如
  `the field's caret starts at x=54.0, want the rows' title column 48.6 (54u at 0.9)`、
  `the numbered family has 9 entries, want 10`。**这条比计划里的 M1 更有价值**：它证明
  `dg-l` 那几轮的期望值现在全靠「默认取景也是出货尺」这一条撑着。
- **M1b** `render()` 退回库存尺 → **3 红**：`the default fixture draws a 488px frame, want
  the shipped ruler's 439px`、`the field block's hairline is at y=55, want 49`、
  `the pin key's box is 25.2 wide` 那一族重新红。
- **M2** 便条门槛写回 1.35 → `the note row's title carries 8083 ink over its subtitle's
  6322 (ratio 1.28, want 1.35)`（浅深各一条）。
- **M3** 浏览器行把 `iconPlate` 还回去 → `the browser row's icon cell is plated
  ({216 216 217 255} over the row's own bare fill {232 232 233 255})`——**换了参照点之后
  仍然抓得到板**。
- 四条都 `md5sum -c` 逐字节还原（`view_test.go 9c2f5ca2…`、`rowweight_test.go 7f96edb8…`、
  `launcher.go b4fbe177…`），随后 `touch` + 复跑全绿（避开测试缓存）。

## 7. 出图（深 / 浅 × 两把尺子）

`/tmp/floter-P13-ec-{stock,shipped}-{light,dark}.png`（720×488 / 720×439）。

- 逐像素 diff（共同区域 720×439）：浅 **316038** 个、深 **316059** 个，最大通道差
  **247 / 230**。
- **这张图上看到的一处变化**：卡自己的面在库存尺上是 `253,253,254`（浅）/ `23,23,25`（深），
  出货尺上是 `246,246,248` / `15,15,16`——**夹具以前画的根本不是这个应用的卡**，180 条
  像素断言量的盒子挂在一张用户看不到的面上。第二处：`y=55` 那条发丝（浅 `239`）在出货
  尺的帧里没有了，它搬到了 `y=49`。

## 8. 这轮为什么让它更轻 / 更有空间感（一句话）

这不是界面轮，但它决定**以后每一轮看到的是不是同一个界面**：180 条像素断言从此量的是
`Spacing 3.6` / 正文 `10.8` 的那张卡——也就是「面差 / 受光边 / 阴影」三档真正落在的那张卡；
在这之前，同一批断言量的是一张 `253` 的、按 4.0 排的框架卡，任何「层级更清楚了」的结论
都可能是尺子的错觉。

## 9. skill 偷了什么（qiaomu-design）

- `references/user-preferences.md`：**「不许用容差把红变绿」** → 本轮删掉了
  `stripfaces` 那条 `27.5..28.5` 的带子，改成十分位精确相等；`resultsedge` 的「气」改成
  **两次取整相减**的推导值，而不是 `±0.5` 的宽容。
- `craft-loop.md` 工艺密度菜单**第 7 项「把测量点写进注释」** → 落成
  `fixtureruler_test.go:35` 的 `49 = 14u × 3.6 − 1, snapped to the painter's row` 与
  `resultsedge_test.go:70-74` 的 `50.4 / 49 / 54.0` 三个数，读者不必再猜。
- `preflight.md` **D2（中文页）**：本轮文档全部中文、数字与单位不混排；仍**未过**的是
  字号门槛本身（`tokens.Caption` 9.9 / 正文 10.8 对「正文 ≥ 14px」），第 52 份证据。
- `preflight.md` **A（反 AI 套路）**：没有新增渐变白边 / 假模糊；**B（设计读取一致性）**：
  这条正是本轮的主题（夹具与生产读同一个寄存器），已由 `fixtureruler_test.go` 钉住；
  **C（动效抽查）**：本轮零动效改动。

## 10. 仍待

1. **下一格（首选）**：`renderIn`（`appearance_test.go:19`）与 `renderAt`
   （`modetextframe_test.go:135`）仍配 `frameHeight`（库存尺）——那是**显式开窗**那一族
   （折叠 / 条带 / 页高把窗口本身当被测对象），换尺要先逐族判 S，另占一格。
2. **需拍板（新）**：便条行的 rank 在生产尺上只有 **1.28**（标题与副标题同字号同墨，
   只差 580/400 的字重）。想真正拉开要么把标题抬到 `tokens.Emphasis`（行盒 44.8 → 46.8，
   **动高度链**），要么给便条行换一档墨——两条都要你点头。
3. **需拍板（沿用）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2「≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 终端弧 S1b / S1c / S2 尾。
5. 抢救 stash 核对（第五十六次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
