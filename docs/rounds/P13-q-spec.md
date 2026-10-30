# P13-q 规格 —— 设置页动作面的「按下」档（三档阶梯的最后一档）

## 意图一句话

设置页里凡是「点一下就执行」的面，静息 → 悬停 → **按下** 必须三档都在；
这一轮补的是**按下**那一档：面沉到 `ControlPress`，几何按族的规矩收一档。

## 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律转通用队列的**状态反馈**格。
`docs/liquid-glass.md` §5 已经写死「按下」这一档，`internal/theme/theme.go:174-181`
的注释也写着 `ControlPress` 是「a pointer down sinks it *below* the resting face」。
但 census（`grep -n "Hovered()"` / `grep -n "Pressed()"`）显示这一族只有一半：

| 面 | 悬停档 | 按下档 |
|---|---|---|
| `barButton`（`settingsui.go:5795`，`.extensions-action-button`） | 有 | **有**（只 `Scale(0.97)`，注释引 `extensions.css:138-140`） |
| `extensionActionFace`（`rows.go:478`，**同一个 CSS 类**） | 有 | **无** |
| `extensionActionButton`（`rows.go:530`，`.extensions-icon-button`） | 有 | **无** |
| `paramIconButton`（`parameditor.go:261`，同上） | 有 | **无** |
| `sessionIconButton`（`rows.go:790` 一带，28u 方格） | 有 | **无** |
| `headerButton`（`settingsui.go:1183`，`.toolbar-button` 28u） | 有 | **无** |
| `sheetActionButton`（`pluginsheet.go:366`，弹层里的胶囊） | 有 | **无** |
| 启动器 `rowControl` / 折叠卡设置格（20u/28u） | 有 | **有**（`ControlPress` + `Scale(0.92)`） |

同一个 CSS 类的两处实现不一致（`barButton` 有 `:active`，`extensionActionFace` 没有），
同一枚 28u 小格在启动器上有三档、在设置页只有两档 —— 这不是设计选择，是漏项。
按下时**完全没有反馈**，指针下去的那一下和悬停一模一样，读起来像「点了没反应」。

## 逐条落地项

族的规矩（写进代码注释，一处引用一处）：

- **胶囊**（`.extensions-action-button` 一族，高 24~28u）：`Scale(0.97)` + `ControlPress` 面。
  0.97 来自 `extensions.css:138-140` 的 `:active`，`barButton` 已经在用，本轮给它补上面。
- **小格**（28u 见方的图标钮）：`Scale(0.92)` + `ControlPress` 面 + 墨到 `TextStrong`。
  0.92 来自启动器 `rowControl`（20u）与折叠卡设置格（28u），同一个应用里同尺寸同触感。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go` `extensionActionFace` | 只有 hover 抬面 | 加 `Pressed()` 臂：`ControlPress` 面 + `Scale(0.97)`，墨 `TextStrong` |
| 2 | `internal/settingsui/rows.go` `extensionActionButton` | `switch` 只有 disabled / hover | 加 `case button.Pressed():`（在 hover 之前，`ControlPress` + `Scale(0.92)`） |
| 3 | `internal/settingsui/rows.go` `sessionIconButton` | 同 2 | 同 2 |
| 4 | `internal/settingsui/parameditor.go` `paramIconButton` | 只有 hover 抬面 | 加按下臂（`ControlPress` + `Scale(0.92)`） |
| 5 | `internal/settingsui/settingsui.go` `headerButton` | 只有 hover 抬面 | 加按下臂（`ControlPress` + `Scale(0.92)`） |
| 6 | `internal/settingsui/pluginsheet.go` `sheetActionButton` | 只有 hover 抬面 | 加按下臂（`ControlPress` + `Scale(0.97)`；`danger` 臂的面是透明的，按下时给面，不给线） |
| 7 | `internal/settingsui/settingsui.go` `barButton` | 有 `Scale(0.97)`、**没有面** | 补 `ControlPress` 面，与 1/6 同一句 |

## 行为契约零改动红线

- 只改**墨与几何变换**：`Background` / `Scale`。不碰高度链、padding、圆角、`Border`、
  文字、i18n、点击回调、焦点顺序、键盘模型。
- `Scale` 是渲染变换（mygo 的 pose transform），不改布局盒 —— 高度链一个像素不动。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线（P13-p 实测）：PASS **2953** / FAIL 0 / SKIP 74 / 测试函数 **2176**。

## mutation 计划（≥2 条，实跑）

- **M1**：`extensionActionFace` 的按下臂退回（删掉），`TestPressedActionFacesSinkTheFace` 必须红。
- **M2**：`sessionIconButton` 的 `case button.Pressed()` 删掉，像素 pin（按下态面值）必须红。
- **M3**：把按下的 `Scale(0.92)` 写成 `Scale(1,1)`（面在、几何不在），几何 pin 必须红。
