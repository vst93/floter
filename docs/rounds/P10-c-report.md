# P10-c 报告：Start Menu 的「算不算可启动」——把与参考的三处分叉和两问交叉钉住

方向来自任务书「现在该做什么」交棒条目。规格 `docs/rounds/P10-c-spec.md`。
**产品行为零改动**（`internal/apps/shortcut.go` 只有注释 +9 行），台账 **V=0 / B / S=0**。

## 1. 先纠正交棒条目的前提（侦察结论）

交棒条目说「`IsLaunchableTarget` 没有任何 pin 钉住 `.com` 的取舍、`uninst` 的匹配范围、
大小写折叠」。**这三条都已有 pin**，实测：

```
$ git log -1 --format=%s -S "TestUninstallersAreNotRows" -- internal/apps/shortcut_test.go
fix(apps): Windows 应用扫描从不读快捷方式的目标——别名/拼音首字母/卸载器过滤/去重四件事一起缺，图标抽取闪黑窗（P7-kq)
```

`internal/apps/shortcut_test.go:139` 的 11 条用例覆盖：`.exe`/`.EXE`/`.bat`/`.cmd` 收、
`.com` 拒（P9-z 加的注释写明「reference drops `.com` here too」）、
`unins000`/`Uninstall`/`uninst` 拒、`readme.txt`/`app.exe,0`/`https://…` 拒。

所以本轮不是从零补 pin，而是**把参考读完、把剩下没说的说清**。参考逐字
（`floter-main/src-tauri/src/commands/apps/windows.rs:539-550`）：

```rust
let lower = target.to_lowercase();
if !lower.ends_with(".exe") && !lower.ends_with(".bat") && !lower.ends_with(".cmd") { return false; }
let stem = Path::new(&lower).file_stem()...;
!stem.contains("uninst") && !stem.contains("unins")
```

## 2. 逐条比对结果：三处语义分叉，全部无 pin

| 分叉 | 参考 | 本仓（实测行号） | 可达性 |
|---|---|---|---|
| **D1 尾随空白** | `ends_with(".exe")` → `false` | `shortcutBaseName` 的 `TrimSpace`（`shortcut.go:237`）→ `true` | 可达：`LinkInfo.LocalBasePath` 是 NUL 结尾字节串，写入方可留尾空格 |
| **D2 只由扩展名构成的名字** | `ends_with` 真 + Rust `file_stem(".exe") == ".exe"` → `true` | `extensionOf` 要求 `i > 0`（`shortcut.go:246`）→ `false` | 不可达 |
| **D3 目录名参与度** | 只看 `file_stem`，目录不参与 | 同（`shortcutBaseName` 只取最后一段） | 可达，且是这条判据的要害 |

D1、D2 是**故意**的（两个都对我们更合理），但零断言 → 下一轮「照参考改」会静默翻掉。
D3 无分叉但零断言：`C:\Program Files\Uninstall Helper\App.exe` 必须仍是行。

## 3. 落地：5 个测试函数（`internal/apps/startmenufilter_test.go`，新产品文件 0 个）

| 测试 | 钉住 |
|---|---|
| `TestTheFilterIsAboutTheFileNameNotThePath` | 7 条路径：目录叫 `Uninstall Helper` / `uninst` / `unins000`、正斜杠、`\\?\` 长路径前缀、中文目录、目录名带点 —— 全部 `true` |
| `TestTheFilterTrimsWhereTheReferenceDoesNot` | D1：尾空格 / 尾 tab / 尾 CRLF → `true`（注释写明参考是 `false`），并钉住**前导**空白不分叉 |
| `TestALiteralDotExeNamesNothing` | D2：`.exe`/`.bat`/`.cmd`/`C:\app\.exe` → `false`；`.exe.txt` → `false`（这条两边一致） |
| `TestTheUninstallerSubstringIsMatchedAnywhereInTheStem` | 子串在名字中间 / 分隔符后 / 别的写法，以及 **`unins` 那半句是唯一能抓 `unins000` 的**（`"unins"` 是 `"uninst"` 的前缀，参考里也是死代码，保真） |
| `TestTheTwoProgramQuestionsDisagreeOnCom` | 同一个 `.com`：`progname.IsLauncher` `true` / `progtype.For("windows")` `true` / `IsLaunchableTarget` `false`；`.exe` 三问全 `true`，`.txt` 全 `false` |

最后一条是本轮的核心：两侧各自早有 pin，但**没有一条断言同时说这两句**，所以
「`IsLaunchableTarget` 干脆调 `progname.IsLauncher`」这个统一只会红在一条没人读的旧用例上。
与 P10-b 的三面 pin 同一手法。

`shortcut.go:179` 的文档注释补了一段，指回新 pin，并写明这两处是我们决定不跟参考。

## 4. mutation：3 条实跑变红 + 逐字节还原

`md5sum internal/apps/shortcut.go` = `bc91bed8d4d742df73b6e4f3a582ded7`（三次还原后 `md5sum -c` 均「成功」）。

**M1** — 把两问统一：接受集合加 `".com"`（`shortcut.go:187`）

```
--- FAIL: TestUninstallersAreNotRows
    shortcut_test.go:162: IsLaunchableTarget("C:\\app\\legacy.com") = true, want false
    shortcut_test.go:162: IsLaunchableTarget("https://example.com") = true, want false
