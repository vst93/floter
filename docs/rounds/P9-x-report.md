# P9-x 报告：「一个文件算不算程序」只留一个答案（`internal/progtype`）

改前 HEAD `5f1ed4d`（工作树干净）→ 本轮 `mygo-rewrite`。
规格：`/tmp/floter-p9x-spec.md`（重派版，含本轮补写的 §E 定案）。

## 意图

「这个路径是不是一个能被运行的程序」在全仓曾有 **5 份实现**、**4 种语义**。先把差异定性
（故意 / 手滑 / 死代码），再把判据收进一个包，并**保留**两处语义上正当的分歧。

## 五份实现的事实（改前实测行号，逐个读过）

| # | 位置 | Windows 策略 | Unix 判据 | 判定 |
|---|---|---|---|---|
| 1 | `internal/inventory/inventory.go:238` `isExecutable` | 扩展名 `.exe/.cmd/.bat/.com` | `!IsDir && Perm()&0o111` | 故意（导出 walk 不该把 `notes.txt` 当程序） |
| 2 | `internal/tools/catalog.go:379` `isExecutable(platform,path)` | 扩展名（同上） | 同上 | 故意（跨平台键，测试可在 Linux 上钉 Windows 阶梯） |
| 3 | `internal/apps/apps.go:493` `isExecutableFile` | **无分支 → 恒 false** | `IsRegular && Perm()&0o111` | **死代码**（见下） |
| 4 | `internal/extensions/runtimepath.go:193` `isExecutableFile` | **`return true`（放行）** | `!IsDir && Perm()&0o111` | 故意（路径来自用户自己的选择，宁可放行） |
| 5 | `internal/apps/commands.go:82` `runnable` | 扩展名（同上） | `IsRegular && Perm()&0o111` | 故意，但**与 1/2 重复**（规格只列了四处，这处是动手时 grep 出来的） |

### §B 的核心问题（apps 那份在 Windows 上是不是坏的）：**不可达，是死代码不是 bug**

证据链（改前行号）：

- `isExecutableFile` 的唯一调用者是 `executableAvailable`（`apps.go:477`、`:483`）；
- `executableAvailable` 的唯一调用者是 `scanLinuxRoot`（`apps.go:452`），它读 `.desktop` 的 `TryExec=`；
- `Scan` 的 `switch runtime.GOOS`（`apps.go:205`）里 `scanLinuxRoots` 只在 `default`（`:221`）分支，
  `windows` 分支走 `scanWindowsRoots` + `scanRegistryPrograms`（`:216-218`）。

⇒ Windows 上只有显式调用导出的 `ScanLinux` 才走得到，产品路径零次。**不新增「Windows 应用列表
为空」的修复**（无证据的改动），只把那句自相矛盾的注释（「emulated on Windows … permissive rather
than wrong」，实际 0666 恒 false）改掉，并把该处接到共享判据的**扩展名**策略：若真有人在 Windows 上
问 `foo.exe`，答案应该是 yes 而不是「一律不是」。

## 落地（1 个新包 + 6 处调用点 + 5 份本地实现删除）

### `internal/progtype`（新）

两个策略 × 两个平台来源，函数体**不重复**（`Host` 是 `For(runtime.GOOS, …)`，`HostLenient` 是
`ForLenient(runtime.GOOS, …)`；两者共用 `file()` / `unixProgram()`）：

- `For(platform, path)` / `Host(path)` —— **扩展名策略**：Windows 上 `.exe/.cmd/.bat/.com` 才是程序。
  用于「替用户探一个名字」，错的 yes 会把非程序塞进列表。
- `ForLenient(platform, path)` / `HostLenient(path)` —— **放行策略**：Windows 上任何非目录文件都算。
  用于「路径是用户自己挑的」，错的 no 会把用户刚选中的程序藏掉。
- Unix 一份：`IsRegular && Perm()&0o111 != 0`（三处原来只有 `!IsDir`，本轮顺带拿到 regular 判据；
  `os.Stat` 跟随符号链接，symlink→文件不受影响）。
- 两条策略在普通文件上**必须分歧**，`TestTheStrategiesDisagreeOnAPlainFile` 把这个分歧钉住：
  将来谁想「清理」成一个函数，先红这条。

### 调用点迁移

| 位置 | 改成 | 策略理由 |
|---|---|---|
| `inventory`：`executableFile = progtype.Host`（`inventory.go:146`）、`sources.go:442` | 扩展名 | 本机判据，替用户探 PATH |
| `tools`：`catalog.go:306`、`:332` | `progtype.For(platform, …)` | 跨平台判据（可能在给非本机平台求值） |
| `apps`：`apps.go:485`、`:491`（`TryExec=`） | `progtype.Host` | 本机判据 + 扩展名（`.desktop` 只在非 Windows 分支读，见上） |
| `apps`：`commands.go:46`（`ScanCommands`） | `progtype.Host` | 本机判据 + 扩展名（Windows 上命令就是 `.exe/.cmd/.bat/.com`） |
| `extensions`：`provider.go:53`、`custom.go:252`、`reconnect.go:68`、`candidates.go:62`、`runtimepath.go:101` | `progtype.HostLenient` | 本机判据 + **放行**（用户自己指的程序） |

