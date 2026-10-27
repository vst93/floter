# P10-z 报告：应用启动走唯一的 spawn——Windows 命令脚本能被启动，detach 只剩一份

规格 `docs/rounds/P10-z-spec.md`。方向：优先级 4（三平台一致性 → 基础功能 → 像素细节），
接 P10-y 报告「仍待」第 8 条（核 `internal/spawn` 的 detach 两臂）。

## 1. 侦察结论：分歧不在 spawn 里，在「谁复制了 spawn」

按建议先逐条核 `detach_unix.go` / `detach_windows.go` 与参考
（`main:src-tauri/src/process_launch.rs:74-86`）：`setsid` 一致，Windows 旗标少一个
`CREATE_NO_WINDOW`（参考自己的注释写明它与 `DETACHED_PROCESS` 同用时被忽略 → 零行为差）。
真正的分歧在往下两跳之后：**应用启动根本不经过 `internal/spawn`**。

- `internal/apps/apps.go:116` `App.Open` 是「点一行应用」的唯一出口，它走 `launchDetached`
  （`apps.go:133`），后者自己 `exec.Command` + 自己 `detachExec` + 自己 reap 线程；
- `internal/apps/detach_unix.go:14`、`detach_windows.go:11` 与 `internal/spawn/detach_*.go`
  **逐字重复**，注释还自称 "restated here so the two cannot drift"；
- 于是参考在 `spawn_application` 里做的 R74（`process_launch.rs:236-262` `windows_cmd_shim_wrap`）
  在这条路径上无处可放。而 `apps.IsLaunchableTarget`（`shortcut.go:194-200`）明确接受
  `.exe`/`.bat`/`.cmd`，`apps.go:311` 的 Start Menu 扫描真的用它收行 →
  **Windows 上一个 `.cmd`/`.bat` 的 Start Menu 条目会被列出来，点它 `CreateProcess` 拒绝脚本文件
  （ERROR_BAD_EXE_FORMAT），用户只看到 `OpenAppFailed` 的提示。**「列得出来、点不动」。

同族第三处：`internal/extensions/cmdshim.go` 也有一个 `.cmd`/`.bat` 判据，但它答的是另一个问题
（captured run 的 `cmd.exe /D /S /C`，`/S` 是它自己的引号契约，且有既有 pin），本轮**不动**它，
只在报告里记下「两处拼写不同、问题不同」。

## 2. 落地

1. `internal/spawn/spawn.go` 新增 `cmdScriptWrap`（参考 `windows_cmd_shim_wrap` 的逐条翻译：
   `.cmd`/`.bat`，大小写无关，只看**程序**；返回 `{"cmd","/c",program,args…}`，参数原样按序）与
   `programArgv(goos, …)`（**平台**决定包不包；Linux/macOS 上 `.cmd` 只是文件名）。
   `programCmd`（`spawn.go:53`）改为调用它 —— `Program`/`ProgramIn`/`TerminalAt` 每个候选
   一次覆盖，没有第二个入口。
2. `internal/spawn/detach_windows.go`：旗标补成参考的三件套
   `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW`（两个 `syscall` 不具名的
   常量按值写出）。参考自己的注释说 `CREATE_NO_WINDOW` 在 `DETACHED_PROCESS` 旁被忽略 → 零行为差。
3. `internal/apps`：**删** `detach_unix.go` / `detach_windows.go`；`launchDetached` 变成
   `launchProgram(homeDir(), program, args...)`，`var launchProgram = spawn.ProgramIn`（seam）。
   `homeDir()` 保留（应用以 home 为 cwd 的现有行为不动），`gio launch` 的 D-Bus 回退不动，
   `mygo.Shell.OpenPath` 的无 argv 分支不动。

**可见行为变化只有一条**：Windows 的 `.cmd`/`.bat` 从「失败」变成「启动」。Linux/macOS 逐字节不变。

## 3. 台账

