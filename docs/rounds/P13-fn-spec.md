# P13-fn 规格：picker 的盒子是**地板**，不是上限

**意图**：设置页 `chooseSelect` 的 picker 盒子是**字面 220px**（`Width(220)`），而它装的东西
（内边距、字形、箭头）全部跟着界面台阶缩放。于是①同一个盒子的「空气占比」在四个台阶上从
**56.7%** 滑到 **35.9%**；②一个比盒子长的值（字体族名可以是设置文件里的任意字符串）被
`SingleLine` 用省略号切掉。这一轮把 220 从**上限**改成**地板**：出厂值永远画同一个盒子（右列
仍是一条线），更长的值借标签列的地，标签不被切。

为什么现在做：上一轮（P13-fk）把它作为「下一格（首选）」记进 §6.132 的 **132-5**，状态
`需拍板（先量）`——「改它会改非默认界面尺寸下 picker 的宽度，要先量」。这一轮就是那次量。

## 0. 先量（实测，720×900，General 页，`launcher.InputWindowWidth`）

四个界面台阶（`valuedomain.ScaleFactors`：tiny 0.8 / small 0.9 = 出厂 / default 1.0 / large 1.1）：

| 台阶 | `Space(1)` = u | picker 盒面（实测） | 盒内家具 = 11u | 盒内净宽 | 最宽出厂族名墨 | 空气占比 |
|---|---|---|---|---|---|---|
| tiny 0.8 | 3.2 | **212** | 35.2 | 184.8 | 80（DejaVu Sans Mono） | 56.7% |
| small 0.9（出厂） | 3.6 | **214** | 39.6 | 180.4 | 90 | 50.1% |
| default 1.0 | 4.0 | **212** | 44.0 | 176.0 | 100 | 43.2% |
| large 1.1 | 4.4 | **212** | 48.4 | 171.6 | 110 | 35.9% |

盒面（pill 自己的面，不含两侧 1px 发丝边与抗锯齿）在四个台阶上是 **212…214**：**盒子不缩放**，
它里面的东西缩放 37.5%（0.8→1.1）。

**病**（39 字的自定义族名 `Iosevka Term Extended Nerd Font Mono`，把同一个 picker 画在 220 与
400 两个盒里逐列对比，差异列就是被切掉的地方）：

| 台阶 | 值自然墨宽 | 与 400 盒的差异列 | 第一处差异列 |
|---|---|---|---|
| tiny 0.8 | 174 | 1（= 盒边，值装得下） | 209 |
| small 0.9 | 199 | **19** | **189** |
| default 1.0 | 221 | **21** | 187 |
| large 1.1 | 239 | **27** | 178 |

也就是说：出厂台阶上，一个长族名从第 189 列起被换成省略号——**它本来有 260u 的标签列可以借**。

## 1. 逐条落地项

| # | 位置（动手前已核） | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go:4718-4723` `chooseSelect` | `ui.Select(...).Label(label).Width(220).Radius(999)`，`cardUnit > 0` 时再叠 `MinHeight(28).Padding(4,8)` | `Radius(999)` 之后分两支：`cardUnit > 0`（扩展面板，`extensions.css` 是字面 px）保持 `Width(220)`；设置页（`cardUnit == 0`）走 `MinWidth(220)`——地板 |
| 2 | `internal/settingsui/settingsui.go:4749` `paletteRow` | 另一枚字面 `Width(220)` | **不动**：调色板名都短（内宽 171.6…184.8 都装得下），且它旁边是 `Space(37)`=148u 的预览块；那是另一个表面的一轮 |
| 3 | `internal/settingsui/pickerfloor_test.go`（新） | 无 pin | `TestALongPickerValueGrowsTheBoxInsteadOfBeingCut`：A 地板（出厂 8 个族名画同一个盒子，216…222）；B 成长（长值的盒增长 = 值的溢出，±6）；C 标签不被切（标签墨列数长值/短值逐列相同，且标签列 ≥1.5× 标签墨） |
| 4 | 同上 | — | opt-in dump `TestDumpP13FNPickerFloor`（`FLOTER_PNG=1`）：small/large × 深浅四张 |
| 5 | `docs/liquid-glass.md` | §6.132 的 132-5 = `需拍板（先量）` | 新增 §6.133 结账 132-5，并把 132-5 标成 `已落地（P13-fn）` |

## 2. 行为契约零改动红线

只改**一块面的宽度来源**（固定 → 地板）。高度链、键盘模型、会话管理、设置写回、i18n 文案、
命令接线**零改动**：`ui.Select` 的交互、`fieldFace` 的材质、`cardRow` 的槽与标签列分工都不动。

## 3. 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                  # 空
go vet ./...                           # 净
GOOS=windows go vet ./...              # 净
GOOS=darwin  go vet ./...              # 净
go test -count=1 ./...                 # 全绿
--- PASS                               # 3412（基线 3411）
--- SKIP                               # 193（基线 192，+1 = 本轮的 opt-in dump）
测试函数                                # 2583（基线 2581，+2）
GOOS=linux/windows/darwin go build ./... # 全过
```

## 4. mutation 计划（每条说明会让哪个 pin 变红）

- **M1**：`settingsui.go` 的 `e.MinWidth(220)` → `e.Width(220)`（回到本轮之前）。预期
  `TestALongPickerValueGrowsTheBoxInsteadOfBeingCut` 的 B 段在 small / default / large × 深浅
  六处变红（「the box grew 0px for a value Npx longer than its room」）。
- **M2**：`e.MinWidth(220)` → `e.MinWidth(120)`（地板调低）。预期 A 段变红：出厂族名各画各的
  盒子（118 / 119 / 132 / 124），右列从一条线变成一条锯齿。
