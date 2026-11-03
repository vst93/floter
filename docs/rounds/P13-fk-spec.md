# P13-fk 规格：`cardRow` 的控制槽 `Shrink(0)` —— 先量它到底会不会把标签列吃光

## 意图

`rows.go:1620` 的控制槽写着 `ui.Row(c).Shrink(0)`：**控件不收缩，让标签列
（`Grow(1).MinWidth(0)`）先让位**。这句话此前只是代码里的一个 flag，`.pending-bugs.md`
里挂了十几轮「需拍板」，`docs/liquid-glass.md` §6.18 的尾巴也写着「与多选箱宽度同族，
需拍板」。本轮先量，再决定要不要动它。

为什么现在做：通用队列里「边界情况 / 像素细节」两类没有专轮；这条是 P13-fj「下一格（首选）」
点名的第一条，且它是**布局边界**（控件不收缩 ⇒ 长控件会把标签列压到 0 再溢出卡框）。

## 现状（动手前读到的真实行号）

- `internal/settingsui/rows.go:1620`：
  `ui.Row(c).Shrink(0).Gap(t.Space(2)).AlignItems(align).Children(control)`
- `internal/settingsui/rows.go:1616`：
  `names := ui.Column(c).Grow(1).MinWidth(0).Gap(a.rowLineGap())`
- 最宽的一枚控件：`internal/settingsui/settingsui.go:4719`
  `ui.Select(c, selected, labels).Label(label).Width(220).Radius(999)`（字体族选择器）

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | 全五页 × 深浅，720×1400 | 未量 | **量**：每行控制槽宽度 / 标签列宽度 / 槽右沿是否越过行内容右沿 |
| 2 | `internal/settingsui/controlslot_test.go`（新） | 无 pin | `TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack`：控件墨跨在 720 与 1600 **逐列相同**、右沿贴内容右沿（±2）、标签列在 1600 长出 880、标签不被切 |
| 3 | 产品代码 | — | **零像素改动**（量出为不必改） |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：本轮**一行不动**。
只加一个测试文件。

## 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                       # 期望空
go vet ./...                                # 期望净
GOOS=windows go vet ./...                   # 期望净
GOOS=darwin  go vet ./...                   # 期望净
go test -count=1 ./...                      # 期望 0 FAIL
--- PASS                                    # 基线 3410
--- SKIP                                    # 基线 192
测试函数（静态计数）                          # 基线 2580
GOOS=linux/windows/darwin go build ./...     # 期望全过
```

## mutation 计划（≥2，每条说明让哪个 pin 变红）

- **M1** `rows.go:1620` 的控制槽 `Shrink(0)` → `Grow(1)`：槽变成弹性的 ⇒ 控件不再贴内容
  右沿、也不再是「固有宽度」⇒ 右沿断言 + 墨跨断言 + 「1600 时控件右移 880」三条全红。
- **M2** `rows.go:1616` 的标签列 `Grow(1)` → `Grow(0)`：标签列不再吃余量 ⇒ 控件跑到扫描带
  之外（"paints nothing"）⇒ 红。
