# P11-au 报告：抽屉放不下时，最后几行要能被滚到

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K` 动作面＝拍板）→ **上一轮「仍待」第 1 条**。P11-at 明写：

> 抽屉在最小高度下最后几行被面板底边裁掉：`640×360` / `720×360` 时 `Safe paste` /
> `Copy on select` 只剩 4u 可见……滚动留待下一条。**别再用「把网格包进 `ui.Scroll`」重领**，
> 那条路本轮已经走到头。

P11-at 把网格包进 `ui.Scroll` 时 `MaxY` 恒为 0，于是放弃了滚动。本轮先弄清**为什么是 0**，
再把这条路走通——它属于用户定下的「界面质感与交互逻辑」优先档：一个打开着却看不见最后
两行的设置抽屉，是交互缺陷。

## 2. 根因（P11-at 的 `MaxY == 0` 到底为什么）

框架里滚动范围是这样算的（`mygo@v0.7.0/ui/scroll.go:155`、`ui/layout.go:597-600`）：

```go
// maxScroll
y = max(0, e.contentH - float64(e.h))
// layoutBox
e.contentH = float64(max(uh, ch) + e.padY())   // uh = 内容自然高, ch = 容器内高
```

要 `MaxY > 0`，**容器自己的高度必须小于内容的自然高**。P11-at 的写法是
`strip(Row, Shrink 0) > ui.Scroll(Grow 1) > grid`：那个 `ui.Scroll` 在**横向** Row 里
`Grow(1)` 只吃**宽度**；它的高度由父 Row 的 `AlignItems(Start)` 定成**内容的自然高**。
于是 `ch == uh`，`contentH - h == 0`——它自己就和内容一样高，没有可滚的距离。

**正解**：把**条带本身**做成滚动容器，让它直接挂在面板那一列上。`ui/layout.go:723` 把
滚动元素的 `minMain` 自动设为 0（`else if !c.scrolls() ...` 跳过内容最小高），所以面板
放不下时它被压在画布之下（P11-at 的地板仍在），并且如实地报出 `MaxY > 0`。
面与线画在**视口**上（不随内容滚走），内边距与网格在内容行里（跟着滚）。

实测修前原文（`ui.Tester`，本仓 HEAD）：

```
640x360 strip={X:0 Y:28 W:640 H:332} MaxY=0.0   Safe paste={Y:356 H:3}
720x360 strip={X:0 Y:28 W:720 H:332} MaxY=0.0   Safe paste={Y:356 H:3}
720x600 strip={X:0 Y:235 W:720 H:365} MaxY=0.0  Safe paste={Y:563 H:22}
```

最小高度下 `Safe paste` 只剩 3u 高（22u 的行被底边裁掉 19u），`Terminal palette` 整行在
视口外；`MaxY=0` 意味着**永远够不到**。

## 3. 落地

- `internal/terminalui/drawer.go:66`：`strip` 从 `ui.Row(...).Shrink(0)` 改为
  `ui.Scroll(c).FillWidth().TrackScroll(&a.DrawerScroll)`；`Background` / `BorderWidth`
  / `BorderColor` / `Label` 留在条带（视口）上；`Padding` / `Gap` / `AlignItems` 与网格、
  关闭钮放进内层 `ui.Row(c).FillWidth()` 内容行。
- `internal/terminalui/terminalui.go:130`：新增 `DrawerScroll ui.ScrollState`（App 级，
  跨帧保留）。注释写清它只在面板比条带矮时有意义。
- `internal/terminalui/terminalui.go:890` 的画布地板 `MinHeight(t.Space(7))` **原样保留**
  （P11-at 的「不让开浮条带」不动）。
- 顺手清掉了一处**死代码**：内层内容行最初写成 `Shrink(0)`，实测把它去掉后，`720×600`
  与 `640×360` 的导出 PNG **逐字节相同**（`08b8334418...`）——因为每一行抽屉都带
  `MinHeight(Space(7.5))`，列本来就压不扁它。留着会误导，改为注释说明「自然高来自行自己
  的 `MinHeight`」。

`drawerGrid` 里网格自己的 `Shrink(0)` **保留**：实测删掉会改变像素（`08b83344...` →
`bd01c6ab...`），它是 P11-at 起就有的宽度约束，不属本轮范围。

## 4. 台账（V / B / S）

- **V = 0**：没有一个墨、半径、字号、padding、gap、floor 被改。
- **B = 0**：既有 pin 零改动全绿（`TestTheDrawerKeepsTheBarsBand`、
  `TestTheTerminalDrawerShowsTheAppearanceRows`、`TestTheDrawerGridCellsKeepTheReferenceFloor`、
  `TestTheDrawerRowsKeepTheirWordsInsideTheirCells`、`TestTheTerminalViewsPaintFromTokensNotLiterals` 等）。
- **S = 1**：条带从 `ui.Row`（视口=行）变成 `ui.Scroll`（视口）+ 内层内容 `ui.Row`。
  **窗口够高时导出 PNG 与 HEAD 逐字节相同**（`720×600`：HEAD `3463ea4b...` = 本轮
  `3463ea4b...`）——结构变了，像素没变。
- 新增测试函数 3 个（2 pin + 1 opt-in dump）：
  - `TestTheDrawerScrollsItsLastRowsIntoView`（`internal/terminalui/drawerscroll_test.go`）：
    2 语言 × 2 最小尺寸，断言 `MaxY > 0` 且滚到头后最后的 `Terminal palette` 行完整落在
    条带视口里。
  - `TestTheDrawerDoesNotScrollWhenItFits`：`720×600` × 深/浅，断言 `MaxY == 0` 且偏移为 0。
  - `TestDumpP11AUDrawerScrolledPNGs`（`FLOTER_PNG=1`，SKIP）。

## 5. 图（`FLOTER_PNG=1`）

`/tmp/floter-P11-au-drawer-end-{light,dark}.png`（640×360 = 最小宽 × 最小高，终端页 + 会话 +
抽屉滚到最末），对照 `/tmp/floter-P11-au-drawer-end-{light,dark}-before.png`。

**我实测在这两张图（浅色臂）上看到了什么**（逐行扫墨带，`y` 是像素行）：

- **修前**：抽屉区最后一条墨带是 **`y=357…359`**——贴着底边被切出的 3 像素字头（`Safe
  paste` 那一行只剩这么高），下面再没有整行；把 `DrawerScroll` 推到头也**不会动**（`MaxY=0`）。
- **修后**：滚到最末后，最后一条墨带是 **`y=324…343`**——一条 20 像素高的**完整行**
  （末尾的 palette 行），底边 `y=357…359` 现在是干净的面色、没有半截字；整段内容上移了
  33u（= `MaxY`），最上面那条墨带从 `50…65` 移到 `28…32`（早先行的尾巴）。

也就是说：修前用户能看到的最底一行是被切成 3px 的字头、且够不到其余；修后滚一下就能看到
完整的最后一行，底边不再有半截字。

## 6. 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2388（基线 2386）
grep -c -- "--- SKIP"                                  → 32（基线 31，+1 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 1954（基线 1951）
三平台 build                                            → 过
```

