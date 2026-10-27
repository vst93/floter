# P11-d 报告：计算器历史的写失败，也不能留下任何痕迹

方向：优先级 3（上一轮报告「仍待」的第一条可做项）。规格 `docs/rounds/P11-d-spec.md`。

## 1. 意图与证据

参考的每一次历史改动都包在 `update` 里（`main:src-tauri/src/calculator_history.rs:169-186`）：
`lock → load_index() → f(&mut entries) → prune_entries → save_index(&kept)?`。`save_index` 失败时
函数直接返回 `Err`，而参考**没有内存缓存**——它每次都从磁盘读，所以「写失败」在参考里天然等于
「什么都没发生」。

本仓的 `internal/calculator.Store` 有缓存，且改前六个写入口全是「先改 `s.entries`，再 `saveLocked()`」：

| 入口（改前行号） | 改前行为 |
|---|---|
| `Add` :230-260 | 先 `s.entries = append(...)` + `pruneLocked()`，失败仍把新 `Entry` 交给调用方 |
| `SetFavorite` :264-281 | 直接改 `s.entries[i].Favorite` |
| `ToggleFavorite` :283-299 | 直接改并把改后的值返回 |
| `Delete` :301-320 | 先 `s.entries = kept` |
| `Clear` :322-334 | 先 `s.entries = kept` |
| `SetRetention` :336-344 | 先改字段 + `pruneLocked()` |

于是两个可见后果：写失败后面板里那条计算**看得见、重启就没了**；`SetRetention` 更狠——写失败时
内存里已经按新容量裁过，磁盘上旧索引还在，**下一次成功写入会把这次从没提交过的裁剪永久落盘**，
旧行就这么被删掉。

全仓只有两个 `saveLocked`（`internal/clipboard/clipboard.go:785`、`internal/calculator/calculator.go:445`），
剪贴板那一面已由 P11-c 收口。本轮补完另一半。

## 2. 落地

### 2.1 `internal/calculator/calculator.go`

- 新增 `mutateLocked(mutate func() error) error`（`saveLocked` 之前）：
  `before := slices.Clone(s.entries)` → 改 → `saveLocked()`；**mutate 失败与 save 失败都 `s.entries = before`**。
  注释指名剪贴板版（P11-c）与参考 `update`。
- 六个写入口全部走它：`Add` / `SetFavorite` / `ToggleFavorite` / `Delete` / `Clear` / `SetRetention`。
- 失败时的返回值：`Add` → `Entry{}, err`；`ToggleFavorite` → `false, err`。**没发布的值不外泄**。
- `SetRetention` 的取舍：**retention 字段不参与回滚**（它是设置页已经写成功的值，`shell.go:1126`
  只是在 `OnChange` 里推过来），回滚的只有 `entries`——即那次从没提交的裁剪。这与 P11-c 里
  `maxItems` 不参与回滚同一条判据。
- 新增 import `slices`。

### 2.2 为什么**不**抽公共 helper

两处没有共享写路径（各自的 `Paths`、各自的 `saveLocked`、各自的目录与恢复策略），剪贴板版的回滚
载荷还带 `pendingRemovals` 与「提交后才删 PNG」。抽出来的公共件只会是「泛型壳 + 两套回调」，
比两份二十行的函数更难读。判据是**语义**一致、**代码**各写一份，两边注释互相指名
（`calculator.go` 的 `mutateLocked` 注释 ↔ `internal/clipboard/writefail_test.go` 顶部）。

### 2.3 新增 pin：`internal/calculator/writefail_test.go`（4 个测试函数）

| 测试 | 钉住什么 |
|---|---|
| `TestFailedAddPublishesNothing` | 失败的 `Add` 不发布（内存 1 条、磁盘 1 条）、不返回条目；解锁后重试能落盘且不叠影子 |
| `TestFailedFavoriteKeepsTheOldStar` | 失败的 `SetFavorite` 不改旗标；失败的 `ToggleFavorite` 既不返回它想发布的值、也不改内存；磁盘旗标不变；重试成功 |
| `TestFailedDeleteAndClearKeepTheRows` | 失败的 `Delete` / `Clear` 不丢行；磁盘不被裁；重试只生效一次 |
| `TestFailedRetentionDoesNotCommitThePrune` | 12 行 + `SetRetention(10, 0)` 写失败 → 内存 12 行、磁盘 12 行、`MaxItems()` 已是 10；下一次成功写入才按新容量裁到 10 |

