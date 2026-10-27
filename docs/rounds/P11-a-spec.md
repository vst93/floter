# P11-a 规格：一个扩展进程在哪个组里、超时杀谁、跑完扫谁 —— 只留一个答案

方向：优先级 4（三平台一致性 → 基础功能 → 像素细节）。接 P10-z 报告「仍待」第 8 条
（`internal/spawn` 已收口，下一处三平台/三表面不一致先侦察再定）。侦察落在
`internal/extensions`：**同一条「跑一个扩展进程」的路径，六个出口有六套进程卫生**。

## 1. 意图（一句话）

一个扩展进程必须进**自己的进程组**，超时/取消时杀掉**整个组**（脚本的孙子一起走），
正常退出后也要**扫一遍组**——这条规则在 run / provider / complete / probe / help / script
六个出口上只能是同一份实现。

## 2. 为什么现在做（证据，行号本轮实测）

参考侧只有一份实现：`main:src-tauri/src/extensions/process_cleanup.rs`
- `configure_command`（`:16-22`）：Unix `command.process_group(0)`，Windows 空实现（注释写明
  Windows 只有直接子进程）。
- `ChildCleanup::kill_and_reap`（`:131-138`）：先 `killpg(SIGKILL)`，再杀直接子进程，再 wait，
  再 `killpg` 一次。
- `ChildCleanup::finish_after_wait`（`:141-144`）：**正常等完之后也 `killpg`**，
  「while also removing any grandchildren that kept the process group alive」。
- `ChildCleanup::drop`（`:194-205`）：future 被取消（不是超时）时同步 `killpg` + `waitpid`，
  否则取消会攒僵尸。
- 用它的出口：`probe_runner.rs:49/60`、`provider.rs:475/706/711`、`lifecycle.rs:542/626-637`、
  `run.rs:734`（`command_output`）、`capability_probe.rs:115`、`install.rs:1600/2836`、
  `commands/extensions.rs:1628`。**七个调用面，一份实现。**

本仓侧（`internal/extensions`，本轮实测行号）：

| 出口 | 位置 | 现状 | 组 |
|---|---|---|---|
| 后台运行（用户点的命令） | `run.go:104` `exec.CommandContext`，`:126` `cmd.Run()` | 无 `SysProcAttr`、无组杀 | **不在组里** |
| provider describe | `provider.go:164`，`:172` | 同上 | **不在组里** |
| provider operation | `complete.go:99`，`:110` | 同上 | **不在组里** |
| 探针 | `probes.go:164` + `isolateProbe` `:165`，`:174` | `Setsid` 有，组杀没有 | 在组里，**但没人杀它** |
| `--help` 派生 | `helpderive.go:32` + `:33`，`:47` | 同上 | 同上 |
| 脚本 `--version` | `scriptruntime.go:54` + `:55`，`:61` | 同上 | 同上 |
| 登录 shell 的 PATH | `runtimepath.go:206` + `:207`，`:208` `cmd.Output()` | `exec.Command` + 5s `select`，**goroutine 永不结束** | 同上 |

后果（都是用户能撞上的）：
1. **超时只杀直接子进程**：`exec.CommandContext` 的默认 `Cancel` 是 `Process.Kill()`。
   一个脚本 `sh -c 'server & wait'` 超时后，`server` 留在机器上——参考的
   `probe_runner.rs:226 timeout_kills_parent_and_grandchild_process_group` 钉的正是这条。
2. **取消（禁用/卸载集成）也只杀直接子进程**：`runregistry` 的 cancel 走 `ctx`，
   `run.go` 的 `runCtx` 一取消同样只死直接子进程（参考 `run.rs:1941`
   `killing_an_extension_aborts_its_run_and_its_process_group`）。
3. **六个出口两种行为**：探针类带 `Setsid`（连控制终端都断了），运行/provider 类什么都不带
   —— 同一个「扩展进程」在两条路径上的进程树卫生不同，这正是「三平台一致性」要收的那类分歧。
4. **`runtimepath.go:206` 的 goroutine 无出口**：5s 到点只让 `select` 返回，`cmd.Output()`
   仍挂着；`SHELL` 指向一个不响应的 shell 时，进程与 goroutine 都留到应用退出。

## 3. 落地项（逐条：位置 → 现状 → 应改成）

1. **新增 `internal/extensions/proctree.go`（可移植）**：
   - `newExtensionCommand(ctx, program, args...) *exec.Cmd`：`exec.CommandContext` +
     `configureProcessGroup` + `cmd.Cancel = func() error { killProcessGroup(cmd); return nil }`。
   - `newProbeCommand(ctx, program, args...) *exec.Cmd`：同上，再叠「探针隔离」（见第 2 条）。
   - `runExtensionCommand(cmd) error`：`cmd.Run()` 之后**再扫一次组**
     （参考 `finish_after_wait`），返回 `Run` 的错误。
2. **`probeprocess_unix.go` / `probeprocess_windows.go` 删除**：`isolateProbe` 不再单独存在，
   「探针要不要自己的会话」变成构造器的一个参数（`configureProcessGroup(cmd, session)`）。
   探针原有的「不碰控制终端」用 `Setsid` 保住；**两者互斥**——`setpgid(0,0)` 会让子进程成为
   组长，组长 `setsid` 直接 EPERM（实现时踩到：探针全部失败，`TestReprobeCommands…` 抓到）。
