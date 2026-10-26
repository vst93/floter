# P10-e 规格：探针名字在 Windows 上走同一个梯子（`LookTool`/`LookTools` 接 `progname.Ladder`）

## 意图（一句话）

「用一个名字找程序」这件事在 Windows 上今天只有目录探针（`tools.Probe`）会补启动器后缀，
`extensions.LookTool` / `LookTools` 不补——同一个名字，两条解析路径两个答案。本轮把
`extensions` 的两条也接到 `progname.Ladder` 上，让「名字 → 程序」三平台只留一个答案。

为什么现在做：这是任务书「三平台一致性 → 基础功能」的第一档，且**同一台机器上两条路径答案不同**
（发现行能连上的程序，深链说找不到）。

## 病灶（动手前实测的行号）

| 位置 | 现状 |
|---|---|
| `internal/extensions/runtimepath.go:94-107` `lookToolIn` | `filepath.Join(dir, name)` 直接 stat，**没有梯子**；Windows 上 `C:\…\rg` 不存在 → 找不到 |
| `internal/extensions/candidates.go:53-77` `lookToolsIn` | 同上（`Name: name` 存的是**请求名**，不是找到的文件名） |
| `internal/extensions/runtimepath.go:101` | 判据是 `progtype.HostLenient`，Windows 上 = 「文件存在且不是目录」 |
| `internal/tools/catalog.go:366-370` `candidateNames` | **已经有**梯子（`progname.Ladder(platform, name)`），`tools.Probe` 因此能解析 `rg` → `rg.exe` |

后果（都是活路径，不是理论）：

1. `floter://register?cmd=rg`（`docs/extensions/README.zh-CN.md:100` 承诺的拼写）→
   `shell/system.go:254` `extensions.LookTool(name)` → Windows 上「找不到」，
   而同一台机器上发现行里的 `rg` 一按就连（发现行给的是完整路径）。
2. `FindInterpreter("powershell")`（`runtimepath.go:109-118` 给的是 `pwsh` / `powershell`，
   裸名）→ Windows 上永远解析不到 `pwsh.exe` → 脚本类工具报「没有解释器」。
3. `system` 型 manifest 的探针名（`SystemRuntimeCandidates` → `lookToolsIn`）同样只认带后缀的拼写。

参考实现的做法（`main:src-tauri/src/extensions/resolver.rs:118-133`
`executable_name_matches`）：Windows 上「不带扩展名的期望名」额外接受
`.exe/.cmd/.bat/.com` 四种拼写；`main:src-tauri/src/extensions/inventory.rs:434-451`
PATH 发现给候选的 `name` 是 `path.file_name()`（带后缀、带原大小写）。

## 逐条落地项

### 1. `lookToolIn` / `lookToolsIn` 接梯子，平台作为参数（可 pin）

- 位置：`internal/extensions/runtimepath.go`、`internal/extensions/candidates.go`
- 现状：签名 `(dirs []string, names ...string)`，循环体 `filepath.Join(dir, name)`。
- 应改成：签名 `(platform string, dirs []string, names ...string)`；
  内层 `for _, probe := range progname.Ladder(platform, name)`；判据
  `progtype.ForLenient(platform, filepath.Join(dir, probe))`（Windows 上仍是「宽松」策略：
  存在且不是目录——**本轮不改**这条判据，见取舍）。
- 循环顺序保持 **目录在外、名字在中、梯子在内**：搜索路径优先级高于后缀优先级
  （`dir1/rg.cmd` 胜 `dir2/rg.exe`），与今天的目录顺序契约一致。
- `LookTool` / `LookTools` 传 `runtime.GOOS`，宿主行为在 Linux/macOS 上逐字节不变
  （`Ladder` 非 Windows 只返回裸名）。

### 2. 候选的 `Name` 改成**找到的文件名**

- 位置：`internal/extensions/candidates.go:73`
- 现状：`Name: name`（请求名）。
- 应改成：`Name: probe`（梯子上的那个文件名）。理由：`ExecutableCandidate.Name` 的注释
  是「the executable file's own name」，参考的候选名就是 `file_name()`
  （`inventory.rs:434-451`），而 `registerFromLink`（`shell/system.go:269`）与 PATH 发现
  （`inventory.go:239` `entry.Name()`）都用文件基名。三处对齐后，同一个程序在「选择器行 /
  发现行 / register 卡片」上是同一个名字。
- Linux/macOS 上 `probe == name`，零变化。

### 3. pin（新文件 `internal/extensions/probeladder_test.go`）

1. 裸名在 Windows 上找到 `rg.exe`；同样的 fixture 在 `linux`/`darwin` 上**找不到**
   （分歧是故意的，钉住）。
2. 梯子顺序：`.exe` > `.cmd` > `.bat` > `.com` > 裸名（5 个子用例逐个降级）。
3. 搜索路径顺序压过后缀顺序：`dir1/rg.cmd` 胜 `dir2/rg.exe`。
4. `LookTools` 的 `Name` 是文件基名（`rg.exe`），且**不是** `progname.FromPath` 的词干
   （`rg`）——把「候选面 / 命令词面」的分歧钉在解析这一层。
5. 空名字即使在 Windows 上、即使目录里有 `.exe`，也不解析（`Ladder("windows","")` 会生成
   `.exe` 等，守卫必须在梯子之前）。

### 4. 顺带钉住 P10-d 交棒条目的答案（写进报告，不另开测试文件）

交棒条目担心 `registerFromLink` 的 `filepath.Base` 与 `detectedToolRequest` 的
`progname.FromPath` 对同一个程序给两个名字。实测**不成立**：`filepath.Base` 与 PATH 发现
（`inventory.go:239` `merge(path, entry.Name(), …)`）以及参考的 `file_name()` 是同一个拼写；
`progname.FromPath` 只在发现名**全空白**时兜底，而 register 自己永远带着名字。
本轮第 2 条的 `Name: probe` 让这条一致性有了 pin（第 3 节第 4 条）。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不动**。
- 非视觉轮：无像素、无 token、无 PNG 要求。
- `LookTool` 的**宿主**行为（Linux/macOS）逐字节不变；`progtype` 的宽松判据不改；
  `tools.Probe` 不改。

## 取舍（写清，不留悬念）

- **不改判据**：`lookToolIn` 用 `ForLenient`（Windows 上「任何存在的文件」）而不是目录探针的
  `For`（扩展名策略）。名字是用户给的，宽松策略是它原本的选择；本轮只补「名字 → 候选文件名」
  这一层，判据留给拍板。
- **不在 shell 层造第二套**：`registerFromLink` 不动，它拿到的路径经 `filepath.Base` 命名，
  与发现行一致；改的是解析，不是命名。

## 门槛与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
go test -count=1 ./...                # 36 包全绿
--- PASS 计数                          # 基线 2075
--- SKIP 计数                          # 基线 15
测试函数数                             # 基线 1685
GOOS=linux/windows/darwin go build ./...  # 全过
```

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `lookToolIn` 内层去掉梯子（`[]string{name}`） | 第 1 条（Windows 裸名） |
| M2 | `Ladder` 结果反序（裸名优先） | 第 2 条（梯子顺序） |
| M3 | `lookToolsIn` 的 `Name` 改回 `name` | 第 4 条（基名 vs 词干） |
| M4 | 空名守卫移到梯子之后 | 第 5 条（空名） |
