# P11-l 报告 · 同一族第 4–7 份拷贝收口（+ prune 平局取舍）

规格 `docs/rounds/P11-l-spec.md`。方向来自选题规则第 3 条（上一轮「仍待」第 3 条：
`internal/statefile` 已建，但同族还有第 4–7 份手写拷贝）。用户实测四条仍卡实机、
合约 §6 全 `已落地`，按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，落在
**基础功能 / 稳定性**（无 GOOS 分支，Linux CI 钉住三平台同一答案）。

## 1. 意图

P11-k 把状态文件落盘收成一个实现 `internal/statefile.Write`，但同一段
`temp → write → fsync → chmod → rename → dir fsync` 还手写在四处（calculator、
extensions config ×2、extensions repository、terminalui）。它们**都无缺陷**（全部
fsync 且 rename），是纯冗余。本轮把它们全部转发到 `statefile.Write`，并补一条
**源码扫描 pin**，让「有人偷偷手写第 8 份」当场变红。

## 2. 落地（行号本轮实测）

| 文件 | 现状（改前） | 改成 |
|---|---|---|
| `internal/calculator/calculator.go:501` `saveLocked` | 手写 temp 块，`nil→[]Entry{}` 归一，`\n` 后缀，模式 0600 | `statefile.Write(s.paths.Index(), append(data,'\n'), 0o600)`；`nil→[]Entry{}` **保留**（写出 `[]` 而非 `null`，逐字节不变） |
| `internal/extensions/config.go:135` `SaveStoredConfiguration` 内联块 | 手写 temp 块，`\n` 后缀，模式 0600 | `statefile.Write(filepath.Join(dir,configFile), append(data,'\n'), 0o600)` |
| `internal/extensions/config.go:663` `atomicWriteFile` | 手写 temp 块，无 `\n`，模式 0600 | `statefile.Write(path, data, 0o600)` |
| `internal/extensions/repository.go:195` `SaveRepository` | 手写 temp 块，模式 **0644** | `statefile.Write(path, data, 0o644)`（模式照旧） |
| `internal/terminalui/terminalui.go:518` `writeFileAtomically` | 手写 temp 块，模式 0600 | `statefile.Write(path, data, 0o600)` |

净减 **140 行**、增 57 行（`git diff --stat`）。

**新增 `internal/ci/statefile_contract_test.go`** `TestStateFilesHaveOneWriter`：
走 `internal/` 与 `cmd/` 的 `.go` 文件（跳过 `internal/statefile` 本身与 `_test.go`），
除白名单 `internal/apps/icns.go`（`sips` 转换的 OS 临时 PNG，读回即删，不是持久状态
文件）外，任何 `os.CreateTemp` 都判红，并提示改用 `statefile.Write`。这就是 P11-k
报告 §6「灵敏度边界」缺的那条契约测试。

## 3. V / B / S 台账

- **V = 0**（零视觉）。
- **B = 0 改动**：落盘字节逐字不变——calculator 的 `[]`/`\n`、config 的 `\n`、
  repository 的 0644、terminalui 的二进制快照、所有 0600/0644 模式全部照旧；写失败
  仍返回 error 交给既有回滚（P11-c/P11-d/calculator `mutateLocked` 一行未动）。
- **S = 0**：没有测试钉临时文件前缀（calculator/extensions/terminalui 的既有 pin 只
  读最终结果），所以零断言改动。

