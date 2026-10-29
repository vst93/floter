# P11-ca 报告 · 终端抽屉进「字盒 / 行盒高度」尺子（P11-bz 仍待第 2 条）

规格：`docs/rounds/P11-ca-spec.md`。选题链：用户实测反馈 F1–F4 已收完 → 合约 §6 全
`已落地`（只剩拍板项）→ P11-bz「仍待」第 2 条原话「`internal/terminalui` 的抽屉还没进
高度尺子……要自己量一遍阶梯，别照抄启动器或设置页的数」。第 1 条整张是拍板项，不许擅自做。

## 1. 实测阶梯（本轮自己量的，16 帧逐帧同一组）

范围：`lang ∈ {en,zh}` × `dark ∈ {false,true}` × `{640×360, 720×560, 900×600, 1100×700}`，
抽屉打开、一个空会话。审计带 = 条带顶边以下的 run，下限 `frameaudit.MinHeight = 10`。

```
17 — 滑轴读数（tokens.Body + mono）           drawer.go:302
20 — 迷你终端预览的一行（t.FontSize - 1 + mono） terminalui.go:1801
22 — 行标签 / select 的当前值（t.FontSize）     drawer.go:318、:343
28 — 抽屉右上角的关闭按钮（barButton）          terminalui.go:1364
30 — 分段控件的 30u 下限                        control.go:110
```

- **这一组与另外两个表面没有一档相同**：启动器 `15/18/22/24/25/28`、设置页
  `15/17/18/19/27/30/30.6/34`。共有的只有 28（启动器的图标方框 = 这里的关闭按钮）。
  这就是「别照抄」的理由：抽屉的 17 是 `tokens.Body`，设置页的 17 是副标题；
  抽屉的 22 是行标签，启动器的 22 是结果行副标题——同数不同源，两边各自钉。
- 祖先盒子（条带的 label、`Palette preview` 的 88u 方框）被 `Heights` 的叶子规则排除，
  **不进阶梯**，本轮也不钉。
- 640×360 是 P11-au 的最小窗口（条带被压到画布之下、自己滚），五档仍然都在——所以
  `must` 就是整个阶梯，不需要按尺寸分档。

## 2. 落地

新增 `internal/terminalui/textheight_test.go`（**产品代码零改动**）：

- `TestTheDrawerStaysOnTheHeightLadder`：16 帧 × `AuditHeights(rungs = must = 五档, tol 0.5)`。
  两件事：没有一档离开阶梯、五档都真的被画出来。
- `TestEachDrawerRungHasItsOwnProducer`：五对 `文字 → 高度`（读数 17 / 预览行 20 /
  行标签 22 / 关闭按钮 28 / 分段控件 30），每段字还必须落在条带的带内。
  加这一条的原因是**只钉集合会漏掉「两档互换」**：把标签改成 20、预览行改成 22，
  集合仍是 `{17,20,22,28,30}`，但设计已经变了。

## 3. mutation（3 条实跑变红 + 1 条绿变异，全部 `md5sum -c` 还原）

- **M1** `control.go:110` `MinHeight(t.Space(7.5))` → `Space(8)`：
  `textheight_test.go:63: en light 640x360: height 32.0 is not on the ladder [17 20 22 28 30] (±0.50)`，
  16 帧全红。
- **M2** `drawer.go:302` `.FontSize(tokens.Body)` → `tokens.Emphasis`：
  `textheight_test.go:63: en light 640x360: no run of height 17.0 was painted (heights [18 20 22 28 30])`
  ——顺带实测到：`tokens.Emphasis` 在这一帧是 **18**，既不在阶梯上、也让 17 消失。
- **M3** 去掉 `drawer.go:379` 的 `PalettePreview(...)`：
  `textheight_test.go:63: en light 640x360: no run of height 20.0 was painted (heights [17 22 28 30])`；
  生产者那条同时红：`textheight_test.go:110: light: the drawer does not draw "floter --version"`。
- **M1b（绿变异，也是账）** `drawer.go:315` 的行下限 `Space(7.5)` → `Space(8)`：
  **画面一个字节都不动**。30 那一档是**分段控件**的下限给的（`control.go:110`），行自己的
  下限被它挡住。所以「30 的产生者」不是 `drawer.go:315`，报告与注释都按实测写，
  别下次把它当 30 的出处。

## 4. 门槛（本轮实测）

- `gofmt -l cmd internal` 空；`go vet ./...` / `GOOS=windows go vet ./...` /
  `GOOS=darwin go vet ./...` 净；三平台 `go build ./...` 过。
- `go test -count=1 ./...` 全绿，`--- PASS` **2655 → 2675**（+20 = 16 帧 + 2 臂 + 2 父），
  `--- SKIP` **54 不变**，测试函数 **2039 → 2041**。
  （基线 2655 / 2039 与 P11-bz 报告一致；我第一次量的 2657 / 2041 里含两个探针测试，
  已在同一轮删掉——上面的数是去掉探针之后的真实值。）

## 5. 仍待（P11-ca）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **终端页的另外两段还没进高度尺子**：标题栏 / 画布上方的浮条（22 / 28）与 PTY 自己
   画的行（20）——它们与抽屉的 20/22 同数不同源，要各自量（PTY 的行高跟着用户设置走，
   很可能不能钉成一个常量，先想清楚再动）。
3. 设置页八档里 **30 与 30.6 只差 0.6u**（容差 0.5），分辨力已经很薄，别再往里加档。
4. 想守「字变大了但盒子没变」得量**墨迹**，不是盒子（P11-bz §4 那一版被邻居元素污染，未落地）。
5. 老盲点不变：只量字盒（图标 / 面 / 描边互压看不见）；`Tester.Find` 按字符串去重；
   滚动容器里被裁掉的行不进 `Texts()`。
6. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」。
