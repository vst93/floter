# P11-c 规格：剪贴板的一次写入失败，不能留下任何痕迹

## 意图

参考的每一次历史改动都包在 `update_entries` 里——「**先改克隆，持久化成功之后才发布内存**」
（`main:src-tauri/src/clipboard_history/store.rs:233-241`，配单测
`failed_persistence_does_not_publish_a_mutated_cache`，`store.rs:403-420`）。本仓的
`internal/clipboard/clipboard.go` 反过来：`insertLocked` / `pruneLocked` / `SetFavorite` / `Remove`
/ `Clear` 都**直接改 `s.entries`**，然后才 `saveLocked()`，失败时只把 `error` 往上抛，内存里那条
根本没落盘的条目仍然在——面板看得见，重启就消失；更糟的是 `pruneLocked`
（`clipboard.go:690-712`）在 save **之前**就 `os.Remove` 掉了被淘汰条目的 PNG，`Remove`
（`:608-626`）与 `Clear`（`:628-647`）同样先删文件再 save：一次写失败之后，内存里还指着的
PNG 已经不在了，面板上的图变成坏图。

上一轮（P11-b）已经把监视器的记忆改成「写成功才记」（`.pending-bugs.md` P11-b「仍待」第 7 条
点名了本仓这条缺口），于是现在的可见症状是：写失败 → 监视器下一拍重试 → `AddText` 看到
`entries[0]` 已经是同一条（内存里那条幽灵），返回 `added=false, err=nil` → 监视器当成成功记下
哈希 → **这次复制永远不会再进历史**。所以本轮把「写失败 = 什么都没发生」这条契约补完整：
内存、磁盘、PNG 三者要么一起前进，要么都不动。

## 为什么现在做

- `.pending-bugs.md` P11-b「仍待」第 7 条（本轮直接取它）。
- 参考有明确实现与单测：`store.rs:233-241` `update_entries`、`store.rs:403-420`。
- 剪贴板族最后一条**不需要拍板**的行为差（间隔 600 vs 900 仍需拍板，本轮不动）。

## 落地项（动手前逐条读过当前代码，行号是当前 HEAD 的真实行号）

### 1. `Store` 多一个「待删文件」暂存：`internal/clipboard/clipboard.go:327-334`

现状：`Store` 只有 `paths / mu / entries / loaded / maxItems / now`，删 PNG 的动作散在
`pruneLocked`、`Remove`、`Clear` 三处，且都在 save 之前。

应改成：加 `pendingRemovals []string`（待删的图片文件名），删除动作统一推后到 save 成功之后。

### 2. 新增 `mutateLocked`：克隆 → 改克隆 → 存盘 → 发布

现状：无。七个写入口各自「先改内存再 `saveLocked`」。

应改成新增一个方法，语义与参考 `update_entries` 逐字对应：

```
before := slices.Clone(s.entries)      // Entry 是值类型，写入口只改字段、不原地改 extra
run the mutation
if save fails: s.entries = before; drop pendingRemovals; return err
if save ok:    delete the pending PNGs; return nil
```

`maxItems` 不参与回滚：容量是设置，`SetMaxItems` 的调用方（`internal/shell/shell.go:1141`）
本来就把它当内存容量用；索引没写成不影响这个数。

### 3. 七个写入口全部走 `mutateLocked`

| 位置 | 现状 | 应改成 |
|---|---|---|
| `SetMaxItems` `:347-355` | `pruneLocked()` 后 `saveLocked()` | `mutateLocked(func() error { pruneLocked(); return nil })` |
| `AddText` `:432-457`（`:453-455`） | `insertLocked` → `pruneLocked` → `saveLocked` | 同上；失败返回 `Entry{}, false, err` |
| `AddImageCaption` `:468-505` | 写 PNG → `insertLocked` → `pruneLocked` → `saveLocked` | 写 PNG 后走 `mutateLocked`；**save 失败时删掉刚写的 PNG**（参考 `monitor.rs:294-299`） |
| `addFilesEntry` `:531-549` | 同 `AddText` | 同上 |
| `SetFavorite` `:593-606` | 改字段 → `pruneLocked` → `saveLocked` | 同上；失败时收藏位必须还是旧值 |
| `Remove` `:608-626` | 切掉条目 + **立刻** `os.Remove` PNG | 切掉条目 + 把 PNG 记进 `pendingRemovals` |
| `Clear` `:628-647` | 同上 | 同上 |

