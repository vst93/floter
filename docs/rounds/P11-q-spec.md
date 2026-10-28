# P11-q 规格 · 「同一个文件」只有一个判据

## 意图

全仓对「这两个字符串是不是同一个文件」有 **七种各自为政的答案**，它们对
空串、大小写、符号链接、硬链接给出**互相矛盾**的结果；其中两处已经是真 bug
（Linux 上把 `/tmp/A` 与 `/tmp/a` 判成同一个文件；已连接的集成仍出现在
「检测到的工具」列表里，因为集合只装了集成一侧的解析拼写）。本轮把这一个问题
收进 `internal/pathid`，并让文件身份**问操作系统**（`os.SameFile`），而不是靠
字符串猜——这样 Windows 的大小写不敏感文件系统、8.3 短名、硬链接，
macOS 的大小写不敏感卷，Linux 的符号链接，三个平台得到同一个答案。

为什么现在做（证据）：

- `internal/extensions/candidates.go:133` `sameExecutable`：`if a == b { return true }`
  → `("", "")` 是 `true`（空路径被当成「同一个程序」）。
- `internal/shell/system.go:367` `sameExecutable`：先判空 → `("", "")` 是 `false`。
  同一个函数名，同一个问题，两个答案。
- `internal/shell/detectedtools.go:145-166`：集合装了 `path` / `Clean(path)` /
  `EvalSymlinks(path)` 三种拼写，**查找只查前两种** → 候选是符号链接
  （`~/.local/bin/rg`）、集成记的是解析后的二进制（`~/.cargo/bin/rg`）时，
  查不到 → 已连接的工具继续出现在 Detected 列表里，可以再连一次。
- `internal/settings/migrate.go:193` `samePath`：`strings.EqualFold` **无条件**折大小写
  → Linux（大小写敏感）上把两个不同的文件判成同一个，迁移会跳过本该复制的文件。
- `internal/inventory/inventory.go:284` `normalized`：只在 Windows 折大小写（正确）；
  `internal/inventory/sources.go:517` `sameRoot`：**无条件**折大小写 + 折分隔符。
  同一个仓库里对「大小写算不算不同」有两套答案。
- `internal/inventory/inventory.go:298` 与 `internal/clipboard/files.go:124` 各写一遍
  `EvalSymlinks` 兜底。

## 落地项（动手前已读代码确认真实行号）

新增 `internal/pathid`（`floter/internal/pathid`）：

| 导出 | 语义 |
|---|---|
| `Key(path) string` | 集合键：空串→空串；`Abs`（失败则 `Clean`）+`Clean`+`ToSlash`；**Windows 折大小写**，其它平台保留大小写 |
| `Keys(path) []string` | 同一个文件的所有拼写：`Key(path)`，以及能解析时 `Key(EvalSymlinks(path))` |
| `Same(a, b) bool` | 空串永不匹配；`Keys` 相交→是；否则两边都能 `Stat` 时用 `os.SameFile`（硬链接 / 8.3 短名 / 大小写不敏感卷由文件系统回答） |
| `SameSpelling(a, b) bool` | 折分隔符 + **无条件**折大小写，给「本机根本不存在的路径表」用（Windows 源表在 Linux 上跑） |

调用点改造：

1. `internal/extensions/candidates.go:133` `sameExecutable` **删除**，`candidates.go:122`
   改调 `pathid.Same`。行为变化：`("", "")` true→false（与 shell 侧一致）。
2. `internal/extensions/candidates.go:84-92` `lookToolsIn` 的 `seen` 键改用 `pathid.Keys`
   （Windows 上大小写不同的同名探针不再各占一行）。
3. `internal/shell/system.go:367` `sameExecutable` **删除**，`system.go:357` 与
   `shell.go:1702` 改调 `pathid.Same`。
4. `internal/shell/detectedtools.go:145-175`：抽 `connectedKeys(paths)` 与
   `connected(set, path)` 两个纯函数，两侧都走 `pathid.Keys` —— 修掉
   「已连接仍出现在 Detected 列表」的不对称。
5. `internal/inventory/inventory.go:284` `normalized` 变一行转发 `pathid.Key`（语义不变）。
6. `internal/inventory/sources.go:517` `sameRoot` 变一行转发 `pathid.SameSpelling`
   （语义不变，无条件折大小写的理由仍成立：源表是 Windows 路径，在 Linux 上跑）。
7. `internal/settings/migrate.go:193` `samePath` **删除**，`migrate.go:112` 改调
   `pathid.Same`。行为变化：Linux 上不再把大小写不同的两个文件判成同一个。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**零改动**。
本轮只改「判据」与它的调用点；不改任何 UI 绘制、不改任何用户可见文案。
唯一可见行为变化是两处 bug 修复（Detected 列表去重、Linux 迁移不误判），
都写进新 pin。

## 门槛命令 + 当前实测基线

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2274（HEAD dc46975 实测）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l   # 1845
三平台 go build ./...
```

## mutation 计划

1. 把 `pathid.Same` 的 `os.SameFile` 兜底删掉 → `TestSameAsksTheFilesystem` 的硬链接
   用例必须红（字符串键永远不相等）。
2. 把 `pathid.Key` 的 Windows 折大小写删掉 → `TestKeyIsThePlatformsSpelling` 的
   Windows 用例必须红（Windows vet 不跑测试，但该用例在 Linux 上通过
   `sameSpelling`/`Key` 的平台分派逻辑仍会被断言到；见报告实测）。
3. 把 `detectedtools.connected` 改回只查 `Key(path)` → 新 pin
   `TestAConnectedToolIsNotDetectedAgainThroughItsSymlink` 必须红。
