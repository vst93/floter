# P10-e 报告：「名字 → 程序」在 Windows 上只有一个答案

方向：任务书「选题」优先级 4（三平台一致性 → 基础功能）自选，理由是**同一台机器上两条解析路径
给出相反答案**——发现行里一按就连上的 `rg`，深链说「找不到」。规格
`docs/rounds/P10-e-spec.md`。

## 1. 先纠正 P10-d 交棒条目的前提（它说的分歧不存在）

交棒条目说 `registerFromLink`（`system.go:269-273`，`Name: filepath.Base(path)` → `RG.EXE`）与
`detectedToolRequest`（`Name` 落空 → `progname.FromPath` → `rg`）对同一个程序**刻意**给两个名字。
逐条核对后**不成立**：

| 面 | 位置 | 同一个 PATH 文件（Windows）的答案 |
|---|---|---|
| PATH 发现给候选的名字 | `inventory.go:239` `merge(path, entry.Name(), …)` | `rg.exe`（文件基名） |
| register 深链给卡片的名字 | `system.go:270` `filepath.Base(path)` | `rg.exe` |
| 参考的候选名 | `inventory.rs:434-451` `path.file_name()` | `rg.exe` |
| 参考的 register 显示名 | `deep_link.rs:1132-1205` `RegisterRequest.command` | 用户**打的**那个词 |
| `progname.FromPath` | `progname.go:100` | `rg` |

`progname.FromPath` 只在发现名**全空白**时兜底（P10-d 的 `detectedToolRequest`），而 register 自己
永远带着名字（`system.go:253-262`：要么是链接里的 `cmd`，要么是解析出的绝对路径），**走不到那条
兜底**。所以两扇门本来就给同一个名字，不需要收口也不需要钉「分叉」。本轮把这条一致性做成了
结构性事实：候选名改成「找到的文件名」，三个消费者（选择器行 / 发现行 / register 卡片）读同一个
`filepath.Base`（`probeladder_test.go` 的 `TestTheCandidateIsNamedAfterTheFileNotTheProbe`）。

参考的 register 深链**不会**自己连接：它解析出候选，然后去 Detected 列表里**标记**匹配的那一行
（`ExtensionsPanel.tsx:1500-1548`），连接仍由用户按；本仓按 P9-m/P9-n 的既定取舍一次连完并标记结果，
名字取自同一条路径。两边的名字因此一致，交互不同（已在前几轮记录，不是本轮改动）。

## 2. 本轮真正的病灶：名字带不带启动器后缀，只有目录探针知道

`extensions.LookTool` / `LookTools` 把名字直接 `filepath.Join(dir, name)` 再 stat
（改前 `runtimepath.go:94-107`、`candidates.go:53-77`），**没有** `progname.Ladder`；而
`tools.Probe`（`catalog.go:300-312`）与 `tools.DetectManagers` 早就在用那张表。Windows 上程序叫
`rg.exe`，于是：

1. `floter://register?cmd=rg`（`docs/extensions/README.zh-CN.md:100` 承诺的拼写）→
   `system.go:254` → 「找不到」；
2. `FindInterpreter("powershell")` 的候选名是裸的 `pwsh` / `powershell`
   （`runtimepath.go:109-118`）→ 永远解析不到 `pwsh.exe` → 脚本类工具报「没有解释器」；
3. `system` 型 manifest 的候选枚举（`SystemRuntimeCandidates` → `lookToolsIn`）同样只认带后缀的拼写。

参考：`resolver.rs:118-133` `executable_name_matches` 在 Windows 上对「不带扩展名的期望名」额外接受
`.exe/.cmd/.bat/.com`——同一个答案，只是它写在解析器里，本仓写在 `progname` 里。

## 3. 落地（产品 2 个文件，测试 4 个文件）

- `internal/extensions/runtimepath.go`：`lookToolIn(platform string, dirs []string, names ...string)`；
  内层 `progname.Ladder(platform, name)`；判据 `progtype.ForLenient(platform, …)`；`LookTool` 传
  `runtime.GOOS`。循环顺序保持**目录在外、名字在中、梯子在内**（搜索路径优先级压过后缀优先级）。
- `internal/extensions/candidates.go`：`lookToolsIn(platform, …)` 同上；候选的 `Name` 由请求名改成
  找到的文件名（`probe`）。
- 测试侧仅改调用点（签名多一个参数）：`provider_test.go:323-327`、`registry_test.go:315`、
  `candidates_test.go:33,54`——**断言一字未动**。
