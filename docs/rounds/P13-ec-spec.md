# P13-ec 规格 · `internal/launcher` 的默认取景收口（73-5 收尾）

轮号 `P13-ec`｜表面 = **测试夹具（不是界面）**｜上游 = `P13-eb`「仍待」第 2 条 /
`liquid-glass.md` §6.86 的 **73-5**（`internal/launcher` 的 `render()` 仍在量库存字 14）

## 1. 意图一句话

把这一包**默认的那把尺子**换到出货尺（`SetTheme` + `themedFrameHeight`），
让 `render()` 的 180 个调用点从此量的都是用户真看到的帧；逐条把 5 个变红的断言按
V / S 归类重钉，**一个容差都不加**。

为什么现在做：§6.86 拆轮表里 `dg-t`（terminalui）、`dg-l`（launcher 的像素断言）
都已收口，`renderShipped` 已经在用；只有 `render()`/`frameHeight()` 这一对还画着
框架库存主题（`Spacing 4.0` / `FontSize 14`）——即 73-5 的「四十多条 pin 在量库存字」。
P13-dp 在 `internal/terminalui` 上证明了这条手续：换尺 → 只判 V 或 S → 逐条重钉。

## 2. 先量（动手前实测）

把 `render()`（`internal/launcher/view_test.go:147`）临时改成
`c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight(maxResultSlots)`：

```
go test -count=1 ./internal/launcher/ → 5 个测试函数红
```

| # | 测试 | 失败串（原文） | 类 |
|---|---|---|---|
| 1 | `TestTheSearchRowLeadsWithANeutralMark` | `65 ink pixels in the 10u gap (x=44..53): the mark is not in the icon cell` | V（列是库存 px） |
| 2 | `TestBrowserRowsDropTheIconPlate` | `the browser row's icon cell is plated ({232 232 233 255} over the row's own bare fill {207 207 208 255})` | S（参照点落在行外） |
| 3 | `TestTheResultListNeverScrollsIntoTheFieldBand` | `dark=false: the field block's hairline is at y=49, want 55` | V（`×4` 是库存单位） |
| 4 | `TestANoteRowsTitleOutweighsItsSubtitle` | `light note: title ink 8083, subtitle ink 6322 (ratio 1.28)` | V（门槛是按库存尺的 1.41 定的） |
| 5 | `TestTheOutputStripControlsAreE1Faces` | `the pin key's box is 25.2 wide, want the strip's own 28u` | V（`28u` 没按步长换算） |

**两条要单独说的量**（本轮真正的发现）：

- **#4 不是容差问题，是这一页在生产上真的只有一条腿。** 便条行的标题画在
  `t.FontSize`（`output.go:969`），副标题画在 `tokens.Body`（`output.go:996`）——
  出货档两者**都是 10.8**，于是 rank 只剩字重（580 对 400）。库存取景里标题是 14、
  副标题是 10.8，才有 1.41。**1.41 是一个生产上不存在的帧。**
- **#2 的图标格没有板。** 逐像素扫过浏览器行的整条带（`row0 = {7.2 72 705.6 36}`，
  `y=81`）：`x=8..232` 全是行的填充 `232`，`x=680` 是 `207`——那是行尾那个元素
  （`y=85..95` 读到 `220`）的半透明边缘，不是「行的填充」。夹具的参照点
  `InputWindowWidth-40` 从来就不是行自己的面，库存尺下只是恰好撞对。

## 3. 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| A | `view_test.go:147` `render()` | `a.View(c)` + `frameHeight(maxResultSlots)`（库存 4.0 / 14） | `c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight(maxResultSlots)`；注释写清这是**默认取景**，`renderIn`/`renderAt` 仍是债（显式开窗那一族，另一格） |
| B | `actionbarfooter_test.go:300-326` | 列 `16/44/54`、行 `<55` 是库存 px | 全部按 `designUnit()` 换算：格 `16u..44u`、空 `44u..54u`、行到发丝 `round(14u×spacing−1)`；断言仍**精确 0** |
| C | `builtinglyphs_test.go:169` | `bare := img.RGBAAt(InputWindowWidth-40, y)` | 取同一行里**行自己的面**：标题盒右沿 + 20px（`x=85`，实测 `232`）；语义不变（仍与选中行的 accent 底色比，不跟面板比） |
| D | `resultsedge_test.go:66,72` | `fieldRowUnits*4 - 1` / `breathUnits*4` | 按出货尺推导：`fieldRow = fieldRowUnits×designUnit()×4`；发丝行 = `round(fieldRow−1)` = 49；气 = `round(fieldRow+breath)−(发丝+1)` = 4（**推导出来的整数，不是容差**） |
| E | `stripfaces_test.go:89` | `key.W < 27.5 || key.W > 28.5`（围着库存 28 的**容差带**） | `tenths(key.W) != tenths(28*designUnit())`（25.2，精确到十分位；容差带删除） |
| F | `rowweight_test.go:39-45` | 门槛 1.35，注释写「实测 1.41」 | 门槛 **1.25**，注释写「出货尺实测 1.28：同字号同墨，rank 只有字重；1.41 是库存尺上 14 对 10.8 的帧」；`580 → 400` 的变异仍会红（1.00） |
| G | 新 `fixtureruler_test.go` | — | 一条 pin：`render()` 与 `renderShipped()` 读**同一个** `c.Theme().FontSize/Spacing`（防两把尺子再分家）；加一个 opt-in 出图（库存尺 / 出货尺 × 深 / 浅 四张） |

**不做**（本轮明确留给下一格，写进报告与 `.pending-bugs.md`）：
`renderIn`（`appearance_test.go:19`）与 `renderAt`（`modetextframe_test.go:135`）仍画库存尺，
它们配的是 `frameHeight(maxResultSlots)` 的**显式开窗**（fold / 折叠那几族把窗口当被测对象），
换尺要先判每族的 S。

## 4. 行为契约零改动红线

本轮**产品代码零改动**（`internal/launcher/*.go` 的非 `_test.go` 文件一个字节不动）：
高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部不碰。只改夹具与文档。

## 5. 门槛命令与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal → 空
go vet ./... ；GOOS=windows/go vet ；GOOS=darwin go vet → 净
go test -count=1 ./... → 0 FAIL
--- PASS 3327 | FAIL 0 | SKIP 161 | 测试函数 2496     ← P13-eb 实测基线
GOOS=linux|windows|darwin go build ./... → 过
```

PASS 只许涨（新增 pin 一条 + 新增 opt-in 出图一条 → 预期 3328 / 162 SKIP）。

## 6. mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1** `render()` 退回库存尺（删 `c.SetTheme`、帧高换回 `frameHeight`）→ 上面 5 条里
   至少 4 条必须重新变红（这是本轮的「取景就是这个取景」证明）。
2. **M2** `rowweight_test.go` 把便条行门槛写回 1.35 → `TestANoteRowsTitleOutweighsItsSubtitle`
   红，串里应是 `ratio 1.28, want 1.35`（证明门槛确实钉在实测值下方，不是随便写）。
3. **M3** 浏览器行把 `plate = a.iconPlate(c, warm, false)` 还回去（`launcher.go:2603-2607`）
   → `TestBrowserRowsDropTheIconPlate` 红（证明 #2 的参照点换过之后**仍然抓板**）。
