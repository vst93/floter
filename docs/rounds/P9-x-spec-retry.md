# P9-x（重派）Spec：把「一个文件算不算程序」的判据收到一处，并暴露四份实现的分歧

## 背景：上一轮（也叫 P9-x）被打断，残骸已清

上一轮在 07:03 死在半步（`pi-floter.log` 0 字节，会话断在一次 `edit` 之后），留下**根本不编译**的
工作树。主线已 `git checkout` 清空，残骸存档在 `/tmp/floter-p9x-orphan/`（`partial.diff` +
`progtype.go`）。**可以先读那份存档**，但别照抄——它的设计有下面 §D 说的问题。

上一轮只处理了 inventory + tools 两处，**漏了 apps 与 extensions**。所以它即使没死，也是漏交付。

## 本轮意图

「这个路径是不是一个能被运行的程序」这一判据在全仓有 **4 份实现**，而且**语义互不相同**。
先定性它们的差异是**故意的**还是**手滑**，再决定收口形状。这是用户优先级「三平台一致性」的正题。

## A. 先读，拿到四份的事实（不许凭记忆）

```
grep -rn "^func isExecutable\|^func isExecutableFile" internal/
```
当前实测位置（**行号会漂，动手前自己确认**）：

| 包 | 函数 | Windows 分支 | Unix 判据 | 签名 |
|---|---|---|---|---|
| `internal/inventory` | `isExecutable` | 按扩展名 `.exe/.cmd/.bat/.com` | `Mode()&0o111 != 0` | `(path)` |
| `internal/tools` | `isExecutable` | 按扩展名（同上） | 同上 | `(platform, path)` ← **平台参数** |
| `internal/apps` | `isExecutableFile` | **没有 Windows 分支** | `IsRegular() && Perm()&0o111 != 0` | `(path)` |
| `internal/extensions` | `isExecutableFile` | **直接 `return true`** | `Mode()&0o111 != 0` | `(path)` |

## B. 必须回答的核心问题：apps 那份在 Windows 上是不是坏的

`internal/apps/apps.go` 的注释写着：

> The mode bits are emulated on Windows (files read as 0666), so a probe there
> is permissive rather than wrong: it never hides an installed entry.

**这句话自相矛盾**：0666 不含任何执行位，所以 `Perm()&0o111 != 0` 在 Windows 上恒为 **false**——
它不是 permissive，是**一个都不放行**。而 `extensions` 那份对 Windows 显式 `return true`（真的 permissive），
`inventory`/`tools` 那两份走扩展名。

本轮要做的第一件事就是**证实或证伪**这个判断，并给出证据：

1. 读 `apps` 里这两个调用点的上下文（`internal/apps/apps.go:477`、`:483` 附近），确认它们是否在
   **Windows 代码路径上**被执行。如果应用发现在 Windows 上是走 macOS 的 `.app` bundle / Linux 的
   `.desktop` 另一条分支，这个 bug 就**不可达**——那也要写清「不可达 + 证据」。
2. 若可达：这是三平台一致性的真 bug（Windows 用户的应用列表可能整个是空的）。
   把它**先钉成一条会红的测试**（用平台参数化的判据，别让测试只在 Linux 上跑），再修，
   修完看它变绿。报告里写死前的报错原文。

## C. 收口形状：分歧不许被抹平

判据有两个**语义上正当**的变体，不许硬合成一个函数：

- **本机判据**：`runtime.GOOS` 决定分支（inventory、apps、extensions 用）。
- **跨平台判据**：调用方给定 platform 字符串，用于「这条 entry 在目标平台上算不算程序」
  （`tools.Probe`/`managerPresent` 的目录名/管理器探测——它可能在给非本机平台求值）。

所以：
1. 新建 `internal/progtype`，导出**两个**（或按你读到的事实决定三个）判据，命名要说清是哪个语义，
   别叫 `IsExecutable` 和 `IsExecutableFor` 这种分不出意图的名字（上一轮的存档就是这个毛病）。
2. **Windows 的两种策略（扩展名 vs 一律放行）必须显式命名并各留文档**，不许悄悄统一成一种。
   选哪个由每处调用点的语义决定，逐个说明理由；统一了就算 B 类回归。
3. 四处调用点全改到 `progtype`，删掉四个本地实现。
   **只查不改也行**的情况：你读到某处的差异其实依赖本地实现的细节，那就保留它并在
   `.pending-bugs.md` 写清为什么。

## D. 上一轮存档里的问题（别继承）

`/tmp/floter-p9x-orphan/progtype.go` 的 `IsExecutable(path)` 与 `IsExecutableFor(platform, path)`
**函数体几乎逐行重复**，只有 `runtime.GOOS` 换成了 `platform`。这就是没做完的样子：
要么 `IsExecutable` 实现成 `IsExecutableFor(runtime.GOOS, path)`，要么承认这是两个判据各自写清。
重派时按 §C 的命名与去重标准做。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、视觉（墨/面/阴影/半径）——本轮全是
**B 类**：测试**零改动**必须全绿。红了是产品回归，改产品不改测试。
唯一例外：如果你证实 §B 的 Windows bug 真实可达，修它属于正当的产品修复，要配套新增 pin。

