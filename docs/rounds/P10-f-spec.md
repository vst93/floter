# P10-f 规格：「这个名字是不是程序」在 `extensions` 里用了另一条判据

方向：任务书「选题」优先级 3 —— 上一轮（P10-e）报告「仍待」第 5 条。它是本轮唯一不需要拍板、
也不是等用户实机复测的条目。规格写完先核对行号，行号以本文件写下的时刻为准。

## 意图（一句话）

`extensions.LookTool` / `LookTools` 是**名字探针**（系统替用户按名字找程序），必须和仓里其它
三处名字探针用同一条严格判据 `progtype.For`；宽松判据 `progtype.ForLenient` 只留给「用户自己
点选了一个已存在的路径」。

## 为什么现在做（证据）

`internal/progtype/progtype.go:8-21` 自己把两套判据的分工写死了：

- 扩展名判据（`For` / `Host`）：**系统替用户按名字探**，错了会把非程序放进列表
  （inventory 的导出遍历、PATH 探针、`.desktop` 的 `TryExec=`）；
- 宽松判据（`ForLenient` / `HostLenient`）：**用户自己点选的路径**，错了会把用户刚选中的程序藏起来。

逐条对照本仓的名字探针：

| 名字探针 | 位置 | 判据 |
|---|---|---|
| 工具目录的 PATH 探针 | `internal/tools/catalog.go:307`（`Probe`）、`:333`（`managerPresent`） | `progtype.For` ✅ |
| `.desktop` 的 `TryExec=` / 裸 `Exec=` | `internal/apps/apps.go:485,491`（`executableAvailable`） | `progtype.Host` ✅ |
| inventory 的 PATH 发现 | `internal/inventory/inventory.go:146`（`executableFile = progtype.Host`）、`sources.go:442`（`LookPath`） | `progtype.Host` ✅ |
| **扩展系统的名字解析** | `internal/extensions/runtimepath.go:120`（`lookToolIn`）、`candidates.go:77`（`lookToolsIn`） | **`progtype.ForLenient` ❌** |

而 `HostLenient` 在本仓只出现在「用户给了路径」的三处：`provider.go:53`（已绑定路径）、
`reconnect.go:68`（重连时用户指定路径）、`custom.go:253`（自定义集成的可执行文件）。它们**不该**动。

参考实现的答案也是严格：PATH 发现逐条目 `is_executable`（`src-tauri/src/extensions/inventory.rs:439`，
判据本体 `:1088-1120`：Windows 只认 `exe/com/bat/cmd`），register 深链的解析走同一份 inventory
候选（`src-tauri/src/deep_link.rs:1136-1180` 的 `resolve_registered_command_in`），所以参考的
「按名字找」从不接受 Windows 上的无扩展名文件。

### 可达的后果（不是理论）

`lookToolIn` 的三个消费者都拿结果当「可以 spawn 的程序」：

1. `internal/shell/system.go:254` —— `floter://register?cmd=…` 解析后直接连；
2. `internal/inventory/sources.go:264`（`desktopLookup`）—— 裸 `Exec=` 的落地文件；
3. `internal/extensions/runtimepath.go:173`（`FindInterpreter`）。

Windows 上搜索路径里一个无扩展名的 `notes.txt`（或任何 `tool.exe.txt` 之外的普通文件，只要
名字被探到）会被判成程序：深链连上一个跑不起来的集成，`.desktop` 行指向一个非程序。
`lookToolsIn` 更直接：它把同一个文件塞进候选列表给选择器。

## 落地项（位置 → 现状 → 应改成）

1. `internal/extensions/runtimepath.go:120`
   现状 `if progtype.ForLenient(platform, candidate) {` → 改成 `progtype.For(platform, candidate)`。
2. `internal/extensions/runtimepath.go:105-111` 文档段
   现状末句「The judgement stays `progtype.ForLenient` — the permissive strategy for a name the
   user gave, unchanged by this round.」→ 改成「判据是 `progtype.For`，因为名字是**探针**不是
   用户点选的路径；`ForLenient` 属于 `provider.go` / `reconnect.go` / `custom.go` 那三处」，并写明
   参考的 PATH 发现与 register 解析都是这条严格判据。
3. `internal/extensions/candidates.go:77`
   现状 `if !progtype.ForLenient(platform, path) {` → `!progtype.For(platform, path)`，注释补一句
   为什么（候选列表是「系统替用户找」的那一面，错了就把非程序列给用户点）。
4. `internal/extensions/probeladder_test.go:81-105`（`TestTheLadderOrderIsTheSuffixOrderThenTheBareName`）
   现状 `order` 含最后一格裸名 `tool`，并断言「只剩裸名时仍解析」。严格判据下 Windows 的裸名文件
   不是程序（参考同样如此），这条 pin 钉的是旧判据 → 改成：循环只走四个后缀级，裸名单独一条断言
   「它在梯子上、但判据拒绝它」，并写清这是**故意**与 `tools.Probe` 一致的答案。断言变强不是变弱
   （旧断言只是「有文件就行」，新断言同时说了梯子顺序与判据）。
5. 新增 `internal/extensions/probejudgement_test.go`，把「两句话」第一次钉在一起：
   - 同一 fixture 上 `lookToolIn("windows", …)` 拒绝无扩展名文件，而 `progtype.ForLenient("windows", 同一路径)`
     接受它 —— 名字探针 vs 用户点选路径的分歧是**定位好的**，不是漏网；
   - 与 `tools.Probe`（另一处名字探针）逐格对照：同一目录、同一组文件，两边答案必须逐格相同；
   - `lookToolsIn` 不把非程序列成候选；
   - `notes.txt` 不会被当成 `notes.txt.exe` 的替代，`tool.exe.txt` 也不会；
   - Unix 半边不变：有执行位的裸名两边都接受（`For` 与 `ForLenient` 在 Unix 上是同一条）。

## 行为契约零改动红线

只改「Windows 上名字探针遇到无扩展名文件」这一个判断；高度链、键盘模型、会话管理、设置写回、
i18n 文案、命令接线一律不动。Linux / macOS 行为**逐字节不变**（两套判据在 Unix 上同一实现），
Windows 上带四个启动器后缀的解析结果也不变（`For` 接受它们）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                 期望空
go vet ./...                          期望净
go test -count=1 ./...                期望 36 包全绿
--- PASS                              基线 2082
--- SKIP                              基线 15
--- FAIL                              基线 0
测试函数                               基线 1692
GOOS=linux / windows / darwin build    全过
```

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. 把 `runtimepath.go` 的 `progtype.For` 改回 `ForLenient` →
   `TestANameProbeRefusesAFileWindowsCannotRun`（新）与 `TestTheLadderOrderIsTheSuffixOrderThenTheBareName`
   的裸名断言变红。
2. 把 `candidates.go` 的 `progtype.For` 改回 `ForLenient` →
   `TestTheCandidateListUsesTheSameJudgementAsTheLookup`（新）变红。
3. 把 `lookToolIn` 的 `progname.Ladder` 去掉（回到裸 `filepath.Join`）→
   P10-e 的 `TestABareNameFindsTheLauncherOnWindowsOnly` 变红（证明本轮没有把上一轮的地基拆掉）。

每条都要：注入 → 看到红（原始报错串抄进报告）→ 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