3. **新增 `proctree_unix.go` / `proctree_windows.go`**：
   - unix `configureProcessGroup(cmd, session)`：`SysProcAttr` 为 nil 就建；`session` 为真就
     `Setsid = true`（pid==pgid 由会话给出），否则 `Setpgid = true`（参考 `process_group(0)`）。
     **两条分支互斥**，不是叠加。
   - unix `killProcessGroup`：`syscall.Kill(-pid, SIGKILL)`，再 `Process.Kill()` 兜底
     （参考注释：provider 可能忽略软信号，所以是 SIGKILL）。
   - Windows 两臂都空/只杀直接子进程（参考 `#[cfg(not(unix))] fn kill_group(&self) {}`）。
4. **六个出口全部换到构造器**：`run.go:104`、`provider.go:164`、`complete.go:99`、
   `probes.go:164`、`helpderive.go:32`、`scriptruntime.go:54` 用 `newExtensionCommand` /
   `newProbeCommand`；`cmd.Run()` 换成 `runExtensionCommand(cmd)`。
5. **`runtimepath.go:206` 收进同一条路**：`exec.Command` + goroutine + `select` 改为
   `newProbeCommand(ctx5s, shell, "-ilc", …)` + `runExtensionCommand`，5s 后进程真的死掉
   （用户可见行为不变：超时仍返回 `nil` 路径表）。
6. **`osversion.go:37` 也收进构造器，但不加超时**（`context.Background()`）：参考的 `commands/system.rs` 同样没有组/超时，本轮只把
   它从「裸 `exec.Command(...).Output()`」换成同一个构造器 —— 行为逐字不变（stdout 还是 trim 后返回），
   好处是源码 pin 可以严格到「`exec.Command` 只允许出现在 `proctree*.go`」。
7. **测试**（`proctree_unix_test.go` + `run_test.go` 追加）：
   - `TestATimedOutRunKillsTheScriptsGrandchildren`：脚本先起一个后台子壳
     `( sleep 0.5; : > marker ) &`，再 `sleep 300`；300ms 超时后断言 `run.TimedOut`，并且等过
     0.5s **marker 没有出现**（判据是「孙子有没有活到写文件」，不依赖 OS 多快回收僵尸）。
   - `TestAFinishedRunSweepsItsProcessGroup`：同一个后台子壳 + `exit 0`；run 成功后 marker 不出现
     （参考 `finish_after_wait`）。
   - `TestAProbeKeepsItsOwnSession`（unix）：`newProbeCommand` 的 `SysProcAttr.Setsid == true`，
     `newExtensionCommand` 的是 `Setpgid == true && Setsid == false`（两个构造器的唯一差别）。
   - `TestTheExtensionSpawnsGoThroughOneConstructor`：扫 `internal/extensions/*.go` 源码，
     `exec.Command(` / `exec.CommandContext(` 只允许出现在 `proctree.go`（含平台文件）里；
     `cmd.Run()` / `cmd.Output()` 只允许出现在 `proctree.go` 里。这条是「只留一个答案」的 pin。

## 4. 行为契约零改动红线

- **高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**：零改动（本轮不碰 shell /
  settings / shortcuts / i18n）。
- **可见行为**：命令的输出、退出码、`TimedOut`、`RunKilledFailure`、provider 错误串、
  探针 `ProbeRecord` 全部逐字不变；唯一变化是「超时/取消/跑完之后，进程组里不该活的进程不活了」。
- **Windows 行为零改动**：参考在 Windows 上也只有直接子进程（`kill_group` 是空实现），
  本仓 Windows 臂照此。
- 探针的 `Setsid`（不碰控制终端）**必须保住**——这是既有 pin 的语义，不是本轮要改的。

## 5. 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
go test -count=1 ./...                   → 全绿
--- PASS                                 → 2207（本轮基线，实测）
--- SKIP                                 → 15（=）
测试函数                                  → 1780（本轮基线，实测）
go test ./... 输出行                      → 36（34 ok + 2 no test files）
GOOS=linux|windows|darwin go build ./... → 全过
```

非视觉轮：本机渲染输出逐字节不变，**不出图**。

## 6. mutation 计划（≥2 条，每条说明哪个 pin 变红）

1. `killProcessGroup` 改成只杀直接子进程（`cmd.Process.Kill()`）：
   → `TestATimedOutRunKillsTheScriptsGrandchildren` 红（孙子 pid 仍活）。
2. `runExtensionCommand` 去掉「跑完扫组」那一次 `killProcessGroup`：
   → `TestAFinishedRunSweepsItsProcessGroup` 红（孙子仍活）。
3. `configureProcessGroup` 去掉 `Setpgid`：→ 两条孙子测试都红
   （`killpg(-pid)` 打不到子进程，pid 不是 pgid）。
4. `newProbeCommand` 丢掉 `Setsid`：→ `TestAProbeKeepsItsOwnSession` 红。
