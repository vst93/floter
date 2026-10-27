# P11-f 规格：读之前先读盘——`usage.Store` 的每个入口都从文件开始

## 意图

**一句话**：`usage.Store.Top` 在没被 `Record`/`Count`/`Seed` 碰过之前回答「空历史」，
于是**冷启动的「常用应用」块是空的**（用户必须在本会话里启动过一个 app 它才出现）——
本轮让每个入口先 `loadLocked`，`Save` 也不许在没读盘时把 `{}` 写回去。

## 为什么现在做（证据）

选题优先级：用户实测四条全卡在实机、合约 §6 已全部「已落地」、上一轮「仍待」第 1 条同样卡在实机，
按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，落在**基础功能**，且**三平台同错**
（本包无 GOOS 分支）。

病灶（`git rev-parse HEAD` = `ec2a1e0`，行号是当轮实测）：

- `internal/usage/usage.go:104-112`：`Top` 只 `s.mu.Lock()` 然后遍历 `s.records`，
  **没有 `s.loadLocked()`**；同包另外两个读入口都有（`Record` `:56`、`Count` `:97`）。
  `records` 为 nil 时 `Top` 返回空切片，`keep` 回调一次都不会被调。
- `internal/usage/usage.go:43-45`：`Open` 只记路径，**不读文件**（懒加载）。
- `internal/shell/shell.go:1576-1595`：`refreshRecents` 是唯一写 `Launcher.SetRecent` 的地方，
  它第一句就是 `a.Usage.Top(...)`（`:1583`），`Count` 在**之后**（`:1585`，而且只在非空路径上）。
- 冷启动时唯一可能先读盘的是 `internal/shell/shell.go:1030` 的 `Seed`，
  而 `internal/usage/usage.go:70-77` 在 `len(counts) == 0` 时**提前返回、不 load**。
  `launch_counts` 是**旧构建**写在 settings 里的字段（`internal/settings/plugins.go:230`），
  本构建的新用户 settings 里根本没有它 → 这条路径对新用户恒为「提前返回」。
- 于是真实冷启动序列是：`Open`（不读）→ `Seed(空)`（不读）→ 扫描到达 → `refreshRecents`
  → `Top`（不读）→ **空**。`internal/shell/recentsstartup_test.go` 的 pin 没抓到，因为它在
  `scanApps` 之前用 `a.Usage.Record(...)` 造数据，而 `Record` 自己会 `loadLocked`——pin 走的
  不是真实冷启动路径（本轮的 pin 用「另一个 store 写盘 → 新 store 只读」来复现）。

参考的行为：参考把启动次数存在 settings 快照里（`launch_counts`），而 settings 在启动时就整份读入，
所以「读的时候一定有值」是参考的隐含契约；本仓把这个字段搬进了 `usage.json`（懒加载），
就欠了「读入口自己负责加载」这一步。

### 同族（三处 `Store`）逐条核对：只有这一处漏

| 包 | 读入口是否先 load | 结论 |
|---|---|---|
| `internal/clipboard` | `Entries` `:402-405`、`Search` `:411-414`、`mutateLocked` `:373-374` 全部 `loadLocked` | 齐（P10-w 已收口） |
| `internal/calculator` | 无 `loadLocked`——`NewStore` 在构造时整份读入（P11-d 已记录该形状） | 齐（eager） |
| `internal/usage` | `Record`/`Seed`(非空)/`Count` 有；**`Top` 没有** | **本轮补** |

## 落地项

### 1. `Top` 先读盘

- 位置：`internal/usage/usage.go:104`。
- 现状：`s.mu.Lock()` 之后直接 `len(s.records)`。
- 应改成：`Lock` 之后、遍历之前 `s.loadLocked()`（与 `Count` 同形）。
  排名/过滤/`limit` 语义一字不动。

### 2. `Save` 先读盘：没读过盘的 store 不许覆盖文件

- 位置：`internal/usage/usage.go:134-142`。
- 现状：`s.records == nil` 时就 `s.records = map[string]Record{}` 然后落盘——
  `Open(dir).Save()` 会把已有 `usage.json` 覆盖成 `{}`（数据丢失；`Store` 没有任何
  「整份替换」的 API，所以不存在需要写空表的调用方）。
- 应改成：`Lock` 之后先 `s.loadLocked()`，再走原来的 marshal/原子替换。
  文件缺失/损坏时的结果与现在一致（空表 → 写 `{}`），所以这是纯粹的「少一条覆盖路径」。

### 3. pin（`internal/usage/usage_test.go`，3 条）

- `TestAColdReadSeesTheFileItDidNotLoad`：store A 写三条记录 → **新** `Open(dir)`（模拟冷启动）
  → 只调 `Top`（不调 `Count`/`Record`/`Seed`）→ 必须按次数降序返回三条。改前红。
- `TestTopLoadsWithoutACountFirst`：同上但用 `keep` 过滤，钉住「`Top` 的 `keep` 会被调用」。
  改前红（`keep` 一次都没被调）。
- `TestSaveWithoutAReadKeepsTheFile`：写盘 → 新 `Open(dir).Save()` → 文件内容仍是原来那份
  （不是 `{}`）。改前红。

### 4. pin（`internal/shell/recentsstartup_test.go`，1 条端到端）

- `TestAColdStartReadsTheRecentsFromDisk`：真实冷启动形状——settings 里**没有** `launch_counts`，
  `usage.json` 由**另一个 store** 写好，`scanApps` 后空查询页必须有「Recently launched」块与行，
  且顺序按次数。改前红（页上无该块）。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- `usage.json` 的格式、文件名、权限（0600）、原子替换：**零改动**。
- `Top` 的排序（次数降序，同数按 `Last` 降序）与 `refreshRecents` 的二次排序：**零改动**。
- `Record` 写失败**不回滚内存**——这是**故意**的，与 P11-c/P11-d 的 `mutateLocked` 不是同一族：
  本包文档明写「Nothing here is authoritative … a write failure never stops a launch」，
  一次失败的计数留在内存里正是本会话该看到的答案，下一次成功 `Save` 会把它提交。
  本轮不动它（写进报告「判不准项」，供拍板）。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2231（只许涨）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 基线 15
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数基线 **1802**（只许涨）。

## mutation 计划（≥2 条，实跑）

1. 删 `Top` 里的 `s.loadLocked()` → `TestAColdReadSeesTheFileItDidNotLoad` 与
   `TestTopLoadsWithoutACountFirst` 变红。
2. 删 `Save` 里的 `s.loadLocked()` → `TestSaveWithoutAReadKeepsTheFile` 变红。
3. （对照）把 `internal/shell/shell.go:1583` 的 `Top` 换成手写遍历 `a.Usage.Count` 之外的东西
   不改——本轮不改 shell 产品代码，第 3 条改成：把 `refreshRecents` 的 `Top` 调用挪到
   `Count` 之后仍绿（证明 shell 侧不需要改动，修的是 store 自己的契约）。
