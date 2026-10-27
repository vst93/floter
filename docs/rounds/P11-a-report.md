# P11-a 报告：一个扩展进程在哪个组里、超时杀谁、跑完扫谁 —— 只留一个答案

规格 `docs/rounds/P11-a-spec.md`。方向：优先级 4（三平台一致性 → 基础功能 → 像素细节），
接 P10-z 报告「仍待」第 8 条（`internal/spawn` 已收口，下一处先侦察再定）。

## 1. 侦察结论：六个出口，六套进程卫生

参考只有一份实现：`main:src-tauri/src/extensions/process_cleanup.rs`。

- `configure_command`（`:16-22`）：Unix `command.process_group(0)`；Windows 空实现。
- `kill_and_reap`（`:131-138`）：`killpg(SIGKILL)` → 杀直接子进程 → wait → 再 `killpg`。
- `finish_after_wait`（`:141-144`）：**正常等完之后也 `killpg`**，清掉「让进程组活着的孙子」。
- `Drop`（`:194-205`）：取消（不是超时）时同步 `killpg` + `waitpid`，否则取消攒僵尸。
- 调用面七个：`probe_runner.rs:49/60`、`provider.rs:475/706/711`、`lifecycle.rs:542/626-637`、
  `run.rs:734`、`capability_probe.rs:115`、`install.rs:1600/2836`、`commands/extensions.rs:1628`。

本仓改造前（`internal/extensions`，改造前实测行号）：

| 出口 | 位置 | 组 | 组杀 |
|---|---|---|---|
| 后台运行 | `run.go:104` / `:126` | 无 | 无 |
| provider describe | `provider.go:164` / `:172` | 无 | 无 |
| provider operation | `complete.go:99` / `:110` | 无 | 无 |
| 探针 | `probes.go:164` / `:174` | `Setsid` | **无** |
| `--help` 派生 | `helpderive.go:32` / `:47` | `Setsid` | **无** |
| 脚本 `--version` | `scriptruntime.go:54` / `:61` | `Setsid` | **无** |
| 登录 shell 的 PATH | `runtimepath.go:206` / `:208` | `Setsid` | 无（且 goroutine 无出口） |
| 宿主版本 | `osversion.go:37` | 无 | 无 |

`exec.CommandContext` 的默认 `Cancel` 是 `Process.Kill()`——**只杀直接子进程**。于是：

1. 超时（`RunTimeout=300s`、探针超时、provider 超时）留下脚本的孙子；
2. 取消（禁用/卸载集成 → `runregistry` cancel → `runCtx` 取消）同样只死直接子进程；
3. 同一件事在两条路径上两种行为：探针类带 `Setsid`（连控制终端都断），运行/provider 类什么都不带；
4. `runtimepath.go:206` 的 `select` 5s 到点只让调用方返回，`cmd.Output()` 的 goroutine 永远挂着。

## 2. 落地

1. 新增 `internal/extensions/proctree.go`：
   - `newExtensionCommand(ctx, program, args…)`：`exec.CommandContext` + `configureProcessGroup(cmd, false)`
     + `cmd.Cancel = killProcessGroup(cmd); return nil`。
   - `newProbeCommand(…)`：同一实现，`session=true`。
   - `runExtensionCommand(cmd)`：`cmd.Run()` 之后**再扫一次组**（参考 `finish_after_wait`）。
2. 新增 `proctree_unix.go`：`configureProcessGroup(cmd, session)` 两分支**互斥** ——
   `session` → `Setsid`；否则 `Setpgid`。`killProcessGroup` = `syscall.Kill(-pid, SIGKILL)` +
   `Process.Kill()` 兜底。
3. 新增 `proctree_windows.go`：`configureProcessGroup` 空，`killProcessGroup` 只杀直接子进程
   （参考 `#[cfg(not(unix))] fn kill_group(&self) {}`）。
4. **删除** `probeprocess_unix.go` / `probeprocess_windows.go`（`isolateProbe` 并入构造器参数）。
5. 八个出口全部换成构造器：`run.go`、`provider.go`、`complete.go`、`probes.go`、`helpderive.go`、
   `scriptruntime.go`、`runtimepath.go`、`osversion.go`；`cmd.Run()` 换成 `runExtensionCommand(cmd)`。
6. `runtimepath.go` 的 goroutine 版改成 `newProbeCommand(ctx 5s, shell, "-ilc", …)` +
   `runExtensionCommand`：超时语义不变（仍返回 `nil` 路径表），但 shell 5s 后真的死掉，
   goroutine 不再无出口。
7. `osversion.go` 的 `exec.Command(...).Output()` 换成同一个构造器（`context.Background()`，
   **不加超时**：参考的 `commands/system.rs` 也没有），stdout 仍 trim 后返回，行为逐字不变。

### 实现时踩到的一个真坑（写进注释与 pin）

`Setpgid` 与 `Setsid` **不能同时置位**：`setpgid(0,0)` 让子进程成为组长，组长再 `setsid`
返回 EPERM，子进程直接起不来。第一版把两者叠加（先 `Setpgid` 后 `Setsid`），
`TestReprobeCommandsComparesWithThePreviousSidecar` 立刻红：
`extensions: the tool did not produce usable help output`。改成互斥参数后全绿。
`TestTheTwoConstructorsDifferOnlyInTheSession` 把这条钉住。

