# P10-u 规格：每用户数据目录只留一个答案（`xdg.DataHome`）

## 意图

「本机的每用户数据目录在哪」这个问题，本仓已经有一个唯一答案——`internal/xdg.DataHome`
（`internal/xdg/xdg.go:26-35`，P9-s 建立，`apps`/`inventory` 全部改读它）。但仍有**两处**
手抄了同一个问题，而且两处都抄错了 XDG 规范的同一条规则：

1. **`internal/terminalui/fonts.go:80-84`**（Linux 臂）把字体目录写成
   `filepath.Join(home, ".local", "share", "fonts")`。这正是 `$XDG_DATA_HOME/fonts`，但写死
   等于**假定** `$XDG_DATA_HOME` 没被改过。设了 `XDG_DATA_HOME=/srv/data` 的用户，字体装在
   `/srv/data/fonts`（fontconfig 自己就按规范读那里），而本仓的字体选择器扫的是
   `~/.local/share/fonts` —— 用户装好的等宽字体在设置页的 Terminal 卡片里**不存在**。
2. **`internal/settings/migrate.go:215-222`**（Linux 臂）读了 `$XDG_DATA_HOME`，但**没有规范
   要求的绝对路径判据**。`XDG_DATA_HOME=rel/dir` 时它返回 `rel/dir` —— 一个相对路径，落到
   进程的当前工作目录上。于是旧版设置的导入会去「用户从哪个目录启动 floter」里找一个
   `com.v.floter/settings.json`：找不到就静默不导入，找到了就导入**别的目录**里的设置。

两处都是「同一个问题两个答案」，`xdg.DataHome` 已经带着正确判据（`xdg_test.go:63-77` 把
`relative/dir` 钉成 `~/.local/share`）。本轮把这两处接到那一个答案上。

为什么现在做：优先级 4（三平台一致性 → 基础功能 → 像素细节）。`inventory`/深链两族已收官
（见 `.pending-bugs.md` 顶部 P10-t §5.6 第 7 条），本轮不碰任何需拍板项，也不需要实机。

## 落地项

### 1. `internal/terminalui/fonts.go` —— Linux 字体目录读数据目录

- 位置：`fontDirs()`（`fonts.go:63-90`），Linux 在 `default:` 臂。
- 现状：`filepath.Join(home, ".local", "share", "fonts")` 写死；`$XDG_DATA_HOME` 被忽略。
- 应改成：`filepath.Join(dataHome, "fonts")`，`dataHome` 由调用方传 `xdg.DataHome()`。
- 为了让「三平台各自的表」在任意宿主上都能钉住（本仓既有做法，见
  `browser.chromiumBrowsers`/`apps` 的平台表），把 `fontDirs()` 拆成
  `fontDirsFor(goos, home, systemRoot, localAppData, dataHome string) []string` +
  `fontDirs()` 取环境后调它。**纯函数，无 IO**，`home`/`systemRoot`/`localAppData` 也一并
  变成入参（原来直接读 `os.Getenv`）。
- Windows / macOS 两臂的**内容一字不改**（`C:\Windows\Fonts` + `%LOCALAPPDATA%\Microsoft\Windows\Fonts`；
  `/System/Library/Fonts`、`/Library/Fonts`、`~/Library/Fonts`）。

### 2. `internal/settings/migrate.go` —— Linux 数据目录走 `xdg`

- 位置：`platformDataDir()`（`migrate.go:200-226`）。
- 现状：Linux 臂手抄 `$XDG_DATA_HOME` + `~/.local/share`，缺绝对路径判据。
- 应改成：同样拆成纯函数 `dataDirFor(goos, home, localAppData, dataHome string) string`，
  Linux（`default:`）臂返回 `dataHome`；调用方 `platformDataDir()` 传
  `xdg.DataHome()`、`os.Getenv("LOCALAPPDATA")`、`os.UserHomeDir()`。
  Windows 臂（`%LOCALAPPDATA%`）与 macOS 臂（`~/Library/Application Support`）内容不变。
- 相对 `$XDG_DATA_HOME` 从此被忽略 —— 判据只有一处（`xdg.DataHome`），不再有两份。

## 行为契约零改动红线

- **高度链**：不涉及。
- **键盘模型 / 会话管理 / 设置写回 / 命令接线**：不涉及。
- **i18n 文案**：不涉及。
- **Linux 行为的两处改动，都是「向规范靠」**：
  - `XDG_DATA_HOME` 设成**绝对路径**时：字体目录从 `~/.local/share/fonts` 变成
    `$XDG_DATA_HOME/fonts`（新目录被扫，旧目录不再被扫——规范里那正是同一个位置）；
    设置的导入路径不变（它本来就读了变量）。
  - `XDG_DATA_HOME` 设成**相对路径**时：设置的导入从「相对 CWD 的路径」变成
    `~/.local/share`（规范要求的回退）。
- **Windows / macOS：零变化**（两臂逐字符保留，pin 钉住）。

## 门槛命令与 PASS 基线

基线（本轮开工时实测，HEAD `327c6ae`）：

- `go test -count=1 ./...` 全绿，`--- PASS` **2152**、`--- SKIP` **15**、`--- FAIL` 0、
  测试函数 **1754**、36 个包。
- `gofmt -l cmd internal` 空；`go vet ./...` 净；三平台 build 过。

本轮跑同一组命令，PASS 只许涨（新增 2 个测试文件里的函数），函数数只许涨。

## mutation 计划（每条都要实跑变红）

1. `fonts.go` 的 Linux 臂改回 `filepath.Join(home, ".local", "share", "fonts")`（无视传入的
   dataHome）→ `TestLinuxFontDirsFollowTheDataHome` 变红。
2. `migrate.go` 的 Linux 臂改回 `filepath.Join(home, ".local", "share")`（无视 dataHome）→
   `TestDataDirForFollowsThePlatformTable` 的 Linux 行变红。
3. `migrate.go` 的 Windows 臂改成 home 派生（`joinFor` 式）→ 同表 Windows 行变红（证明这张
   表真的逐臂在断言，不是只钉了 Linux）。

## 台账

**V = 0 / B = 0 / S = 2**：`fontDirs` 与 `platformDataDir` 从「读环境的函数」变成「纯函数 +
一层取环境的调用」。既有测试零改动必须全绿（B 判据）；没有任何视觉数值变化，不出图。