## 7. 变异（3 条实跑变红并还原）

`internal/terminalui/drawer.go` 的 `md5sum` 基线 = `5aa4f9dc4b067e1eefb6995ea5492b22`，
每条改完 `touch` 后复跑，还原后 `md5sum -c` 逐字节核对 = 成功。

1. **条带改回 `ui.Row(c).FillWidth().Shrink(0)`（去掉 `TrackScroll`）** →
   ```
   drawerscroll_test.go:51: en 640x360: the drawer reports no scroll range (MaxY=0.0) —
   the strip is 332.0u tall and its content does not fit, so the last rows are clipped
   with no way to reach them
   --- FAIL: TestTheDrawerScrollsItsLastRowsIntoView
   ```
2. **删掉 `TrackScroll(&a.DrawerScroll)`（保留 `ui.Scroll`）** → 同一条报错
   （状态不再被写，`MaxY` 仍是 0）。
3. **`ui.Scroll` 换成 `ui.Box`（不设滚动位）** → 同一条报错（容器不再按滚动元素给
   `minMain=0`，面板直接裁掉内容）。

> 说明：三条都先撞在「`MaxY <= 0`」这条主断言上。pin 的第二半（「滚到头后最后一行完整
> 在视口里」）是给「报了范围却没把最后一行移进来」这种部分修复留的守门，**没有被独立
> 变异验证**——一个只改 `MaxY`、不改位移的注入需要改框架，不属本轮。

## 8. 仍待（P11-au）

1. **抽屉关闭再打开时的滚动偏移**：现在**保留**（像常驻面板）。若用户要「一打开就在顶
   部」（同 P11-ao / P11-ar「一份新内容从自己的开头开始」），在 `ToggleDrawer` 打开那一刻
   置 `a.DrawerScroll = ui.ScrollState{}` 即可——**一行**，但那是**交互决定**，本轮没擅自做。
2. 上一轮那条拍板项原样有效：外观抽屉要不要在窄窗口（< 900）折成**单列**。
3. 五条老拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
4. 侦察结论（本轮实测，别当 TODO 重领）：`900×360` 及以上宽度的最小高度**不需要滚动**
   （900 宽网格自然高 261 < 视口，`MaxY=0`）；只有 640/720 宽的最小高度要。
