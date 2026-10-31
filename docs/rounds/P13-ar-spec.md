# P13-ar 规格：卸载对话框里那张整卡不可用的卡片

## 意图

**「不可用」在同一张卡上被写了两遍（还有一条灯忘了熄）：把 `part.Fixed` 那一行收成
「一道淡 + 灯熄了」。** 为什么现在做：P13-aq 把「禁用面只淡一道」立成了契约
（`docs/liquid-glass.md` §6.2b-6 / §6.14），并在「仍待 1」里点名了这一处尾巴——
它是全仓最后一处「产品自己发明的淡」（0.6，不在框架的 0.5 那档上），而且里面还套着
一个自带 `Disabled` 的勾选框，是这一族里唯一会被淡两遍的元件。

## 证据（改动前实测，同一把尺：`dimProbe` 的读数法，见 `dimface_test.go`）

| 位置 | 量 | 值（浅 / 深） |
|---|---|---|
| `uninstall.go:440` `card.Opacity(0.6)` | 整棵子树一道 0.6 的淡（不是框架档） | 面离地面 11 / 17（可用态 17 / 24） |
| 同臂 `tokens.LitFace(card)` | 卡戴着冠线 = 「这块面可以被碰」 | 冠线行 − 面 **+3 / +12** |
| `uninstall.go:487` `box.Disabled(true)` | 勾选框自带 `Disabled`，叠在卡的 0.6 之下 | 墨/面 **46 / 49**（可用 148 / 154 = **0.32**） |

框架侧的事实：`mygo@v0.7.3/ui/paint.go:257-259` 对**自带** `flagDisabled` 的元件乘 0.5；
`mygo@v0.7.3/ui/element.go:1348-1355` 的 `IsDisabled()` 沿父链上溯。两者合起来意味着：
把 flag 挪到**卡的根**上，子件仍然不可用（游标、命中、语义都在），但画笔只乘一次。

## 逐条落地项

1. `internal/settingsui/uninstall.go:440`（`case part.Fixed`）
   - 现状：`card.Opacity(0.6)` + `card.Cursor(NotAllowed)` + `tokens.LitFace(card)`
   - 改为：`card.Disabled(true).Cursor(ui.CursorNotAllowed)` + `tokens.DimFace(card)`
2. `internal/settingsui/uninstall.go:487`（固定行里的勾选框）
   - 现状：`box.Disabled(true)` + `box.Cursor(NotAllowed)`
   - 改为：只留 `box.Cursor(ui.CursorNotAllowed)`（`Disabled` 从卡根继承）
3. `internal/settingsui/fixedpartdim_test.go`（新）
   - ① 冠线行不许亮于面；② 面离地面的台阶 ≥6 级（复用 `assertDisabledFace`）；
     ③ 字/面对比 ≥ 启用态的 0.40（`oneFadeRatio`）；④ 卡里的勾选框也只许淡一道
     （禁止把 `Disabled` 再挂回子件）× 深浅两套。
4. `internal/settingsui/fixedpartdim_dump_test.go`（新，opt-in）
   - 四行组件卡 × 深浅两套出图，`FLOTER_P13AR_TAG=before` 拍改前那一帧。
5. `docs/liquid-glass.md` §6.15（新）
6. `.pending-bugs.md` 顶部本轮段 + 「仍待」

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一律不动**。特别地：

- 卡与框的游标仍是 `CursorNotAllowed`（`TestComponentizedUninstallCardAnswersThePointer`
  是 B 类，零改动必须绿）；
- 几何（`Padding(3u)` / `Gap(3u)` / `Radius(2u)` / `Border(1, ControlEdge)`）逐字段照抄；
- 只有 `Value != nil` 的行才写回草稿——固定行仍不写回。

## 门槛命令 + PASS 基线（实测，本轮开工时）

```
gofmt -l cmd internal                                # 空
go vet ./... / GOOS=windows / GOOS=darwin            # 净
go test -count=1 ./...                               # 全绿
go test -count=1 -v ./... | grep -c "--- PASS"       # 3060（基线）
go test -count=1 -v ./... | grep -c "--- SKIP"       # 98
测试函数（只数 cmd/ + internal/）                    # 2268
三平台 go build ./...                                # 过
```

## mutation 计划（≥2 条）

| # | 注入 | 应当变红的 pin |
|---|---|---|
| M1 | 整条旧写法回灌：`Opacity(0.6)` + `LitFace` + `box.Disabled(true)` | 两条 pin 全红（冠线 +3/+12；勾选框 0.32） |
| M2 | 只把 `box.Disabled(true)` 挂回子件（其余保持新写法） | 勾选框那条红（0.26/0.27），冠线那条**应当仍绿** |
| M3 | 只把 `DimFace` 换回 `LitFace`（其余保持新写法） | 冠线那条红，勾选框那条**应当仍绿** |
