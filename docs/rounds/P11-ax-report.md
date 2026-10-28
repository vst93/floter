# P11-ax 报告：抽屉关闭再打开，从它自己的开头开始

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目
（`docs/raycast-design.md` §6 全表 `已落地`，只剩 §4.6 的 `⌘K` 动作面＝**拍板**）→
**上一轮「仍待」第 1 条**（P11-au/av/aw 连续三轮挂着「抽屉关闭再打开时的滚动偏移」）。

前几轮把它标成「交互决定，需拍板」，但答案其实已经由本仓两条先例给出：

- P11-ao：换一个设置页从顶上开始；
- P11-ar：一份新的捕获输出从它自己的开头开始。

两者都是「一份**新**内容打开时，从它自己的开头开始」。抽屉的关闭再打开正是同一件事，
本轮按这条既有规则落地，不再当拍板项挂着。方向属用户 2026-10-28 定的「交互逻辑」优先档。

## 2. 根因（实测）

`DrawerScroll` 是 **App 级**字段（`internal/terminalui/terminalui.go:130`），和列表的
`ListState` 一样，容器要跨帧保留偏移。但 `ToggleDrawer`（`internal/terminalui/drawer.go:17`）
只翻 `a.Drawer` 布尔：

```go
func (a *App) ToggleDrawer() { a.Drawer = !a.Drawer }
```

于是关闭再打开时，上一轮滚到的偏移原样留着——一份「新打开的抽屉」出现在上次滚到的
位置。实测（`640×360`，`MaxY=33`）：滚到 `MaxY` 后关闭、再打开，`DrawerScroll.Y` 仍是
`33.0`（= `MaxY`），即新抽屉停在最末一行。

## 3. 落地

`internal/terminalui/drawer.go` 的 `ToggleDrawer`：打开时（`!a.Drawer` → `a.Drawer`）置
`a.DrawerScroll = ui.ScrollState{}`。关闭时不动（关着的抽屉没有可见偏移；下一次打开
已经归零）。**逐帧路径不碰**——`strip.TrackScroll(&a.DrawerScroll)`（`drawer.go:86`）每帧
都跑，若在绘制里归零，滚轮每帧被清零（P11-ar 的教训：`showOutput` 只在换页时重置）。

归零的是整个 `ScrollState{}`（含 `MaxY`），不是只把 `Y` 设 0：`MaxY` 由 `TrackScroll`
在下一帧按布局重新算，先清成零值不会留下上一份内容的旧范围。

## 4. 台账（V / B / S）

- **V = 0**：产品墨与面零改动（抽屉的面、线、几何、字阶一个字节没动）。
- **B = 0**：行为契约零改动（高度链、键盘模型、会话管理、设置写回、i18n、命令接线全不碰）。
  唯一的行为变化是「打开抽屉时的起始偏移」，这是本轮的目标本身。
- **S = 0**：结构没变，只改了 `ToggleDrawer` 的一处赋值 + 新增一条 pin。
- 既有 pin：`internal/terminalui` 全套零改动全绿（含 `TestTheDrawerScrollsItsLastRowsIntoView`、
  `TestTheDrawerDoesNotScrollWhenItFits`、`TestTheDrawerSeamIsOnItsTopEdge` 等）。

## 5. 新 pin

`internal/terminalui/drawerscroll_test.go` 的 `TestReopeningTheDrawerStartsAtItsTop`
（`640×360` 最小窗口，`MaxY=33`）：

1. **第一臂（重开回顶）**：打开 → 滚到 `MaxY` → 关闭 → 再打开 → 断言
   `a.DrawerScroll.Y == 0`。
2. **第二臂（开着时仍能滚）**：抽屉开着时置 `Y = MaxFloat32` → `Frame` → 断言
   `a.DrawerScroll.Y > 0`。这条防止用「干脆把滚动关掉」让第一臂变绿（那会让 P11-au 的
   「最后几行够得到」重新坏掉）。

## 6. mutation（两条实跑变红并还原）

`md5sum` 逐字节核对还原（`drawer.go` = `957c841cddbc5c7b851406528a78efec`），`touch` 后复跑确认绿。

