# P13-al 规格：指针的第三档落在 shell 的四个键上

## 意图

`docs/liquid-glass.md` §5 把「按下」写死为一档（面沉到 `--glass-control-press` + 一次几何收档）。
设置页在 P13-q 补上了它，而 **shell 的四个键（插件窗口 bar 的重跑键 / 关闭键、便条窗口的 ✕、
toast 的 ✕ 与动作胶囊）一直只有静息与悬停两档**：指针按下去的那一帧与悬停帧**逐像素相同**。
这一轮把同一档补到这一族，让「按下去有回应」在全仓只剩一种读法。

## 为什么现在做（证据）

- `docs/liquid-glass.md` §6 无 `未开始` 行 → 走通用队列的「状态反馈」；P13-q 的账本
  （`docs/rounds/P13-q-report.md` §2）明确把范围限定在 `internal/settingsui`，
  shell 与 terminalui 从没进过那份账。
- 实跑确认（本轮动手前的像素，`internal/shell/pressrung_test.go` 的第一版读数）：
  `grep -n "Pressed()" internal/shell/{detach,pinned,notices}.go` → **0 命中**，
  而同一批文件里 `Hovered()` 有 4 处（`detach.go:261`、`pinned.go:99`、
  `notices.go:453`、`notices.go:475`）。
- 悬停帧与按下帧逐像素相同：M1 变异（把按下臂的面换回悬停面）实测
  light `220/220/221` ↔ `220/220/221`、dark `53/53/55` ↔ `53/53/55`，两帧无差。

## 逐条落地项

| # | 位置（动手前核过） | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/shell/detach.go:261` `barButton` 闭包（重跑键 + 关闭键共用） | `if !enabled {…} else if button.Hovered() { LitFace(ControlHover) }` | 在 `!enabled` 与 `Hovered` 之间插 `else if button.Pressed()`：`LitFace(Background(ControlPress))` + `Scale(0.92)` |
| 2 | `internal/shell/pinned.go:99` 便条 ✕ | `if close.Hovered() { LitFace(ControlHover) }` | `if close.Pressed() { LitFace(ControlPress) + Scale(0.92) } else if close.Hovered() { … }` |
| 3 | `internal/shell/notices.go:475` toast ✕ | `dismissInk := TextTertiary; if button.Hovered() { LitFace(ControlHover); dismissInk = TextStrong }` | 按下臂在前：`LitFace(Background(ControlPress).Scale(0.92))`，`dismissInk` 保持 `TextStrong`（参考的 `:active` 继承 `:hover` 的墨） |
| 4 | `internal/shell/notices.go:453` toast 动作胶囊 | `LitFace(action); if action.Hovered() { Background(ControlHover) }` | 按下臂在前：`Background(ControlPress)` + `Scale(0.97)` + `LitFace` |
| 5 | `internal/shell/detach.go` 常量块 | 无按下档的几何数 | 新增 `pressScaleChip = 0.97` / `pressScaleKey = 0.92`（P13-q 的族规矩：胶囊 0.97、小格 0.92），四处引用同一对常量 |

**族规矩**：几何按形状分（胶囊 0.97 / 图标小格 0.92），面只有一个（`ControlPress`），
一对线（`LitFace`）在每个臂里各画一次——`internal/ci/litarm_contract_test.go` 的臂级
census 只认「臂内」或「同一函数臂外的共用」，而这几处的悬停臂各自带线、函数外没有共用线。

## 行为契约零改动红线

高度链（bar 的 `Space(7)` / 键的 `Size`）、键盘模型（`Focusable` / `Label` / `PassThrough` 的
`live` 语义）、会话管理、设置写回、i18n 文案、命令接线**一个字节都不动**：
`Scale` 是渲染变换，`Find` 拿到的布局盒不变（这一条由第二枚 pin 反向钉住）。
视觉轮只改墨与面。

## 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./... / GOOS=windows|darwin go vet ./...        # 必须净
go test -count=1 ./...                                 # 全绿
PASS 计数：基线 3045（HEAD 1f8e580d 之后 P13-aj/ak 已到 3045）
测试函数：2249 → 2252（+3：两条 pin + 一条 opt-in 出图）
SKIP：92 → 93
GOOS=linux|windows|darwin go build ./...               # 全过
```

## mutation 计划（≥2 条）

- **M1**（面）：把三处按下臂的 `ControlPress` 换回 `ControlHover` → 「按下的面 = 悬停的面」，
  `TestTheShellKeysSinkWhenThePointerGoesDown` 的第二条判据变红（实测 light 220↔220 / dark 53↔53）。
- **M2**（几何）：把 `Scale(pressScaleKey, pressScaleKey)` 换成 `Scale(1, 1)` → 面还在、盒子不收，
  `TestTheShellKeysSinkTheirBox` 变红（实测 45→45 / 32→32）。
- 两条都要 `md5sum -c` 逐字节还原并 `touch` 后复跑一次。
