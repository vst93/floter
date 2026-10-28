# P11-ap 规格 · 一条没有底的浮带不该吃掉它下面那行的指针

## 1 意图

启动器的 chips 行（剪贴板/浏览器/计算器模式的 `全部/收藏/图片…`）浮在结果列表上，
它的 20u 带子**什么也不画**，底下的行照常可见。这条带子现在却是命中目标：从 chips 到
窗口右边，指针落在带子里既不悬停任何行、也点不动任何行。本轮让它把指针交还给下面那一行。

为什么现在做（选题顺序）：用户实测反馈 F1/F2/F3 已由 P11-y/z/aa 收完；合约 §6 全表
`已落地`，只剩 `⌘K` 动作面＝拍板；上一轮「仍待」第 1 条是五条拍板项。于是按
「界面质感与交互逻辑 → 基础功能 → 一致性 → 像素」里的第一档找，本轮找到的是一条
**交互逻辑的真 bug**（不是像素细节）。

## 2 证据（实测，动手前）

夹具：剪贴板模式，40 条文本条目，720×488 帧，滚轮下滚两格（`Visible()` 从 0 到 3）。

| 量 | 值 |
|---|---|
| 字段行 | `t.Space(14)` = 56（`height.go:23` `fieldRowUnits`） |
| chips 行的顶 | `fieldRow + Space(1)` = **60**（`launcher.go:1289`） |
| chips 行的带高 | `filtersHeight` = `Space(5)` = **20**（`filters.go:170`）→ 带子 **60..80** |
| 列表的顶内边距 | `listEdge = edge + chips` = 80（`launcher.go:1182`），随内容滚走 |
| 行的盒子（实测） | 标题矩形 y=70 → 盒子 **54..94**（40u 行 + 6u 间距，下一行 116） |

滚两格后逐点扫指针（每次 `hoveredRow` 归 -1 再 `Move` + `Frame`）：

```
x=200 dead(无悬停): 40 42 44 46 48 50 52 54 60 62 64 66 68 70 72 74 76 78 80
x=340 dead:         40 42 44 46 48 50 52 54 60 62 64 66 68 70 72 74 76 78
x=700 dead:         40 42 44 46 48 50 52 54 60 62 64 66 68 70 72 74 76 78
```

`40..54` 是字段块（不透明的 `FieldFill`，行在它下面被盖住，看不见——吃指针是合法的）。
`60..78/80` 是**这条空带子**：全宽、20u，行在里面看得见，却既不高亮也点不动。
点击实测：`ClickAt(340, 70)`（带子里）`Selected` 仍是 0；`ClickAt(340, 120)`（带子下）
选到 1。

参考侧不会出现这个形状：`.launcher-filter` 是 `.launcher-results` 滚动器的**兄弟**
（正常流，`main:src/App.tsx:3407`；`main:src/styles/launcher.css:450` 是
`position: relative`），行永远不从它底下过。本仓让行从字段与 chips 底下滚过去是**既有
设计**（`launcher.go:1176-1182` 的注释），所以这条浮带必须自己声明透明——与 P11-af 的
终端标题栏同一条判据（`internal/terminalui/terminalui.go:1310,1317`）。

## 3 逐条落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/launcher/filters.go:187` 的绝对定位 `ui.Row` | 无 `PassThrough`，整条 20u 带子是命中目标 | 加 `.PassThrough()`：带子自身不是命中目标，**子节点照旧是**（`ui/runtime.go:1210` 只跳过自己的框） |
| `internal/launcher/filters.go` 的提示字 `ui.Text(...).Grow(1).TextAlign(End)` | 它的盒子是整个 `Grow(1)` 尾巴（实测把 x=340 也盖住），指针落在提示字那一整段空白上同样死 | 加 `.PassThrough()`：提示是字，不是控件（参考 `.launcher-filter__hint` 从不吃指针） |

不动的：chips 本身的 `Focusable`/`Clicked`（它们是带子里唯一的控件，必须仍可点）；
带子的位置、高度、字阶、颜色、几何；`listEdge` 与窗口高度链（`filtersHeight` 一字节不改）。

## 4 行为契约零改动红线

高度链（`fieldRowUnits`/`breathUnits`/`rowUnits`/`filtersHeight`）、键盘模型（Tab 换 chip
的逻辑在 `cycleFilter`，不碰）、会话管理、设置写回、i18n 文案、命令接线——本轮一个都不动。
视觉轮只改「谁接指针」。

## 5 门槛与基线（本轮实测，用当前值）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS                     → 2377（基线 2376）
--- SKIP                     → 28（基线 27，+1 是 opt-in 截图 dump）
测试函数                     → 1943（基线 1941）
三平台 go build ./...        → 过
```

## 6 mutation 计划（≥2，说明各自让哪个 pin 变红）

1. 摘掉带子那一行的 `.PassThrough()` → `TestTheChipsBandLeavesTheRowsBeneathItToThePointer`
   在 `(340, 61)` 起的每个采样点报 `finds no row`。
2. 只摘掉提示字的 `.PassThrough()`（带子的留着）→ 同一条 pin 同样红：提示字的 `Grow(1)`
   盒子横跨带子中段，x=340 与 x=660 都在它里面。