- 新文件 `internal/extensions/probeladder_test.go`（7 个测试函数）：裸名在 Windows 找到 `.exe` /
  同一 fixture 在 linux+darwin 找不到 / 解释器名走梯子 / 梯子顺序 `.exe>.cmd>.bat>.com>裸名`
  （逐级降级的 5 个子用例 + `tool.exe.txt` 不算）/ 目录顺序压过后缀 / 候选名是文件基名而不是命令词 /
  空名即使在 Windows 上也不解析 / 宿主行为对照。

## 4. 台账

**V = 0**（非视觉轮，无像素、无 token、无 PNG）。
**B = 0**（既有 pin 零改动：只有 5 处调用点补参数，断言未动）。
**S = 5 处调用点**（`lookToolIn`/`lookToolsIn` 多一个 `platform` 参数：旧 `(dirs, names…)` → 新
`(platform, dirs, names…)`；理由是把平台变成可注入的量，Linux 的 CI 才钉得住 Windows 的答案，
与 `progtype.For(platform, path)`、`tools.Probe(entry, dirs, platform)` 同一形状）。

## 5. 门槛（实测）

```
gofmt -l cmd internal                 空
go vet ./...                          净
go test -count=1 ./...                36 包全绿
--- PASS                              2082   （基线 2075，+7 = 本轮 7 个新函数）
--- SKIP                              15     （= 基线）
--- FAIL                              0
测试函数                               1692   （基线 1685，+7）
GOOS=linux / windows / darwin build    全过
```

## 6. mutation（4 条实跑，逐条变红，`md5sum -c` 逐字节还原 + `touch` 复跑）

| # | 注入 | 红的是 | 原始报错串 |
|---|---|---|---|
| M1 | 梯子换成宿主平台（`Ladder("", name)`，Windows 上等于不补后缀） | 4 个函数 | `windows lookToolIn("rg") = "", false; want the .exe` · `windows lookToolIn([pwsh powershell]) = "", false; want pwsh.exe` · `lookToolIn = "", false; the first directory wins over the better suffix` · 梯子顺序 4 行 |
| M2 | 梯子结果反序（裸名优先） | 顺序 pin | `with [tool.exe tool.cmd tool.bat tool.com tool] present, lookToolIn("tool") = "…/tool", true; want "…/tool.exe"`（共 4 行） |
| M3 | 候选 `Name` 改回请求名 | 命名 pin | `candidate Name = "rg"; want the file's own name "rg.exe"` · `register would call it "rg.exe"` |
| M4 | 空名守卫移到梯子之后 | 空名 pin | `lookToolIn("") = "…/.exe"; an empty name names nothing` |

还原后 `md5sum -c /tmp/p10e.md5` 两行皆 `成功`；`gofmt -l internal` 空。

## 7. 取舍与已知限制（写清，不留悬念）

1. **判据不改**：仍是 `progtype.ForLenient`（Windows 上「存在且不是目录」），不是目录探针的
   `For`（扩展名策略）。名字是用户给的，宽松是它原本的选择；本轮只补「名字 → 候选文件名」这一层。
2. **大小写**：梯子是按用户给的名字拼的，所以是**小写**（`rg` → `rg.exe`）。Windows 文件系统不区分
   大小写，同一个文件在 `RG.EXE` 名下也能解析，但回给调用者的路径/名字是探针拼写。要做到与
   `read_dir` 的原名一致需要每个探针读一次目录，本轮不花这个 I/O；这条限制被
   `TestTheCandidateIsNamedAfterTheFileNotTheProbe` 的最后一个子用例**明着钉住**（大小写不敏感的
   文件系统上期望解析成功，敏感的文件系统上期望不解析）。
3. **不在 shell 层造第二套**：`registerFromLink` 一行未动（它拿到的路径经 `filepath.Base` 命名，
   与发现行一致）；改的是解析，不是命名。

## 8. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 35 轮，本仓无可改代码。
2. 需拍板四条不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**。
3. `apps.ShortcutTargetName` 对「无扩展名」返回 `""`（参考 `windows.rs:321` 返回整名）。行为不可达，
   改它要改既有 pin，**留待拍板**（`shortcut.go:208-210`）。
4. 探针拼写的小写化（本文 §7.2）：要「与目录里的原大小写一致」得加一次目录读；留待需要时再做。
5. **下一轮第一项**：`LookTool` 判据 `ForLenient` 与目录探针 `For` 的分歧——同一台 Windows 机器上，
   用户点选路径（宽松）与系统替用户找名字（扩展名策略）是两个答案，今天两边各自有 pin 但**没有一条
   同时说这两句**；先读参考 `is_executable` 在 PATH 发现与 `inspect_executable` 两处的用法再定。
