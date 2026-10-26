# P10-f 报告：「这个名字是不是程序」——名字探针用严格判据，点选路径才宽松

方向：任务书「选题」优先级 3（上一轮报告「仍待」第 5 条）。规格 `docs/rounds/P10-f-spec.md`。

## 1. 病灶：同一个问题，`extensions` 用了另一条判据

`progtype` 这个包自己的文档（`progtype.go:8-21`）就把两套判据的分工写死了：**名字探针**用扩展名
判据（错了会把非程序放进列表），**用户点选的路径**用宽松判据（错了会把用户刚选的程序藏起来）。
逐条核对本仓的名字探针，只有 `extensions` 的两处站错了队：

| 名字探针 | 位置 | 改前判据 |
|---|---|---|
| 工具目录的 PATH 探针 | `tools/catalog.go:307` `Probe` / `:333` `managerPresent` | `progtype.For` |
| `.desktop` 的 `TryExec=` / 裸 `Exec=` | `apps/apps.go:485,491` `executableAvailable` | `progtype.Host` |
| inventory 的 PATH 发现 | `inventory/inventory.go:146`、`sources.go:442` `LookPath` | `progtype.Host` |
| **扩展系统的名字解析** | `extensions/runtimepath.go:134` `lookToolIn`、`candidates.go:82` `lookToolsIn` | **`progtype.ForLenient`** |

`HostLenient` 在本仓只出现在三处「用户给了路径」：`provider.go:53`（已绑定路径）、
`reconnect.go:68`（重连指定路径）、`custom.go:253`（自定义集成的可执行文件）。它们不动。

参考实现的答案同样是严格：PATH 发现逐条目 `is_executable`（`inventory.rs:439`，判据本体
`:1088-1120`：Windows 只认 `exe/com/bat/cmd`），register 深链的解析走同一份 inventory 候选
（`deep_link.rs:1136-1180`），所以参考的「按名字找」从不接受 Windows 上的无扩展名文件。

可达后果（不是理论）：Windows 搜索路径里一个无扩展名的普通文件会被
`floter://register?cmd=…`（`shell/system.go:254`）连成集成、被 `.desktop` 裸 `Exec=` 当成落地
文件（`inventory/sources.go:264` `desktopLookup`）、被 `SystemRuntimeCandidates` 塞进选择器
（`candidates.go:82`）。

## 2. 落地（产品 2 行 + 2 段注释，测试 2 个文件）

- `internal/extensions/runtimepath.go:134`：`progtype.ForLenient` → `progtype.For`；`:112-124` 文档段
  改写成「名字是探针不是点选路径」，并点名另外三处名字探针、参考的 `is_executable`、以及宽松判据
  真正属于的那三个文件。
- `internal/extensions/candidates.go:82`：同上（`!ForLenient` → `!For`）；`:56-62` 补一段
  「选择器是最不能宽松的地方」。
- `internal/extensions/probeladder_test.go`：`TestTheLadderOrderIsTheSuffixOrderThenTheBareName` 的
  循环只走四个后缀级；裸名单独一条断言——**它在梯子上，但 Windows 判据拒绝它**，Unix 上它仍是
  唯一拼写。旧断言钉的是旧判据，注释写明 `P10-f 重定判据`。
- 新增 `internal/extensions/probejudgement_test.go`（5 个函数）：
  1. `TestANameProbeRefusesAFileWindowsCannotRun`：**两句话第一次同时说**——同一文件名字探针拒绝、
     点选路径（`ForLenient`）接受；同目录的 `notes.txt.exe` 仍然解析（对照，证明拒绝只关于后缀）。
  2. `TestTheCandidateListUsesTheSameJudgementAsTheLookup`：`LookTool` 拒的，`LookTools` 不许列。
  3. `TestTheNameProbesAgreeWithTheToolCatalog`：与 `tools.Probe` 逐格对照（7 组 fixture），
     连选中的文件名也一致。
  4. `TestTheSuffixIsNotInventedFromAnotherFile`：`tool.exe.txt` / `tool.exe.old` 不是 `tool`。
  5. `TestTheUnixHalfIsUnchangedByTheJudgement`：Unix 上两套判据是同一条，有执行位就解析。

## 3. 台账

