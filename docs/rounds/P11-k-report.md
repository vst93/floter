# P11-k 报告 · 状态文件落盘只有一个写法（补上缺的 flush 与 rename）

规格 `docs/rounds/P11-k-spec.md`。方向来自选题规则第 4 条（用户实测四条全卡实机复测、
合约 §6 全 `已落地`、上一轮「仍待」第 1 条同样卡实机），按用户定的
「三平台一致性 → 基础功能 → 像素细节」自查，落在**基础功能 / 稳定性**：无 GOOS 分支，
Linux CI 上钉住三平台同一答案。

## 1. 意图

应用自己的状态文件（设置与恢复副本、常用应用、剪贴板索引与其 PNG、窗口状态）落盘时必须
走**同一条** `temp → write → fsync → chmod → rename → dir fsync`；改前同一段十几行被抄了
七份，并且抄歪了两处。

## 2. 病灶（行号本轮实测，`grep -n "os.CreateTemp\|os.Rename\|os.WriteFile" internal/`）

| 位置 | 现状 | 缺什么 |
|---|---|---|
| `internal/settings/write.go:14` `writeFileAtomic` | temp+Write+**Sync**+Close+Chmod+Rename+dir Sync | —（最完整的一份） |
| `internal/usage/usage.go:156` `saveLocked` | temp+Write+**无 Sync**+Close+Chmod+Rename | **从不 flush**：rename 可能先于字节落盘 → 断电后「文件在、内容空」。注释还写着「writes the index atomically」 |
| `internal/clipboard/clipboard.go:534` 图片落盘 | `os.WriteFile(…/x.png)` | **不 rename**：崩在 truncate 与 write 之间 → 索引里一条永远解不开的 PNG |
| `internal/settings/windowstate.go:106,109` | 两次 `os.WriteFile` | **不 rename**：截断的 `window-state.json` → 下次启动读默认值（位置丢了） |
| `internal/clipboard/clipboard.go:786` `saveLocked` | temp+Sync+Chmod+Rename | 只差 dir Sync（并入统一实现） |
| `internal/calculator/calculator.go:495`、`extensions/config.go:104,681`、`extensions/repository.go:195`、`terminalui/terminalui.go:518` | 各自一份同族实现 | 无缺陷（都 fsync 且都 rename），是第 4–7 份拷贝；**本轮不改**（见 §6） |

## 3. 落地

- **新增 `internal/statefile`**（唯一实现）：`Write(path, data, mode)` = `MkdirAll` →
  `CreateTemp(".floter-state-*.tmp")` → `Write` → `syncFile` → `Close` → `Chmod(mode)` →
  `renameFile` → 目录 `Sync`（Windows 无目录 fsync，失败忽略，与 `settings/write.go` 原注释同一判据）。
  两个包级接缝 `syncFile` / `renameFile` 让「先 flush 再 rename」与「rename 被拒时目标不动」
  成为可断言的。
- `internal/settings/write.go`：`writeFileAtomic` 的函数体 → `statefile.Write`（两个调用点
  `settings.go:484,487` 与 `write_unix_test.go:29` 不动；`Save` 的「先 backup 后 primary、
  两份 0600」契约原样）。
- `internal/usage/usage.go`：`saveLocked` 整段 temp 块 → `statefile.Write(s.path, append(data,'\n'), 0o600)`
  ——**补上 fsync**（这是本轮唯一的「静默数据丢失」级修复）。
- `internal/clipboard/clipboard.go`：`saveLocked` → 同一实现；图片落盘 `os.WriteFile` →
  `statefile.Write(…, 0o600)` ——**补上 rename**。
- `internal/settings/windowstate.go`：两次 `os.WriteFile` → `statefile.Write(backupPath, data, 0o600)`
  + `statefile.Write(newPath, encoded, 0o644)` ——**补上 rename**；两份文件模式与顺序不动。

## 4. V / B / S 台账

