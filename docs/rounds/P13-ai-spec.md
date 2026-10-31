# P13-ai 规格 · 框架的灰盘按钮：全仓最后四枚没有「光」的控件

**意图一句话**：`internal/` 与 `cmd/` 里只剩**四枚**控件由框架的默认按钮面画出来——它有面、有边，
但四条边**同值**（画框），没有一条线说它比地面高。这一轮把这四枚收成 E1，顺手把「不许再用框架
默认面」写成一条 CI 账。

**为什么现在做**：`docs/liquid-glass.md` §6 已无 `未开始` 行，走通用队列的「细节打磨 / 空间感」。
`grep -n "ui\.Button(\|ui\.PrimaryButton(" internal/ cmd/`（排除 `_test.go`）恰好吃到**四枚**，
全部落在设置页这一个表面上——一轮一个表面，不跨表面。

---

## §1 证据（先量）

**框架给的是什么**（`mygo@v0.7.3/ui/widgets.go:152-186`，本仓 `go.mod` 钉 `v0.7.3`）：

```go
b.Padding(t.Space(1.5), t.Space(3.5)).Gap(t.Space(1.5)).Radius(t.Radius)
base, hover, pressed, fg, border := t.Surface, t.SurfaceHover, t.SurfacePressed, t.Text, t.Border
b.Background(base).TextColor(fg)
if border.A > 0 { b.Border(1, border) }
b.styleFn = ... // Pressed → pressed, Hovered → hover
```

即：`Surface`（= `tokens.Control`）+ **一圈四边同值的 `Border`**（= `tokens.InputStroke`），
**没有受光边、没有底对侧、没有字重**。`internal/theme/theme.go:486-490` 把 `ui.Theme` 的
`Surface` / `Border` / `SurfaceHover` / `SurfacePress` 依次接到 `control` / `stroke` /
`controlHover` / `controlPress`，所以框架按钮的**面**是对的，缺的是 §2 的「面要有一条受光边」。

**实测**（`lensProbe`，中灰 128 地面，`ui.Button(c, "New session")` 在 (20,20) 起画，行剖面取自
文字中线，列剖面取自盒子中线）：

| 外观 | 面（盒内中线行） | 顶边行 | 底边行 | 左边列 | 右边列 |
|---|---|---|---|---|---|
| 浅色 | **119** | 108 | 108 | 108 | 108 |
| 深色 | **141** | 151 | 151 | 151 | 151 |

**四条边一个值**——这正是 §2 判据要退掉的「靠画框分层」：盒子有边界，但边界不指向任何方向，
读不出「它在 z 上」。同一页上其它每一个动作胶囊都戴 E1 的一对面（顶 `LensRim`、底 `LensEdge`），
只有这四枚没有。

盒子几何：`y=20..49` = **30u 高**，`Radius = t.Radius`（`--radius-sm`，9u），padding 6u/14u。

## §2 四处调用点（行号本轮核对）

| # | 位置 | 哪一枚 | 什么时候在屏上 |
|---|---|---|---|
| 11-2a | `internal/settingsui/settingsui.go:1529` | 会话空态的「新会话」 | 会话页无会话时 |
| 11-2b | `internal/settingsui/settingsui.go:1631` | 会话页页头的「新会话」 | 会话页有会话时 |
| 11-2c | `internal/settingsui/custom.go:269` | 自定义快捷键草稿行的「绑定」 | 草稿行打开时 |
| 11-2d | `internal/settingsui/settingsui.go:4745` | 自绘驻留的「应用」 | 选到「自定义」时 |

## §3 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 11-1 | `internal/settingsui/rows.go`（`actionButton` 旁） | 无此函数；四枚各写一句 `ui.Button` | 新增 `a.pageButton(c, label)`：`ui.ButtonBase` + **保持** `MinHeight(Space(7.5))`/`Padding(Space(1.5), Space(3.5))`/`Radius(t.Radius)`（盒子 30u、9u 圆角**逐像素不动**）+ 面 `Control`/`ControlHover`/`ControlPress` + `Border(1, ControlEdge)` + `tokens.LitFace` + 墨 `TextStrong`/`Text` 600 + `theme.ColorStep` 颜色过渡 + 按下 `Scale(0.97)` |
| 11-2 | 上表四处 | `ui.Button(c, …).Clicked()` | `a.pageButton(c, …).Clicked()`（**点击接线、i18n 文案、键盘路径一字不动**） |
| 11-3 | `internal/ci/stockbutton_contract_test.go`（新文件） | 没有这条账；四枚是「人肉 grep 才发现」 | 反向 census：`internal/` 与 `cmd/` 的非测试源码里任何 `ui.Button(` / `ui.PrimaryButton(` 都红；唯一豁免是行尾写 `// stock-button: <理由>`（当前**零**豁免） |
| 11-4 | `internal/settingsui/pagebutton_test.go`（新文件） | 四枚从无像素 pin | 两条账（深浅各跑）：①顶行亮于面 ≥6 级、底行暗于面 ≥3 级、左右列 = 面（不是画框）；②三档有序：`ControlPress` 面 ≠ 悬停面 ≠ 静息面，且悬停时受光边仍在 |
| 11-5 | `docs/liquid-glass.md` §6.8（新增小节） | 表里没有这一族 | 逐行记 11-1…11-4 的落成值与实测数 |

## §4 行为契约零改动红线

- **点击**：四处仍然是「`.Clicked()` → 同一个回调」，回调体不改。
- **文案 / i18n**：四处的 label 仍来自 `i18n.Settings`，不新增字串。
- **键盘**：`ui.ButtonBase` 与 `ui.Button` 同一个基类（`coreButtonBase`：`Row().Center().Focusable().Shrink(0)`），
  Tab/Enter/Space 路径不变。
- **布局**：`MinHeight` / `Padding` / `Radius` 与框架逐字段相同，盒子 30u × 原位不动。
- **只改墨与面**：面档、两条内线、字重、按下收 0.97（§5 的第三档，P13-q 同一句）。

## §5 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（**改前实测**，`/tmp/p13ai-baseline.log`）：PASS **3036** / FAIL **0** / SKIP **89** /
测试函数 **2241**。改后：PASS **3043** / FAIL 0 / SKIP **90**（+1 opt-in 出图）/ 测试函数 **2245**。

## §6 mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

| # | 注入 | 应该红在 |
|---|---|---|
| M1 | `pageButton` 里删掉 `tokens.LitFace(button)` | `TestThePageActionWearsOneLitFace`（顶行不再亮于面） |
| M2 | `pageButton` 的 `Border(1, tokens.ControlEdge)` 换回框架的 `t.Border`，并把 `LitFace` 换成四边 `InsetShadow(…, ControlEdge)`（恢复「画框」） | 同一条 pin 的「左右列 = 面」与「顶亮底暗」两半 |
| M3 | `pageButton` 的字重 600 → 删掉（退回框架的默认字重） | 字重没有像素 pin，预期**不红**——如实记录，不硬凑 |
| M4 | 在 `custom.go:269` 退回 `ui.Button(c, copy.ShortcutsBind)` | `TestNoControlWearsTheFrameworksStockButton`（census 点名 `custom.go`） |
