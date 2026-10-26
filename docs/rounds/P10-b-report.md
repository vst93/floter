# P10-b 报告：「命令词面的 stem」补三面 pin（B 类，产品零改动）

方向：任务书「现在该做什么」交棒条目 → P10-a「仍待」第 4 条。
规格 `docs/rounds/P10-b-spec.md`。

## ① 病灶（改前 HEAD `ab39488`，实测行号）

同一个文件名在仓里有**三个面**，三份答案：

| 面 | 函数 | 位置 | 规则 |
|---|---|---|---|
| 别名面 | `apps.executableName` | `internal/apps/apps.go:732` | `filepath.Base`，一个字符都不剥 |
| 词干面 | `apps.fileStem` | `internal/apps/shortcut.go:226` | 剥**任意**最后一个扩展名，保留大小写 |
| 命令词面 | `progname.FromPath` | `internal/progname/progname.go:100` | 只剥 `.exe/.cmd/.bat/.com`，小写 |

命令词面有三个生产调用点：`internal/extensions/custom.go:257`（请求没带 Command 时派生）、
`internal/shell/shell.go:2557`（发现行按名字找安装配方）、`internal/shell/shell.go:2687`
（发现行缺 Name 时的显示名）。

**改前没有一条测试区分这三面**：`TestTheTwoStemRulesDisagreeOnPurpose`（P10-a）只钉了
词干面 vs 命令词面，且用例全是小写；别名面（`executableName`）零直接 pin；
`detected_test.go` 的安装配方用例也全是小写路径。于是「把 `progname.FromPath` 换成
`apps.fileStem`」这种合并不会有任何测试变红——但 `RG.EXE` 会派生 `RG`，
`validCustomCommand`（`customvalidate.go:118`）只收小写字母/数字/`-`/`_`，
用户装好的程序从此连接不上。

## ② 落地（4 条新 pin，产品代码零改动）

1. `internal/apps/filestem_test.go` — `TestTheThreeNameFacesDisagreeOnPurpose`：
   5 条路径 × 三个面的值（`/c/bin/RG.EXE` → 别名 `RG.EXE` / 词干 `RG` / 命令词 `rg`；
   `report.tar.gz` → `report.tar.gz` / `report.tar` / `report.tar.gz` …），
   外加三条后果断言（别名面不丢字符、词干面丢一段、只有命令词面小写）。
2. `internal/extensions/commandword_test.go` — `TestTheDerivedCommandIsTheLauncherStemNotTheFileStem`：
   走真实路径 `normalizeCustom`（`custom.go:257`），7 条文件名（四种启动器后缀、无后缀、
   `rg.exe.txt`、`report.tar.gz`），合法者断言派生的 `Command`，带点者断言
   `CustomIssueCommandShape`。
3. 同文件 — `TestOnlyTheCommandFaceIsALegalCommandWord`：三个面的答案逐条过
   `validCustomCommand`，只有命令词面为真（`RG.EXE` 的三个答案 `RG.EXE`/`RG`/`rg`）。
4. `internal/shell/commandword_test.go` — `TestTheDetectedRowMatchesTheCatalogByTheCommandWord`：
   `Flameshot.EXE` / `Flameshot` / `FLAMESHOT` 三条都命中目录的小写探针并给出 winget 配方
   （换词干面会得到 `Flameshot`，匹配不上 `flameshot`，安装按钮静默消失）。

**未加 pin 的一处**：`shell.go:2687`（`connectDetected` 的显示名 fallback）。它要求跑
`connectLocalTool` 的异步全链路（真的执行程序的 `--help`），属既有 `surfacefeedback_test.go`
的领域；它用的也是 `progname.FromPath`，由第 2 条间接覆盖，故本轮不重复造慢测。

台账 **V=0 / B 既有 pin 零改动全绿 / S=0**。产品文件（`internal/**` 非测试）改动 0 行。

## ③ 门槛实测

- `gofmt -l cmd internal` → 空
- `go vet ./...` → 净
- `go test -count=1 ./...` → 36 包全绿
- `--- PASS` **2065**（基线 2051，+14：4 个新函数 + 10 个子用例）
- `--- SKIP` **15**（= 基线）
- `--- FAIL` **0**
- 测试函数 **1675**（基线 1671，+4）
- 三平台 `GOOS=linux/windows/darwin go build ./...` 全过

## ④ mutation（3 条实跑变红 + 逐字节还原 + `touch` 复跑）

改前基线 md5（`/tmp/p10b-md5.txt`）：
`custom.go 91063466b148cbbc0ea6faa63c59db65`、`progname.go 5f8b122c2de1ffe8f00ca42e045e8803`。

**M1 — 把 `custom.go:257` 的派生换成「剥任意扩展名」（即向词干面合并）**

注入：`request.Command = progname.FromPath(...)` → `base := filepath.Base(...); strings.TrimSuffix(base, filepath.Ext(base))`。
红：`TestTheDerivedCommandIsTheLauncherStemNotTheFileStem/RG.EXE`，原始报错
`commandword_test.go:61: RG.EXE: a legal derived command was refused: extensions: the connection request is not usable: commandShape`。
其余 6 条子用例仍绿（小写名两规则同答），证明这条 pin 钉的正是分岔点。
还原：`git checkout -- internal/extensions/custom.go` → `md5sum -c` 两文件成功 → `touch` 后复跑绿。

**M2 — 把 `progname.Stem` 的小写去掉（大小写不再折叠）**

注入：`name = strings.ToLower(strings.TrimSpace(name))` → `name = strings.TrimSpace(name)`。
红 4 个包、6 条函数：`progname` 的 `TestStemDropsOneLauncherSuffix` / `TestFromPathIsTheStemOfTheBaseName`、
`apps` 的 `TestTheTwoStemRulesDisagreeOnPurpose` / `TestTheThreeNameFacesDisagreeOnPurpose`、
`shell` 的 `TestTheDetectedRowMatchesTheCatalogByTheCommandWord`（3 子用例）、
`extensions` 的 `TestTheDerivedCommandIsTheLauncherStemNotTheFileStem/RG.EXE` /
`TestOnlyTheCommandFaceIsALegalCommandWord` / 既有 `TestCustomCommandShapeAppliesToTheDerivedWord`。
原始报错（两条代表）：
`commandword_test.go:49: detectedInstallCommand("/c/Program Files/Flameshot/Flameshot.EXE") = "", want "winget install --id Flameshot.Flameshot -e" (the catalog's probes are lowercase)`
`filestem_test.go:151: command face = "RG.EXE", want rg (lowercased)`。
还原 + `md5sum -c` 成功 + `touch` 复跑四包全绿。

**M3 — 把 `progname.Stem` 改成剥任意扩展名（向词干面合并的另一方向）**

注入：后缀循环 → `if i := strings.LastIndexByte(name, '.'); i > 0 { return name[:i] }`。
红：`progname.TestStemDropsOneLauncherSuffix`（`progname_test.go:46: Stem("rg.exe.txt") = "rg.exe", want "rg.exe.txt"`）、
`apps.TestTheTwoStemRulesDisagreeOnPurpose`、`apps.TestTheThreeNameFacesDisagreeOnPurpose`
（`filestem_test.go:138: command face of "/opt/report.tar.gz" = "report.tar", want "report.tar.gz"`）。
`shell`/`extensions` 本轮的新 pin 在此方向仍绿（fixture 无内层点）——如实记录，不假装覆盖。
还原 + `md5sum -c` 成功 + `touch` 复跑绿。
