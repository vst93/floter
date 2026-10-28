# P11-ay 规格：抽屉「滚到末」的 pin 钉到真正被裁的那一行

## 1. 意图与为什么现在做

P11-ax 报告 §6 的 **M2** 实测发现：`TestTheDrawerScrollsItsLastRowsIntoView`
（P11-au 的正面 pin）**对「滚动是否工作」不敏感**——它声称钉「滚到末以后最后几行
完整落在视口里」，但它断言的 `Terminal palette` 行在 `640×360` 的 `Y=0`（未滚）时
就已经整行落在视口里（`strip={Y:28 H:332}`、`palette={Y:224 H:22}`）。于是即使把
滚动彻底关掉，这条 pin 的「滚到末」那一步也照样绿。

P11-ax 把它记进「仍待」第 1 条（「要重造夹具，另开一轮」），本轮就是那一轮。
方向属用户 2026-10-28 定的优先档：界面质感与交互逻辑（P11-ax 已确认这条属交互逻辑
的正确性，不是像素细节）。

## 2. 根因（实测，本轮复测）

`640×360` 实测抽屉各行几何（`internal/terminalui/drawer.go` 的网格，`en`）：

| 行 | Y（Y=0） | H（Y=0） | Y（滚到末） | H（滚到末） |
|---|---|---|---|---|
| `Terminal appearance`（条带） | 28 | 332 | 28 | 332 |
| `Font size` | 46 | 22 | — | — |
| `Terminal palette` | 224 | 22 | 191 | 22 |
| `Palette preview` | 259 | 88 | — | — |
| `Copy on select` | **357** | **3** | 324 | 22 |
| `Safe paste` | **357** | **3** | 324 | 22 |

即：**真正被裁的是网格最后一行**（`Copy on select` / `Safe paste`，Y=0 时只有 3u 高，
被条带底边 360 裁掉 19u）。旧的 pin 断言的 `Terminal palette` 在 Y=0 时已经完整，
所以对滚动不敏感。

## 3. 落地项

位置：`internal/terminalui/drawerscroll_test.go` 的
`TestTheDrawerScrollsItsLastRowsIntoView`。

- **现状**：滚到末后 `tt.Find(scopy.Palette)`，断言其矩形落在条带视口内。
  `Palette` 在 Y=0 时 `H=22`（完整），对滚动不敏感。
- **应改成**：
  1. **前置断言**：Y=0 时 `tt.Find(scopy.PasteSafe)` 的 `H < wholeRow`（20u）——
     夹具必须真的让最后一行被裁（否则测试空转）。
  2. **滚到末**：`a.DrawerScroll.Y = math.MaxFloat32` → `tt.Frame()` → 断言
     `tt.Find(scopy.PasteSafe)` 的 `H >= wholeRow`（回到整行高，不再被裁）。
  3. 保留原有的「落在条带视口内」矩形断言（`last.Y >= strip.Y` 且
     `last.Y+last.H <= strip.Y+strip.H`）。
- `wholeRow = 20.0`：网格最后一行是 `drawerRow` 的 `MinHeight(t.Space(7.5))`（默认
  档 22u）；被裁时实测 `H=3`，整行时 `H=22`。20 是「整行」的保守下界。

**为什么钉 `PasteSafe` 而不是别的**：它是网格的**最后一行**（`Copy on select` 与它
同一行、同在网格末尾），Y=0 时被裁得最狠（3u）。`Palette` 是网格**最宽**的那一行，
排在中间，永远在视口内——这正是旧 pin 失效的原因。

## 4. 行为契约零改动红线

- **产品代码零改动**：本轮只改测试断言，`internal/terminalui/drawer.go` 一个字节不动
  （`md5sum` = `957c841cddbc5c7b851406528a78efec`）。
- **高度链**：不动。
- **键盘模型**：不动。
- **会话管理**：不动。
- **设置写回**：不动。
- **i18n 文案**：不动。
- **命令接线**：不动。
- 视觉轮只改「一条 pin 钉哪一行」，不改墨与面。

## 5. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
三平台 go build ./...
```

**本轮开工基线（实测）**：PASS **2412** / FAIL **0** / SKIP **34** / 测试函数 **1960**。

## 6. mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

1. **M1**：把 `drawer()` 的 `strip.TrackScroll(&a.DrawerScroll)` 之前加一行
   `a.DrawerScroll = ui.ScrollState{}`（每帧清零，即 P11-ax 报告 §6 的 M2 缺陷）
   → 新 pin 的**第二臂**（`last.H < wholeRow`）红：滚到末后最后一行仍是 `H=3`。
   **这条正是旧 pin 会漏掉的**（旧 pin 断言 `Palette`，每帧清零后它仍在视口内）。
2. **M2**：删掉 `.TrackScroll(&a.DrawerScroll)` → 条带不再报范围
   → 新 pin 的**第一臂**（`MaxY <= 0`）红：`MaxY=0`，最后几行没有可滚的距离。