删除：`inventory.isExecutable`、`tools.isExecutable`、`apps.isExecutableFile`、`apps.runnable`、
`extensions.isExecutableFile`。`grep -rn "0o111" internal/ cmd/ | grep -v progtype/` 现在**为空**。

## 测试台账（V/B/S）

- **V = 0**（非视觉轮，无墨/面无像素 pin 变动）。
- **B = +4，既有 pin 零改动全绿**（`inventory` / `apps` / `extensions` / `tools` 的 `Probe`/`Installed`/
  `DetectManagers`/`Look` 行为 pin 全部原样通过）。
- **S = 1 处迁移（断言逐字不变）**：`tools/platformcriteria_test.go` 的
  `TestTheWindowsExecutableTestNeedsAnExecutableExtension` 直接 pin 了本地 `isExecutable`，随实现删除
  搬到 `internal/progtype/progtype_test.go`，改叫 `TestForReadsTheWindowsLadderByExtension`，
  **每条断言与报错文案原样**，只把 `isExecutable(PlatformWindows, …)` 换成 `For("windows", …)`
  （顺便钉住 tools 的平台键拼写就是 `"windows"`）。同文件另 4 条 `Probe`/`Look` 行为 pin 一字未动。
- 新增 pin 5 条（净 +4 函数）：扩展名阶梯（含 group 位与「目录不是程序」两半）、
  `Host == For(runtime.GOOS, …)`、空平台回落本机、放行策略、两策略必须分歧。

## 门槛实测（全跑）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净（GOOS=windows go vet 五个受影响包也净）
go test -count=1 ./...                    → 33 包全绿（+1 = 新包 progtype），FAIL 0
--- PASS                                  → 2041（基线 2037，+4）
--- SKIP                                  → 15（=15，零新增）
测试函数                                   → 1661（基线 1657，+4，0 减）
GOOS=linux/windows/darwin go build ./...  → 三平台过
```

## mutation（4 条实跑变红 + 逐字节还原 + `touch` 复跑）

改前 md5：`internal/progtype/progtype.go 93d95cc43303a6f5b1c11ad0a2433cc4`。

- **M1** `For` 的 Windows 分支从 `windowsProgram` 改成 `windowsFile`（策略对调）→ 6 条红：
  `progtype_test.go:38: notes.txt was executable on windows`（另 `README`/`rg`/`makefile` 同形）、
  `progtype_test.go:151: the extension rule accepted a plain file on windows`、
  `tools platformcriteria_test.go:79: a bare rg counted as installed on windows`、
  `platformcriteria_test.go:107: a bare apt was detected on windows: [winget apt]`。
- **M2** Unix 判据 `0o111` → `0o100`（只认 owner 位）→ 1 条红：
  `progtype_test.go:71: a file with only the group execute bit was not executable`。
- **M3** 唯一那张扩展名表删掉 `.com` → 1 条红：
  `progtype_test.go:32: old.com was not executable on windows`（证明四个名字只从一处读）。
- **M4** 去掉 regular-file 两半（`file()` 不再挡目录 + `unixProgram` 不再查 `IsRegular`）→ 3 条红：
  `progtype_test.go:68: a directory was executable on linux`、
  `progtype_test.go:71: a directory was runnable on linux under the permissive rule`、
  `progtype_test.go:134: a directory was runnable on windows`。
- 每条：注入 → 抄原始报错 → `cp` 还原 → `md5sum -c` 显示「成功」→ `touch` 后复跑全绿。
- **一条失败的变异（如实记录）**：把 `HostLenient` 改成转发 `For`（策略合并）**没有红**——因为本机是
  Linux，两条策略在 Unix 半边逐字节相同，Windows 半边在 Linux 上不可达。这正是 `extensions` 那五处
  调用点的「接线」无法在本机钉住的原因（策略本身已由 `ForLenient` 的 pin 钉住）。记入仍待。

## 仍待（下一轮候选）

1. 用户侧实机复测（Windows 透明 / 三大金刚 / 卡死 / 几何日志）——本仓无可改代码。
2. 需用户拍板的四条（`.pending-bugs.md` P9-g 表）。
3. **`extensions` 五处 `HostLenient` 接线在 Linux 主机上不可钉**（见上条变异）：要钉得先给
   `extensions` 一个平台接缝（像 `inventory.executableFile` 那样的 var），属结构改动，需拍板。
4. **同族下一件（基础功能）**：`executableCommand`（`extensions/custom.go:299`）与
   `executableName`（`shell/shell.go:2706`）是**逐行相同**的两份「去掉 Windows 启动器后缀」实现；
   另 `inventory.CuratedStem`（`curated.go:60`）与 `tools.candidateNames`（`catalog.go:370`）各存一份
   四个后缀的字面量。与「是不是程序」不同族（是拼写），故本轮不动。