**V=0 / B=0 / S=1**。S = `internal/apps` 的两份 detach 文件删除、`launchDetached` 从自建
`exec.Command` 改为转发 `spawn.ProgramIn`。没有任何既有测试改断言：新增三个测试
（`TestAWindowsCommandScriptIsGivenToTheInterpreter`、`TestACommandScriptIsOnlyWrappedOnWindows`、
`TestTheAppLaunchPathIsTheSpawnPackagesOwn`），旧的全绿。

## 4. 门槛（全部实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
go test -count=1 ./...                   → 全绿，0 FAIL
--- PASS                                 → 2207（基线 2204，+3）
--- SKIP                                 → 15（=）
测试函数                                  → 1780（基线 1777，+3）
go test ./... 输出行                      → 36（34 ok + 2 no test files）
GOOS=linux|windows|darwin go build ./... → 全过
```

非视觉轮：本机输出逐字节不变，不出图。

## 5. mutation（3 条实跑，全部变红，`md5sum -c` 逐字节还原 + `touch` 复跑）

1. `cmdScriptWrap` 后缀判据换成 `.exe`：
   ```
   --- FAIL: TestAWindowsCommandScriptIsGivenToTheInterpreter (0.00s)
   --- FAIL: TestACommandScriptIsOnlyWrappedOnWindows (0.00s)
       spawn_test.go:218: windows launches "C:\\tools\\flameshot.cmd" [a], want "cmd" ["/c" "C:\\tools\\flameshot.cmd" "a"]
   ```
2. `programArgv` 去掉 `goos != "windows"` 闸门：
   ```
   --- FAIL: TestACommandScriptIsOnlyWrappedOnWindows (0.00s)
       spawn_test.go:223: linux launches "cmd" [/c C:\tools\flameshot.cmd a], want the program untouched ("C:\\tools\\flameshot.cmd" [a])
       （darwin、freebsd 同样三行）
   ```
3. `launchDetached` 丢掉 `homeDir()`：
   ```
   --- FAIL: TestTheAppLaunchPathIsTheSpawnPackagesOwn (0.00s)
       apps_test.go:862: launch 0 = {dir: program:gio ...}, want {dir:/home/tar program:gio ...}
       apps_test.go:862: launch 1 = {dir: program:nautilus ...}, want {dir:/home/tar program:nautilus ...}
   ```
   还原后 `go test ./internal/spawn/ ./internal/apps/` → 两个包都 ok。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 55 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 无扩展名返回 `""`（`shortcut.go:208-210`）、register 只留 `cmd`、
   `open` 是否拒一切 query、未知宿主是否静默忽略。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
5. 剪贴板族两条：轮询间隔 `clipboardPoll=600ms` vs 参考 900；「复制回来的条目该不该置顶」。
6. **新拍板候选（P10-z）**：参考 `spawn_in_own_scope`（`process_launch.rs:186-234`）在 Linux 上把
   被启动的应用放进**自己的 systemd user scope**（`systemd-run --user --scope --quiet
   --unit=floter-launch-<pid>-<seq> -- <program> <args…>`，1500ms 观察期，包装器快速非零退出则
   回退普通 detached spawn），理由是 `setsid` 只换 session **不换 cgroup**，
   `KillMode=control-group` 时应用会跟着 floter 一起被收走。本仓目前只有 `setsid`。
   **这是改变进程生命周期的行为改动，且本机无法用眼睛验证「应用真的活过了 floter 的 scope 拆除」**，
   按纪律不擅自实现 —— 要做的话建议单开一轮，先定「找不到 `systemd-run` / 没有 user manager 时
   是否静默回退」，再拿真机复测。
7. 同族小差：`internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 与参考的 `cmd /c` 拼写不同
   （`/S` 是它的引号契约，`internal/i18n/copy.go` 的 `.cmd`/`.bat` 注入说明依赖它）—— 属另一个问题，不动。
8. 下一轮建议：**剪贴板轮询间隔 600ms vs 900ms** 是唯一还留在剪贴板族里的行为差（有参考常数、
   有 pin、不牵 UI）；若判定它属于「行为节奏」需拍板，则改做
   `internal/tools` 或 `internal/progname` 里下一处三平台不一致（先侦察再定，别照抄本行）。
