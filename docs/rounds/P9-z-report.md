# P9-z 报告：Windows 启动器后缀阶梯只留一个答案（`internal/progname`）

> **轮号说明（并发冲突，如实记录）**：本轮自查选题时选的轮号是 P9-y，但同一 worktree 里有
> **另一个 pi 进程**（PID 3695167，12:52 起）正在做「mygo v0.6.3 → v0.6.4 升级」轮，并在 13:00–13:02
> 写了 `docs/rounds/P9-y-spec.md` 与 `docs/AGENT-NOTES.md`（轮次档案约定）。为了不把两轮混成一个
> 轮号，本轮的交付物改用 **P9-z**，且提交时只 `git add` 本轮自己的路径（不用 `-A`），
> 避免把对方的在飞改动卷进本轮 commit。

方向来自选题规则第 3 条 → `.pending-bugs.md` P9-x「仍待」第 5 条（同族另一半：P9-x 统一了
「判据」，本轮统一「拼写」）。规格 `/tmp/floter-p9y-spec.md`。

改前 HEAD `ffc032d`，工作树干净；`git pull --ff-only` 已是最新。

## 1. 病灶（实测行号，改前）

`.exe/.cmd/.bat/.com` 四个后缀在仓库里有 **5 份字面量**，被三个方向读：

| # | 位置 | 方向 |
|---|---|---|
| 1 | `internal/progtype/progtype.go:34` `windowsProgramExtensions` | 接受（`filepath.Ext` 比表） |
| 2 | `internal/tools/catalog.go:370` `candidateNames` | 生成（Windows 探针阶梯） |
| 3 | `internal/inventory/curated.go:60` `CuratedStem` | 剥（裸名 → 词干） |
| 4 | `internal/extensions/custom.go:301` `executableCommand` | 剥（路径 → 词干） |
| 5 | `internal/shell/shell.go:2708` `executableName` | 剥（路径 → 词干） |

`#4` 与 `#5` 除函数名/注释外逐行相同（复制粘贴）。`#3/#4/#5` 三份剥法也逐字相同。

### 参考核对（`main:src-tauri/src/`，只读对照）

- `commands/extensions.rs:847` `executable_display_name` = `file_name()`，**不剥后缀**；
- `commands/apps/windows.rs:353` `executable_name` = `file_stem()`，剥**任意**扩展名；
- `commands/apps/windows.rs:538` `is_launchable_target` = `.exe/.bat/.cmd` 三件，**没有 `.com`**。

⇒ 参考没有单一答案，且方向也不全。我们的四件表是 mygo 自己的规则（比 `file_stem` 准：
`foo.tar.gz` 不会被削成 `foo.tar`）。本轮**只收口，不改语义**，并把参考的三处差异写进包注释。

## 2. 落地

- **新包 `internal/progname`**（零依赖：`path/filepath`/`runtime`/`strings`）：
  - `launcherSuffixes` = 唯一一份四后缀表；
  - `IsLauncher(name)`（接受）/ `Stem(name)`（剥，含 lower+TrimSpace）/ `FromPath(path)`
    （`Stem(filepath.Base(path))`）/ `Ladder(platform,name)`（生成，`""` 回退 `runtime.GOOS`）；
  - 包注释写明「一张表三个方向」，以及**三处故意更窄**的集合属于别的问题、钉在各自包：
    `apps.IsLaunchableTarget` 三件（无 `.com`）、`extensions.ProgramIsCmdScript` 两件（要不要
    `cmd.exe`）、`apps` 注册表图标路径只认 `.exe`。
- `progtype.go`：删 `windowsProgramExtensions`，`windowsProgram` → `progname.IsLauncher(path)`。
- `tools/catalog.go`：`candidateNames` 保留函数名（测试钉着），函数体 → `progname.Ladder`。
- `inventory/curated.go`：`CuratedStem` 保留函数名（`TestCuratedStemIsTheOneSpellingRule` 钉着），
  函数体 → `progname.Stem`。
- `extensions/custom.go`：**删** `executableCommand`，两处调用点 → `progname.FromPath`
  （顺手去掉调用点外那层多余的 `strings.ToLower`——`FromPath` 已小写）。
- `shell/shell.go`：**删** `executableName`，两处调用点 → `progname.FromPath`（同上）。
- `apps.executableName(argv []string)` **不动**：它剥的是 argv 的路径前缀，不剥后缀、不改大小写，
  对齐参考 `executable_name`，不是同族。
- 收口后 `grep -rn '"\.exe"' internal/ cmd/ | grep -v _test` 只剩 3 行，全是上面那三处**故意更窄**
  的集合 + 新包的表本身。

## 3. 测试台账（V/B/S）