## 4. 门槛实测（本轮全跑）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
go test -count=1 ./...       → 全绿，FAIL 0
--- PASS 计数                → 2264（基线 2262，+2）
--- SKIP                     → 15（无新增）
--- FAIL                     → 0
测试函数（grep ^func Test|Benchmark|Fuzz）→ 1840（本轮起点 1838，+2）
GOOS=linux|windows|darwin go build ./... → 三平台过
```

`internal/calculator` `internal/extensions` `internal/terminalui` `internal/ci`
四个包连跑 5 次全 `ok`（稳定性）。

## 5. mutation（4 条实跑变红 + 逐字节还原）

1. **`statefile.Write` 直接 `return nil`（不写任何字节）**：证明调用点真的走这个实现。
   ```
   --- FAIL: TestPersistenceAndRecovery
       calculator_test.go:255: the index was not written: stat .../calculator-history/index.json: no such file or directory
   --- FAIL: TestSnapshotSaveAndRestore
       view_test.go:338: stat .../terminal-snapshot: no such file or directory
   --- FAIL: TestSaveConfigurationSplitsSecrets
   ```
   calculator / terminalui / extensions 三包同时红。还原：`cp` 备份 → `md5sum -c` 成功 → `touch` 复跑 3 包 `ok`。
2. **删掉 `statefile.Write` 里的 `syncFile(temporary)`**（flush 顺序）：
   ```
   --- FAIL: TestASyncFailureLeavesTheTargetUntouched
       statefile_test.go:115: Write error = <nil>, want the refused flush
   ```
   还原同上（md5 成功，`touch` 后 `ok`）。
3. **把 `os.CreateTemp` 注入 `calculator.go` 的 `saveLocked`**（模拟有人手写第 8 份）：
   ```
   --- FAIL: TestStateFilesHaveOneWriter
       statefile_contract_test.go:70: internal/calculator/calculator.go calls os.CreateTemp: state files must go through statefile.Write ...
   ```
   还原同上（md5 成功，`touch` 后 `ok`）。
4. **把 prune 平局 fix 退回旧比较器**（见 §6）：新 pin 变红。
   ```
   --- FAIL: TestPruneTieBreakDropsTheOldest
       calculator_test.go:392: capacity prune kept [j i h g f e d c b a], want [l k j i h g f e d c] (the newest, not the oldest, must survive a tie)
   ```
   还原同上（md5 成功，`touch` 后 `ok`）。

## 6. 附带修复：prune 的平局取舍（这是本轮**发现的真 bug**，不是收口）

跑门槛时 `TestFailedRetentionDoesNotCommitThePrune` 间歇性失败。实测**在改动前的
P11-k 代码上 `-count=1` 连跑 10 次就红 4 次**（`pass=6 fail=4`），所以它是**预存
的 flaky**，与本轮收口无关——本轮只是撞上了。

根因是产品 bug，不是测试写错：`pruneLocked`（`calculator.go:402`）按 `CreatedAt`
升序 `sort.SliceStable` 后丢前 `excess` 个，注释写的是「drops the oldest」。但行列表
是**新在前**，当多个 `CreatedAt` 相等时（冻结时钟、或一毫秒内连加），稳定排序保留
索引升序 → 被丢的是**最前面（最新）**的行，与契约正好相反。旧代码只因为真实场景下
每次 `Add` 的 fsync 让时间戳错开才「看起来正常」。

修法（`calculator.go:402`）：`CreatedAt` 不等时按时间升序；相等时按索引**降序**
（列表越靠后＝加得越早＝越旧），先丢旧的。新增 pin `TestPruneTieBreakDropsTheOldest`
（冻结时钟、容量 MinMaxItems、加 12 行，断言存活的是最新 10 行 `l..c`）。

台账：**B = 1**（行为修正，新 pin 钉住；正常场景时间戳不同，行为零变化）。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 67 轮，
   本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 +
   计算器 `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS
   快捷键词序 + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **状态文件落盘这一族彻底收口**：`internal/` 里除 `statefile` 外不再有 `os.CreateTemp`
   写状态文件，且有 `TestStateFilesHaveOneWriter` 守着；**别在里面再找小修**。
4. 本轮顺带发现并修掉 calculator 的 prune 平局 bug；`internal/calculator` 的写失败回滚
   与 prune 平局都已有 pin，**别再当候选重领**。
5. 下一处候选：`internal/calc`（875 行）与 `internal/calculator`（1204 行）两个包的边界
   是否只有一个答案（P11-g 已收口规范形，剩「每次写要不要从磁盘重读」需拍板）。
