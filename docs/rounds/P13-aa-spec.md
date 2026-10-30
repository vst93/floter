# P13-aa 规格 · 启动器两条「带」上的四个小面：各缺半套 E1

**意图一句话**：启动器里最后四块**能被指针碰**的小面，把「面」和「一对线」补齐——
一条带上的两块面读同一种材质，用户不必去猜哪块「能按」。

**为什么现在做（证据，行号本轮实测）**：

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律走通用队列的「状态反馈」。P13-h / P13-i /
P13-j / P13-l / P13-m / P13-p 把「悬停小格 / 控件面」收口成 E1（面 + 顶受光边 + 底对侧），
但那一族的账本只扫了**有边框**的面。下面四处**没有边框**（或边框是变量），所以三种 census
都照不到——P13-p 换成的材质 census 判据是「面 + 边 ⇒ 必须有一对线」，**没有边**的面不进视野。

| 位置 | 现状 | 缺什么 |
|---|---|---|
| `internal/launcher/output.go:517-520` | 捕获输出页页头两个 28u 方键（`↗` 钉住 / `⧉` 独立窗口）：`button.Hovered()` → `Background(tokens.ControlHover)` | 只有面，**一条线都没有**；同一个启动器里 `rowControl`（20u）/`fieldSettingsButton`（28u）自 P13-h 起就是 `LitFace` |
| `internal/launcher/power.go:191-192` | 电源确认条的 **cancel** 键悬停臂：`plane, edge = ControlHover, ControlEdge` + `Border(1, edge)` | 「一块被画框圈住的平涂」——正是 §6.2b-2 / §6.2b-3 判过的那个状态，只是这里框是变量，单行 grep 照不到 |
| `internal/launcher/power.go:200-202` | 电源确认条的 **execute** 键：`RaisedWarm` + `Shadow(ShadowContact)` + `InsetShadow(0,1,0,0, LensRim)` | 只有**顶**受光边，没有底对侧（§3 的「顶亮一分、底暗一分」只做了一半）；而它注释里写着「exactly the action bar's own raised pane」，那一条（`launcher.go:2444`）自 P12-b 起就是 `tokens.LitFace` |
| 同处（对照） | `launcher.go:3204` 的反馈带 / toast：`ElevContact` + `tokens.LitFace` | 参考实现：**接触影与一对线可以同时在场**（§6.1 第 1-7 行），所以 execute 补对侧不是「多画一条线」 |

**这轮为什么让它更有空间感**：一块面「能被碰」这件事现在靠两种语言说——有边框的那几块
顶亮一分、底暗一分；这四块要么什么都没有，要么只亮半边。补齐之后，同一个面板里
**所有可碰的小面都说同一句话**，而不可碰的（透明面 / 图标墨）一条线都不戴。

## 逐条落地项

1. `output.go:517-520`（`control` 闭包）
   - 现状：`if button.Hovered() { button.Background(tokens.ControlHover) }`
   - 改成：`if button.Hovered() { button = tokens.LitFace(button.Background(tokens.ControlHover)) }`
   - **不补 `Border`**：面差就是它的边界（P13-j 的 toast chip 同一条判据）。
2. `power.go:186-199`（`confirmPowerButton` 的材质选择）
   - 现状：`plane`/`edge` 二选一后 `button.Border(1, edge).Background(plane)`
   - 改成：在 `Background` 之后按「有没有面」分支——`plane.A > 0` 时 `tokens.LitFace(button)`。
     cancel 的静息面是 `ui.Transparent`（A=0），**不戴线**（§1 禁令 3：透明面上一条白线
     就是「类玻璃边」；P13-p 9-20b 已判过同一条）。
3. `power.go:200-202`（execute 的抬升面）
   - 现状：`Shadow(...)` + `InsetShadow(0, 1, 0, 0, tokens.LensRim)`（半对）
   - 改成：`Shadow(...)` 保留，`InsetShadow(0,1,…)` 换成 `tokens.LitFace(button)`（补底对侧）。

## 行为契约零改动红线

只改墨与面。**不动**：高度链（`power.go` 的 30u 带、`output.go` 的 strip 高度）、键盘模型
（Enter / Escape 的确认与取消）、会话管理、设置写回、i18n 文案（四个标签一个字不改）、
命令接线（`PinText` / `DetachOutput` / `Power` 的回调签名与调用时机）。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 必须净
GOOS=windows go vet ./...                 # 必须净
GOOS=darwin  go vet ./...                 # 必须净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P13-z 实测，本轮开工前复跑一次核对）：PASS **3004** / FAIL 0 / SKIP **83** /
测试函数 **2218**。

## mutation 计划（≥2，每条写明会让哪个 pin 变红）

- **M1** 把 `output.go` 的悬停臂改回 `Background(tokens.ControlHover)`（撤 `LitFace`）
  → `TestTheOutputStripControlsAreE1Faces` 的「顶内线亮于面」变红。
- **M2** 把 `power.go` 的 cancel 悬停臂的 `LitFace` 撤掉（留面 + 框）
  → `TestThePowerChipsAreE1Faces` 的 cancel 那条「顶内线亮于面」变红。
- **M3** 把 execute 的 `LitFace` 换回 `InsetShadow(0,1,0,0, LensRim)`
  → 同 pin 的 execute 那条「底内线暗于面」变红。
- **M4** 给 cancel 的**静息**（透明）臂也戴上 `LitFace`
  → 反面 pin（透明面不许有受光边）变红。

## 出图

`/tmp/floter-P13-aa-strip-{light,dark}-{before,after}.png`：捕获输出页页头（指针停在钉住键上）
与电源确认条（指针停在 cancel 上）各一帧。opt-in dump，`FLOTER_PNG=1`。