### 4. `pruneLocked` `:690-712` 只登记、不删

现状：被淘汰条目的 PNG 在 save 之前就被 `os.Remove`。

应改成：把文件名 append 到 `pendingRemovals`，由 `mutateLocked` 在 save 成功后统一删。
这样「一次失败的 prune」不会让还活着的条目指向已删除的文件。

### 5. 行为契约零改动红线

`Add*` 的返回值三元组、去重/折叠规则（P10-x）、文件身份（P10-w）、搜索字段（P10-w）、
容量与保留期、`ErrNotFound`、`loadLocked` 对旧索引的修补、监视器与各调用方
（`internal/shell/system.go`、`internal/launcher/*`）的错误处理——**一个字节都不动**。
成功路径的磁盘结果（索引内容、PNG 位置、删除时机）与改前完全一致。

## 门槛命令 + PASS 基线（本轮实测，HEAD `2ce2148`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿，34 ok + 2 no test files
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2218
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 15
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
grep -rn "func Test" --include=*_test.go internal cmd | wc -l   # 1789
```

本轮是行为轮：PASS 只许涨，函数数只许涨，SKIP 不许涨。

## 新增 pin（台账 B 类，全部是行为/度量）

`internal/clipboard/writefail_test.go`（新文件）：

1. `TestFailedWritePublishesNothing`——索引不可写时 `AddText` 返回错误，`Entries()` 里没有它；
   解除阻塞后同一次复制能落盘（= 参考 `failed_persistence_does_not_publish_a_mutated_cache`）。
2. `TestFailedWriteKeepsTheLoadedHistory`——已有一条历史，写失败后历史**逐字段等于**改前。
3. `TestFailedImageWriteLeavesNoFileBehind`——索引不可写时 `AddImageCaption` 失败，
   `images/` 下不留 PNG。
4. `TestFailedPruneKeepsTheDroppedImages`——容量 10、写满 11 张图，第 11 次写失败：
   被淘汰那张的 PNG 仍在盘上、第 11 张的 PNG 不在；解除阻塞后重跑一次，两张的状态反过来。
5. `TestFailedMutationsKeepTheOldState`——`SetFavorite` / `Remove` / `Clear` 在索引不可写时
   全部返回错误，收藏位、条目、PNG 都是改前的样子。

台账：**V=0 / B=5 / S=0**（本轮无视觉改动，不出图）。

## mutation 计划（每条实跑，看它真的红，再 `md5sum -c` 还原 + `touch` 复跑）

- **M1**：`mutateLocked` 里删掉 `s.entries = before`（失败也不回滚）→ 预期 pin 1、2、5 红。
- **M2**：把 `pendingRemovals` 的删除挪到 `saveLocked()` **之前** → 预期 pin 4 红。
- **M3**：删掉 `AddImageCaption` 失败路径里「删掉刚写的 PNG」→ 预期 pin 3 红。
- **M4**：`SetFavorite` 绕开 `mutateLocked`（改完直接 `saveLocked`）→ 预期 pin 5 红。

## 不做（写进 `.pending-bugs.md`）

- 轮询间隔 600ms vs 900ms（需拍板）。
- 「复制回来的条目该不该置顶」（需拍板）。
- P11-b「仍待」第 6 条：关开关的 `return` 改成 `continue` 抓不到——属另一个可观测会话计数，另一轮。
