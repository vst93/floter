# P13-ae 规格 · 名字是一行，不是一段（会话名 / 包名截断）

方向：**细节打磨**（通用队列「边界情况 / 文案密度」——§6 差距表没有 `未开始` 行）。
一个家族一次修完：**凡是「程序/别人给的名字」所在的标题行，都按参考 CSS 截断，不折行。**

## 一、意图（一句话）

会话行与集成行的标题里放的是**值**（shell 自己设的标题、发布者起的包名），不是本仓写的
句子；这种名字一长就折行，把那一行撑高、把行下面的灰块和右侧两颗按钮一起推下去——
列表的节奏正好在最需要读的那一行上断掉。

## 二、证据（先量，后改；实测于本轮开工时）

| 事实 | 出处 |
|---|---|
| 参考给了**两条不同的规则**：基类 `.settings-row__label` 只 `min-width: 0`（**可以折**），会话行额外 `overflow: hidden; text-overflow: ellipsis; white-space: nowrap` | `main:src/styles/settings.css:522-544` 与 `:1249-1260`（注释原文：*"a daemon-chosen session name … has to truncate rather than wrap, or one row would grow and break the list's rhythm"*） |
| 集成行的名字也是 `nowrap` + `ellipsis`，父行 `.extension-row__title` 是 `display: flex`（**没有** `flex-wrap`），版本 `span` 是 `flex: 0 0 auto` | `main:src/styles/extensions.css:884-915` |
| 本仓 `rowAligned` 的标题**裸画**（无 `SingleLine`），会话行与集成行都用它 | `internal/settingsui/rows.go:937`（`rowAligned`）、`settingsui.go:1579`（会话行） |
| 实测：460u 宽下，长会话名把标题画成 **72u**（四行），短名 **18u**；目录行从 `y=187.2` 被推到 `y=241.2` | 本轮 `TestASessionNameIsOneLineWhateverItsLength` 在**还原成 HEAD 代码**时的报错串 |
| 实测：560u 宽下长包名画出 **36u**（两行），版本 chip 从标题行掉到 `y=337.7`（标题行 `y=315.7..333.7` 之下） | 本轮 `TestAnIntegrationNameIsOneLineWhateverItsLength` 的报错串 |
| 集成行标题原本是 `.Wrap()`，所以「名字撑不下」的后果是**版本被挤到下一行**，不是名字被截 | `settingsui.go:2496`（改前） |

## 三、落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui/rows.go` `rowAligned` | 标题裸画；折行由容器宽决定 | 抽成 `rowTitle(..., name rowName, ...)`，`rowName{Text, Nowrap}` 两个字段：prose（默认）继续折行，`Nowrap` 走 `SingleLine().Ellipsis("\u2026").Tooltip(全名)` |
| 2 | `settingsui/rows.go` | — | 新增 `rowValue()`：`rowAligned` 的「名字是值」变体，一行转发 `rowName{Nowrap: true}` |
| 3 | `settingsui/settingsui.go:1579` 会话行 | `a.rowAligned(...)` | `a.rowValue(...)` |
| 4 | `settingsui/settingsui.go:2496` 集成行标题 | `Row().FillWidth().Wrap().GapX(2).GapY(1)` + 裸 `Text(integration.Name)` | 去掉 `Wrap()/GapY`（参考的 `nowrap` flex 行）+ 名字 `SingleLine().Ellipsis("\u2026").Shrink(1).MinWidth(0).Tooltip(全名)` |
| 5 | 测试 | — | `internal/settingsui/rowname_test.go`：3 条账（会话名一行 / 截断只截画笔不截数据 + 悬停全名 / 集成名一行且版本不掉行） |
| 6 | `docs/liquid-glass.md` §4.1 | 无此规则 | 新增「一行就是一行」小节：规则、范围、为什么 prose 不截 |
| 7 | `docs/liquid-glass.md` §6.2b / §6.4 | 无此行 | 各补一行（材料无关，记的是**文字行为**这一层，表里明写「非材质行」） |

**为什么加 `Tooltip`（参考没有）**：参考的会话在别处也能看到；本仓的设置页是读一个会话名
的唯一位置，截断而没有出口 = 名字读不到。`Tooltip` 只显示同一个字符串（不新增 i18n 文案），
先例是 `settingsui/config.go:475` 的 `SingleLine().Tooltip(...)`。

## 四、行为契约零改动

高度链（`rowFloorPx` 44u、`cardRow` 的 padding/gap）、键盘模型、会话管理、设置写回、
i18n 文案、命令接线**一字节不改**。集成行标题去掉了 `GapY`，但那是标题行**折行时**才生效的
间距——一个 `nowrap` 行永远用不到它；行高地板仍是 66u（`cardTitleFloor`）。

## 五、门槛（本轮实测基线）

开工时（HEAD = `1bc03d96` / P13-ad）：

```
go test -count=1 ./...                              → 全绿
PASS 3023   FAIL 0   SKIP 86   测试函数 2230
```

本轮新增 3 个测试函数 + 1 个 opt-in dump（`t.Skip`），期望：

```
PASS 3026   FAIL 0   SKIP 87   测试函数 2234
```

## 六、mutation 计划（每条都实跑）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | 会话行改回 `a.rowAligned(...)` | `TestASessionNameIsOneLineWhateverItsLength`（36/72 vs 18） |
| M2 | `rowTitle` 里去掉 `.SingleLine().Ellipsis("…")`，只留 `Tooltip` | 同上（名字重新折行） |
| M3 | 去掉 `.Tooltip(name.Text)`，只留 nowrap | `TestATruncatedSessionNameStillReadsInFull`（悬停只画出 1 次） |
| M4 | 集成行标题加回 `.Wrap().GapY(1)` | `TestAnIntegrationNameIsOneLineWhateverItsLength`（版本掉到下一行） |