**V = 0**（非视觉轮：无像素、无 token、无 PNG）。
**B = 0**（行为契约零改动：高度链、键盘、会话、设置写回、i18n、命令接线未动；Linux/macOS 逐字节
不变，Windows 上四个启动器后缀的解析结果也不变）。
**S = 1 处断言重写**：`probeladder_test.go` 的裸名子用例，旧「只剩裸名仍解析」→ 新「裸名在梯子上、
Windows 判据拒绝它」，断言变强（同时说梯子顺序与判据），注释写明旧值 → 新值。

## 4. 门槛（实测）

```
gofmt -l cmd internal                 空
go vet ./...                          净
go test -count=1 ./...                36 包全绿（34 有测试 + 2 无测试文件）
--- PASS                              2087   （基线 2082，+5 = 本轮 5 个新函数）
--- SKIP                              15     （= 基线）
--- FAIL                              0
测试函数                               1697   （基线 1692，+5）
GOOS=linux / windows / darwin build    全过
```

## 5. 变异（3 条实跑，全部变红并逐字节还原）

1. `runtimepath.go` 的 `progtype.For` 改回 `ForLenient` →
   ```
   --- FAIL: TestANameProbeRefusesAFileWindowsCannotRun (0.00s)
       probejudgement_test.go:40: windows lookToolIn("notes.txt") = ".../001/notes.txt"; a file with no launcher suffix is not a program
       probejudgement_test.go:40: windows lookToolIn("tool") = ".../001/tool"; a file with no launcher suffix is not a program
   --- FAIL: TestTheNameProbesAgreeWithTheToolCatalog (0.00s)
       probejudgement_test.go:114: files [tool], request "tool": tools.Probe = "", lookToolIn = ".../002/tool", true; the two name probes must agree
   --- FAIL: TestTheLadderOrderIsTheSuffixOrderThenTheBareName (0.00s)
       probeladder_test.go:108: lookToolIn("tool") = ".../005/tool"; a file Windows cannot run by name is not a program
   ```
2. `candidates.go` 的 `progtype.For` 改回 `ForLenient` →
   ```
   --- FAIL: TestANameProbeRefusesAFileWindowsCannotRun (0.00s)
       probejudgement_test.go:43: windows lookToolsIn("notes.txt") = [{notes.txt .../notes.txt [path]}]; a chooser must not offer a file Windows cannot run
   --- FAIL: TestTheCandidateListUsesTheSameJudgementAsTheLookup (0.00s)
       probejudgement_test.go:76: name "tool": lookToolIn ok = false, lookToolsIn listed = true; one name, one judgement
       probejudgement_test.go:84: lookToolsIn = [{tool.exe ...} {tool ...}]; want the one launcher
   ```
3. `lookToolIn` 只走梯子最后一格（等于拆掉 P10-e 的梯子）→
   ```
   --- FAIL: TestABareNameFindsTheLauncherOnWindowsOnly (0.00s)
       probeladder_test.go:46: windows lookToolIn("rg") = "", false; want the .exe
   --- FAIL: TestTheInterpreterNamesResolveThroughTheLadder (0.00s)
       probeladder_test.go:70: windows lookToolIn([pwsh powershell]) = "", false; want pwsh.exe
   ```
   第三条同时证明本轮没有把上一轮的地基拆掉。

每条都：注入 → 看到红 → `cp` 还原 → `md5sum -c` 两份文件「成功」→ `touch` 后复跑绿。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 36 轮，本仓无可改代码。
2. 需拍板四条不变（P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `apps.ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. 探针拼写是小写（梯子按用户给的名字拼），与 `read_dir` 原名在 Windows 上可能只差大小写；
   要与原名一致得每个探针读一次目录，**本轮仍未花这个 I/O**（P10-e 已钉住限制）。
4. **下一轮第一项**：`registerFromLink` 的绝对路径分支（`shell/system.go:250-253`）**完全不判**
   「是不是程序」——`floter://register?cmd=C:\notes.txt` 会直接连；参考的 register 解析走 inventory
   候选（`deep_link.rs:1136-1180` → `resolver.rs`），绝对路径也在同一份候选里。先读参考确认
   「绝对路径不在 PATH 里」时它的答案，再决定本仓是拒绝还是走宽松判据（可能触及 i18n 文案，需拍板）。
