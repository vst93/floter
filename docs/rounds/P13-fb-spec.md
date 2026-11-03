# P13-fb 规格：终端抽屉的行名，比设置页同一角色的行名低一档

轮号 P13-fb（HEAD 前 P13-fa）。方向：通用队列 · **文案密度 / 字阶**（P13-fa「仍待」第 1 条
指向的「状态反馈 / 空态满态」之外，同一族的下一条：同一角色两个寄存器）。表面 = **终端页 ·
外观抽屉**（`internal/terminalui/drawer.go`）。

## 意图一句话

终端抽屉是**同一份设置行的第二份实现**：它画的是「一行名字 + 一件控件」，设置页画的是
同一件事——两处的**名字**此前穿两个寄存器，抽屉那份低一档（正文档 + 次墨），于是同一个
应用里「一行叫什么」有两种大小、两种明度。

为什么现在做：§6.120（P13-ey）刚把「一页两句话两个寄存器」收掉，§6.121（P13-ez）把
「答案不该比它回答的数字小」收掉——这两条是同一个毛病的两种形状。抽屉是这一族里**最后
一份没有对齐的拷贝**，而且它离得最近：用户从设置页切到终端页，看到的是同一批轴
（字体、调色板、行高）以两种字阶画出来。

## 现状（`internal/terminalui/drawer.go`，行号已核）

| 位置 | 角色 | 现状 | 应改成 |
|---|---|---|---|
| `drawer.go:391`（`drawerRow`） | 行名 | `FontSize(t.FontSize)` + `FontWeight(560)` + `TextSecondary` | `tokens.Emphasis` + 580 + `TextStrong` |
| `drawer.go:416`（`drawerRowStacked`） | 行名 | 同上 | 同上 |
| `drawer.go:434`（`drawerPalette`） | 行名 | 同上 | 同上 |

对照（`internal/settingsui/rows.go:1726`，§6.117 落地后的那一支）：

```go
ui.Text(c, label).FontSize(tokens.Emphasis).FontWeight(580).TextColor(tokens.TextStrong)
```

`t.FontSize` 是**正文档**（`theme.go:463` `round1(BodyUnits * f)`，与 `tokens.Body` 同值），
`tokens.TextSecondary` 是**次墨**。也就是说抽屉的行名与它旁边的读数
（`drawerReadout`：`tokens.Body` + `TextStrong` + 600）只差字重，**名字比它命名的数字还小
半档**——与 §6.121 收掉的毛病同形，只是发生在另一份实现里。

## 落地项（视觉轮只改墨与面）

1. 三处行名：字号 `t.FontSize` → `tokens.Emphasis`，字重 560 → 580，墨
   `TextSecondary` → `TextStrong`。
2. **不动**：行盒 `MinHeight(Space(7.5))`、网格 `drawerCellFloor` / `paletteCellFloor`、
   行内缝（`drawerRow` 的 `Gap(Space(2.5))`、`drawerRowStacked` 的 `Gap(Space(1.5))`）、
   `SingleLine()` / `Shrink(0)`、控件本身、读数（`drawerReadout`）、条带的内边距与
   `BorderWidth`/`LensRim` 发丝、`ScrollEdge` 软边、`ToggleDrawer` 与写回路径。

## 行为契约零改动红线

高度链（条带高 = 内容 + 两条内边距 → 决定画布高 → 决定 PTY 行数）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线：**一个字节不动**。行名变大 0.9pt 只改墨，行盒有
`MinHeight(Space(7.5))` = 27 的地板，`SingleLine()` 保证不回绕。

## 门槛命令 + PASS 基线（实测当前值）

```
gofmt -l cmd internal                                  # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...
go test -count=1 ./...                                 # 3385 PASS / 0 FAIL / 184 SKIP（P13-fa 实测）
GOOS=linux/windows/darwin go build ./...
```

测试函数 2555 条；本轮 +2 条 pin + 1 条 opt-in 出图。

## mutation 计划（≥2 条，各说明会让哪个 pin 变红）

- **M1** 把 `drawerRow` 的字号改回 `t.FontSize`：pin 1（行名穿字阶的 emphasis 档）变红。
- **M2** 把 `drawerRowStacked` 的墨改回 `tokens.TextSecondary`：pin 2（行名比它命名的读数更
  亮）变红。
- **M3**（等价写法）`tokens.Emphasis` → `t.Emphasis`？不存在——改用 `round1(13*f)` 字面量
  不现实，改为把字重 580 → 560（视觉几乎不变）：预期 pin 仍绿，记作覆盖边界。

## 结果（P13-fb 实际发生的事）

本规格的前提（抽屉行名该穿设置页的 `Emphasis`/580/`TextStrong`）被**证伪**：量下来它撞
两条已落地契约（读数才是这一行最响的一半 707 : 713；条带高 = 画布高 = PTY 行数，54.4 →
56.4）。实际落地 = 生产代码零改动 + `docs/liquid-glass.md` §6.123「故意不做」+ 两条 pin。
详见 `docs/rounds/P13-fb-report.md`。
