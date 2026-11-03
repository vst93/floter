# P13-er 规格 · 浮层满态：最后一张卡的边被自己的底边软边吃掉

**意图**：让浮层（plugin sheet）**滚到底**时，最后一张卡与视口下沿之间有一格真正的空气，
而不是「软边刚好盖住内边距」。表面 = **插件配置浮层**（`internal/settingsui/pluginsheet.go`）。

**为什么现在做**：`P13-eq`「仍待」第 1 条（§6.111）等拍板，于是走通用队列的**满态**；
`P13-eq` 的报告里已经顺手核到三张面「滚到底时最后一块内容与视口下沿的空气」不同：
设置页 `Space(6)`（21.6）、浮层 `Space(3)`（10.8，**恰等于它自己的软边高**）、
启动器输出列表无下内边距。一个表面一轮，先量浮层——它是唯一「空气被自己的机制吃掉」的那张。

## 动手前实测（HEAD = c216e31e，`pluginSheetFrame` 720×600，深浅同值）

| # | 量的是什么 | 读数 |
|---|---|---|
| 1 | 滚到底时带子最下几行是空白 | **11 行**；第 12 行就是最后一张卡的下边（`spreadAt` 17~31 级，是卡自己的边，不是字） |
| 2 | 底边软边多高 | `Height(t.Space(3))` = **10.8px**（`pluginsheet.go` 里那条 `glass.ScrollEdge`） |
| 3 | 浮层给自己的下留白 | `Padding(t.Space(1), t.Space(4), t.Space(3), t.Space(4))` = **10.8px** |
| 4 | 同一套滚动收口在设置体上 | `Padding(edge, 0, t.Space(6), 0)`，`edge` = `Space(4)` = 14.4 的软边，下留白 **21.6 = 软边 + 一格 `Space(2)`**（`settingsui.go:1559`） |
| 5 | 页头有没有被碰 | 停在开头时顶部空白 **11 行**，墨从第 12 行起（§6.72 的 0.2px 余量） |

结论：规矩是「**软边 + 一格 `Space(2)` 的呼吸**」，设置页代入 14.4 + 7.2 = 21.6 一直是对的；
浮层代入自己的 10.8 应得 **18.0（`Space(5)`）**，它写的却是 10.8 —— 满态下卡的下边离软边上沿
只剩 **1.2px**，最后一张卡读起来是被切开的，不是到头了。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `pluginsheet.go:73` 的 scroller 内边距 | `Padding(t.Space(1), t.Space(4), t.Space(3), t.Space(4))` | 底边 `Space(3)` → **`Space(5)`**（+7.2px = 一格），公式与设置体同一个；顶部 `Space(1)` 与左右 `Space(4)` 不动 |
| 2 | 同上（注释） | 无 | 把「软边 + 一格呼吸」这条规矩、两张面的代入值、以及 1.2px 的病写进原地 |
| 3 | `internal/settingsui/sheetbottomair_test.go`（新） | 无 | `TestThePluginSheetKeepsAirAboveItsBottomFade`：滚到底时最后一张卡的边离带子下沿 ≥ 软边 + `Space(2)`（两条都从这张面自己的主题读，不写死像素）；页头仍在 11 行 |
| 4 | 同上（opt-in） | 无 | `TestDumpP13ERSheetBottom`（`FLOTER_DUMP_P13ER=1`）：rest / end × light / dark 四张 PNG |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**零改动**。本轮只动**一个内边距
的底值**：内容的几何（页头、每一行、每个分组）全部不动，变化的只有「内容盒比视口矮多少」，
也就是滚动范围 `MaxY` 345.2 → **352.4**（+7.2）。

## 门槛命令与 PASS 基线（本轮实测，HEAD = c216e31e）

```
gofmt -l cmd internal      → 空
go vet ./...               → 净（含 GOOS=windows / darwin）
go test -count=1 ./...     → 全绿
--- PASS 计数              → 3356（基线 3353）
测试函数                    → 2527（基线 2525：+1 pin、+1 opt-in 出图）
SKIP                       → 175（基线 174：新增的 opt-in 出图）
三平台 build                → 过
```

## V / B / S 台账计划

- **V 3**：`MaxY` 345.2 → 352.4（`sheetcutedge_test.go`、`sheetnamerung_test.go`、
  `sheetgroupboundary_test.go` 各一处）。三处的判据本身（「边是画出来的、不是几何」）不动，
  基线跟着内容高度走，注释写 `P13-er 重定基线：旧值 → 新值`。
- **B 0**：行为零改动。
- **S 0**：结构没变。

## mutation 计划（3 条，实跑）

- **M1** 底部内边距退回 `Space(3)`：`TestThePluginSheetKeepsAirAboveItsBottomFade` 必红
  （`the last card ends 11 rows above the band's bottom … want 17.999999`），证明 pin 真的量
  这一格，且改前后差的就是这条内边距。
- **M2** 把呼吸加到**顶部**（`Space(1)` → `Space(5)`，底部退回）：red 落在「页头挪了」
  （`11 blank rows … want 11` 变成 18），证明这一轮只许动下沿。
- **M3（对照）** 底部 `Space(5)` → `Space(6)`（21.6）：必须**仍绿**（18 → 21 行），证明 pin
  钉的是「软边 + 一格」这条下限，不是一个常数。
