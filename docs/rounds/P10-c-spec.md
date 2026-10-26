# P10-c 规格：Start Menu 的「算不算可启动」按参考逐条核对，三处分歧与两问交叉第一次钉住

## 0. 意图（一句话）

把 `apps.IsLaunchableTarget` 对参考 `is_launchable_target` 的**逐条保真度**钉成断言：
已有的四条 pin 不动，补上**与参考真正分叉的三处**（尾随空白 / 只由扩展名构成的名字 /
目录名不参与），再补一条**与 `progname.IsLauncher` 的故意分歧**交叉 pin。**产品文件 0 行改动**。

## 1. 为什么现在做（证据）

任务书「现在该做什么」交棒条目指向这里，但它给的前提**已经过期**，本轮先纠正再落地：

| 交棒条目说 | 实测 |
|---|---|
| 「`IsLaunchableTarget` 没有任何 pin」 | **有**。`internal/apps/shortcut_test.go:139` `TestUninstallersAreNotRows`（P7-kq 落地，`git log -1 -S` 可查），11 条用例 |
| 「`.com` 的取舍没有 pin」 | **有**。同表 `{C:\app\legacy.com, false}`，注释写明「reference drops `.com` here too」 |
| 「`uninst` 的匹配范围没有 pin」 | **有**。`unins000.exe` / `Uninstall.exe` / `uninst.exe` 三条 |
| 「大小写折叠没有 pin」 | **有**。`app.EXE` 一条 |

所以本轮不是「补 pin 从零」，而是**把参考读完、把剩下的分叉找出来**。

参考实测（`floter-main/src-tauri/src/commands/apps/windows.rs:539-550`，逐字）：

```rust
fn is_launchable_target(target: &str) -> bool {
    let lower = target.to_lowercase();
    if !lower.ends_with(".exe") && !lower.ends_with(".bat") && !lower.ends_with(".cmd") {
        return false;
    }
    let stem = Path::new(&lower).file_stem()...;
    !stem.contains("uninst") && !stem.contains("unins")
}
```

Go 版（`internal/apps/shortcut.go:185-192`，实测行号）：

```go
name := shortcutBaseName(target)                              // 237：TrimSpace + 两种分隔符
switch strings.ToLower(extensionOf(name)) {                   // 246：最后一个点，且要求 i>0
case ".exe", ".bat", ".cmd":
default: return false
}
stem := strings.ToLower(strings.TrimSuffix(name, extensionOf(name)))
return !strings.Contains(stem, "uninst") && !strings.Contains(stem, "unins")
```

逐条比对后的结论：**三处语义分叉**，全部无 pin。

### D1 尾随空白：我们收，参考不收（`shortcut.go:237` `TrimSpace`）

`C:\app\App.exe␠`：参考 `lower.ends_with(".exe")` → **false**；我们 trim 后 → **true**。
`.lnk` 的 `LinkInfo.LocalBasePath` 是 NUL 结尾的字节串，写入方留一个尾空格是合法的；
Windows 自己启动时也会吞掉尾空格，所以**我们这一侧是对的**，但没有任何断言说明这是
**故意的**——下一轮「照参考改」会把它当 bug 删掉，且删掉后测试全绿。

### D2 只由扩展名构成的名字：我们拒，参考收（`shortcut.go:246` `i > 0`）

目标字面量 `.exe`：参考 `ends_with(".exe")` → true，`Path::file_stem(".exe")` 在 Rust 里是
`.exe`（前导点不是分隔符），stem 不含 `uninst` → **true**；我们 `extensionOf` 要求
`i > 0`，扩展名为 `""` → **false**。
两个答案都不可达（`.lnk` 不会指向字面量 `.exe`），但**没有 pin 说明我们选了哪一个**。

### D3 目录名不参与（无分叉，但无 pin）

`C:\Program Files\Uninstall Helper\App.exe`：两边都只看**文件名**的 stem，→ true。
「过滤器关于文件名，不关于路径」这句是这条判据的**要害**（装在名为 `Uninstall …` 的目录下的
真程序不能被丢），但今天零断言。顺带 `C:/app/App.exe`（正斜杠）与 `\\?\C:\app\App.exe`
（长路径前缀）也是零断言。

### D4 `unins` 是 `uninst` 的前缀（冗余，保真）

`"uninst".contains("unins")` 恒真，所以第二个 `Contains` 是死代码——**参考里也是死代码**，
我们是忠实移植。今天唯一能钉住 `unins` 那半句的是 `unins000.exe`（它不含 `uninst`），
`uninst` 子串在名字**中间**的情况（`MyUninstaller.exe`）无 pin。

