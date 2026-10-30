# P13-u 规格 · 设置页剩下的四处两道淡：一道淡收尾（不含 accent / 警告环）

## 意图（一句话）

`docs/liquid-glass.md` §6.2b-6/-6b 已经定过规矩——**禁用只有一个深度，档位属于控件根**；
这一轮把设置页里**还留着产品自己那道 `Opacity`** 的四块控件收进同一条算术：开关、会话小格、
卸载胶囊（在飞）、确认带上的胶囊。**accent 主按钮与警告色胶囊不做**（见「不做」一节）。

为什么现在做：P13-t「仍待」第 2 条点名了这四处（`uninstall.go:361`、
`settingsui.go:5871`、`rows.go:829`、`rows.go:668`），并写明「前四处与 `lockedControl`
同形，可一轮收掉」。它们是 P13-s / P13-t 那一族在同一页上的尾巴：同一张设置页上，
禁用胶囊已经是 −9 级（浅色），禁用开关却还是 −4 级（0.3 有效），禁用会话小格是 0.2
——用户读到的是「有的控件不可用，有的控件快没了」。

## 证据（改前实测，见报告「实测」表）

四处都是 `Disabled`（框架 0.5）**再乘**产品自己的 `Opacity`：

| # | 位置 | 有效淡度 | 算式 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go:668` `disabledSwitch` | **0.30** | `.Opacity(0.6)` × 框架 0.5 |
| 2 | `internal/settingsui/rows.go:829` `sessionIconButton` 禁用臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 |
| 3 | `internal/settingsui/uninstall.go:361` `uninstallActionButton` 在飞臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 |
| 4 | `internal/settingsui/settingsui.go:5871` `barButton` 禁用臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 |

## 落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `rows.go:667` `disabledSwitch` | `sw.Disabled(true).Opacity(0.6).Cursor(default)` | 只留 `Disabled(true)` + 默认光标（**槽没有受光边可熄**，§6.2b-6b 的规矩：整块退一半、内线各留一半）。`busySwitchControl` 走同一个 helper，自动跟随 |
| 2 | `rows.go:829` `sessionIconButton` 的 `disabled` 臂 | `button.Opacity(0.4).Cursor(default).Disabled(true)` | `button = lockedControl(button)`——**透明面**：没有面也没有边，只淡一道（P13-p 的反向 pin 守着「透明面上不许有受光边」，本轮不碰材质） |
| 3 | `uninstall.go:361` `uninstallActionButton` 的 `busy` 臂 | `button.Disabled(true).Opacity(0.4).Cursor(default)`，材质恒 `LitFace` | `Disabled(true)` + 默认光标；材质按 `busy` 分支：`DimFace`（底对侧还在、受光边熄了）/ `LitFace` |
| 4 | `settingsui.go:5871` `barButton` 的 `disabled` 臂 | `button.Disabled(true).Opacity(0.4).Cursor(default)`，材质恒 `LitFace` | 同上：一道淡 + 材质分支 |

三处（2/3/4）复用 `lockedControl`（P13-t 的 helper）与 `theme.Tokens.DimFace`（P13-s 的材质），
**不新增 token、不新增 helper**。`lockedControl` 的 doc 补一句「非表单控件也用这一条」
（它现在不只服务配置表单）。

## 不做（写进「需用户拍板」，本轮不实现）

- `config.go:341`（配置表单的 accent Save）与 `settingsui.go:3835`（About 页 `updateButton`）
  的禁用臂仍是 `.Opacity(0.4)` / `.Opacity(0.6)`：accent 要淡的是**色相**（fill 本身），
  不是「面还在」——先定「禁用 accent = accent 与卡面按什么比例混」，另起一轮（P13-s 9-25）。
- `rows.go:895` `sessionKillConfirm`：`RaisedWarm` + `ShadowContact` + `LensRim` 的暖色抬升面，
  `DimFace` 会把警告环洗淡（§6.2b 9-26 已判 `故意不做`）。
- `uninstall.go:427` `card.Opacity(0.6)`（`part.Fixed` 的不可勾选卡片）：**它不是两道淡**——
  那一臂没有挂 `Disabled`，0.6 是唯一的一道；且它戴着 `LitFace` 的完整一对线。判 `故意不做`。

## 行为契约零改动红线

- 四处只改**墨与面**：`Disabled(true)` 的去留、点击守卫（`Clicked() && !disabled`）、
  光标、`busy` / `disabled` 参数的判定链一个字节不动。
- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动。
- 滚动不受影响（`ui/input.go:515-536`）。

## 门槛命令 + PASS 基线（实测于开工时）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...    → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2977（基线）
                            ... | grep -c -- "--- FAIL" → 0
                            ... | grep -c -- "--- SKIP" → 77
测试函数数（`git grep -h "^func Test" HEAD -- '*_test.go' | wc -l`） → 2195
GOOS=linux/windows/darwin go build ./...               → 过
```

## mutation 计划（每条都要实跑变红）

- **M1** `disabledSwitch` 加回 `.Opacity(0.6)` → 开关的「一道淡」pin 变红（读数偏深）。
- **M2** `sessionIconButton` 的禁用臂加回 `.Opacity(0.4)` → 会话小格的墨对比 pin 变红。
- **M3** `uninstallActionButton` 的材质分支退回恒 `LitFace` → 在飞胶囊的「顶内线不亮」pin 变红。
- **M4** `barButton` 的 `DimFace` 换成 `LitFace` → 确认带胶囊的「顶内线不亮」pin 变红。
- **M5**（反向）给 `sessionIconButton` 的透明面补一条 `LitFace` → P13-p 的反向 pin（透明面上不许有
  受光边）必须变红；若不变红，说明那条 pin 的覆盖面不够，写进报告。

## 判据

1. 开关：静息 / 禁用两帧的四个读数，禁用帧必须**正好**是「静息与地面各一半」的混合
   （复用 `assertOneFade`）。
2. 三块面（在飞胶囊 / 确认带胶囊 / 会话小格）：面离地面 ≥6 级且 ≤15 级（是禁用面不是静息面），
   顶内线**不亮**，底内线仍是对侧（浅 ≥5 / 深 ≥2 级）；墨对比 ≥60 级。
3. 会话小格是**透明面**：面 = 地面（不做面台阶断言），只钉墨与「没有受光边」。
