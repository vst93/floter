# P13-fj 规格：空态的上下空气 —— 区域自己那 24u 就是它的呼吸

## 意图

通用队列「空态满态」的最后一格（P13-fi「仍待」第 1 条）。`emptyState`
（`internal/settingsui/empty.go:36`）是四个调用点共用的一块区域：§6.9 钉了**部件之间**
的三对缝（8 / 12 / 23px），§6.13 钉了出口的**种类**（板 vs 一行字），但**区域自己的上下
空气**从来没量过——它此前只是 `Gap(t.Space(2)).Padding(t.Space(6)).Center()`
（`empty.go:61-62`）这一句散文，和「floor 是 92u / 160u」这个看起来像高度的数。

为什么现在做：状态反馈（P13-fi）已收口，`liquid-glass.md` §6 差距表没有 `未开始` 行，
空态族里只剩这一格没量；且「空气」正是总目标第 3 条（空间感）唯一还能量的东西。

## 现状（动手前核过的行号）

| 位置 | 现状 |
|---|---|
| `internal/settingsui/empty.go:61-62` | `region := ui.Column(c).FillWidth().MinHeight(floor).Gap(t.Space(2)).Padding(t.Space(6)).Center()` |
| `empty.go:63-83` | `head`（`Gap(t.Space(1))`：标记 22u + 标题）、`hint`（`MaxWidth(Space(80))`）、`action`（`Margin(lead,0,0,0)`，板 12u / 文字 2u） |
| 调用点 | 会话页 `settingsui.go:1639`（floor 160u + 板）、自定义快捷键 `custom.go:50`（floor 0 → 92u + 一行字）、集成页 `settingsui.go:1861`（floor 0，无 hint 无出口） |

## 落地项（先量后改）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | 四个调用点的区域盒 | 高度未知；floor 被当成高度 | **量**：盒高 / 内容 + 2×24u / floor 三列并排，写进报告与 §6.131 |
| 2 | 上空气 | 从未量过 | **量**：`title.Y − box.Y` 必须 = `Space(6) + Space(5.5) + Space(1)`（24u + 22u + 4u），即标记盒顶就在内边距上 |
| 3 | 下空气 | 从未量过 | **量**：`box.Bottom − last.Bottom` 必须 = `Space(6)`，三个调用点同一个数 |
| 4 | 产品代码 | — | **零像素改动**：量完若都在同一个数上，不改 |

## 行为契约零改动红线

高度链（`Geometry.Height` / `RowCount`）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线：**一字节不动**。本轮只加测试与文档；`empty.go` 只在变异时被临时改过，已还原。

## 门槛命令 + PASS 基线（实测，HEAD = P13-fi）

```
gofmt -l cmd internal                        # 空
go vet ./...                                 # 净
GOOS=windows go vet ./...                    # 净
GOOS=darwin  go vet ./...                    # 净
go test -count=1 ./...                       # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3408（基线）
测试函数                                      # 2577（基线）
SKIP                                         # 191（基线；本轮 +1 = opt-in dump）
GOOS=linux/windows/darwin go build ./...      # 过
```

## mutation 计划（≥2 条，实跑）

- **M1**：`empty.go:62` 的 `Padding(t.Space(6))` → `Space(5)`。
  预期：`TestAnEmptyRegionBreathesWithOneNumberOnBothSides` 两侧同时红（18.0 对 21.6），
  三个 arm × 两套外观共 12 处。
- **M2**：`empty.go:70` 的 `head` 加 `.Margin(t.Space(1), 0, 0, 0)`。
  预期：只有**上空气**红（25.2 对 21.6），下空气不动——证明两侧是两条独立的账。
