# P13-i 规格 · 设置页的十处控件面：`Lens` 的受光边一次都没亮过

> 方向：**细节打磨（`settingsui` 表面）**，`docs/liquid-glass.md` §2 的 E1 判据落到
> 这一张表面剩下的十个控件上。不新开表面、不动度量、不动行为。

## 1. 意图一句话

设置页里还剩十处 `theme.Tokens.Lens`（四层控件透镜），它们的第四层——四边
`ControlEdge` 的环——**画在第一条 `LensRim` 之后**，于是那条被 P12-a 抬到 0.09 的
受光边在每一处都被压在下面：这一页的控件至今**一次都没亮过**。

## 2. 为什么现在做（证据）

- `internal/theme/theme.go:1056-1061`：`Lens` 的四层按「rim → sheen → edge → ring」
  追加；`InsetShadow` 后加的画在上面（`LensInset` 的注释已写明这条顺序语义）。
- mygo `ui/paint.go:1241-1262`（`insetShadow`）：内阴影的 hole 从 **padding edge**
  （border 内侧）算起，`spread=1` 的环正好占 padding 第一行；`LensRim` 的 `y=1,
  spread=0` 的 hole 也是 padding 第一行 —— **两条线落在同一个像素行，环后画，rim 输**。
- P13-h 在启动器的 28u 图标板上实测到同一个根因（浅色档顶边 212 < 面 221，是一条
  **暗**线；深色档 97）。那一轮只修了启动器的三个 draw point；这一轮是同一族在
  设置页上的十处。
- 这十处控件都各自带着 `Border(1, ControlEdge)`（两处悬停小格是 hover 时才加），
  所以环与边框是**同一条边的第二次上色**：退掉环不改几何（外框还在，边缘仍是
  2px 的「边框 + 内线」），只改内线那一像素的墨。

## 3. 逐条落地项（行号本轮实测）

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 9-1 | `settingsui/rows.go:486` `extensionActionFace`（`.extensions-action-button` 胶囊，动作/保存/丢弃共用） | `Border(1, edge)` + `tokens.Lens(button)`：内圈 = 第二条 `ControlEdge`，顶边是暗线 | `tokens.LitFace(button)`：顶 `LensRim` + 底 `LensEdge`，**无环** |
| 9-2 | `settingsui/rows.go:542` `extensionIconButton` 的 `Hovered()` 臂（配置工具栏 30u 图标钮） | hover：`Border(1, ControlEdge)` + `ControlHover` + `Lens` | `LitFace` |
| 9-3 | `settingsui/parameditor.go:272` `paramIconButton` 的 `Hovered()` 臂（参数编辑器 28u 图标钮） | 同上 | `LitFace` |
| 9-4 | `settingsui/settingsui.go:1197` `headerButton` 的 `Hovered()` 臂（设置页页头两个 28u 图标钮） | hover：`ControlHover` + `Lens`（静息透明） | `LitFace` |
| 9-5 | `settingsui/settingsui.go:5832` `barButton`（丢弃条的动作胶囊） | `Border(1, edge)` + `Lens` | `LitFace` |
| 9-6 | `settingsui/uninstall.go:365` 卸载对话框的动作胶囊 | `Border(1, edge)` + `Lens` | `LitFace` |
| 9-7 | `settingsui/uninstall.go:427` `uninstallPartRow` 的 `part.Fixed` 臂（不可改的组件行） | `Border(1, ControlEdge)` + `Lens` | `LitFace` |
| 9-8 | `settingsui/uninstall.go:439` 同函数的 `Hovered()` 臂 | 同上 + `ControlHover` | `LitFace` |
| 9-9 | `settingsui/uninstall.go:441` 同函数的 `default` 臂 | 同上 | `LitFace` |
| 9-10 | `settingsui/savealert.go:149` 保存告警的 ✕（24u，静息透明） | hover：`ControlHover` + `Lens` | `LitFace` |

**判为「故意不做」的两处**（本轮只写结论，不改代码）：

- `control/control.go:123` `SegmentedIndex` 的选中段（`.settings-option--active`）：
  它自己的身份线是**紧跟其后**追加的 `InsetShadow(0,0,0,1,AccentEdge)`，四边都在，
  把 rim 与 edge 全盖住。换成 `LitFace` 会画出两条立刻被 accent 环遮住的线——像素上
  不可见，§2「一个面只说一句话」也不允许再叠一条白冠线。它是设置页与终端抽屉共用的
  控件（跨表面），留原样。
- `shell/notices.go:440` toast 的动作 chip：它是 **toast 那一张表面**（P13-g）里的 E1
  控件，且**没有**自己的边框——环就是它唯一的边界。退成 `LitFace` 会同时改边界与
  墨，属于下一张表面的活。记进 §6.6 的 `未开始` 行。

## 4. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个都不动**：
本轮只替换十处 `tokens.Lens(x)` 为 `tokens.LitFace(x)`，不增删 `Border`、`Background`、
`Padding`、`Size`、`Transition`、`Scale`、`Opacity`，不动任何 `Clicked`/`Changed` 分支。

## 5. 门槛命令 + PASS 基线（本轮实测起点）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... && GOOS=darwin go vet ./...  # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2907
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## 6. mutation 计划（每条说明会让哪个 pin 变红）

1. **把 `rows.go:486` 的 `LitFace` 换回 `Lens`** → `TestTheActionPillIsAFaceNotARing`
   的「左右两侧列 = 面」断言红（环回到侧列）。
2. **只画顶边、丢掉 `LitFace` 的底对侧**（手写 `InsetShadow(0,1,0,0,LensRim)`）→
   同一个 pin 的底边断言红（§3「顶亮一分、底暗一分」缺一半）。
3. **把 `LitFace` 换成 `EdgeHighlight` 的冠线**（浮层那条）→ `liteNearest` 的
   「离 E1 的线更近」断言红（E1/E2 两种边各管一档）。
4. **给胶囊补一条外投影** → pin 的「块外两像素仍是地」断言红（§2 判据 2）。
5. **把某处的 `Border(1, ControlEdge)` 顺手删掉** → pin 的「最外一行仍是 `ControlEdge`」
   断言红（几何不该变，这一轮只改墨）。
