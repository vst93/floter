# P11-d 规格：计算器历史的写失败，也不能留下任何痕迹

## 意图

`internal/calculator.Store` 与剪贴板 `Store` 是**同一形状**：内存里一份 `entries`，每次写直接改它，
然后 `saveLocked()`，失败只把 `error` 抛上去。参考的每一次改动都包在 `update` 里——**每次从磁盘
重新读、改、存，失败就返回 `Err`，调用方手里的列表仍是上一次成功的那份**
（`main:src-tauri/src/calculator_history.rs:169-186`）。本仓于是有同一个症状：一次写失败之后，
面板里能看到那条计算，重启就消失；`SetRetention` 的剪枝更狠——写失败时内存里已经被裁掉的旧行
在磁盘上还在，下一次成功写入会把它们**永久删掉**（裁剪被当成已提交）。

上一轮 P11-c 已经把剪贴板这一面收口（`mutateLocked`：克隆 → 改 → 存 → 成功才发布）。
本轮把同一契约补到计算器：**内存与磁盘要么一起前进，要么都不动**。这是「同一个问题只留一个答案」
的最后一半——全仓只有两个 `saveLocked`（`internal/clipboard/clipboard.go:785`、
`internal/calculator/calculator.go:445`），做完就没有第三处了。

## 为什么现在做

- `.pending-bugs.md` P11-c「仍待」第 6 条直接点名（`calculator.go:264` `SetFavorite` 等先改内存再 save，
  且**零 pin** 覆盖写失败）。
- 参考有实现可对照：`calculator_history.rs:169-186` `update`（lock → `load_index()` → `f(&mut entries)`
  → `prune_entries` → `save_index(&kept)?`；`save_index` 失败时函数返回 `Err`，内存里没有副本可污染）。
- 优先级：上一轮报告「仍待」的第一条可做项（前一条是用户实机复测，本轮做不到）。

## 落地项（动手前逐条读过当前代码，行号是当前 HEAD 的真实行号）

### 1. 新增 `mutateLocked`：`internal/calculator/calculator.go`（`saveLocked` 之前，约 :445）

现状：无。六个写入口（`Add:230` / `SetFavorite:264` / `ToggleFavorite:283` / `Delete:301` /
`Clear:322` / `SetRetention:336`）各自「先改 `s.entries`（或先改 retention 字段）再 `saveLocked()`」。

应改成：

```go
func (s *Store) mutateLocked(mutate func() error) error {
	before := slices.Clone(s.entries)
	if err := mutate(); err != nil {
		s.entries = before
		return err
	}
	if err := s.saveLocked(); err != nil {
		s.entries = before
		return err
	}
	return nil
}
```

与剪贴板版的差别，以及为什么**不抽公共 helper**：两处没有共享写路径（各自的 `Paths`、各自的
`saveLocked`），而剪贴板版的回滚载荷还带 `pendingRemovals` 与「提交后才删 PNG」；抽出来的公共
helper 只会是一层泛型壳 + 两套回调，读起来比两份二十行的函数更难。两边**语义**由 pin 对齐
（各写一份 + 互相指名的注释），不共享代码。

### 2. 六个写入口全部走 `mutateLocked`

| 入口 | 现状（真实行号） | 应改成 |
|---|---|---|
| `Add` :230-260 | 先 `s.entries = append(...)` + `pruneLocked()`，再 `saveLocked()`；失败仍返回新 `Entry` | 折叠 + 插入 + 剪枝放进闭包；失败返回 `Entry{}, err` |
| `SetFavorite` :264-281 | 直接改 `s.entries[i].Favorite`，`ErrNoEntry` 在改之前返回 | 闭包内查找并改；`ErrNoEntry` 从闭包返回（回滚无副作用） |
| `ToggleFavorite` :283-299 | 直接改并读回 `favorite`，再 `pruneLocked()` + save | 闭包内改；失败返回 `false, err`（没发布的值不外泄） |
| `Delete` :301-320 | 先 `s.entries = kept` 再剪枝 + save | 闭包内重建切片 |
| `Clear` :322-334 | 先 `s.entries = kept` 再 save | 闭包内重建切片 |
| `SetRetention` :336-344 | 先改 `maxItems`/`retentionDays` 再剪枝 + save | 闭包内改字段 + 剪枝 |

`SetRetention` 的取舍：**retention 字段不参与回滚**。容量与窗口是设置页的值，设置文件早已写成功
（`shell.go:1126` 只是在 `OnChange` 里把新值推给 store）；写索引失败时回滚的是 `entries`——
磁盘上那份旧索引必须仍然是内存里那份，否则下次成功写入会把「从没提交过的裁剪」落盘。
这与 P11-c 里 `maxItems` 不参与回滚同一条判据。

### 3. 零行为改动的红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**都不动**。
- 剪枝规则（按 `created_at` 而非位置裁、收藏两轴豁免）、折叠规则（同 expression 提升到顶并带过收藏）、
  错误值（`ErrNoEntry` / `errEmptyExpression` 的文案与类型）**逐字不动**。
- `Add` 失败时的返回值从「新 Entry + err」变成「零 Entry + err」：调用方 `launcher/calculator.go:212`
  只看 `err`（真分支），无行为变化。

## 门槛命令 + PASS 基线（本轮实测，不用旧报告的数字）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux/windows/darwin go build ./...               # 三个都过
```

基线（本轮动手前实测）：`--- PASS` **2223**、`--- SKIP` **15**、`--- FAIL` **0**、测试函数 **1794**、
`ok` 行 34 + 2 no test files。目标：PASS 2223 + 新增 pin 数，SKIP 与 FAIL 不变，函数数 +新增数。

## mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

- **M1**：`mutateLocked` 删掉 `saveLocked` 失败后的 `s.entries = before`（只留 mutate 失败的还原）
  → 「写失败不发布」系列（`Add`/`SetFavorite`/`Delete`/`Clear`）全红。
- **M2**：`SetRetention` 绕开 `mutateLocked`，改回「先剪枝再 save」→ 「裁剪没提交就不算数」pin 红。
- **M3**：`ToggleFavorite` 在闭包外先把 `favorite` 读出来（用改后的值返回）→ 「失败不外泄值」pin 红。
- **M4**：`Add` 把闭包里的折叠/插入改回直接操作 `s.entries`（绕过 `mutateLocked`）→ `Add` 的写失败 pin 红。

每条实跑：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑确认仍绿。

## 台账预判

**V=0 / B=5 / S=0**（纯行为轮，不出图）。新增 pin 全部是行为类，写在
`internal/calculator/writefail_test.go`（新文件，与 `internal/clipboard/writefail_test.go` 同形）。
