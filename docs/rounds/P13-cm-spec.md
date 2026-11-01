# P13-cm 规格 · 同一个结局：终端页用暖色说，会话列表用灰说

轮号：**P13-cm**（上一轮 P13-cl，`6b962440`）
选题：通用队列 `状态反馈`（P13-cl「仍待」第 1 条是 i18n 红线、已进拍板；本轮取它后面的
第一件不需要拍板的事）。

## 意图（一句话）

**「这个程序失败了」这个事实，在全仓只有一处用色相说（终端页的常驻便条），
另一处（会话列表的状态词）用灰说——同一个结局，两个寄存器。本轮把它们并成一档。**

为什么现在做：P13-bg 已经为**捕获输出页**立过同一条判据（`docs/liquid-glass.md` §6.26：
「三种结局逐像素相同」是缺陷，色相是语义不是强调，失败臂 `t.Warning`）。会话列表是
**同一件事实的第三处**，那条判据没有落到它身上——这正是「同族问题一次修完」漏下的一片。

## 证据（行号本轮实测）

| # | 位置 | 现状 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:1746` `sessionMeta` | 状态词只有两档：`Running` → `rowSegmentStrong`，否则 `rowSegmentTertiary`。**`ExitCode` 根本没进 `Session`**（`settingsui.go:580-603` 只有 `Running` / `Note`） |
| 2 | `internal/settingsui/rows.go:944-950` | 角色枚举只有 `rowSegmentTertiary` / `rowSegmentStrong` 两枚 |
| 3 | `internal/settingsui/rows.go:1176-1180` | 画笔只认 `rowSegmentStrong`，其余一律 `tokens.TextTertiary` |
| 4 | `internal/terminalui/terminalui.go:1315-1318` `residentNote` | **已经有这一档**：`if session.ExitCode != 0 { color = t.Warning }`。终端页说「失败」是暖色 |
| 5 | `internal/terminalui/terminalui.go:427-441` `NoteOf` | 结局的**词**已经分档（`SessionExited` / `SessionExited · exit N`），只有**墨**没分 |
| 6 | `internal/shell/shell.go:1053-1070` | `settingsui.Session` 由这里拼出，`Note` 已经来自 `NoteOf`，`ExitCode` 没有传 |

于是同一个「`exit 3`」在终端页是暖色便条，在会话列表是一枚灰色小字——**两页说同一件事，
两种语气**。用户 2026-10-30 的原话「空间感不够 / 全糊在一起」在这一条上的表现就是：
失败与成功在列表里**同墨同重**，只有三个字符的差别（`· exit 3`）。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `Session` | 无失败信息 | 加 `Failed bool`：程序以非零码结束（`Note` 已经写着码，这一位只回答「要不要用暖色说」） |
| 2 | `internal/settingsui/rows.go` 角色枚举 | 两枚 | 加 `rowSegmentWarning`（第三枚，`t.Warning`） |
| 3 | `internal/settingsui/settingsui.go` `sessionMeta` | `Running` 二选一 | 三选一：运行中 → `Strong`；已结束且 `Failed` → `Warning`；其余 → `Tertiary` |
| 4 | `internal/settingsui/rows.go` 画笔 | 只认 `Strong` | 认 `Strong` 与 `Warning`（`t.Warning`，`c.Theme()` 那一枚，与 `residentNote` 同一个来源） |
| 5 | `internal/terminalui` 新增 `Failed(index)` | 无 | 读 `Sessions[index].ExitCode != 0`（只在已结束时为真；`Resident` 为假时返回 false） |
| 6 | `internal/shell/shell.go` 拼 `settingsui.Session` | 不传 | 传 `Failed: a.Terminal.Failed(i)` |

**不动的**（红线）：`Note` 的文案与 `NoteOf` 的措辞（i18n 零改动）；行的几何（高度 / 圆角 /
padding / gap / 字阶 / 字重）；`rowSegmentStrong` 与 `rowSegmentTertiary` 的既有语义与色值；
`residentNote`（终端页那一档本轮只作为判据引用，不改）；会话的 list / attach / kill 语义；
`Session` 其余字段。

## 门槛（本轮实测基线，改前先跑一遍记账）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...   → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3263（HEAD 实测）
GOOS=linux/windows/darwin go build ./...               → 过
```

PASS 计数只许涨。测试函数数量不许减少。新增 `t.Skip` 只允许是 opt-in 出图那一条。

## 变异计划（≥2 条，说明每条会让哪个 pin 变红）

- **M1**：`sessionMeta` 的失败臂退回 `rowSegmentTertiary`（即本轮之前的写法）→
  `TestTheFailedSessionEndsInTheWarningHue` 的暖色相断言变红（`r−b ≥ 40` 不成立）。
- **M2**：画笔把 `rowSegmentWarning` 也画成 `TextTertiary`（角色加了、墨没接）→ 同一条 pin
  变红，证明红的是**像素**不是枚举。
- **M3**（反向对照）：把**运行中**那一臂也画成 `Warning` → 「运行中不许是暖色」那条断言变红，
  证明暖色是**结局**的记号，不是「这一行是重点」。

每条都要实跑：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑绿。
