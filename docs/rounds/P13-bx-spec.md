# P13-bx 规格：终端空态的两条出路行，按下去此前与悬停帧逐像素相同

轮号 `P13-bx`｜分支 `mygo-rewrite`｜基线 HEAD `66435145`（P13-bw）
选题：通用队列「状态反馈」的另一半（禁用 / 只读面普查完之后剩下的**指针第三档**尾账）

## 0. 意图一句话

终端空态的两条「出路」行（`↩ Start a session` / `⌘N New command`）是**整行可点**的
（`emptyLine` 的 `line.Clicked()`），悬停时它抬墨，可**按住不放的那一帧与悬停帧
逐像素相同**——用户读到的不是「按到了」，而是「按坏了」。本轮补第三档，且用**面**，
因为这一行已经有 `Padding(0.5u, 1u)` + `RadiusSM` 的盒子（面画得下，不用改几何）。

为什么现在做：P13-al / P13-am / P13-bv / P13-bw 把「按下与悬停同帧」这一族在
shell / 终端浮条 / 设置页 / 启动器收完了，唯独终端**空态**的两条行不在那份普查的
名单里（普查按 `Hovered()` 对 `Pressed()` 的函数级形状扫，这两行把 `Clicked()` 写在
`line.Children` 之后、`Hovered()` 写在前面，形状对不上）。而它是新用户打开终端页
**唯一能点的东西**。

## 1. 证据（file:line，动手前复核过）

- `internal/terminalui/terminalui.go:964-999`（`emptyLine`）：
  - `:964` `line := ui.Row(c).Focusable().Padding(t.Space(0.5), t.Space(1)).Radius(tokens.RadiusSM)…`
  - `:974-977` `if line.Hovered() { color = tokens.TextStrong }` —— 只有悬停一档；
  - `:995-997` `if line.Clicked() { run() }` —— 整行是点击目标；
  - 全函数**没有** `Pressed()`。
- 对照（同一种「整行是目标」的既有读法）：
  - `internal/settingsui/rows.go:1405-1416`（`cardRow`）：`row.Pressed()` → `t.SurfacePressed`，
    `row.Hovered()` → `t.SurfaceHover`，两档都是**面**、都不动几何；
  - `internal/terminalui/terminalui.go:1475`（`barButton`）：本包自己的三档（面族）。
- `theme/theme.go:487` `t.SurfacePressed = p.controlPress` —— 与设置页行按下是**同一档 token**。

## 2. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `terminalui.go` `emptyLine` 的墨/面 | `color := tokens.TextSecondary; if line.Hovered() { color = tokens.TextStrong }`；面永远是零 | 写成 `switch`：`Pressed` → `TextStrong` + `Background(t.SurfacePressed)`；`Hovered` → `TextStrong`；静息 → `TextSecondary`。按下**保留**悬停的抬墨（指针还压在上面） |
| 2 | 同函数注释 | 只写「指针抬**字**一档，不画面」 | 补前提：抬墨是**悬停**的说法；按下是**面**的说法（同 `settingsui.cardRow` 的第三档），并写明「行不是键，不缩放」 |
| 3 | 键帽 | `tokens.LitFace(badge)` | **不动**（P13-d §6.3 3-8 已钉） |

**红线**：`Padding`/`Radius`/`Gap`/`Focusable`/`Label`、键帽、`run()` 接线、
`c.Shortcut(0, ui.KeyEnter)` 与 `Actions.NewCommand` 的调用：一个字节不动。
只改**墨与面**。

## 3. 门槛命令 + PASS 基线（实测于 HEAD `66435145`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3217
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

SKIP 基线 121、测试函数 2376。

## 4. mutation 计划（≥2，每条说明让哪个 pin 变红）

- **M1**：把按下臂画成悬停臂（即本轮之前的行为：不画面）。→
  `TestTheEmptyStateWaysOutSinkWhenPressed` 的「按下面 ≠ 悬停帧」一条红。
- **M2**：按下时把悬停的抬墨也一起退回去（`Pressed` 臂只画面、不抬墨）。→
  同文件「按下仍抬着墨（不退回静息）」一条红。
- **M3**（附加）：给按下臂加 `Scale(0.97)`。→ 「按下没搬盒子」一条红（键帽/标签盒子
  在两帧之间必须逐值相同）。

## 5. 出图

`/tmp/floter-P13-bx-emptylines-{dark,light}-{rest,hover,press}.png`（opt-in dump，
`FLOTER_PNG=1`），报告里写图上看到的一处变化。