- **V = 0**（零视觉）。
- **B = 0 改动**：落盘内容逐字节不变（`usage`/`clipboard` 末尾那一个 `\n`、`windowstate`
  两份文件的内容与先后顺序、所有模式 0600/0644 全部照旧）；写失败仍返回 error 并交给既有
  回滚（P11-c / P11-d / settings `Update` 的 rollback 一行未动）。
- **S = 1**：`internal/settings/write_test.go:59` 的「不留临时文件」断言钉的是临时文件前缀，
  前缀从 `.settings-*.tmp` 换成 `.floter-state-*.tmp`（同一个写入器服务所有状态文件，名字
  不再按包分）。断言的内容不变（写完目录里不许留下写入中的文件），注释已写
  `P11-k 重定基线（S：结构变了）`。

## 5. 门槛实测

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
go test -count=1 ./...       → 34 ok + 2 no test files，全绿
--- PASS 计数                → 2262（基线 2257，+5）
--- SKIP                     → 15（无新增）
--- FAIL                     → 0
测试函数                     → 1833（基线 1828，+5）
GOOS=linux|windows|darwin go build ./... → 三平台过
```

新增 5 条 pin 都在 `internal/statefile/statefile_test.go`：新字节+模式+无残留临时文件、
缺目录自动创建、字节逐字不变（空文档与尾随换行）、flush 失败目标不动且无残留、rename
失败目标不动且无残留。

## 6. mutation（2 条实跑变红 + 逐字节还原）

1. **删掉 `statefile.Write` 里的 `syncFile(temporary)` 那一步**（`perl -0pi` 删 4 行）：
   ```
   --- FAIL: TestASyncFailureLeavesTheTargetUntouched (0.00s)
       statefile_test.go:115: Write error = <nil>, want the refused flush
   FAIL	floter/internal/statefile	0.002s
   ```
   还原：`cp /tmp/p11k-statefile.bak` → `md5sum -c` 「成功」→ `touch` → 复跑 `ok`。
   （第一次用 `git checkout --` 还原失败：新文件未入库，`git checkout` 报「未匹配任何 Git
   已知文件」——已改用备份 cp，并把 md5 核对打出来。）
2. **把 `renameFile(name, path)` 换成 `os.WriteFile(path, data, mode)`**（即退回「就地写」）：
   ```
   --- FAIL: TestARenameFailureLeavesTheTargetUntouched (0.00s)
       statefile_test.go:147: Write error = <nil>, want the refused rename
   FAIL	floter/internal/statefile	0.001s
   ```
   还原同上（`md5sum -c` 成功，`touch` 后整包 `ok`）。

**灵敏度边界（如实记）**：两条变异都打在**唯一实现**上；调用点「是否真的走了这个实现」
没有独立 pin——把某个调用点改回手写 temp 块，现有断言（模式、无残留、回滚）都不会红。
要钉住调用点得做源码扫描（`internal/ci` 那套），属另一条契约，本轮没做。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 66 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 +
   计算器 `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **同一族的第 4–7 份拷贝**（无缺陷，纯收口，下一轮可一次做完）：
   `internal/calculator/calculator.go:495`、`internal/extensions/config.go:104,681`、
   `internal/extensions/repository.go:195`、`internal/terminalui/terminalui.go:518`。
4. **调用点的源码扫描 pin**（见 §6 灵敏度边界）：一个「`internal/` 里除 `statefile` 外不再
   出现 `os.CreateTemp`」的契约测试，才能让「有人偷偷手写一份」变红。
5. `internal/inputtext`（68 行 / 7 条）本轮**已复核**：与参考 `main:src/terminal/inputNormalize.ts`
   逐条同答案（Zs 表、`\r?\n$` 只去一个、干净串零拷贝），`internal/` 里唯一的 `.Send` 与
   两条 shell 手递都过它——**不是缺陷，别再当候选重领**。
