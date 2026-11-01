# P13-cn 规格：设置页最后两个「两档的可点面」

> 队列：通用队列 → **状态反馈** 的另一半（禁用 / 只读面之外的那一半：**可点面**）。
> 上一轮 P13-cm 处理的是「同一个结局被说成两件事」（墨）；这一轮处理「按住不放的那一帧
> 与悬停帧逐像素相同」（面）。同一族在 P13-q / P13-al / P13-bv / P13-bw / P13-bx 已经修过
> 五批，**这一轮把这一族结案**。

## 意图一句话

设置页里还剩两个**可点面只有两档**（静息 / 悬停）的地方，按住不放的那一帧与悬停帧
**逐像素相同**——手指按下去，界面一声不响。给它们补上第三档（按下 = 往静息的回退），
并且只动面，不动几何、不动文案、不动行为。

## 为什么现在做（证据，动手前实测行号）

先量账：把 `internal/{shell,settingsui,launcher,terminalui}` 里
`Hovered()` 有、`Pressed()` 没有的函数挑出来，逐个看：

| 位置 | 现状 | 判 |
|---|---|---|
| `internal/launcher/filters.go:219` | 过滤 chip 只有 hover | **需拍板**（§6.40-5：参考自己就没有面） |
| `internal/launcher/power.go` | 确认带 chip 只有 hover | **需拍板**（§6.40-4：暖面没有按下档 token） |
| `internal/settingsui/settingsui.go:1393/1405` `sidebarRow` | 悬停抬到 `RowHover`；按下**什么都不做** | 本轮改 |
| `internal/settingsui/settingsui.go:7084` `runOutputToggle` | 悬停抬到 `ControlHover`；按下**什么都不做** | 本轮改 |

其余全部已经有第三档（`launcher.rowControl` / `output.go:660` / `terminalui.barButton:1484` /
`terminalui.emptyLine:1003` / `shell.detach barButton` / `shell.notices` / `shell.pinned` /
`settingsui.parameditor` / `pluginsheet` / `savealert` / `uninstall` / `rows.go actionWithGlyph`）。

**`runOutputToggle` 还有一个额外发现**：它把悬停状态读进了变量
（`hovered := button.Hovered()`，`settingsui.go:7084`），而
`internal/ci/litarm_contract_test.go` 的臂级普查只认 `case X.Hovered():` / `if X.Hovered() {`
——**这个函数对普查完全隐形**（`len(arms)==0` 直接 `continue`）。所以它悬停时那块
`ControlHover` 面**没有一对线**（§2 判据 1 的 E1 缺件）也一直没被发现。这一轮把它改成
`switch` 三档 + 一份共享的一对线（普查的逃生口 ②，`savealert.go` / `power.go` 同一个写法），
它会同时进入普查视野。

## 逐条落地项

### 1. `sidebarRow`（`internal/settingsui/settingsui.go`）

- 位置：`a.sidebarRow`，`selected` 分支之后、`color` 之前。
- 现状：`if selected { RowSelected + LitFace } else if row.Hovered() { RowHover }`。
- 应改成：按下**压过**选中与悬停，取行阶梯的第三档 `tokens.RowPressed`
  （浅 `rgba(0,0,0,0.08)` / 深 `rgba(255,255,255,0.11)`，`theme.go:745/871`）。
  **不戴 `LitFace`**：启动器结果行按下的那一档也不戴（`launcher.go:2510-2514`），
  两张列表是同一句话——按下去是「沉进玻璃」，不是「站起来」。
- 墨：`color` 的判据加 `|| row.Pressed()`，与悬停同为 `t.Text`。

### 2. `runOutputToggle`（`internal/settingsui/settingsui.go` + `runOutputToggleFace`）

- 位置：`a.runOutputToggle` 的 `hovered := button.Hovered()` 一段；以及
  `runOutputToggleFace(tokens theme.Tokens, hovered bool) ui.Color`（`settingsui.go:6906`）。
- 现状：静息 `ui.Transparent` / 悬停 `ControlHover` / 按下与悬停**逐像素相同**。
- 应改成：三档 `switch`——按下 `tokens.ControlPress`（深 `0.05` / 浅 `0.032`）、
  悬停 `ControlHover`、静息透明；然后 **`if plane.A > 0 { tokens.LitFace(button) }`**
  一份面共用一对线（普查逃生口 ②）。墨同样按 `plane.A > 0` 抬到 `t.Text`。
- 几何**一个像素不动**：不加 `Scale`。参考的 `:active` 变换是给胶囊的
  （`.extensions-action-button:active { scale(0.97) }`），而这一行是**整宽的文字控制**
  （`extensions.css:1243-1256` 的 `.extension-row__output-toggle`，chevron + 一个词），
  整宽收缩会把 chevron 从行首挪进去，那是布局，不是墨。

## 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
  `sidebarRow` 的 `row.Clicked()` → `selectPage(i)` 一字不动；`runOutputToggle` 的
  `button.Clicked()` → `a.setRunOutputOpen(id, true)` 一字不动。
- 本轮只改「面」与「墨的档位」，不新增 token、不新增 i18n 文案。

## 门槛命令 + PASS 基线（本轮开工前实测）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./...                               # 净
GOOS=darwin  go vet ./...                               # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 3267（基线）
GOOS=linux|windows|darwin go build ./...                # 三个都过
```

测试函数数：**2421**（HEAD 实测，`grep -c "^func Test"`）。

## 新 pin（`internal/settingsui/pressrung2_test.go`）

三件事，深浅各一遍，全部读像素：

1. **按下 ≠ 悬停**（本轮根因）：两帧同一像素位置的通道差 ≥ 6 级。
2. **按下的面在静息与悬停之间**（第三档是回退，不是第三种颜色）：对
   `sidebarRow`，`rest < press < hover`（浅色档亮度递减方向）——用
   「press 与 hover 的差 ≥ 6 且 press 与 rest 的差 ≥ 6」表达，避免绑定方向。
3. **按下只改自己那一块**：按下帧里相邻的行 / 卡片自己的面一个字节没动。

## mutation 计划（≥2 条，每条说明哪个 pin 变红）

- **M1**：`sidebarRow` 的按下臂删掉（退回两档）→ pin ① 红（差 0）。
- **M2**：`runOutputToggle` 的按下臂改成 `ControlHover`（与悬停同面）→ pin ① 红。
- **M3**：`sidebarRow` 按下臂改成 `RowSelected`（不是第三档，是选中档）→ pin ② 红
  （按下与选中面差 0）。
