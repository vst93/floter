# P13-bv · 规格：保存失败横幅的 ✕ —— 这一族最后一枚「按下去纹丝不动」的键

> 总目标（2026-10-30）：细节打磨 + 轻便液态玻璃 + 空间感。选题来自通用队列「状态反馈」，
> 是 P13-al / P13-am（shell 四个键、终端浮条五个键）那一条线剩下的最后一处。
> 合约 `docs/liquid-glass.md` §5「按下」+ §6.40（本轮新增）。报告 `docs/rounds/P13-bv-report.md`。

## 一、意图一句话

**「按下」是这一支唯一被允许的几何变化（§5 `Scale(0.986)`，小格族 `Scale(0.92)`）；
设置页保存失败横幅的那枚 ✕ 是这一族里最后一枚按下去与悬停逐像素相同的键——把第三档补上。**

为什么现在做：全仓 `Hovered()` 计数对 `Pressed()` 计数的普查（`internal/launcher`、
`internal/settingsui`、`internal/shell`、`internal/terminalui`，排除 `_test.go`）里，
`savealert.go` 是 `hover=1 / press=0` 的一处；P13-q 已经把设置页同族的 `headerButton` /
`paramIconButton` / `sessionIconButton` / `barButton` 的第三档补齐，P13-al / P13-am 收掉了
shell 与终端浮条，**剩下这一枚没有第三档**。

## 二、现状（动手前逐行核过，行号以本轮 HEAD 为准）

| 位置 | 现状 |
|---|---|
| `internal/settingsui/savealert.go:141-155` | ✕ 是 `Size(t.Space(6))`（24u）、静息透明；唯一的指针臂是 `if button.Hovered() { Background(ControlHover); LitFace }` |
| 同上（按下） | **没有 `Pressed()` 臂**：按住不放的那一帧与悬停帧逐像素相同 |
| `internal/ci/litarm_contract_test.go:60-79` | 指针臂的面必须戴一对线；逃生口 ② = 「同一个函数在臂之外画过一次那一对线」（`power.go` 的 `if plane.A > 0 { LitFace }` 就是这个形状） |

## 三、落地项（1 处产品改动 + 1 条 pin）

| # | 位置 | 应改成 |
|---|---|---|
| 1 | `savealert.go` 的 ✕ | 面写成变量 `plane`，`switch { case Pressed: ControlPress + Scale(0.92); case Hovered: ControlHover }`，`button.Background(plane)` 之后 `if plane.A > 0 { LitFace(button) }`——一份面、所有状态共用一对线（逃生口 ②）。一对线在按下档**保留**：这一族的面是「一块能被碰的板」，压下去仍在地面之上（同 P13-q 在设置页的取舍）。 |
| 2 | `internal/settingsui/savealertpress_test.go`（新） | 4 条判据（悬停画了面 / 按下画了另一个面 / 按下面在横幅面与悬停面之间 / 按下没重排横幅、没搬盒子），深/浅各一遍；1 条 opt-in dump。 |

## 四、行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**全部零改动**：本轮只动
`Background` / `Scale` / `InsetShadow` 三种绘制属性，`Label(copy.Close)`、`Clicked()` →
`dismissSaveFailed()`、24u 的 `Size`、横幅的 `Float` 面与 `EdgeHighlight` 冠线一个字节没动
（pin ④ 断言按下帧里横幅自己的面逐字节不变）。

## 五、门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿
--- PASS                                  → 基线 3215 → 3216
--- SKIP                                  → 基线 119 → 120（+1 = opt-in dump）
--- FAIL                                  → 0
GOOS=linux|windows|darwin go build ./...  → 三平台过
```

## 六、mutation 计划（≥2 条，写明会让哪个 pin 变红）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | 按下档画成悬停面（`plane = tokens.ControlHover`）——即本轮之前的实际行为 | `savealertpress_test.go` 判据 ②（「whether or not the button is down」） |
| M2 | 删掉臂外的共享一对线（`if plane.A > 0 { LitFace }`） | `internal/ci` 的 `TestEveryPointerStateFaceNamesItsPair`（`savealert.go` 两个臂各报一条） |