- **V = 0**（本轮不碰视觉）。
- **B = 既有 pin 零改动全绿**（`platformcriteria_test.go` 的阶梯 pin、`TestCuratedStemIsTheOneSpellingRule`
  逐字未动；`progtype` 的 Windows 判据 pin 全绿 → 说明「`filepath.Ext` 比表 ≡ 后缀比表」在三种边界
  上等价）。
- **新增 pin 5 条**（`internal/progname/progname_test.go`）：
  `TestTheThreeDirectionsReadOneTable`（同一张表：每个后缀被接受 ⇔ 被剥掉 ⇔ 在阶梯里）、
  `TestStemDropsOneLauncherSuffix`（含 `rg.exe.txt` 不剥、`  rg.bat ` 去空白）、
  `TestFromPathIsTheStemOfTheBaseName`、`TestIsLauncherSeesTheFourAndNothingElse`、
  `TestLadderIsTheWindowsOneAndTheBareNameEverywhereElse`。
- **既有表 +1 行**：`apps.TestUninstallersAreNotRows` 加 `C:\app\legacy.com → false`，把
  「launchable 集合故意比四件窄」钉住（此前 `.com` 那一格没人管）。
- 净变化：测试函数 1661 → **1666**（+5，0 减）；PASS 2041 → **2046**（+5）。

## 4. 门槛实测（改后）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet progname/progtype/tools/apps   # 净
go test -count=1 ./...         # 全绿
--- PASS 2046   --- SKIP 15   --- FAIL 0
测试函数 1666 · go list 36 个包（34 ok + 2 no test files）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

## 5. mutation（5 条实跑变红 + 逐字节还原）

改前 md5：`progname.go ae623013709c119a3bbf4988e39bf3f2`、`apps/shortcut.go ccb448da53c52ebdf8d778678e290ece`。

| # | 注入 | 变红的 pin（原始报错摘录） |
|---|---|---|
| M1 | 表里删掉 `.com` | `rg.com was not accepted as a launcher`；`Stem(rg.com) = "rg.com", want rg`；`the windows ladder [rg.exe rg.cmd rg.bat rg] does not offer rg.com`；`FromPath("/usr/local/bin/x.com") = "x.com", want "x"`；跨包：`CuratedStem("rg.com") = "rg.com", want "rg"`（`inventory_test.go:93`） |
| M2 | `Stem` 去掉 `ToLower` | `Stem("RG.EXE") = "RG.EXE", want "rg"`；`FromPath("/opt/tools/RG.EXE") = "RG.EXE", want "rg"`；跨包：`CuratedStem("RG.EXE") = "RG.EXE", want "rg"` + `the curated lookup disagrees with the stem rule` |
| M3 | `Ladder` 的 Windows 分支去掉尾随裸名 | `the windows ladder = [rg.exe rg.cmd rg.bat rg.com], want [… rg]`；跨包：`tools: platformcriteria_test.go:33 the windows ladder = …` |
| M4 | `FromPath` 取目录不取文件名 | `FromPath("/usr/bin/rg") = "/usr/bin", want "rg"`（6 行同族） |
| M5 | `IsLaunchableTarget` 放宽到四件 | `IsLaunchableTarget("C:\\app\\legacy.com") = true, want false`；顺带 `IsLaunchableTarget("https://example.com") = true, want false`（**这就是 `.com` 必须留在窄集合外的原因**：URL 的 `.com` 会被当成程序） |

**如实记录一条失败过的变异**：M4 第一版写成 `Stem(path)`，`filepath` 变成未使用 import →
`[build failed]`，这**不算** pin 变红；改成 `Stem(filepath.Dir(path))`（能编译、语义错）后才真正红。
还原：`md5sum -c` 两次「成功」，`touch` 后复跑全绿。

## 6. 视觉

本轮不是视觉轮（没有墨/面/布局改动），按规则不出 PNG；`internal/launcher`、`settingsui`、
`theme` 的像素 pin 全部零改动通过（2046 PASS 里包含它们）。

## 7. 仍待

1. 用户侧实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志）——连续第 29 轮挂着。
2. 需拍板四条（P9-g 表）+ `connectLocalTool` 拒绝权限仍 `WarnFeedback`（P9-n 记）。
3. `extensions` 五处 `HostLenient` 接线在本机不可钉（要平台接缝，属拍板）。
4. `xdg.WalkEntries` 真实符号链接下潜只有单元 pin（P9-w 记）。
5. **下一轮候选（基础功能）**：`apps.ShortcutTargetName`（`shortcut.go:198`）与
   `apps.desktopFileStem`（`apps.go:710`）都用「剥**任意**扩展名」，而本轮新包提供的是「剥**启动器**
   后缀」——两套拼写规则并存，需要先核参考（`windows.rs:353` `file_stem` 对 `ShortcutTargetName`、
   `linux.rs:412/435` 对 `desktopFileStem`）再决定是合并还是钉成两个策略（P9-x 的「两策略」范式）。
