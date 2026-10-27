# P10-u 报告：每用户数据目录只留一个答案（`xdg.DataHome`）

规格：`docs/rounds/P10-u-spec.md`。开工 HEAD `327c6ae`（P10-t），工作树干净。

## §1 意图与证据

「本机的每用户数据目录在哪」在本仓已经有一个答案——`internal/xdg.DataHome`
（`internal/xdg/xdg.go:29`，P9-s 建立；`apps`/`inventory` 全部改读它，`xdg_test.go:63-77`
把规范的「相对值忽略」规则钉住）。**仍有两处手抄了同一个问题**，且两处都漏了同一条规则：

| 位置 | 现状（开工时） | 后果 |
|---|---|---|
| `internal/terminalui/fonts.go:63`（Linux 臂） | `filepath.Join(home, ".local", "share", "fonts")` | 设了绝对 `XDG_DATA_HOME` 的用户，字体装在 `$XDG_DATA_HOME/fonts`（fontconfig 就按规范读那里），设置页 Terminal 卡片与终端的字体选择器里**看不到**这些字体 |
| `internal/settings/migrate.go:206`（Linux 臂） | 读 `$XDG_DATA_HOME` 但**没有绝对路径判据** | `XDG_DATA_HOME=rel/dir` 时返回相对路径，落到进程 CWD：旧版设置导入会去「用户从哪个目录启动 floter」里找 `com.v.floter/settings.json` —— 找不到就静默不导入，找到了就导入别的目录的设置 |

为什么现在做：优先级 4（三平台一致性 → 基础功能 → 像素细节）。`inventory` 族与深链族已收官
（`.pending-bugs.md` 顶部 P10-t §5.6 第 7 条），本轮不碰任何需拍板项，也不需要实机。

## §2 落地

- `internal/terminalui/fonts.go`：`fontDirs()` 拆成纯函数
  `fontDirsFor(goos, home, systemRoot, localAppData, dataHome string) []string`
  （`fonts.go:80`）+ 一层取环境的调用（`fonts.go:63`，`dataHome` 取 `xdg.DataHome()`）。
  Linux 臂第一项改成 `filepath.Join(dataHome, "fonts")`（`fonts.go:99`）。
  **Windows / macOS 两臂逐字符保留**（`C:\Windows\Fonts` + `%LOCALAPPDATA%\Microsoft\Windows\Fonts`；
  `/System/Library/Fonts`、`/Library/Fonts`、`~/Library/Fonts`）。
- `internal/settings/migrate.go`：`platformDataDir()`（`migrate.go:206`）拆成纯函数
  `dataDirFor(goos, home, localAppData, dataHome string) string`（`migrate.go:225`）+ 取环境的
  调用（`migrate.go:211`，传 `xdg.DataHome()`）。Linux（`default:`）臂返回 `dataHome`；
  Windows 臂仍返回 `%LOCALAPPDATA%`，macOS 臂仍返回 `~/Library/Application Support`。
- 新增 `internal/terminalui/fontdirs_test.go`（2 函数）、
  `internal/settings/datadir_test.go`（1 函数，6 个子测试）。

拆成纯函数的理由与本仓既有做法一致（`browser.chromiumBrowsers`、`apps` 的平台表）：一张平台表
如果只有宿主平台那一臂被编译，另外两臂的**内容**就没法在任何机器上断言——而本轮改的正是
「三平台各自的答案」。

### 行为面

- Windows / macOS：**零变化**（两臂内容与调用顺序不变，pin 逐项钉住）。
- Linux，`XDG_DATA_HOME` 为**绝对路径**：字体目录从写死的 `~/.local/share/fonts` 变成
  `$XDG_DATA_HOME/fonts`（规范里那本来就是同一个位置）；设置导入路径不变（它本来就读变量）。
- Linux，`XDG_DATA_HOME` 为**相对路径**：字体目录与设置导入都落到 `~/.local/share`
  （`xdg.DataHome` 的规范回退），不再落到 CWD 相对的路径上。
- 一处零效果边界：`os.UserHomeDir()` 失败时，Linux 的设置数据目录从 `""` 变成
  `xdg.DataHome()`（其 `homeDir()` 回退 `/`，即 `/.local/share`）。两者都指向一个不可能存在的
  目录，`os.Stat` 都失败、导入都不会发生；无测试依赖旧值。

## §3 新增 pin

