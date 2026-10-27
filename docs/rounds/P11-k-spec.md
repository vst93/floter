# P11-k 规格 · 状态文件落盘只有一个写法（补上缺的 flush 与 rename）

## 意图（一句话）

应用自己的状态文件（设置、常用应用、剪贴板索引与其 PNG、窗口状态）落盘时必须走**同一条**
`temp → write → fsync → chmod → rename → dir fsync`，所以「写到一半断电」在任何一处都
不会留下半份文件；现在七处各写一套，其中一处**从不 fsync**、两处**根本不 rename**。

## 为什么现在做（证据，行号本轮实测）

选题规则：用户实测四条全卡实机（`.pending-bugs.md` 顶部「⚠️ 用户实测反馈」）、合约 §6 全
`已落地`、上一轮「仍待」第 1 条同样卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」
自查，落在**基础功能 / 稳定性**（无 GOOS 分支，Linux CI 上钉住三平台同一答案）。

同一个问题，七份答案（`grep -n "os.CreateTemp\|os.Rename" internal/`）：

| 位置 | 现状 | 缺什么 |
|---|---|---|
| `internal/settings/write.go:14` `writeFileAtomic` | temp+Write+**Sync**+Close+Chmod+Rename+dir Sync | —（最完整的一份，本轮成为唯一实现） |
| `internal/usage/usage.go:156` `saveLocked` | temp+Write+Close+Chmod+Rename | **没有 `Sync`**（注释却写「writes the index atomically」）；也没有 dir Sync |
| `internal/clipboard/clipboard.go:786` `saveLocked` | temp+Write+Sync+Close+Chmod+Rename | 没有 dir Sync |
| `internal/clipboard/clipboard.go:534` 图片落盘 | `os.WriteFile(.../x.png)` | **不是原子写**：崩在半路 → 索引里一条永远解不开的 PNG |
| `internal/settings/windowstate.go:106,109` | 两次 `os.WriteFile`（backup、primary） | **不是原子写**：截断的 `window-state.json` 下次启动读默认值 |
| `internal/calculator/calculator.go:495` | temp+Write+Sync+…+Rename+dir Sync | —（与 settings 那份逐字相同，第三份拷贝） |
| `internal/extensions/config.go:104,681`、`repository.go:195`、`terminalui/terminalui.go:518` | 各自 temp+Sync+Chmod+Rename | 与上面同族；本轮**只点名不改**（见「本轮不做」） |

真正的缺陷是前两列带粗体的三条：**一次从不 fsync**（断电后 rename 可能先于数据落盘，
文件在、内容空）与**两处从不 rename**（写一半就有一个可读的半份文件）。

## 逐条落地项

1. **新包 `internal/statefile`**（唯一实现）
   - `Write(path string, data []byte, mode os.FileMode) error`：`MkdirAll(dir, 0755)` →
     `CreateTemp(dir, ".floter-state-*.tmp")` → `Write` → `syncFile` → `Close` → `Chmod(mode)`
     → `renameFile` → 目录 `Sync`（尽力而为，Windows 无目录 fsync，失败忽略：与
     `settings/write.go:47-52` 的既有注释同一判据）。
   - 两个测试接缝（包级函数变量，仓库既有 `StubToolCandidates` 同一写法）：
     `syncFile = func(*os.File) error`、`renameFile = os.Rename`。它们让「先 flush 再
     rename」和「rename 失败时目标不动」成为可断言的，而不是只能靠读代码。
   - 临时文件名统一 `.floter-state-*.tmp`（三处旧前缀 `.settings-` / `.usage-` /
     `.index-` 消失；`settings/write_test.go:59` 的「不留临时文件」断言随之改到新前缀）。

2. `internal/settings/write.go`：`writeFileAtomic` 的函数体换成 `statefile.Write(path, data, mode)`
   （两个调用点 `settings.go:484,487` 与 `write_unix_test.go:29` 不动）。

3. `internal/usage/usage.go:156` `saveLocked`：整段 temp 块换成
   `statefile.Write(s.path, append(data, '\n'), 0o600)` —— **补上 fsync**。

4. `internal/clipboard/clipboard.go`：
   - `saveLocked`（`:786`）→ `statefile.Write(s.paths.Index(), append(data, '\n'), 0o600)`；
   - 图片落盘（`:534-538`）→ `statefile.Write(filepath.Join(s.paths.Images(), file), png, 0o600)`
     —— **补上 rename**。

5. `internal/settings/windowstate.go:103-110`：两次 `os.WriteFile` →
   `statefile.Write(backupPath, data, 0o600)` + `statefile.Write(newPath, encoded, 0o644)`
   —— **补上 rename**；两份文件的模式**不动**（备份 0600 / 主文件 0644 是既有契约，改它属
   另一条行为）。

## 行为契约零改动红线

- 落盘**内容逐字节不变**（含 `usage`/`clipboard` 末尾那一个 `\n`、`calculator` 的缩进、
  `windowstate` 的两份文件内容与顺序：先 backup 后 primary）。
- 文件**模式不变**：settings 两份 0600、usage 0600、clipboard 索引/PNG 0600、
  windowstate backup 0600 / primary 0644。
- 错误契约不变：写失败仍返回 error，由既有调用方回滚内存（P11-c / P11-d / settings
  `Update` 的 rollback 全部不动）。
- 零视觉、零 i18n、零键盘、零高度链、零命令接线改动。

## 门槛命令 + PASS 基线（本轮实测当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2257（P11-j 实测），只许涨
GOOS=linux|windows|darwin go build ./...               # 三平台过
```

## 变异计划（≥2 条，都打在唯一实现上）

1. `statefile.Write` 删掉 `syncFile(temporary)` 那一步 → `TestASyncFailureLeavesTheTarget`
   红（期望 error，实际 nil）。
2. `statefile.Write` 删掉 `renameFile(name, path)` 改成 `os.Rename` 之前直接
   `os.WriteFile(path, data, mode)`（即退回「就地写」）→ `TestARenameFailureLeavesTheTarget`
   红（期望 error 且目标仍是旧字节）。

## 本轮不做（点名，进「仍待」）

`internal/calculator/calculator.go:495`、`internal/extensions/config.go:104,681`、
`internal/extensions/repository.go:195`、`internal/terminalui/terminalui.go:518` 仍是各自
一份 `temp+Sync+Chmod+Rename`。它们**没有缺陷**（都 fsync 且都 rename），只是同一段代码
的第四到第七份拷贝；改它们要动四个包的 import 与四个包的既有 pin，超出「一轮只做一件事」。
