# P11-c 报告：一次写失败，内存、索引、PNG 三者都不动

规格 `docs/rounds/P11-c-spec.md`。方向来源：`.pending-bugs.md` P11-b「仍待」第 7 条
（剪贴板 `Store` 的写失败没有回滚）。台账 **V=0 / B=5 / S=0**（纯行为轮，不出图）。

## 1. 意图与证据

参考的每一次历史改动都包在 `update_entries` 里——「先改克隆，持久化成功之后才发布内存」
（`main:src-tauri/src/clipboard_history/store.rs:233-241`，配单测
`failed_persistence_does_not_publish_a_mutated_cache`，`store.rs:403-420`），图像捕获在 save 失败时
还会把刚写的 PNG 收回去（`monitor.rs:294-299`），prune 只在 save **成功之后**才删被淘汰条目的
PNG（`monitor.rs:309-322`）。本仓改造前恰好三条都反着：

- `internal/clipboard/clipboard.go`（改前）`AddText:453-455` / `AddImageCaption:501-503` /
  `addFilesEntry:546-548` / `SetFavorite:598-601` / `Remove:617-622` / `Clear:640-645`：
  先改 `s.entries`，再 `saveLocked()`，失败只把 error 往上抛——**幽灵条目**留在面板里，重启消失。
- `pruneLocked`（改前 `:690-712`）在 save **之前** `os.Remove` 掉被淘汰条目的 PNG；
  `Remove`/`Clear` 同样先删文件再 save。一次写失败之后，**还活着的条目指向已删除的图**。
- 与 P11-b 合起来的可见后果：写失败 → 监视器下一拍重试 → `AddText` 看到 `entries[0]` 就是那条
  幽灵（同哈希）→ 返回 `added=false, err=nil` → 监视器当成成功记下哈希 → **这次复制永远进不了历史**。

## 2. 落地

全部在 `internal/clipboard/clipboard.go`：

| 位置 | 改法 |
|---|---|
| `Store`（`:327-341`） | 加 `pendingRemovals []string`：一次未提交的改动只**登记**要删的图 |
| 新增 `mutateLocked`（`:356-392`） | `slices.Clone` 快照 → 跑改动 → `saveLocked()`；失败把 `entries` 换回快照、丢掉待删清单；成功才 `dropPendingImagesLocked` |
| `SetMaxItems` / `AddText` / `AddImageCaption` / `addFilesEntry` / `SetFavorite` / `Remove` / `Clear` | 七个写入口全部走 `mutateLocked`；`Add*` 失败返回 `Entry{}, false, err` |
| `AddImageCaption` 失败路径 | save 失败时 `os.Remove` 刚写的那张 PNG（参考 `monitor.rs:294-299`） |
| `pruneLocked` / `Remove` / `Clear` | 只把文件名 append 到 `pendingRemovals`，删除推迟到提交之后 |

`maxItems` 不参与回滚：容量是设置（`internal/shell/shell.go:1141` 把设置值推给 store），
索引没写成不影响这个数——这一点写进了 `mutateLocked` 的注释。

**行为契约零改动**：`Add*` 三元组签名、折叠/去重规则（P10-x）、文件身份（P10-w）、搜索字段、
容量与保留期、`ErrNotFound`、`loadLocked` 对旧索引的修补、监视器与
`internal/launcher`、`internal/shell` 的调用面——全部未动。成功路径的磁盘结果与改前逐字节一致
（索引内容、PNG 位置、删除时机）。

## 3. 新增 pin（`internal/clipboard/writefail_test.go`，B 类 5 条）

`blockIndex` 用「把 `index.json` 换成非空目录」让 `saveLocked` 的最后一步 rename 失败——
不用权限位，因为测试可能以 root 跑（root 会绕过 `0555`），而目录 rename 在所有平台都会被拒。
这一步发生在临时文件写完**之后**，正是磁盘满 / 只读盘的真实形态。

| pin | 钉住的行为 |
|---|---|
| `TestFailedWritePublishesNothing` | 写失败的 `AddText` 不在历史里；解除阻塞后同一次复制能落盘（新开 store 从磁盘读得到） |
| `TestFailedWriteKeepsTheLoadedHistory` | 已加载的历史（含收藏位）在三种 `Add*` 全部失败后逐字段不变 |
| `TestFailedImageWriteLeavesNoFileBehind` | 写失败时 `images/` 下不留 PNG |
| `TestFailedPruneKeepsTheDroppedImages` | 容量 10 写满后第 11 张失败：被淘汰那张的 PNG **仍在**、新写的那张 **不在**、历史仍是 10 条；解除阻塞后重跑一次，两张的状态反过来（提交后删除真的发生） |
| `TestFailedMutationsKeepTheOldState` | `SetFavorite`/`Remove`/`Clear` 失败时历史与 PNG 都不动；解除阻塞后同样的三个操作照常生效 |

**改前红验证**（不是变异，是把产品文件换回 HEAD 版跑新 pin）：5 条全红，报错串例如
`a failed write published 1 entries: [{ID:726e… Kind:text Text:first …}]`、
`a failed write deleted the image of an entry it did not drop: stat …/images/9a6c….png: no such file or directory`。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 34 ok + 2 no test files，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2223   （基线 2218，+5）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 15     （= 基线）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
grep -rn "func Test" --include=*_test.go internal cmd | wc -l   # 1794   （基线 1789，+5）
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

## 5. 变异（4 条实跑，全部变红，`md5sum -c` 逐字节还原 + `touch` 复跑绿）

基线 `internal/clipboard/clipboard.go` = `614daa7cfa6fd14b8f4eb176966d9673`。

| # | 注入的缺陷 | 变红的 pin（原始报错串） |
|---|---|---|
| M1 | 删掉 `mutateLocked` save 失败分支里的 `s.entries = before` | 5 条全红；`a failed write published 1 entries: [{… Text:first …}]`、`a failed prune changed the history: 10 entries`、`a failed mutation changed the history: before [… Favorite:false] after [… Favorite:true]` |
| M2 | 把 `dropPendingImagesLocked()` 挪到 `saveLocked()` **之前** | 2 条红；`a failed write deleted the image of an entry it did not drop: stat …/images/cb23….png: no such file or directory`、`images on disk = 9, want 10`、`a failed mutation deleted the image file: …` |
| M3 | 删掉 `AddImageCaption` 失败路径的 `os.Remove(刚写的 PNG)` | 2 条红；`a failed image write left 1 file(s) behind: [- e107….png]`、`images on disk = 11, want 10` |
| M4 | `SetFavorite` 绕开 `mutateLocked`（改完直接 `saveLocked`） | 1 条红；`a failed mutation changed the history: before [… Favorite:false] after [… Favorite:true]` |

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 58 轮。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：#692 日语 locale、#642 `MYGO_LDFLAGS`、
   `ShortcutTargetName` 无扩展名返回 `""`、register 只留 `cmd`、`open` 是否拒一切 query、
   未知宿主是否静默忽略；外加剪贴板轮询间隔 600ms vs 900ms、「复制回来的条目该不该置顶」。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
5. P11-b「仍待」第 6 条仍开着：把监视器关开关的分支从 `return` 改成 `continue` 没有 pin 能抓到
   （需要「一个采样器」的可观测会话计数）。本轮没碰，属另一轮。
6. 本轮发现、未做：`internal/calculator` 的 `Store`（`calculator.go:264` `SetFavorite` 等）是
   同一形状的「先改内存再 save」。它不归剪贴板族，且没有任何 pin 覆盖写失败；要做就是一整轮，
   先侦察再定。