### 已知与参考的一处时序差（有意留下）

参考在 `child.wait()` 返回后**立刻** `killpg`，我们的 `runExtensionCommand` 是
`cmd.Run()` 返回后扫组；一个**攥着管道**的孙子会让 `Run` 等到 `WaitDelay`（run=2s、provider/probe=0.5-1s）
才返回，于是扫组晚一点点。孙子仍然会死（承诺不变），只是晚 ≤2s。要完全一致得自己 `Start` + 自建管道，
超出本轮范围，记在这里。

## 3. 台账

**V=0 / B=0 / S=2**。
- S-1：`probeprocess_*.go` 删除，`isolateProbe` → `configureProcessGroup(cmd, session)`。
- S-2：八个出口的 `exec.Command*` + `cmd.Run()` 改为 `newExtensionCommand`/`newProbeCommand` +
  `runExtensionCommand`。
没有改任何既有断言（V=0）；没有产品回归（B=0）。新增 4 个测试函数。

## 4. 门槛（全部实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
go test -count=1 ./...                   → 全绿，0 FAIL
--- PASS                                 → 2211（基线 2207，+4）
--- SKIP                                 → 15（=）
测试函数                                  → 1784（基线 1780，+4）
go test ./... 输出行                      → 36（34 ok + 2 no test files）
GOOS=linux|windows|darwin go build ./... → 全过
```

非视觉轮：本机渲染输出逐字节不变，**不出图**。

## 5. mutation（4 条实跑，全部变红，`md5sum -c` 逐字节还原 + `touch` 复跑）

1. `killProcessGroup` 只杀直接子进程（`syscall.Kill(-pid)` → `syscall.Getpid()`）：
   ```
   --- FAIL: TestATimedOutRunKillsTheScriptsGrandchildren (0.50s)
       proctree_unix_test.go:63: the timed-out run left its grandchild alive
   --- FAIL: TestAFinishedRunSweepsItsProcessGroup (0.51s)
       proctree_unix_test.go:81: the finished run left its grandchild alive
   ```
2. `runExtensionCommand` 去掉跑完扫组：
   ```
   --- FAIL: TestAFinishedRunSweepsItsProcessGroup (0.51s)
       proctree_unix_test.go:81: the finished run left its grandchild alive
   ```
3. `configureProcessGroup` 去掉 `Setpgid`：
   ```
   --- FAIL: TestATimedOutRunKillsTheScriptsGrandchildren (0.53s)
       proctree_unix_test.go:63: the timed-out run left its grandchild alive
   --- FAIL: TestAFinishedRunSweepsItsProcessGroup (0.51s)
       proctree_unix_test.go:81: the finished run left its grandchild alive
   --- FAIL: TestTheTwoConstructorsDifferOnlyInTheSession (0.00s)
       proctree_unix_test.go:94: a run is not in its own process group
   ```
4. `newProbeCommand` 丢掉 session（`true` → `false`）：
   ```
   --- FAIL: TestTheTwoConstructorsDifferOnlyInTheSession (0.00s)
       proctree_unix_test.go:105: a probe did not ask for its own session
   ```
   （注意：这一条**只有 pin 会红**——探针去掉 `Setsid` 仍能跑出结果，`TestReprobeCommands…` 不红。
   这正是它值得被钉的原因。）

每条还原后 `md5sum -c /tmp/p11a.md5` 两个文件都「成功」，`touch` 后复跑全绿
（`go test -count=1 ./internal/extensions/` → `ok … 18.180s`）。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 56 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 无扩展名返回 `""`（`shortcut.go:208-210`）、register 只留 `cmd`、
   `open` 是否拒一切 query、未知宿主是否静默忽略。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
5. 剪贴板族两条：轮询间隔 `clipboardPoll=600ms` vs 参考 900；「复制回来的条目该不该置顶」。
6. systemd user scope（P10-z 第 7 条，拍板项）。
7. **本轮留下的时序差**（§2 末尾）：攥管道的孙子让扫组晚 ≤ `WaitDelay`；要完全对齐参考的
   「wait 一返回就 killpg」需要自建 `Start` + 管道，属另一轮。
8. 同族小差：`internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 与参考 `cmd /c` 拼写不同
   （`/S` 是它自己的引号契约，`internal/i18n/copy.go` 的 `.cmd`/`.bat` 注入说明依赖它）—— 不动。
9. 下一轮建议：**剪贴板轮询间隔 600ms vs 参考 900ms**（剪贴板族唯一还留着的行为差，一处常数
   + 一处 pin，P10-v…P10-x 收官时点名的最后一条）。若判定它属「行为节奏需拍板」，则转
   `internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 与参考 `cmd /c` 的拼写核对
   （它的 `/S` 是自有引号契约，要先读 `internal/i18n/copy.go` 的注入说明再定，别直接改）。
   `apps` 注册表链已在 P7-kr / P10-q…P10-r 逐条核过两遍，不要当 TODO 重领。
