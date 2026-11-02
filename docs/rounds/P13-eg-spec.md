# P13-eg · 启动器最后一批「量的不是这个设计」的夹具，以及藏在出货夹具里的库存尺常数

**意图**：把 `internal/launcher` 里**最后 13 处**自己搭取景、没装出货主题的夹具收进
`themedView`，并修掉 4 处「夹具已经换到出货尺、但断言还quote 库存 `Space()`」的常数；
最后补一条**能分辨两把尺子**的 pin —— 因为换尺本身一条红都没出，说明这一族 pin 全是相对读数。

**为什么现在做**：`P13-ef` 的「仍待」第 1 条（选题链第 3 条）。`docs/liquid-glass.md` §6.86 的
**73-5** 尾巴：这一包还有一批夹具画的是框架库存墨（`Spacing 4.0` / `FontSize 14`），
也就是**用户看不到的那张卡**。

## 现状（动手前逐处读过代码，行号为 P13-eg 开工时的 HEAD = `c7e03a83`）

裸 `ui.NewTester(func(c){ a.View(c) }, …)`，13 处：

| 文件:行 | 帧高 | 量的是什么 |
|---|---|---|
| `actionbarfooter_test.go:915` | `charged`（应用自报） | 页脚是否越过窗口底边 |
| `bench_test.go:29` / `:41` | 720×420 | 两条 benchmark（按键一帧多久） |
| `filters_test.go:117` | `WindowHeight("small")+200` | 分组标题在不在 |
| `filters_test.go:1118` | `WindowHeight("small")+120` | 按住修饰键时行不被滚进视口 |
| `headingcolumn_test.go:37` / `:52` / `:124` | `+200` | 标题列 == 行文字列 |
| `headingfold_test.go:296` | 300 | 标题在折边上 |
| `snapshotpin_test.go:21` | 720×420 | 一帧取几次设置快照 |
| `view_test.go:1029` | `+240` | 内置行 / 最近行的顺序 |

「地面自钉」的 4 处（判过：地面该留，尺该换）：

| 文件:行 | 现状 |
|---|---|
| `dimplate_test.go:36` | `c.SetTheme(ui.LightTheme()/DarkTheme())` 只为借 `Dark` 位；地面 28,29,33 / 247,247,248 自钉 |
| `plateface_test.go:67` / `:275` | 同上，且 `tokens = a.tokens(c)` 只用到 `Dark` |
| `plate_lens_test.go:24` | 固定深色 |

**已经换到出货尺、但断言还在 quote 库存 `Space()` 的常数** 4 处（`render` / `renderIn` 已在
P13-ed/ef 换尺）：

| 文件:行 | 现状 | 出货尺的真值 |
|---|---|---|
| `litedge_test.go:87` | `int(ui.LightTheme().Space(2))` = 8 | 7.2 |
| `filesmarks_test.go:147` | 同上 = 8 | 7.2 |
| `raycast_test.go:390` | `int(ui.LightTheme().Space(fieldRowUnits))` = 56 | 50.4 |
| `tipglass_test.go:317` | `ui.LightTheme().Space(1.5)` = 6 | 5.4 |

## 落地项

1. **13 处裸夹具 → `themedView(a)`**（`themedView` 是 P13-ef 定的唯一取景入口）。
   帧高**不动**：帧高那一族已由 P13-ef 收口，这一轮只换墨（`filters:1118` 的 `+120` 保留，
   因为它的判据是「滚不滚」，换帧高会把判据一起换掉）。
2. **4 处地面自钉的夹具**：`c.SetTheme(a.tokens(c).Theme)`，地面常量保留；`dimplate` /
   `plate_lens` 补 `tt.SetDark(...)`，否则 `a.tokens(c)` 读的是 `c.Theme().Dark` = 亮（会把深色面
   合成在白纸上，P13-ee 的教训）。
3. **`plateFace` 探针改成量布局给的盒子**：`a.iconPlate(c,…).Bounds()` + 新 `pixelBox`
   （`floor(min)..ceil(max)`，painter 的格子）。原来写死 `int(ui.LightTheme().Space(7))` = 28，
   出货尺的板是 `X 47.4 W 25.2` → 画 47..72（26px），探针两侧读到的是**地面**。
4. **悬停小格**：`int(ui.LightTheme().Space(5))` = 20 → `int(designUnit()*20)` = 18。
5. **4 处常数**换 `spaceU(n) = designUnit()*4*n`（新助手，落在 `view_test.go` 的 `designUnit` 旁边）。
6. **新 pin** `TestTheSectionHeadingStartsAtTheShippedTextColumn`（`headingcolumn_test.go`）：
   两臂各钉一把尺子 —— 出货 `heading.X 48.6 / title.Y 88.5 / heading.H 17`，库存 `54 / 22`。
   这一族此前**只做相对比较**（`sameColumn`），换尺换错都绿。

## 行为契约零改动红线

产品代码 **0 行**。高度链（`Geometry.Height`）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线全部不碰；只动 `_test.go` 的取景与断言常数。

## 门槛命令 + 本轮 PASS 基线（实测）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./...  /  GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 3332（P13-ef 基线 3332）
三平台 build
```

## mutation 计划

- **M1** `plateFace` 退回写死 28 的盒子 → `TestTheIconPlateIsAFaceNotARing` 必须红
  （左列读到地面 `{246 246 248}`）。
- **M2** 悬停小格退回 `int(ui.LightTheme().Space(5))`（20）→ `TestTheHoveredRowControlIsAFaceNotARing` 必须红。
- **M3** 新 pin 的出货臂退回裸 `ui.NewTester` → 新 pin 必须红（`x=54, want 48.6`），
  证明它真能分辨两把尺子。