| 测试 | 钉住什么 |
|---|---|
| `TestLinuxFontDirsFollowTheDataHome` | Linux 表首项 = 传入 dataHome + `/fonts`；写死的 `~/.local/share/fonts` **不再出现在表里**；其余三项（`~/.fonts`、`/usr/share/fonts`、`/usr/local/share/fonts`）原序 |
| `TestFontDirTablesPerPlatform` | darwin 三项原序；Windows 有 `LOCALAPPDATA` 时两项、无时只一项且 `SystemRoot` 回退 `C:\Windows` |
| `TestDataDirForFollowsThePlatformTable`（6 子测试） | windows=`LOCALAPPDATA`（无则 `""`）、darwin=`~/Library/Application Support`（无 home 则 `""`）、linux=传入的 dataHome 且**不**从 home 重推 |

## §4 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿 |
| `--- PASS` | **2161**（基线 2152，+9 = 2 个函数 + 1 个 6 子测试函数） |
| `--- SKIP` | 15（=，无新增 skip） |
| `--- FAIL` | 0 |
| 测试函数 | **1757**（基线 1754，+3） |
| 包 | 36（=） |
| `GOOS=linux/windows/darwin go build ./...` | 三平台过 |
| `GOOS=darwin go vet ./...`、`GOOS=windows go vet ./internal/settings ./internal/terminalui` | 净 |

本轮非视觉，不出图（无像素/墨面改动）。

## §5 mutation（3 条，全部实跑变红并逐字节还原）

1. `fonts.go` Linux 臂改回 `filepath.Join(home, ".local", "share", "fonts")`：
   ```
   --- FAIL: TestLinuxFontDirsFollowTheDataHome (0.00s)
       fontdirs_test.go:24: first font directory = "/home/tester/.local/share/fonts", want "/srv/data/fonts" (the data home's own fonts)
       fontdirs_test.go:29: the data home's fallback "/home/tester/.local/share/fonts" is still in the table: [...]
       fontdirs_test.go:45: font directory 0 = "/home/tester/.local/share/fonts", want "/srv/data/fonts"
   ```
2. `migrate.go` Linux 臂改成 `filepath.Join(home, ".local", "share")`：
   ```
   --- FAIL: TestDataDirForFollowsThePlatformTable/linux_is_the_data_home_it_was_given (0.00s)
       datadir_test.go:81: dataDirFor("linux", "/home/tester", "", "/srv/data") = "/home/tester/.local/share", want "/srv/data"
   ```
3. `migrate.go` Windows 臂改成 home 派生：
   ```
   --- FAIL: TestDataDirForFollowsThePlatformTable/windows_uses_LOCALAPPDATA (0.00s)
       datadir_test.go:81: dataDirFor("windows", "C:/Users/tester", "C:/Users/tester/AppData/Local", "/srv/data") = "C:/Users/tester/.local/share", want "C:/Users/tester/AppData/Local"
   --- FAIL: TestDataDirForFollowsThePlatformTable/windows_without_LOCALAPPDATA_answers_nothing (0.00s)
       datadir_test.go:81: dataDirFor("windows", "C:/Users/tester", "", "/srv/data") = "C:/Users/tester/.local/share", want ""
   ```
   第 3 条的作用是证明这张表**逐臂**在断言，而不是只钉了 Linux 那一行。

每条都 `cp` 备份 → `md5sum` 记录 → 注入 → 看到红 → 还原 → `md5sum -c` **成功** → `touch` 后复跑
（全绿，避免测试缓存复用）。

## §6 台账

**V = 0 / B = 0 / S = 2**

- V：无视觉数值改动，不出图。
- B：既有测试**零改动**全绿（本轮没有改任何既有测试文件）。
- S：`fontDirs`（`fonts.go:63`）与 `platformDataDir`（`migrate.go:206`）从「读环境的函数」变成
  「纯函数 + 一层取环境的调用」；两个新测试文件读的是纯函数。

## §7 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 50 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. icns 转换要不要落盘缓存（P10-t 新记）、SVG/XPM 图标（P10-s 新记）——仍是拍板项。
6. 下一轮建议：`internal/browser` 的 `osascript`/CDP 两臂已按参考逐条核对（本轮顺带复核：
   `tabs.go:60` 的 `darwin`/`default` 分支、`ActivateTab` 的降级到 `OpenURL`、`parseAppleScriptTabs`
   的五段切分与 sentinel、`parseCDPTargets` 的 `type=="page"` 与 `window 0` 全部与
   `main:src-tauri/src/browser_data/tabs.rs` 一致，**没有分歧可改**）。下一处不需要拍板的表面建议
   `internal/clipboard` 的跨平台写入路径（`shell.writeClipboard` 在三条系统上的行为是否同一答案）。
