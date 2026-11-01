# P13-cz 规格 · 会话页的满态：列表与页脚的关系（先量，不改）

**轮号**：P13-cz（流动队列「空态 / 满态」→ 满态那一半）
**表面**：设置页 › Sessions（`internal/settingsui/settingsui.go` 的 `sessions`）
**纪律**：`docs/liquid-glass.md` §6 已无 `未开始` 行 → 走通用队列（间距节奏 / 状态反馈 /
空态满态 / 文案密度 / 边界情况）。本轮取「空态满态」的**满态**那一半。

## 1. 意图一句话 + 为什么现在做

> 会话页的空态已经被收过两轮（P13-aj 的三件东西、P13-ap 的出口按种类取间距），
> 但**满态**（列表有多条会话时，「列表 → 组注 → 页脚出口」这条关系）从来没量过；
> P13-aq 的「仍待」第 4 条②点名了它。本轮只量、只钉，不动画笔。

证据（先量，实测浅色档 720×760，`interface step` 默认）：

| 位置 | file:line | 实测 |
|---|---|---|
| 页面列 gap 5.5u | `internal/settingsui/rows.go:55-57`（`pageColumn`） | 注底 349 → 胶囊顶 371 = **22px** |
| 组内 gutter 2u | `internal/settingsui/rows.go:99-110`（`sectionGroupPadded`） | 卡底 318 → 注顶 327 = **8px** |
| 页脚出口 | `internal/settingsui/settingsui.go:1646-1725`（`sessions` 末尾的 `pageButton`） | 胶囊盒 371..401 = **31px**（30u + 一对 1px 边） |

**结论**：三个数都落在 §4.3 的阶梯上——8px「同一块」< 12px「同一段的下一行」<
22px「一个可以按的新东西」，与空态那一支（0.5u / 3u）是同一句话（间距按「这个东西
是什么」取，不按「它在第几行」取）。所以本轮**不改**，把它们钉住。

第二条要量的是「页脚是不是钉在视口底」：换成一枚会话后两个间隙一个像素都不动 →
页脚**跟着内容走**，不是 fixed 底栏，也不读行数。

## 2. 落地项

| # | 位置 | 现状 | 本轮 |
|---|---|---|---|
| 1 | `internal/settingsui/sessionsfoot_test.go`（新） | 这一族没有任何 pin | `TestTheSessionsPageFooterRidesItsContent`：卡底→注 `8`、注底→胶囊 `22`、胶囊盒 `31`；三行会话的卡片必须长过 100px |
| 2 | 同上 | —— | `TestTheSessionsPageFooterDoesNotReadTheRowCount`：1 行与 3 行的两个间隙逐值相等，且 3 行的卡片严格高于 1 行 |

**产品代码零改动**（这也是本轮的正确读数：`gofmt`/`vet`/测试全绿之外，**渲染像素一个
都没变**——没有改动画笔，所以不出前后对比图；P11-cr 走的是同一条判定）。

## 3. 行为契约零改动红线（逐条核对）

- 高度链：`session` 行的 47px 步距、`pageButton` 的 30u、`sectionGroup` 的 gutter——**一个数没动**。
- 键盘模型：`Sessions` 侧栏项、页脚胶囊、行内两枚 28u 键——**没接任何新 handler**。
- 会话管理：`SelectSession` / `CloseSessionAt` / `NewSession` 三个 `Actions` **签名与调用点不变**。
- 设置写回：`sessionsSortOldest` 未触碰。
- i18n：文案零新增、零改动（pin 只读 `en.PageSessions` 等既有键）。
- 命令接线：无。

## 4. 门槛与 PASS 基线

基线（本轮开工实测，P13-cy 之后）：**PASS 3300 / FAIL 0 / SKIP 143**。
本轮目标：PASS **3302**（+2），FAIL 0，SKIP 不变（新 pin 不是 opt-in 截图 dump）。

```
gofmt -l cmd internal                     # 空
go vet ./... ; GOOS=windows/darwin go vet ./...   # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  # 3302
GOOS=linux/windows/darwin go build ./...  # 过
```

## 5. mutation 计划（两条，都实跑）

- **M1**：`internal/settingsui/rows.go` 的 `pageColumn` 把 `Space(5.5)` 改成 `Space(4.5)`。
  预期红：`TestTheSessionsPageFooterRidesItsContent` 的「page button stands 18px under the hint,
  want the page column's 22u gap」。
- **M2**：同文件 `sectionGroupPadded` 把 `a.cardTheme(c).Space(2)` 改成 `Space(3)`。
  预期红：同一条的「the hint stands 12px under the card, want the group's 8u gutter」。

两条都必须 `md5sum -c` 逐字节还原，`touch` 后复跑一次绿。
