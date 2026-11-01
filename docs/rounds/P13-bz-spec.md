# P13-bz 规格 · 设置页的侧栏与内容此前是同一个 z 平面

> 选题来源：`.pending-bugs.md` P13-by「仍待」第 1 条（上一轮报告里自己量到、留给这一轮的格子）。
> 队列位置：任务书优先级 3「上一轮报告仍待的第一条」。`docs/liquid-glass.md` §6 无 `未开始` 行，
> 所以这不是新表面，是已落地表面上的**一处层级算术错误**。

## 意图

**侧栏此前不是一个「面」，它只是地上一层 0.17 的浅色薄罩；两栏之间唯一的分界是那根 1px 竖发丝线。**

用户 2026-10-30 的原话是「所有东西挤在同一个 z 平面上，靠 1px 描边分家」——
设置页的侧栏就是这句话的算术来源。

## 证据（改前实测，本轮自己量的）

`theme.ForPreferences(settings.Default(), dark, ui.Preferences{})`，默认 47% 框：

| | 地 `Background` | `FaceOver(地, Sunken)` | 差 |
|---|---|---|---|
| dark | `14,14,16` | `15,15,17` | **+1 级** |
| light | `247,247,248` | `246,246,248` | **+1 级** |

渲染整页（720×900，`PageGeneral`，`ui.Tester`）后在选中行 `General` 的中线上横扫，
侧栏 `x=10..140` 与内容 `x=400..719` 在**行外**读到的是同一对像素
（dark `15,15,16` / light `246,246,248`）——两栏之间只隔 `settingsui.go:1267` 的
`hairlineRule(c, true)`（0.055 的白/黑，实测 5 级）。

`docs/liquid-glass.md` §1 的材质表里，内容页是**底**（`Sunken` = 底 + 0.09），
侧栏被写成了同一个底。于是这一页只有「底」和「底上的块」两级，侧栏作为**容器**没有自己的面。

## 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go:1247`（侧栏列 `ui.Column(...).Width(148)` 的 `Background`） | `Background(a.tokens(c).Sunken)` —— 与内容页同一个薄罩，与地差 **1 级** | `Background(a.tokens(c).Control)` —— **面**档（dark 0.10 白 / light 0.07 黑），压在窗口地上实测 dark `14→39`（**+25**）、light `247→230`（**−17**）。内容页仍留 `Sunken`（**底**，§1 的分配不动） |
| 2 | 同上，注释 | 说「the sidebar sits on the same one so the two read as one pane」 | 改成「侧栏是**面**、内容是**底**」，并写明为什么冠线不画在这里 |
| 3 | `internal/settingsui/liteedge_test.go:194` | `pane := imagePixel(img, 2, …)` —— 取的是**窗口地**，不是行脚下的面 | 取侧栏自己的面（行下方的列内像素）。V 重定基线 |
| 4 | `internal/settingsui/raycast_test.go:157` | 同上，`want := panelOver(tokens.RowSelected, pane)` 用的是窗口地 | 同上，`pane` 换成侧栏自己的面。V 重定基线 |
| 5 | 新增 pin `internal/settingsui/sidebarface_test.go` | — | ① 侧栏的面与内容页的面在深/浅两套外观上都 ≥ `liteFaceFloor`(8) 级；② 侧栏的面对**地**也是 ≥ 8 级（它是面不是薄罩）；③ 选中行仍站在侧栏面上 ≥ 8 级（面差没有被这一改吃掉）；④ **面差压过接缝**：两栏交界处的差 ≥ 那根竖发丝线自己造成的差的 2 倍——即「不再靠 1px 描边分家」是可测的 |

**为什么侧栏不戴 `LitFace` 的冠线**：`§2` 的 E1 是「落在一个底上的块」——它有上边和下边。
侧栏是一列**贴着窗口边**的通高容器（上接页头发丝、下接窗口下沿），给它画冠线就是给面板画框，
正是这一支要退掉的东西。它的层级由**面差**表达（§1 第一条手段），这已经足够：实测 25 / 17 级。

**为什么内容页不跟着抬**：`§1` 把内容页钉在**底**（阅读面），卡片才是它上面的**面**。
把内容也抬成面会让「卡片 vs 内容」的差归零——同一页里两个面糊在一起，比现在更糟。

## 行为契约零改动红线

- 高度链：`FieldRowUnits` 页头 + `Grow(1)` 两栏，一个字节不动。
- 键盘模型：`a.Sidebar` 的列表、`FocusRing`、`↑/↓` 跨页导航，零改动。
- 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动。
- 度量：148u 侧栏宽、`Space(3)` 列间距、1px 竖发丝、行 `Space(8.5)` 高，全部零改动。
  **本轮只改一列背景的墨**（`Sunken` → `Control`），不新增/删除任何元素。

## 门槛（基线 = 本轮实测的当前 HEAD）

```
gofmt -l cmd internal                 # 必须空
go vet ./...                          # 必须净
GOOS=windows go vet ./...             # 必须净
GOOS=darwin  go vet ./...             # 必须净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3222，只许涨
GOOS=linux/windows/darwin go build ./...            # 必须过
```

## mutation 计划（≥2 条，每条都要实跑看到红）

- **M1**：把侧栏的背景改回 `Sunken` → `sidebarface_test.go` 的 ①②④ 三条判据同时红
  （面差 1 级 < 8；交界处退化成一根发丝线）。
- **M2**：把选中行也抬成 `Control`（`row.Background(tokens.Control)` 覆盖 `RowSelected`）
  → 判据 ③ 红（选中行与它脚下的面同值）。
- **M3**（可选）：把 `Control` 换成 `ControlHover` → 判据 ① 仍过但 ③ 会变松；
  不选它，改为把竖发丝的宽度从 1 加到 4 → 判据 ④ 仍过（面差不依赖它），
  这条用来证明 ④ 钉的是面差而不是线。

## 出图

`/tmp/floter-P13-bz-settings-{dark,light}-{before,after}.png`（`ui.Tester` 整页，720×900，`PageGeneral`）。
