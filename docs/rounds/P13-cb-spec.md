# P13-cb 规格：锁定的字段还亮着半盏灯——把「锁定」带进字段材质本身

选题来源：`.pending-bugs.md` 的 P13-ca「仍待」第 1 条（上一轮自己量到、自己写下的下一格），
优先级 3（上一轮报告「仍待」第一条）。`docs/liquid-glass.md` §6 差距表无 `未开始` 行。

## 意图（一句话）

字段被锁定时，它的**受光边要跟着状态一起熄**——锁定后的字段应该是「一个灭了灯的凹槽」，
而不是今天这块「一块平涂 + 一条半亮的顶线」。

为什么现在做：P13-ca 已经把这格量清了，并且证明了**墨这一层改不动**：

- 现状（`lockProbe`，`extensionField` → `lockedControl`）：面 230→238（浅）/ 39→26（深），
  冠线 − 面 = **+2 / +10**，底对侧 − 面 = **0 / 0**（`internal/settingsui/lockedform_test.go:340-378`）。
- 两条事后撤回的改法都失败并已回退（`docs/liquid-glass.md` §6.44）：`DimFace` 摘不掉已画的冠线，
  `InsetShadow(0,1,0,0,Control)` 盖回去在深色下反而更亮（36 → 47）。

所以这一轮按「改结构不改墨」：把锁定状态**在建元素时**交给字段材质。

## 落地项（动手前已核过行号）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/textinput.go:74` `fieldFace` | 无条件 `tokens.LitFace(e.Background(Control).Border(1, ControlEdge))` | `fieldFace(c, e, lit bool)`：`lit` 为假时走 `tokens.DimFace(...)`（只留底对侧暗线，不画冠线）。**一个字段材质仍然只有这一个决定点** |
| 2 | `internal/settingsui/textinput.go:36` `textInput` | 直接 `a.fieldFace(c, ui.TextInput(...))` | `textInput` 保留为 `lit = true` 的壳；新增 `textInputLit(c, value, lit)`，快捷键/`HandleInput` 全部留在里面（不复制） |
| 3 | `internal/settingsui/textinput.go:118` `extensionField` | `a.textInput(...)` + 面板盒 | 同上：`extensionField` 保留为 `lit = true` 的壳，新增 `extensionFieldLit(c, value, m, lit)` |
| 4 | `internal/settingsui/config.go:502` `configSelect` | 末行 `a.fieldFace(c, e)` | 加 `lit bool` 参数（唯一调用点 `config.go:560`，同一张表单，同一处判断） |
| 5 | `internal/settingsui/config.go:643 / :701`（数字字段、文本字段） | `a.extensionField(...)` → 稍后 `lockedControl(e)` | `a.extensionFieldLit(..., !locked)`，`lockedControl` 那一行**保留**（状态还是得挂） |
| 6 | `internal/settingsui/config.go:560`（下拉） | `a.configSelect(c, &selected, labels, label)` → `lockedControl` | 传 `!locked`，`lockedControl` 保留 |
| 7 | `internal/settingsui/{custom.go:335, parameditor.go:187,222, settingsui.go:4507,4533,4869}` | `a.fieldFace(c, e)` | 传 `true`（这些字段没有锁定臂，材质不变） |
| 8 | `internal/settingsui/lockedform_test.go:178` A 臂 | `assertOneFade(..., crownLit=true)`，注释写「本轮不改」 | `crownLit=false`（锁定后冠线 = 面），注释改成「P13-cb 已熄灯」；探针的 build 函数改成走 `extensionFieldLit(..., !isLocked)`，与产品同一处判断 |
| 9 | `internal/settingsui/lockedform_test.go:340` E 臂 | 冠线只 `t.Logf` | 冠线与面的差 **≤ 1 级**（真的断言，不再只是记录）；`TestTheLockedFormHasOneDepth` 的 build 也同步 |

## 行为契约零改动红线

只改**字段的受光边**这一条线。不动：高度链（`extensionFieldMetrics` 28/30/999 那一族）、
键盘模型（`textInput` 里的 `Shortcut`/`HandleInput`）、会话管理、设置写回（`draft`/`Changed`/`Submitted`）、
i18n 文案、命令接线。`Disabled` 状态、`Cursor`、0.5 那道淡、面离地的台阶全部照旧。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿
--- PASS 计数                         → 3226（基线）
测试函数                              → 2383
SKIP                                  → 123
三平台 go build ./...                 → 过
```

## mutation 计划

- **M1** `fieldFace` 的 `lit=false` 臂退回 `LitFace`（即「忘了熄灯」）→
  `TestTheLockedFieldFadesOnce` 的 `crownLit=false` 臂必须红（冠线 − 面 > 1 级）。
- **M2** `fieldFace` 的 `lit=true` 臂也走 `DimFace`（即「连静息字段也熄灯」）→
  `TestTheLockedFormHasOneDepth` / `liteedge_test.go` 的 E1 一对线 pin 必须红
  （静息字段的冠线归零）。
- 还原方式：`cp` 备份 + `md5sum -c` 逐字节核对 + `touch` 后复跑（**不用** `git checkout`，
  见 P13-bz 的教训）。
