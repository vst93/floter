# P11-ca · 终端抽屉进「字盒 / 行盒高度」那把尺子（自己量一遍阶梯）

选题依据：用户实测反馈 F1–F4 全部已收（P11-bj/bk/bo、P11-z、P11-bg/bc/bf…）→
`docs/raycast-design.md` §6 全部 `已落地`（只剩拍板项）→ 上一轮（P11-bz）「仍待」第 2 条
原话：

> **`internal/terminalui` 的抽屉还没进高度尺子**：条带 + 行的盒子与另外两个表面不是同族，
> 要自己量一遍阶梯，别照抄启动器或设置页的数。

## 1. 意图

P11-bz 把「一帧画出了哪几档高度」变成了一把尺子，并给**启动器**（六档 `15/18/22/24/25/28`）
与**设置页**（八档 `15/17/18/19/27/30/30.6/34`）各钉了一份阶梯。终端抽屉是同一把尺子的
第三个表面，但它的行不是 `rows.go` 建的那种「标题 + 副标题」的行，而是条带自己的
`drawerRow` / `drawerRowStacked` / `drawerPalette`，档位与另外两个表面**没有任何一档相同**
（它是终端面的控件：滑轴、开关、分段控件、迷你终端预览）。所以本轮**实测**它自己的阶梯并钉住，
不抄任何旧数字。

尺子本身（`internal/frameaudit/heights.go`）本轮零改动：它已经能报
「某个高度不在阶梯上」（`Ladder`）与「该出现的那一档没画」（`Missing`）。

## 2. 现状（本轮实测，动手前的真实行号）

| 位置 | 现状 | 高度 |
| --- | --- | --- |
| `control.go:110` 分段控件的 `.MinHeight(t.Space(7.5))` | 分段控件 / 开关的 30u 下限（`drawer.go:315` 的行下限**同值**，被它挡住） | **30** |
| `drawer.go:318` / `:343` 行标签 `FontSize(t.FontSize)` | 行标签、select 的值 | **22** |
| `drawer.go:300-306` `drawerReadout`（`tokens.Body` + mono） | 滑轴读数 `14 px` / `1.20×` | **17** |
| `terminalui.go:1801` `PalettePreview` 里的迷你终端（`t.FontSize - 1` + mono） | 预览的三行 | **20** |
| `terminalui.go:1364` `barButton` | 抽屉右上角的关闭按钮 | **28** |

- 条带自己是 `Scroll`（`drawer.go:80-89`），它的**label 盒子**（整条带）与预览的**盒子**
  （`Palette preview`，88u）都是祖先，`frameaudit.Heights` 的叶子规则本来就把它们排除
  ——所以它们**不该**出现在阶梯里，本轮也不钉它们。
- 实测范围：`lang ∈ {en,zh}` × `dark ∈ {false,true}` × `{640×360, 720×560, 900×600, 1100×700}`
  = 16 帧，**每一帧都是同一组 `17 20 22 28 30`**（逐帧实测，见 §4 M0）。
  640×360 是 P11-au 的「条带被压到画布之下、自己滚」的最小窗口，仍在同一组上。

## 3. 落地项

1. 新增 `internal/terminalui/textheight_test.go`：
   - `TestTheDrawerStaysOnTheHeightLadder`：16 帧，`frameaudit.AuditHeights` 的
     `rungs = must = {17,20,22,28,30}`，容差 0.5（框架按分数像素排版，同一档在相邻帧会报成
     16.999998 与 17）；审计范围 = 条带顶边以下的 run（与 `draweroverlap_test.go` 的
     `auditDrawerBandFrom` 同一段），下限 `frameaudit.MinHeight`。
     断言两件事：**没有一档离开阶梯**（`Ladder`）、**五档都在每一帧被画出来**（`Missing`）。
   - `TestEachDrawerRungHasItsOwnProducer`：把「哪段字画成哪一档」也钉住——只钉高度集合
     会漏掉「两档互换」（标签 22 → 20、读数 17 → 22 时集合仍是 `{17,20,22,28,30}`，但那
     已经不是设计）。五对 `文字 → 高度`：滑轴读数 17 / 预览行 20 / 行标签 22 / 关闭按钮 28 /
     分段控件 30，且每段字必须落在条带的带内（同一句话在别处也有时不会串味）。
2. 产品代码**零改动**。本轮只加尺子臂。

## 4. 门槛与基线（本轮实测）

- 动手前：`go test -count=1 ./...` 全绿，`PASS = 2657`，`SKIP = 54`，
  `grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l` = **2041**。
- 动手后：PASS 只许涨（预期 +2 … +4），函数数只许涨，SKIP 不变。
- `gofmt -l cmd internal` 空；`go vet ./...` + `GOOS=windows/darwin go vet ./...` 净；
  三平台 `go build` 过。

## 5. mutation 计划（每条都要实跑、看到红、还原并 `md5sum -c`）

- **M1** `control.go:110` 的 `MinHeight(t.Space(7.5))` → `t.Space(8)`（30 → 32）：
  第 1 条 pin 的 `Ladder` 必须报 `height 32.0 is not on the ladder`（实测 16 帧全红）。
- **M1b（绿变异，也是账）** `drawer.go:315` 的行下限 `7.5` → `8`：**画面一个字节不动**
  ——这一档是控件的下限给的，行自己的下限被它挡住。别把 `drawer.go:315` 当 30 的产生者。
- **M2** `drawer.go:302` 的 `.FontSize(tokens.Body)` → `tokens.Emphasis`（读数 17 → 22）：
  第 1 条 pin 的 `Missing` 必须报「没有 17」，第 2 条 pin 必须报读数的盒子不是 17。
- **M3** `drawer.go` 里 `PalettePreview(...)` 那一行删掉（预览整块不画）：第 1 条 pin 的
  `Missing` 必须报「没有 20」，第 2 条 pin 必须报 `floter --version` 找不到。

## 6. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**本轮一条都不碰**。
终端画布与 PTY 行数、抽屉的 `MinHeight` 语义、滚轮路径全部原样——本轮连一个产品字节都不改。
