# P11-z 规格：红线只回答语义 + 终端画布不再套第二圈框（用户实测 F2 + F3）

## 意图

用户实测反馈 F2「整体红色线条太扎眼」与 F3「终端页面外圈那圈类玻璃边偏离『轻』的质感」。
两条是同一片界面质感（红描边 + 双层框），本轮一起做：**把红色收回到「错误/破坏性」与
§5 的四类状态，把终端画布的 1px 描边删掉，让面板成为唯一的框。**

为什么现在做：任务书把用户实测反馈排在第一，F1（终端异常字符）已由 P11-y 修完，
F2/F3 是它后面紧挨着的两条，且用户原话就是「先优化当前的界面、质感、交互逻辑」。

## 现状（行号为动手前实测，`git log` = `6c38930`）

| 位置 | 现状 |
|---|---|
| `internal/theme/theme.go:648` | 浅色 `strokeActive: ui.RGBA(229, 72, 77, 0.55)`（红 0.55） |
| `internal/theme/theme.go:727` | 深色 `strokeActive: ui.RGBA(245, 84, 78, 0.55)`（红 0.55） |
| `internal/theme/theme.go:446`、`:554` | `strokeActive` → `Tokens.InputStrokeActive` |
| `internal/shell/shell.go:4260-4264` | `a.cardFocused()` 为真时整张启动器卡的 1px 边换成 `InputStrokeActive` |
| `internal/shell/shell.go:4345` | `Border(1, stroke)`：那圈红就是它画的 |
| `internal/terminalui/terminalui.go:873-876` | 终端画布的边同样是「有焦点 → `InputStrokeActive`」 |
| `internal/terminalui/terminalui.go:888` | `ui.Box(...).Radius(mountCorner).Border(1, edge)`：面板(12) 里再套画布(10) 的 1px 框 → 双层框 = 用户说的「玻璃边」 |
| `internal/shell/cardedge.go:96-100` | Linux 聚焦时另加 `inset 0 0 0 1px AccentGlow`（红 0.10/0.12）：同一个「窗口有焦点」语义的第二圈红 |
| `internal/theme/theme_test.go:129-133` | 注释「The focused step takes the accent in both.」——旧基线的话 |
| `internal/shell/cardedge_test.go:68-80` | 钉住 Linux 聚焦卡的那圈 `AccentGlow` |

判据（`docs/raycast-design.md` §5）一句话：accent 只能回答「哪个被选中 / 哪个是开的」。
**「整个窗口有键盘焦点」不是这两件事里的任何一件**——启动器窗口在用时永远有焦点，
这圈红是常量装饰。§5 第 4 条把「焦点环」列进四类，本轮把它的边界写清：**控件级**的
焦点环（框架 `FocusRing`、输入框 caret）保留；**窗口/卡片级**的整圈描边不属此列，
改中性明度差表达。

## 应改成

### F2 红线

1. `strokeActive`（`InputStrokeActive`）改成**中性、比静息档亮一档的发丝线**：
   - 浅色 `ui.RGBA(0, 0, 0, 0.16)`（静息 `stroke` 是 0.09）
   - 深色 `ui.RGBA(255, 255, 255, 0.16)`（静息 0.09）
   §5 的「预算只约束色相，不约束明度」正是这条的授权：焦点靠**明度**说，不靠色相。
   静息/聚焦仍可分辨（0.09 → 0.16），`TestTheFocusFaceAndTheFocusStrokeMoveTogether`
   与 `theme_test.go:131` 的「≠」关系不变。
2. `internal/shell/cardedge.go`：Linux 聚焦卡的 `AccentGlow` inset 环**删掉**。同一个
   「窗口有焦点」语义，同样的整圈红；焦点由 `EdgeHighlightActive` 与
   `SheenAtFocus` 的亮面表达（两者都保留）。`AccentGlow` 字段按 P8-e 对
   `AccentTintQuiet` 的先例标 `Deprecated`、**不删字段**（主题值 pin 仍在）。
3. **不动**的（合法用途，逐条点名）：`AccentEdge`/`AccentEdgeStrong`（分段控件选中段
   keyline `internal/control/control.go:124`、主 CTA `internal/settingsui/rows.go:481`）、
   侧栏前导图标、开关 ON、权限 ●、`Warning` 语义色（错误提示）、框架 `FocusRing`
   （控件级）、`Selection`（文本选择）。

### F3 终端画布

4. `internal/terminalui/terminalui.go:888` 删掉 `Border(1, edge)`，连同 `:873-876` 的
   `edge` 计算：**面板（12，`panelCorner`）是唯一的框**，画布只留 `Radius(mountCorner)`
   的填充。不加高光、不加渐变、不加 rim/sheen（既有偏好：不要 floating/overlay 观感）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不动**。本轮只改
墨（两个颜色值）与面（删两条描边）。

## 门槛命令 + PASS 基线（动手前实测）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
GOOS=windows go vet ./...    # 净
GOOS=darwin  go vet ./...    # 净
go test -count=1 ./...       # 2344 PASS / 15 SKIP / 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2344
测试函数数（grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l）  # 1902
三平台 build                 # 过
```

## 台账预判

- **V**（视觉数值重定基线）：`theme.go` 两个 `strokeActive`；`cardedge_test.go:68-80`
  的 Linux 环断言（旧 `AccentGlow` → 新「无 inset 环」）；`theme_test.go:131` 的注释。
- **B**：其余全绿（`TestTheSidebarChosenGlyphKeepsTheAccent`、`TestTheChosenSegmentKeepsItsAccentKeyline`、
  `TestTheSwitchOnStateCarriesTheAccent`、`TestThePermissionDotAndTheRegisterChipKeepTheAccent`、
  `TestTheTerminalViewsPaintFromTokensNotLiterals`、`panelCorner`/`mountCorner` 两条几何 pin）。
- **S**：无（结构只少两条描边调用，没有新结构）。

## 新增 pin（正面 + 反面成对）

1. `internal/theme`：`TestTheFocusedCardEdgeIsNeutral` —— 两套外观下
   `InputStrokeActive` 三通道差 ≤ 8（中性），且明度与静息档**不同**（0.09 → 0.16）。
2. `internal/shell`：`TestTheFocusedCardDrawsNoAccentRing` —— `cardEdge(linux, focused)`
   里不再有 `Spread == 1` 的描边，且没有任何描边等于 accent 系 token。
3. `internal/terminalui`：`TestTheTerminalCanvasDrawsNoSecondFrame` —— 画布左边缘的
   像素等于画布填充（没有 1px 描边）；面板角仍是 12、画布角仍是 10（几何 pin 不动）。

## mutation 计划（每条都要看到具体哪条 pin 变红）

- **M1**：把 `strokeActive` 改回 `ui.RGBA(229, 72, 77, 0.55)` → `TestTheFocusedCardEdgeIsNeutral` 红。
- **M2**：把 `cardEdge` 的 Linux `AccentGlow` inset 环加回去 → `TestTheFocusedCardDrawsNoAccentRing` 红。
- **M3**：把画布的 `Border(1, tokens.InputStroke)` 加回去 → `TestTheTerminalCanvasDrawsNoSecondFrame` 红。

## 出图

`FLOTER_PNG=1` 跑 dump，深/浅各两张：

- `/tmp/floter-P11-z-red-<dark|light>.png` —— 启动器卡（有焦点）
- `/tmp/floter-P11-z-terminal-<dark|light>.png` —— 终端页
