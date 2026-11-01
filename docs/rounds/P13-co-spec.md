# P13-co · 终端外观抽屉：读数没有自己的槽，滑轴被自己的数字推走

## 1. 意图（一句话）

抽屉里那三个「滑轴 + 读数」的尾部，读数是**多宽就多宽**的：字号从 8 拖到 10，
读数多出一个字形，整条尾部（含滑轴本身）被挤走一个字形宽——**手指按着的那根滑轴
自己挪位**。给读数一个固定宽度的槽，读数再变宽也不许推走它左边的东西。

为什么现在做：这是通用队列「间距节奏 / 状态反馈」交界上的一件，属于
`docs/liquid-glass.md` §4 的**间距刻度**在控件内部的最后一段，也是 motion 意义上
「跟手」的反面（拖动时被拖的东西动了）。§6 差距表没有 `未开始` 行，选题按
`.pending-bugs.md`「仍待（P13-cn）」第 7 条（通用队列「文案密度」或「空态满态」）
旁边的这一格——**它比那两条都更硬：有像素证据、无 i18n、无交互语义**。

## 2. 证据（动手前读的真实行号，HEAD `712ac7ac`）

| 位置 | 现状 |
|---|---|
| `internal/terminalui/drawer.go:330-334` | `drawerReadout` 只有 `Font`/`FontSize`/`TextColor`/`FontWeight`/`FontFeatures("tnum")`，**没有宽度**：盒子 = 这一帧那个数字的自然宽 |
| `internal/terminalui/drawer.go:352-359` | `drawerRow` 的尾部槽是 `Grow(1)` + `Justify(ui.End)`，孩子是 `[滑轴(固定 27.5u), 读数]`——右对齐，所以读数变宽 = 滑轴左移同样的量 |
| `internal/terminalui/drawer.go:166-178` | 字号读数 `fmt.Sprintf("%d px", …)`，范围 8…48（`settings.MinFontSize` / `MaxFontSize`）：`8 px` 4 个字形，`48 px` 5 个字形 |
| `internal/terminalui/drawer.go:196-208` | 行高读数 `fmt.Sprintf("%.2f×", …)`，范围 0.50…2.00：**恒 5 个字形**——它的稳定是格式碰巧给的，不是槽给的 |

实测（pin 的报错串，见报告）：字号 8 → 48，滑轴中心位移 **7 px**；同一个动作
9 → 10（拖过一位数）也是 **7 px**。

## 3. 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `drawer.go` `drawerReadout` | 盒子 = 数字的自然宽 | `MinWidth(t.Space(drawerReadoutSlot))` + `TextAlign(ui.End)`：读数住进一个固定宽的槽，数字仍然右对齐（单位对齐不破） |
| 2 | `drawer.go` 新增常量 `drawerReadoutSlot` | — | **10u = 40px**：最宽读数是 `48 px`（5 个 mono 字形 ≈ 36px），40px 装得下并留 1 个字形余量，且比「凭空多出一截空气」短 |
| 3 | 三个调用点 | — | **零改动**（签名不变）。行高与字号共用同一个槽宽，行高那一条从「碰巧稳」变成「按规矩稳」 |

**不做**：不给槽加面/线（它不是控件，是读数）；不动 `drawerRangeWidth`（27.5u，
§6.3 钉死）；不动行高 `MinHeight(7.5u)`；不动 `Gap(2)`。

## 4. 行为契约零改动红线

高度链（`drawerRow` 的 `MinHeight`、网格 `GapY`、条带滚动）**一个数不动**；
键盘模型、会话管理、设置写回（`a.preview` / `a.set` 的时机与内容）、i18n 文案、
命令接线**零改动**。这一轮只改读数盒子的宽度与对齐。

## 5. 门槛（PASS 基线用实测）

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 必须净
GOOS=windows go vet ./...                 # 必须净
GOOS=darwin  go vet ./...                 # 必须净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3276（P13-cn，HEAD 未变）
GOOS=linux/windows/darwin go build ./...  # 必须过
```

## 6. mutation 计划（每条说明会让哪个 pin 变红）

| # | 注入 | 期望 |
|---|---|---|
| M1 | 去掉 `MinWidth(...)`（回到自然宽） | 「滑轴中心」pin 红：`7.0 px` 位移 |
| M2 | 槽宽改成 `t.Space(3)`（12px，装不下 `48 px`） | 「读数折行」pin 红：盒子高度 8 px 与 48 px 不等 |
| M3 | 槽宽改成 `t.Space(16)`（64px，凭空多出一截空气） | 「槽宽 = 一个读数的宽」pin 红（`W` 越过 pin 的上界） |

## 7. 出图

`/tmp/floter-P13-co-readout-{light,dark}-{before,after}.png`（opt-in dump，
`internal/terminalui/readoutslot_dump_test.go`），并给出前后差分像素数。