## 门槛（实测基线，别用旧报告数字）

```
gofmt -l cmd internal            # 必须空
go vet ./...                     # 必须净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 基线 2037，只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2 条，要打到「分歧」上才有意义）

- **M1**：把 `progtype` 的 Windows 分支从「扩展名」改成「一律 true」→ 应让 inventory/tools 的
  Windows 侧 pin 变红（证明扩展名判据真的在被检验，不是死代码）。
- **M2**：把 Unix 判据的 `0o111` 改成 `0o100`（只认 owner 位）→ 应有 pin 红。
- **M3（若 §B 判为可达 bug）**：把 apps 的判据换成本轮新形状前先跑一次原状，确认它能红会绿。

每条：注入 → 看到真的红（抄原始报错串）→ 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑避免缓存。

## 交付

- 四份实现的定性表（故意/手滑 + 证据 file:line）进报告。
- `progtype` 收口 + 四处调用点迁移 + 删除本地实现。
- 报告写 `/tmp/floter-p9x-report.md`，**增量写**。
- `git add -A && git commit && git push origin mygo-rewrite`，轮号 **P9-x**。
- 追加 done.log（`~/.hermes/state/iteration/floter-done.log`）。

**禁止全盘扫描**（`find /`、`grep -rln /`）——曾卡死 53 分钟。只扫 `internal/`、`cmd/`、`docs/`
与 `/home/tar/go/pkg/mod/github.com/egoist/`。

发现必须改既有交互行为才做得对的 → 写进 `.pending-bugs.md`「需用户拍板」，不要擅自实现，
转去做下一件不需要拍板的事。

---

## §E 重派定案（本轮实测，动手前核对过行号）

### 四份实现的事实（HEAD `5f1ed4d`）

| 包 | 位置 | Windows 策略 | Unix 判据 |
|---|---|---|---|
| `inventory` | `inventory.go:238` `isExecutable` | 扩展名 `.exe/.cmd/.bat/.com` | `!IsDir && Perm()&0o111` |
| `tools` | `catalog.go:379` `isExecutable(platform,path)` | 扩展名（同上） | 同上 |
| `apps` | `apps.go:493` `isExecutableFile` | **无分支 → 恒 false** | `IsRegular && Perm()&0o111` |
| `extensions` | `runtimepath.go:193` `isExecutableFile` | **`return true`（放行）** | `!IsDir && Perm()&0o111` |

### §B 的判定：apps 那份在 Windows 上**不可达**（不是坏，是死）

证据：`isExecutableFile` 的唯一调用者是 `executableAvailable`（`apps.go:477`、`:483`），
唯一调用者又是 `scanLinuxRoot`（`apps.go:452`，读 `.desktop` 的 `TryExec=`）；`Scan` 的
`switch runtime.GOOS` 里 `scanLinuxRoots` 只在 `default`（`apps.go:221`）分支。
所以 Windows 上只有显式调用导出的 `ScanLinux` 才会走到，产品路径上零次。
⇒ 不新增「Windows 应用列表为空」的产品修复（那是无证据的改动），只把注释里那句自相矛盾的
「permissive」改掉，并把该处接到共享判据的**扩展名**策略上（`.desktop` 的 `TryExec` 在
Windows 上若真被问，答案应是 `.exe` 而不是「一律不是」）。

### 收口形状（`internal/progtype`）

两个策略 × 两个平台来源，函数体不重复（每个都转发到同一份 `file()`/`unixProgram()`）：

- `For(platform, path)` / `Host(path)` → **扩展名策略**（inventory、tools、apps）。
- `ForLenient(platform, path)` / `HostLenient(path)` → **放行策略**（extensions：路径来自用户自己的
  选择，宁可放行也不隐藏）。
- Unix 一份：`IsRegular && Perm()&0o111 != 0`（apps 原来就最严；另外三处顺带拿到 regular 判据，
  `os.Stat` 跟随符号链接，故 symlink→文件不受影响）。

### 调用点迁移（4 处，本地实现全删）

| 包 | 改成 | 理由 |
|---|---|---|
| `inventory` | `executableFile = progtype.Host`；`sources.go:441` `progtype.Host` | 本机判据 |
| `tools` | `progtype.For(platform, …)`（2 处） | 跨平台判据（可能在给非本机平台求值） |
| `apps` | `progtype.Host`（2 处） | 本机判据 + 扩展名策略 |
| `extensions` | `progtype.HostLenient`（5 处） | 本机判据 + 放行策略 |

### 测试台账

- **S（迁移，断言逐字不变）**：`tools/platformcriteria_test.go` 直接 pin 了本地 `isExecutable`，
  随实现删除改成 `progtype.For`；同文件另两条 `Probe` 行为 pin 不动。
- **B**：`inventory`/`apps`/`extensions` 既有 pin 零改动必须全绿。
- **新增**：`internal/progtype/progtype_test.go`（扩展名阶梯、host==For(GOOS)、放行策略、
  两策略在普通文件上必须分歧、空平台回落）。
