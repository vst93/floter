# P11-at 规格：抽屉不让出浮条那条带

## 意图

用户 2026-10-28 定的优先档是「界面质感与交互逻辑」，F1/F2/F3 已由 P11-y/z/aa 收完，本轮按
「上一轮『仍待』→ 自己找」继续在这档里找。找到的是一处**画在两块像素上的同一件东西**：
终端窗口缩到它自己允许的最小高度（`settings.MinTerminalHeight` = **360**，`internal/settings/settings.go:95`）
再打开外观抽屉，抽屉从 **y=0** 开始画——正好画在浮着的标题条底下。

## 证据（本轮实测，`ui.Tester` 的绘制盒）

| 窗口 | 报错串（原文，修前） |
| --- | --- |
| 720×360 | `"Font family" {X:12 Y:17 W:81 H:22} paints over "Terminal" {X:12 Y:3 W:628 H:22} (overlap 81.0x8.0)` |
| 720×360 | `"Close terminal settings" {X:680 Y:10 W:28 H:28} paints over "Close terminal" {X:680 Y:0 W:28 H:28} (overlap 28.0x18.0)` |
| 720×360 | `"14 px" {X:642 Y:19.5 W:28 H:17} paints over "New command" {X:648 Y:0 W:28 H:28} (overlap 22.0x8.5)` |
| 640×360（= 最小宽 × 最小高） | 同上三条（`"字体"` / `"关闭终端设置"` / `"14 px"`，zh 同样命中） |
| 720×380 | `"Close terminal settings" {X:680 Y:25 W:28 H:28} paints over "Close terminal" {X:680 Y:0 W:28 H:28} (overlap 28.0x3.0)` |

## 根因

`internal/terminalui/terminalui.go` 的 `View` 是一列：画布 `body := ui.Box(c).Grow(1).FillWidth()`、
抽屉（`internal/terminalui/drawer.go:57` 的 `strip`，`Shrink(0)` = 自然高度）、标题条
（`titleBar`，`Absolute().Top(0).Height(Space(7))`，浮在最上层）。画布没有地板高度，抽屉按网格
的自然高度要位置：720 宽时网格自然高 **365**，比面板的最小高度 360 还高，于是画布被压到 0、
抽屉从 y=0 起画，标题条又浮在 y=0..28 —— 两段字落在同一块像素上。

## 落地项

1. `internal/terminalui/terminalui.go` · `View` 的画布 → 加 `MinHeight(t.Space(7))`。
   浮条的带宽就是画布的地板：抽屉放不下时排在画布之下、被面板底边裁掉，绝不上移。
   **`Space(7)` 是 `titleBar` 自己的高度**（`terminalui.go:1309` 的 `Height(t.Space(7))`），
   不是新常量的猜测。
2. `internal/terminalui/drawer.go` · `drawer` 的注释写清这条地板与「多出来的行被面板底边裁掉」
   这一后果，以及为什么没有顺手做成滚动（见报告 §5：本布局里滚动容器的 `MaxY` 恒为 0）。
   代码零改动（`Shrink(0)`、`AlignItems(Start)`、padding、gap 与修前逐字节相同）。

## 行为契约零改动红线

高度链：只有画布地板一条新增，浮条仍是 `Absolute` 的 28u、抽屉仍是自然高度；键盘模型、
会话管理、设置写回、i18n 文案、命令接线零改动。墨与面零改动。

## 门槛（本轮实测基线）

```
gofmt -l cmd internal                                  → 空
go vet ./... / GOOS=windows / GOOS=darwin              → 全净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2386（基线 2385）
grep -c -- "--- SKIP"                                  → 31（基线 30，+1 opt-in 截图 dump）
func Test 计数                                          → 1951（基线 1949，+2）
三平台 build                                            → 过
```

## mutation 计划（≥2，实跑）

1. 把画布那条 `MinHeight(t.Space(7))` 删掉 → `TestTheDrawerKeepsTheBarsBand` 在
   `en 640x360` 报 `the drawer starts at y=0.0, the bar's band ends at y=28.0`，
   并连带报出修前那三条重叠串。
2. 把 `Space(7)` 改成 `Space(3)`（半条带）→ 同一条 pin 在 640/720 两个宽度、两个语言上
   报 `y=12.0` 仍然压进浮条，证明 pin 量的是「与浮条的关系」而不是「某个具体数」。
