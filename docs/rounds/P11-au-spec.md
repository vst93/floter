# P11-au 规格：抽屉放不下时，最后几行要能被滚到

## 意图（一句话）

终端窗口缩到最小高度时，外观抽屉最后几行不再被面板底边无声裁掉——条带**自己**成为
滚动容器，滚到头能看到全部行。

## 为什么现在做

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K` 动作面＝拍板）→ **上一轮「仍待」第 1 条**——P11-at 明写：

> 抽屉在最小高度下最后几行被面板底边裁掉：`640×360` / `720×360`（`settings.MinTerminalHeight`
> = 360）时抽屉条带从 y=28 起、自然高 365，`Safe paste` / `Copy on select` 只剩 4u 可见。
> 本轮已把「抽屉爬进浮条带」修掉……滚动留待下一条。

属用户定下的「界面质感与交互逻辑」优先档：一个打开着却看不见最后两行的设置抽屉，
是交互缺陷，不是像素细节。

## 现状（本轮实测，file:line 以本轮 HEAD 为准）

- `internal/terminalui/drawer.go:51` `func (a *App) drawer`：条带是一个 **`ui.Row` +
  `Shrink(0)`** 的普通行，里面直接放网格与关闭钮。`Shrink(0)` 让它**永不收缩**，
  面板比它矮时它按自然高度排、多出的部分被面板底边裁掉。
- `internal/terminalui/terminalui.go:890` `body := ui.Box(c).Grow(1).MinHeight(t.Space(7))`：
  P11-at 给画布加的地板，保证抽屉不让开浮条那条带。这条**保留不动**。
- P11-at 报告 §5 记着：把网格包进 `ui.Scroll(...)` 后 `DrawerScroll.MaxY` 恒为 **0**。
  **本轮找到原因**：那个 `ui.Scroll` 在**横向** Row 里 `Grow(1)`，只吃宽度；它的高度由
  父 Row 的 `AlignItems(Start)` 定成**内容的自然高**，于是它自己就和内容一样高，
  `contentH - h == 0`（`ui/scroll.go:155` 的 `maxScroll`）——没有可滚的距离。
- 实测（`ui.Tester`，本轮修前原文）：

  ```
  640x360 strip={X:0 Y:28 W:640 H:332} MaxY=0.0   Safe paste={Y:356 H:3}
  720x360 strip={X:0 Y:28 W:720 H:332} MaxY=0.0   Safe paste={Y:356 H:3}
  720x600 strip={X:0 Y:235 W:720 H:365} MaxY=0.0  Safe paste={Y:563 H:22}
  ```

  最小高度下 `Safe paste` 只剩 3u 高（22u 的行被底边裁掉 19u），`Terminal palette`
  整行在视口外、且 `MaxY=0` 意味着**永远够不到**。

## 落地项（逐条）

1. **把条带本身做成滚动容器。**
   位置：`internal/terminalui/drawer.go:66`。
   现状：`strip := ui.Row(c).FillWidth().Shrink(0).Padding(...).Gap(...).AlignItems(Start).
   Background(...).BorderWidth(0,0,1,0)...`
   应改成：`strip := ui.Scroll(c).FillWidth().TrackScroll(&a.DrawerScroll).
   Background(...).BorderWidth(0,0,1,0)...Label(...)`；面与线画在**视口**上（不随内容
   滚走），内边距与网格放进一个 `ui.Row(c).FillWidth().Shrink(0)` 的内容行里（跟着滚）。
   ——`ui.Scroll` 是 `coreBox`（竖向滚动），面板列的直接孩子；`layout.go:723` 的
   `!c.scrolls()` 条件让滚动元素在列里 **`minMain` 自动为 0**，所以面板放不下时它被
   压在画布之下、并如实地报出 `MaxY > 0`，无需 `Shrink(0)/MinHeight(0)` 的额外断言。

2. **新增 App 级滚动状态。**
   位置：`internal/terminalui/terminalui.go:125`（`Drawer` 字段之后）。
   现状：无。
   应改成：`DrawerScroll ui.ScrollState`，注释写清它只在面板比条带矮时有意义。

3. **画布地板不动。** `terminalui.go:890` 的 `MinHeight(t.Space(7))` 原样保留。

## 不动的东西（行为契约零改动红线）

- 高度链：`MinTerminalHeight`、画布 `MinHeight`、条带 padding/gap、`drawerCellFloor`、
  `paletteCellFloor`、`drawerRangeWidth`——一个字不动。
- 墨/字阶/面/线：`tokens.Control` / `HairlineSoft` / `drawerReadout` 等零改动。
- 键盘模型、会话管理、设置写回、i18n 文案、命令接线：不碰。
- 「抽屉不让开浮条带」（P11-at 的 pin）原样必须绿。

## 台账预判

- **V = 0**：没有一个墨、半径、字号、padding、gap 被改。
- **B = 0**：既有 pin 零改动。
- **S = 1**：条带从 `ui.Row`（视口=行）变成 `ui.Scroll`（视口）+ 内层内容 `ui.Row`
  ——结构真变了，但**绘制盒在窗口够高时逐像素不变**（内容在最顶、padding 相同）。

## 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2386 → 期望 2388
grep -c -- "--- SKIP"                                  # 基线 31 → 期望 32（+1 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l  # 基线 1951 → 1954
三平台 build                                            # 过
```

## mutation 计划（≥2 条，实跑）

1. **把条带改回 `ui.Row` + `Shrink(0)`（去掉 `TrackScroll`）** → 期望
   `TestTheDrawerScrollsItsLastRowsIntoView` 红，报
   `the drawer reports no scroll range (MaxY=0.0)`。
2. **把内层内容行的 `Shrink(0)` 改成 `Grow(1)`**（让内容被压进视口而不是保持自然高）
   → 期望网格被压扁、最后一行退化成几 u 高；pin 的「滚到头后最后一行不完整」红，
   或 `MaxY == 0` 红。
3. （附加）**把 `TrackScroll(&a.DrawerScroll)` 删掉** → 状态不再被写，同一 pin 红。

每条变红后 `md5sum -c` 逐字节还原、`touch` 再复跑绿。

## 视觉证据

`/tmp/floter-P11-au-drawer-end-{light,dark}.png`（`FLOTER_PNG=1`，640×360 最小窗口、
抽屉滚到最末），以及对应的 `-before` 版本。