`blockIndex` 的手法与剪贴板版一致：把 `index.json` 换成**非空目录**，让 `saveLocked` 的最后一步
`os.Rename` 失败（不用权限位——测试可能以 root 跑）。与剪贴板版的唯一差别：它会**带回原索引字节**，
`unblockIndex` 再写回去，这样 pin 才能读到「失败之后磁盘上是什么」。

## 3. 台账

**V=0 / B=4 / S=0**（纯行为轮，不出图）。四条 pin 全是行为类，**改前全红**（见 §5）。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
go test -count=1 ./...                   → 全绿（EXIT=0）
--- PASS                                 → 2227（基线 2223，+4）
--- SKIP                                 → 15（=）
--- FAIL                                 → 0（=）
ok 行                                    → 34 + 2 no test files（=）
测试函数                                 → 1798（基线 1794，+4）
GOOS=linux/windows/darwin go build ./... → 三个都过
```

## 5. 变异（4 条实跑，全部变红并逐字节还原）

每条：注入 → 跑 `-run TestFailed` → 抄原始报错 → `cp` 还原 → `md5sum -c` 逐字节核对 →
最后 `touch` 两个文件后复跑一次（避开测试缓存）。

| # | 注入的缺陷 | 变红的 pin（原始报错串） |
|---|---|---|
| M1 | `mutateLocked` 的 save 失败分支删掉 `s.entries = before` | 4 条全红：`a failed write published 2 entries: [...]`、`a failed un-star published the new flag` + `a failed toggle published the flipped flag`、`a failed Delete dropped a row: [...]`、`a failed SetRetention published its prune: [...]` |
| M2 | `SetRetention` 绕开 `mutateLocked`，改回「先剪枝再 save」 | `TestFailedRetentionDoesNotCommitThePrune`：`a failed SetRetention published its prune: [...]` |
| M3 | `ToggleFavorite` 失败时返回它想发布的值（`return favorite, err`） | `TestFailedFavoriteKeepsTheOldStar`：`a failed ToggleFavorite returned the value it wanted to publish` |
| M4 | `Add` 失败时把构造好的 `Entry` 交回去（`return entry, err`） | `TestFailedAddPublishesNothing`：`a failed Add returned an entry that was never published: {ID:...}` |

M1…M4 每条之后 `md5sum -c /tmp/p11d-calculator.md5` → `internal/calculator/calculator.go: 成功`。

**改前红验证**：把 `calculator.go` 换回 `HEAD` 版、只留新 pin 跑一遍——4 条全红
（`a failed Add returned an entry that was never published`、`a failed un-star published the new flag`、
`a failed ToggleFavorite returned the value it wanted to publish`、`a failed Delete dropped a row`、
`a failed SetRetention published its prune`），随后 `md5sum -c` 确认换回的是修好的那份。

M3 抓到过一次**pin 太弱**：第一版只对「已收藏」的行做失败 toggle，翻过来的值是 `false`，
所以「返回了没发布的值」这个缺陷抓不到。改成对一条**未收藏**的行 toggle（翻成 `true`）后才变红——
这也是本轮唯一一次改测试，原因是断言方向不全，不是放宽断言。

## 6. 需用户拍板（本轮无新增）

P11-b 表里的「剪贴板轮询间隔 600 vs 900」不变，本轮未动。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 59 轮。
2. 需拍板不变（下文 P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`、register 只留 `cmd`、`open` 是否拒一切 query、
   未知宿主是否静默忽略、剪贴板轮询间隔、复制回来的条目是否置顶。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
5. P11-b「仍待」第 6 条仍开着：关开关的分支从 `return` 改成 `continue` 抓不到（需要「一个采样器」
   的可观测会话计数）。
6. **本轮发现、未做**：参考的计算器**每次写都从磁盘重新读**（`update` 里的 `load_index()`），本仓
   `Store` 只在 `NewStore` 时读一次。于是「外部改了 `index.json`（手改 / 另一进程 / 上一个实例
   还没退出）」在本仓会被下一次写入整体覆盖。剪贴板那边 P10-w 已经用 `loadLocked` 处理过同一问题；
   计算器要不要也刷新是一个**语义选择**（会改变「内存里那份是权威」这条隐含假设），需要拍板。
7. 同族小差（不动）：`internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 是自有引号契约，非缺陷。