### D5 同一个 `.com`，两个问题两个答案（`progname.go:58` vs `shortcut.go:185`）

`progname.IsLauncher("legacy.com")` → **true**（`progtype` 的 Windows 判据 `progtype.go:104`
也走它：`legacy.com` 算程序）；`IsLaunchableTarget` → **false**。
两侧各自有 pin（`progname_test.go:69`、`shortcut_test.go:139`），但**没有一条断言同时说这两句**
——所以「把 `IsLaunchableTarget` 改成 `progname.IsLauncher`」这个看似无害的统一，
今天只会红一条**没人会读的**用例；改成一条明写的交叉 pin 才会红在**说人话的地方**
（与 P10-b 的三面 pin 同一手法）。

## 2. 落地项（逐条）

位置全部为**本轮实测行号**。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | 新文件 `internal/apps/startmenufilter_test.go` | 不存在 | `TestTheFilterIsAboutTheFileNameNotThePath`：D3 的 6 条（目录名含 `Uninstall` / 正斜杠 / 长路径前缀 / 反斜杠尾 / 中文目录），全部 `true` |
| 2 | 同上 | — | `TestTheFilterTrimsWhereTheReferenceDoesNot`：D1，`C:\app\App.exe ` 与 `\tC:\app\App.exe`，我们 `true` 且注释写明参考是 `false` |
| 3 | 同上 | — | `TestALiteralDotExeNamesNothing`：D2，`.exe` / `.bat` / `.cmd` 三个字面量，我们 `false` 且注释写明参考是 `true` |
| 4 | 同上 | — | `TestTheUninstallerSubstringIsMatchedAnywhereInTheStem`：D4，`MyUninstaller.exe` / `unins000.exe` / `x-uninst.exe` → `false`；`install.exe` / `inuninstallable.exe`（含 `uninst`？→ 是，`uninstallable` 含 `uninst`，取 `false`）逐条写清 |
| 5 | 同上 | — | `TestTheTwoProgramQuestionsDisagreeOnCom`：D5，`.com` 三问并列（`progname.IsLauncher` true / `progtype` 的 Windows 策略 true / `IsLaunchableTarget` false），`.exe` 三问全 true，作对照 |
| 6 | `internal/apps/shortcut.go:179-192` 注释 | 只说「reference drops exactly those」 | 补一句指回新 pin：`startmenufilter_test.go` 钉住三处**故意**分叉，别把注释当许可照抄参考 |

产品行为改动：**零**。`shortcut.go` 只动注释。

## 3. 行为契约零改动红线

本轮不碰：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、`docs/raycast-design.md` 的任何 token。
`ScanWindows` 的调用序（`IsLaunchableTarget` → 去重 → 别名）不动。

## 4. 门槛命令 + PASS 基线（本轮实测，非旧报告）

```
gofmt -l cmd internal                                  # 期望空（实测空）
go vet ./...                                           # 期望净（实测净）
go test -count=1 ./...                                 # 期望 0 FAIL（实测 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2065，只许涨
go test -count=1 ./... | grep -cE "^(ok|\?)"           # 基线 36 包
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 基线 15，不许涨
grep -rn "^func Test" internal cmd | wc -l             # 基线 1675 函数
GOOS=linux/windows/darwin go build ./...                # 三平台
```

## 5. mutation 计划（每条说明会让哪个 pin 变红）

| # | 注入的缺陷 | 期望变红 |
|---|---|---|
| M1 | `shortcut.go:187` 接受集合加 `".com"`（把两问统一成 `progname.IsLauncher` 的方向） | `TestTheTwoProgramQuestionsDisagreeOnCom` + 既有 `TestUninstallersAreNotRows` |
| M2 | 删掉 `shortcut.go:191` 的 `unins` 那半句 | 既有 `TestUninstallersAreNotRows`（`unins000.exe`）+ 新 `TestTheUninstallerSubstringIsMatchedAnywhereInTheStem` |
| M3 | `shortcut.go:186` 改成 `name := target`（去掉 `shortcutBaseName` 的 trim，回到参考的「整串 ends_with」语义） | 新 `TestTheFilterTrimsWhereTheReferenceDoesNot` |

每条：注入 → 看红 → 抄原始报错 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑。

## 6. V / B / S 台账（预判）

- **V = 0**：不碰视觉数值。
- **B**：全部新 pin 走真实生产函数，既有 pin 零改动。
- **S = 0**：无结构变更。
