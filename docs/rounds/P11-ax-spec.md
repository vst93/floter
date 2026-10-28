# P11-ax 规格：抽屉关闭再打开，从它自己的开头开始

## 1. 意图一句话 + 为什么现在做

终端的外观抽屉（drawer）关闭再打开时，**保留**上一次的滚动偏移——一份新打开的
抽屉出现在上次滚到的位置。改成「每次打开都从开头开始」，与 P11-ao（换设置页从顶
上开始）、P11-ar（一份新的捕获输出从它自己的开头开始）同一条规则。

**为什么现在做**：用户 2026-10-28 定的优先级是「界面质感与交互逻辑优先」。这是
P11-au/P11-av/P11-aw 连续三轮「仍待」第 1 条——一直标着「交互决定，需拍板」。
但它的答案已经由同一个仓库的两条先例给出：

- `internal/settingsui` 的换页回顶（P11-ao）；
- `internal/launcher` 的新输出从开头开始（P11-ar）。

两处都是「一份**新**内容打开时，从它自己的开头开始」，而抽屉的关闭再打开正是
同一件事。本轮按这条既有规则落地，不再当拍板项挂着。

## 2. 逐条落地项（动手前已读代码确认行号）

### 落地项 1：`ToggleDrawer` 打开时归零滚动偏移

- **位置**：`internal/terminalui/drawer.go:17`
- **现状**：
  ```go
  func (a *App) ToggleDrawer() { a.Drawer = !a.Drawer }
  ```
  它只翻 `a.Drawer` 布尔，`a.DrawerScroll`（`terminalui.go:130`）原样保留。
- **应改成**：打开时（`!a.Drawer` → `a.Drawer`）置 `a.DrawerScroll = ui.ScrollState{}`。
  关闭时不动（关着的抽屉没有可见的滚动位置；下一次打开已经归零）。
- **为什么不碰逐帧路径**：`strip.TrackScroll(&a.DrawerScroll)` 每帧都跑
  （`drawer.go:74`），若在绘制里归零，滚轮每帧被清零——P11-ar 的教训
  （`showOutput` 只在换页时重置，逐帧路径不碰）。

### 落地项 2：回归 pin

- **新 pin**：`internal/terminalui/drawerscroll_test.go` 的
  `TestReopeningTheDrawerStartsAtItsTop`：
  - 最小高度（`640×360`）打开抽屉 → 滚到 `MaxY` → 关闭 → 再打开；
  - 断言 `a.DrawerScroll.Y == 0`（从开头开始）。
- **同一条 pin 的第二臂**（防止「干脆把滚动关掉」）：抽屉**开着**时滚轮仍然能滚
  （`MaxY > 0` 且滚完之后 `Y > 0`）——归零只发生在「打开」这个动作上。

## 3. 行为契约零改动红线

- **高度链**：不动（抽屉条带高度、`MinHeight(Space(7))`、面板列全是既有值）。
- **键盘模型**：不动（抽屉的键处理、`Esc` 关闭、齿轮开合全不碰）。
- **会话管理**：不动。
- **设置写回**：不动（`a.set` 不变）。
- **i18n 文案**：不动。
- **命令接线**：不动（`a.Actions.Settings = a.ToggleDrawer` 不变）。
- 视觉轮只改「一份新抽屉的起始偏移」，不改墨与面。

## 4. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
三平台 go build ./...
```

**本轮开工基线（实测）**：PASS **2411** / FAIL **0** / SKIP **34** / 测试函数 **1959**。

## 5. mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

1. **M1**：把 `ToggleDrawer` 的归零那行删掉（回到 HEAD 的 `a.Drawer = !a.Drawer`）
   → `TestReopeningTheDrawerStartsAtItsTop` 的第一臂红（`Y` 仍是 `MaxY`）。
2. **M2**：在 `drawer()` 的逐帧路径里归零（`strip.TrackScroll` 之前置
   `a.DrawerScroll = ui.ScrollState{}`）→ `TestTheDrawerScrollsItsLastRowsIntoView`
   红（滚到 `MaxY` 之后下一帧又被清零，`Palette` 行回到底边外）；同时
   `TestReopeningTheDrawerStartsAtItsTop` 的第二臂红（开着时滚不动）。
3. **M3**（可选）：把归零写成 `a.DrawerScroll.Y = 0`（只清偏移、不清 `MaxY`）
   → 第一臂可能仍绿（`Y==0`），但第二臂需要 `MaxY` 仍 > 0；这条用来确认 pin
   的第二臂不是靠「连 MaxY 一起清掉」过的。