--- FAIL: TestTheTwoProgramQuestionsDisagreeOnCom
    startmenufilter_test.go:191: IsLaunchableTarget("…/001/legacy.com") = true, want false
```

**M2** — 删掉 `unins` 那半句（`shortcut.go:191`）

```
--- FAIL: TestUninstallersAreNotRows
    shortcut_test.go:162: IsLaunchableTarget("C:\\Program Files\\App\\unins000.exe") = true, want false
--- FAIL: TestTheUninstallerSubstringIsMatchedAnywhereInTheStem
    startmenufilter_test.go:137: IsLaunchableTarget("C:\\app\\unins000.exe") = true, want false
    startmenufilter_test.go:137: IsLaunchableTarget("C:\\app\\unins000.dat.exe") = true, want false
```

**M3** — `name := target`（去掉 `shortcutBaseName`，回到参考的整串语义）

```
--- FAIL: TestTheFilterIsAboutTheFileNameNotThePath
    startmenufilter_test.go:56: IsLaunchableTarget("C:\\Program Files\\Uninstall Helper\\App.exe") = false, want true
    startmenufilter_test.go:56: IsLaunchableTarget("C:\\app\\uninst\\WXWork.exe") = false, want true
    startmenufilter_test.go:56: IsLaunchableTarget("C:\\unins000\\App.exe") = false, want true
--- FAIL: TestTheFilterTrimsWhereTheReferenceDoesNot
    startmenufilter_test.go:78: IsLaunchableTarget("C:\\app\\App.exe ") = false, want true
    startmenufilter_test.go:78: IsLaunchableTarget("C:\\app\\App.exe\t") = false, want true
    startmenufilter_test.go:78: IsLaunchableTarget("C:\\app\\App.exe\r\n") = false, want true
```

每条之后 `cp` 还原 + `md5sum -c` + `touch internal/apps/*.go` 再复跑（避开测试缓存）。

## 5. 门槛（全部实测，本轮数字）

```
gofmt -l cmd internal                空
go vet ./...                         净
go test -count=1 ./...               0 FAIL
--- PASS                             2070   （基线 2065，+5）
--- SKIP                             15     （= 基线）
包数                                  36     （= 基线）
测试函数                              1680   （基线 1675，+5）
GOOS=linux/windows/darwin build      三平台全过
```

视觉轮才出图，本轮是基础功能轮（B 类），无表面改动 → 无 PNG。

## 6. 台账

- **V = 0**：不碰任何视觉数值 / token。
- **B**：5 个新函数走真实生产路径（`IsLaunchableTarget`、`progname.IsLauncher`、`progtype.For`），
  既有 pin **零改动**全绿。
- **S = 0**：无结构变更；`shortcut.go` 只有 +9 行注释。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 33 轮，本仓无可改代码。
2. 需拍板四条不变（P9-g 表），外加 v0.6.4 带来的两条：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**。
3. `apps.ShortcutTargetName` 对「无扩展名」返回 `""`，参考 `windows.rs:321` 返回整名。
   行为不可达，改它要改既有 pin，**留待拍板**（`shortcut.go:208-210` 注释）。
4. `shell.go:2684` `connectDetected` 的显示名 fallback（`tool.Name` 空 → `progname.FromPath`）
   仍无直接 pin：要跑异步全链路。**建议**抽纯函数 `detectedToolName(tool)` 再测，别造慢测。
   本轮未做（一轮一件）。
5. 下一轮建议：上面第 4 条——抽纯函数 `detectedToolName(tool)` + 钉 `--version` 默认值。
   另一条候选（三平台一致性，本轮侦察时顺带确认**不是**缺口）：`.desktop` 的
   `Hidden`/`NoDisplay` 过滤已落地且有 pin（`apps.go:647-649`，`apps_test.go:92-107`），
   所以「只有 Windows 有『算不算可启动』的过滤」这句话不成立，不要当 TODO 领。
