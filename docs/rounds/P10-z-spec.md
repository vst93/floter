# P10-z 规格：应用启动走唯一的 spawn——Windows 命令脚本能被启动，detach 只剩一份

方向：优先级 4（三平台一致性 → 基础功能 → 像素细节）。P10-y 报告「仍待」第 8 条指到
`internal/spawn`，核 `detach_unix.go` / `detach_windows.go` 时发现**真正的分歧不在 spawn 里**：
应用启动根本不经过 spawn。

## 1. 意图（一句话）

`internal/apps` 自己复制了一份 detach（`apps/detach_unix.go`、`apps/detach_windows.go`），
于是参考在 `spawn_application` 里做的两件事——**Windows 的 `.cmd`/`.bat` 换解释器**（R74）与
**Windows 的 `CREATE_NO_WINDOW`**——在「点一行应用」这条路径上根本无处可放；
把这条路径接回 `internal/spawn`，让 detach 与解释器跳转只有一份答案。

## 2. 证据（动手前实测行号）

| 位置 | 现状 |
| --- | --- |
| `internal/apps/apps.go:116` `App.Open` | 应用启动的唯一出口 |
| `internal/apps/apps.go:133-145` `launchDetached` | 自建 `exec.Command` + 自建 detach + 自建 reap 线程 |
| `internal/apps/detach_unix.go:14` / `detach_windows.go:11` | 与 `internal/spawn/detach_*.go` 逐字重复（注释自称 "restated so the two cannot drift"） |
| `internal/apps/shortcut.go:194-200` `IsLaunchableTarget` | Start Menu 的 `.exe`/**`.bat`**/**`.cmd`** 都算可启动 |
| `internal/apps/apps.go:311` | 上面这条判据真的被 Start Menu 扫描用来收行 |
| `internal/spawn/spawn.go:53` `programCmd` | 每个启动路径都经过这里，但对 `.cmd`/`.bat` 没有任何处理 |
| 参考 `main:src-tauri/src/process_launch.rs:236-262` `windows_cmd_shim_wrap` | `.cmd`/`.bat` → `cmd /c <program> <args…>`，只按**程序**后缀判断，大小写无关 |
| 参考 `process_launch.rs:74-86` | Windows 旗标 = `DETACHED_PROCESS \| CREATE_NEW_PROCESS_GROUP \| CREATE_NO_WINDOW` |

**可达性**：Windows 上 Start Menu 里一个 `.cmd`/`.bat` 条目会被 `IsLaunchableTarget` 收进列表，
用户点它 → `App.Open` → `CreateProcess` 拒绝脚本文件（ERROR_BAD_EXE_FORMAT）→
「列得出来、点不动」，并且走 `OpenAppFailed` 提示。参考在 `spawn_application` 里恰好修掉了这一格。

## 3. 落地项

### 3.1 `internal/spawn`：解释器跳转成为唯一判据（纯函数）

- 新增 `cmdScriptWrap(program string, args []string) ([]string, bool)`：参考
  `windows_cmd_shim_wrap` 的逐条翻译——后缀 `.cmd`/`.bat`（大小写无关）才包，
  返回 `{"cmd","/c",program,args…}`；`mycmd`、`tool.cmd.exe`、`C:\x\a.exe` 原样。
- 新增 `programArgv(goos, program string, args []string) (string, []string)`：**平台**决定要不要包
  （Linux/macOS 上 `.cmd` 只是一个文件名，绝不跳转）；`programCmd`（`spawn.go:53`）改为调用它，
  于是 `Program`/`ProgramIn`/`TerminalAt` 的每个候选一次全部覆盖，无第二个入口。

### 3.2 `internal/spawn/detach_windows.go`：旗标补齐

`DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP` → 再加 `CREATE_NO_WINDOW`（参考 `:74-86`；
Windows 文档写明它与 `DETACHED_PROCESS` 同用时被忽略，所以**零行为变化**，只是把参考的旗标集合对齐）。

### 3.3 `internal/apps`：删掉自己那份 detach

- 删 `internal/apps/detach_unix.go`、`internal/apps/detach_windows.go`。
- `launchDetached` 变成 `launchProgram(homeDir(), program, args...)`，其中
  `var launchProgram = spawn.ProgramIn`（**seam**：让「应用启动=spawn」这件事可被测试读出来）。
  `homeDir()` 保留 —— 现有行为（应用以 home 为 cwd 启动）不动。
- `apps.go` 去掉不再使用的 `os/exec` 导入。

## 4. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
本轮不碰任何 UI 文件，本机输出逐字节不变（不出图）。
启动语义**唯一变化**是 Windows 的 `.cmd`/`.bat` 从「失败」变成「启动」，以及 Linux/macOS 完全不变。
`gio launch` 的 D-Bus 回退、`homeDir()` 的 cwd、`mygo.Shell.OpenPath` 的无 argv 分支全部保持。

## 5. 门槛与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净
go test -count=1 ./...       # 全绿
--- PASS 计数 2204（基线 2204，本轮 +3 起） / --- SKIP 15 / --- FAIL 0 / 1777 测试函数
GOOS=linux|windows|darwin go build ./...
```

## 6. mutation 计划（每条都要实跑变红 + `md5sum -c` 还原 + `touch` 复跑）

1. `cmdScriptWrap` 的后缀判据从 `.cmd`/`.bat` 改成 `.exe` → `TestAWindowsCommandScriptIsGivenToTheInterpreter` 变红。
2. `programArgv` 去掉 `goos == "windows"` 的闸门（任何平台都包）→ `TestACommandScriptIsOnlyWrappedOnWindows` 变红。
3. `launchDetached` 的 `launchProgram` 换回 `exec.Command` 直连（或漏掉 `homeDir()`）→
   `TestTheAppLaunchPathIsTheSpawnPackagesOwn` 变红。

## 7. V / B / S 台账

预期 **V=0 / B=0 / S=1**（S：`internal/apps` 的 detach 文件删除、`launchDetached` 结构改为转发）。