**M1** —— 删掉 `ToggleDrawer` 的归零（回到 HEAD 的 `a.Drawer = !a.Drawer`）：
```
--- FAIL: TestReopeningTheDrawerStartsAtItsTop
    a reopened drawer starts scrolled (Y=33.0, was 33.0 at its end) — a fresh drawer
    must start at its own top (P11-ax, the same rule as P11-ao/P11-ar)
```

**M2** —— 把归零挪进逐帧路径（`drawer()` 里 `strip := ui.Scroll(...)` 之前置
`a.DrawerScroll = ui.ScrollState{}`）：
```
--- FAIL: TestReopeningTheDrawerStartsAtItsTop
    the drawer did not scroll to its end (Y=0.0)
```
（每帧清零 → 滚轮永远滚不动，第二臂的意图被第一臂的「滚到末」前置检查先抓到。）

两条都还原后 `md5sum -c` 一致、`git diff` 只剩预期的那一处。

### 侦察结论（本轮实测，别当 TODO 重领）

**M2 没有让 `TestTheDrawerScrollsItsLastRowsIntoView` 变红。** 那条 P11-au 的 pin 声称钉
「滚到末以后 `Terminal palette` 行完整落在视口里」，但实测（`640×360`）：`Palette` 行在
**`Y=0`（未滚）时就已落在条带视口内**（`strip={Y:28 H:332}`，`palette={Y:224 H:22}`，
`224+22 < 28+332`），所以它「滚到末」那一步即使滚动被彻底关掉也照样绿——**这条 pin 的
「滚到末」臂对「滚动是否真的工作」是不敏感的**。本轮的 `TestReopeningTheDrawerStartsAtItsTop`
第二臂补上了这一格（`MaxFloat32` 之后断言 `Y > 0`），M2 实跑证明它会红。P11-au 的那条
pin 本身是否要重造（换成「内容确实比视口高、末行在 Y=0 时被裁」的夹具）属另一轮，记在
`.pending-bugs.md` 的「仍待」。

## 7. 视觉证据（我实测看到了什么）

`/tmp/floter-P11-ax-drawer-{light,dark}-{scrolled,reopened}.png`（`FLOTER_PNG=1`，
`640×360` 最小窗口 + 会话 + 抽屉）。四张图四个不同的 md5（浅/深 × 滚到末/重开），
说明「重开」确实换了一帧，不是同一张图换个名字。

我量了抽屉里两个标签的盒子（`tt.Find`，浅色）：

| | `Font size` 行 | `Terminal palette` 行 |
|---|---|---|
| 滚到末（scrolled） | `{Y:29 H:6}` —— 第一行被顶出视口，只剩 6u 字头 | `{Y:191 H:22}` |
| 重开（reopened） | `{Y:46 H:22}` —— **整行落在顶部** | `{Y:224 H:22}` |

**我看到的那处变化**：滚到末再关、再开，抽屉的第一行 `Font size` 从「只剩 6u 的
字头」变回「完整 22u 落在条带顶部」——新抽屉从它自己的开头开始，而不是停在上一轮
滚到的末尾。

## 8. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...`：净
- `GOOS=windows go vet ./...`：净
- `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：全绿（FAIL 0）
- `go test -count=1 -v ./... | grep -c -- "--- PASS"`：**2412**（基线 2411 → +1）
- 三平台 `go build ./...`：过
- SKIP：**34**（不变）
- 测试函数：**1960**（1959 → +1）

## 9. 仍待

1. **P11-au 的「滚到末」pin 对「滚动是否工作」不敏感**（见 §6 侦察结论）——要重造夹具，
   另开一轮。
2. 老拍板项原样有效：外观抽屉 < 900 折单列 / caret / 默认浅色 3.04:1 / 终端滚动缓冲 /
   `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 的终端便条投影。
3. **侦察结论（本轮实测，别当 TODO 重领）**：终端页浅色整帧是纯白 `(255,255,255)`——这是
   **预期的**：插件 `terminal.LightTheme().Background` 就是 `#FFFFFF`，`Transparent: true`
   下 `draw` 在 `colors.Background != theme.Background` 时填这个背景色，所以浅色会话的画布
   是白底（深色会话恰好与面板同色 `(24,24,27)` 才显得「没填」）。**不是缺陷**，别当
   「浅色画布没画面板」重领。
