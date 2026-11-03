# P13-fs 规格：空态那一对「标记与它那句」，三个调用点各量一次

## 意图

通用队列「空态满态」的最后一格——**空态里那几句自己的距离**（P13-fr 报告「仍待」第 1 条）。
§4.3（P13-aj）把空态的三对缝量成 8 / 12 / 23px，但**只在会话页那一套字形上量过**
（`internal/settingsui/emptyink_test.go` 走 `a.View` + `PageSessions`，标记 `square-terminal`）。
`emptyState`（`internal/settingsui/empty.go:36`）是三个调用点共用的一块区域，**标记由调用点挑**：
会话页 `square-terminal`（`empty.go:116` 的 face）、自定义快捷键 `keyboard`
（`custom.go:50`）、集成页 `package`（`settingsui.go:1861`）。

标记的**盒**是一个数（`Space(5.5)` = 19.8px），可墨不填满盒子——墨有多高是**字形自己的
纵横比**。于是「标记与它那句」这一对在墨上的距离随字形摆，而它正是 §4.3 里「同一块」那一档。

为什么现在做：`docs/liquid-glass.md` §6 没有 `未开始` 行；空态族的间距 / 出口 / 满态 / 空气
四格都已落地，剩下没量的只有这一对在**另外两个字形**上的表现，以及理由折行后的**段内行距**。

## 现状（动手前核过的行号）

| 位置 | 现状 |
|---|---|
| `internal/settingsui/empty.go:52-53` | `Gap(t.Space(2)).Padding(t.Space(6))`，区域一个数 |
| `empty.go:62` | `head := ui.Column(c).Center().Gap(t.Space(1))` —— 标记与句子之间那 4u |
| `empty.go:66-67` | `ui.Icon(c, glyph).Size(iconSize, iconSize)`：盒是方的，墨按字形纵横比落进去 |
| `empty.go:74-78` | `ui.Text(c, hint).FontSize(tokens.Caption).LineHeight(1.5).MaxWidth(t.Space(80))` —— 唯一能折行的一件 |
| `emptyink_test.go:139-160` | 只在 `PageSessions` 上量，判据是「标记→句子 比 句子→理由 至少近 2px」 |

## 落地项（先量后改）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | 三个调用点的标记 | 只有会话页被量过 | **量**：标记墨高 / 标记→句子 / 句子→理由 三列并排（`emptypara_test.go` 的 `emptyParaProbe`，逐 arm 用该 arm 真发的字形） |
| 2 | `keyboard` 那一处 | 未知 | **量**：若这一对在墨上不再「更近」，按 §4.3 的判据记账 |
| 3 | 理由折行 | 从未量过 | **量**：段内墨缝 / 行距 / 与上一件的距离（en 折两行、zh 一行） |
| 4 | 产品代码 | — | **零像素改动**：量完若规则成立，不改；量出「一个数 vs 一个墨距」的张力，写进合约并判 `需拍板` |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一字节不动**。
本轮只加 `internal/settingsui/emptypara_test.go` 与文档。

## 门槛命令 + PASS 基线（实测，HEAD = P13-fr `f22d5185`）

```
gofmt -l cmd internal                        # 空
go vet ./...                                 # 净
GOOS=windows go vet ./...                    # 净
GOOS=darwin  go vet ./...                    # 净
go test -count=1 ./...                       # 3422 PASS / 0 FAIL / 196 SKIP（2596 函数）
GOOS=linux/windows/darwin go build ./...     # 三平台过
```

## mutation 计划（≥2 条）

- **M1**：`head` 的 `Gap(t.Space(1))` → `Gap(t.Space(2))`（把「同一块」退回区域那一档）
  → 标记→句子变宽，`TestTheMarkHangsOffItsSentenceOnEveryShippedMark` 的三个 arm 全红。
- **M2**：`emptyState` 的 `iconSize` 默认 `Space(5.5)` → `Space(4)`（标记变小）
  → 墨高与墨距同时变，同一 pin 红。
- **M3**：理由的 `LineHeight(1.5)` → `LineHeight(1.2)`。**实测是一次空变异**（墨距 16.0、盒高 30.0
  逐值不变，一个 pin 都没红）——`ui.Text` 的折行排版吃字体自己的行进给，`LineHeight` 不参与。
  这条空变异本身是本轮结论之一（合约 138-4），于是换 **M3′**：`MaxWidth(t.Space(80))` →
  `MaxWidth(t.Space(40))` → `TestAWrappedWhyIsStillOneParagraph` 的「2 行」判据红。
