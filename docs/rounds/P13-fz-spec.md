# P13-fz 规格：插件窗口的下沿 —— 同一族里最后两个刀口

**意图一句话**：这个应用里每一块**自由滚动**的面都有下沿，只有两个插件窗口
（捕获输出页、便条）没有——滚到中间时正文最后一行在窗口最后一行像素上被水平切断。
这一轮把这两块面补上同一道 `glass.ScrollEdge`，并证明它**不吃布局、不盖字、空白上不显形**。

**为什么现在做**：`docs/liquid-glass.md` §6 差距表没有 `未开始` 行，选题走通用队列；
上一轮（P13-fy）「仍待」第 1 条 `140-5` 需拍板。这一处是同族里最后一块没进过账的面，
而且是**可见的**（不是量尺轮）：设置页（§6.16 / P13-as）、启动器满态（§6.1 表 1-10）、
会话抽屉（§6.141）、捕获输出页的浮条（§6.20）都有下沿，这两个窗口没有。

**来源**：`git stash@{2}`（`8a69c7f861`「floter orphan 2026-11-03 15:58」）里有一条未接续的
未提交改动——它给 `detachedBodyView` 补了下沿，注释引用的 §6.121 已过期，也没有任何 pin。
本轮把它接上、改成一对窗口（含 `pinned.go`）、把 § 号落到 §6.145，并补上判据与变异。

## 现状（动手前实测，720×460）

| 面 | 现状 |
| --- | --- |
| `internal/shell/detach.go:396-426` `detachedBodyView` | `ui.Scroll(...).Fill().Padding(Space(3.5))`，正文滚到窗口底边即被切；`MaxY` 1151.8 时窗口最后一行仍是满对比字形（spread **139 / 134**，与它上方那一带同值） |
| `internal/shell/pinned.go:121-126` `pinnedBody` | 同一个 `.plugin-window` 表面、同一个滚动器、同一个内边距，同样没有下沿 |

## 落地项

| # | 位置 | 现状 | 改成 |
| --- | --- | --- | --- |
| 1 | `detach.go` `detachedBodyView` | 无下沿 | 正文之后一枚 `Absolute().Bottom(0).Left(0).Right(0).Height(Space(detachedEdgeInset)).PassThrough()` + `glass.ScrollEdge{Bottom: true, Background: tokens.Opaque}` |
| 2 | `pinned.go` `pinnedBody` | 无下沿 | 同上（同一常数、同一条边、同一条自门控算术） |
| 3 | `detach.go` 常量块 | 无 | 新增 `detachedEdgeInset = 2.5 // 10u`，注释写明它必须**小于** `detachedBodyInset`（3.5 = 14u） |
| 4 | `internal/shell/pluginwindowedge_test.go` | 无 | 三条判据 + 一条 opt-in dump |
| 5 | `internal/ci/liquidglass_contract_test.go` | 分隔行只认 `\|---` 前缀，`\| --- \| --- \|` 那种写法被当成数据行（P13-fy 的 §6.144 表把这条检查打红） | 按单元格读：全是 `-`/`:` 的行就是分隔行（解析修正，不是放宽断言） |

**为什么 `Background: tokens.Opaque`**：两个窗口的根节点都被刷成 `tokens.Opaque`
（`detachedView:219` / `pinnedView:79`），窗口本来就是不透明的；设置页那条
`FaceOver` 链（§6.17 / §6.18）在这里不成立（薄罩会被乘成 0.06）。

**为什么高度是 10 而不是设置页的 14.4**：这条边的门是「滚到底时它落在空白上」
（§6.19 的几何门）。正文内边距 14 就是最后一行停下的地方，所以边必须 < 14。
10 留 4px 余量，同时足够画出 9 行的渐变（实测 `deepest fade 9 rows`）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一行不动**。
这道边是叠层：`.Absolute()` + `.PassThrough()`，不进布局、不吃指针。
判据 3 就是这条红线的量尺（`MaxY` 必须还是 1151.8）。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净（windows / darwin 同）
go test -count=1 ./...         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3440（P13-fy 实测）
GOOS=linux/windows/darwin go build ./...            # 三平台过
```

## mutation 计划

| # | 注入 | 期望变红 |
| --- | --- | --- |
| M1 | `Height(t.Space(detachedEdgeInset))` → `Height(t.Space(0))` | 判据 1（最后一行 == 上方那一带，100%） |
| M2 | `detachedEdgeInset = 2.5` → `4`（16 > 内边距 14） | 判据 3（`band 16 >= inset 14`） |
| M3 | `Background: tokens.Opaque` → `tokens.Control` | 判据 1 的自门控（滚到底时带上留下一道 239 / 29 的带子） |
| M4 | 去掉 `.Absolute()`（边进布局） | 判据 3（`MaxY` 1151.8 → 1160.8） |
