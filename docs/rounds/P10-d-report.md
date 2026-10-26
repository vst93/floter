# P10-d 报告 · 发现行的连接请求：两处 fallback 搬进纯函数并钉住

规格 `docs/rounds/P10-d-spec.md`。方向来自任务书「现在该做什么」交棒条目（上一轮 P10-c「仍待」第 4 条）。

## 1. 先看交棒条目的前提站不站得住

交棒条目说这两条 fallback「只由异步全链路间接覆盖」。查了一遍，比它说的更糟：

- `grep -rn "connectDetected" internal/ cmd/` 全仓只有三处——`shell.go:821`（装进
  `Settings.Actions.ConnectDetected`）、定义、`system.go:269`（`registerFromLink` 调用）。
  **没有任何测试函数**碰它。
- `registerFromLink`（`system.go:269-273`）自己就把 `Name: filepath.Base(path)` 与
  `VersionArgs: "--version"` 显式填满，所以那唯一一个非 UI 的调用者**连间接都走不到**这两支。
- 测试里的 `ConnectDetected` 全是 settingsui 侧注入的假回调
  （`settingsui/detectedaction_test.go:88`、`view_test.go:3397`、`extensionrowface_test.go:376`），
  它们看到的是**发现行**（点按钮那一刻），不是这层算术。

于是「异步全链路间接覆盖」实际是零覆盖，而 `--version` 这一支是**活的**：
`computeDetectedTools`（`detectedtools.go:172-177`）从不设 `VersionArgs`，每个发现行连接都靠这里补默认值。

## 2. 可达性（写进代码注释的三条）

| 分支 | 可达？ | 依据 |
|---|---|---|
| `Name` 有值 → 用（trim） | 是 | 发现行日常路径；`.desktop` 的 `Name=` 前后带空白时 `unescapeDesktop` 原样留下 |
| `Name` 只剩空白 → 退到路径命令词 | **是** | `apps.go:607-609` 把 `Name=   ` 存成 `"   "`（`:666` 的 `name == ""` 判的是原始空串），行就这样带着空白名进来 |
| `Name` 与 `Path` 都空 | 否 | `computeDetectedTools` 先跳空路径（`detectedtools.go:168`）；register 深链拿到绝对路径或解析结果 |
| `VersionArgs` 空 → `--version` | **是** | 每个发现行连接 |
| `VersionArgs` 只有空白 → 原样保留 | 否 | 发现方从不设它；register 传 `--version` |

最后一行是本轮顺带钉住的一个**古怪但无害**的事实：`"   "` 非空所以原样交出去，
真正把它变空表的是 `customRequest` 的 `strings.Fields`（于是版本探针不跑）。产品里不可达，
钉住只为将来有人想改成「trim 后再判空」时能看到差异。

## 3. 落地（产品 +1 纯函数、+5 测试函数）

`internal/shell/shell.go`：
- `connectDetected`（原 `:2684-2704`，12 行算术）→ 一行 `a.connectLocalTool(detectedToolRequest(tool))`。
- 新增 `detectedToolRequest(settingsui.DetectedTool) settingsui.CustomTool`：Name 的 trim + 兜底、
  VersionArgs 的默认值、`Program`/`Description` 透传、`Platforms: []string{settingsui.CurrentPlatform()}`
  （连同它「不填就被 `platformRequired` 拒」的注释一起搬过去）。
  **行为零改动**：搬的是同几行，字面量 `--version` 没变，顺序没变。

`internal/shell/detectedconnect_test.go`（新文件，5 个函数）：
- `TestADetectedNameIsUsedAsItIs`——有值就用：前后空白被 trim、内部空白（`My Tool`）保留、
  `RG.EXE` **不**被剥（与 fallback 那一支刻意相反）、中文名保留。
- `TestABlankDetectedNameFallsBackToThePathWord`——`" "` / `"\t"` / `"\n"` / 全角空格 `U+3000`
  都落空；`/usr/bin/RG.EXE` → `rg`、`/opt/tools/ripgrep.exe` → `ripgrep`、
  `/usr/bin/rg.exe.txt` → `rg.exe.txt`（只剥一层）、`C:\bin\rg.exe` 在 Linux 宿主上 → `c:\bin\rg`
  （分隔符是宿主的，整条 Windows 路径当**一个**文件名）。
- `TestTheBlankNameFallbackHasItsOwnFloor`——名字与路径都空时给出的**不是**空串而是 `"."`
  （`filepath.Base("")` 的答案，`progname.FromPath` 一路照搬）。这是本轮唯一一次「以为会是 A、
  实测是 B」：我原本断言 `""`，跑出来 `.`，于是把 `.` 连同「产品里不可达」一起钉住。
- `TestVersionArgsDefaultOnlyWhenEmpty`——`""` → `--version`；`-V`/`version`/`--version --verbose` 原样；
  `"   "` 原样（见 §2 末行）。
- `TestTheDetectedRequestCarriesThePathDescriptionAndPlatform`——路径与描述透传、平台 = 宿主、
  表单字段（`Mode`/`Command`/`Args`/`Version`/脚本两项/`Params`）保持零值
  （发现行不经过表单，`customRequest` 才补 `Mode: "executable"`）。

台账：**V = 0 / B 既有 pin 零改动全绿 / S = 0**。本轮不是视觉轮，无像素改动，故不出 PNG
（没有可看的变化）；视觉合约 `docs/raycast-design.md` 未触及。

## 4. 门槛（全部实测，命令即任务书那六条）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿（34 ok + 2 无测试包 = 36 包）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2075（基线 2070，+5）
                                                     SKIP → 15（= 基线）
                                                     FAIL → 0
grep -rn "^func Test" --include=*_test.go internal/ cmd/ | wc -l → 1685（基线 1680，+5）
GOOS=linux/windows/darwin go build ./...               → 三平台过
```

## 5. mutation（4 条，全部实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

`md5sum internal/shell/shell.go` 改前 = `22bad3a57d33bd3deaf9dc099bfbd8aa`，每次还原后 `md5sum -c` 成功。

| # | 注入 | 实测报错（原串） |
|---|---|---|
| M1 | `name := strings.TrimSpace(tool.Name)` → `name := tool.Name` | `Name "  ripgrep  " → "  ripgrep  ", want "ripgrep"` / `Name " " + Path "/usr/bin/rg" → " ", want "rg"` / `Name "" + Path "" → "   ", want "." (filepath.Base of nothing)`（3 个函数红） |
| M2 | 删掉 `versionArgs` 的 fallback | `VersionArgs "" → "", want "--version"` |
| M3 | `Platforms: []string{settingsui.CurrentPlatform()},` 整行删掉 | `Platforms = [], want [linux]` |
| M4 | fallback 改用 `filepath.Base(tool.Path)` | `Name "" + Path "/usr/bin/RG.EXE" → "RG.EXE", want "rg"` / `... "/opt/tools/ripgrep.exe" → "ripgrep.exe", want "ripgrep"` / `... "C:\\bin\\rg.exe" → "C:\\bin\\rg.exe", want "c:\\bin\\rg"` |

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 34 轮，本仓无可改代码。
2. 需拍板四条不变（P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**。
3. `apps.ShortcutTargetName` 对「无扩展名」返回 `""`，参考 `windows.rs:321` 返回整名。行为不可达，
   改它要改既有 pin，**留待拍板**（`shortcut.go:208-210`）。
4. **下一轮第一项**：`detectedToolRequest` 抽出来后，`registerFromLink`（`system.go:269-273`）
   与它**刻意**对同一个程序给出两个名字——register 传 `filepath.Base(path)`（`RG.EXE` 就是 `RG.EXE`），
   发现行落空时才走 `progname.FromPath`（`rg`）。这条分叉今天两条路都只由异步链路间接碰到，
   而且「register 永远走不到 fallback」这件事本身没有 pin。先确认参考里 register 深链是否也保留原名
   （`src-tauri` 的 register/connect 命令），再决定是钉住分叉还是收口。
5. `progname.FromPath("")` 返回 `"."`（`filepath.Base("")` 的答案）。本轮只在 shell 侧钉住，
   没动 `internal/progname`——它有别的调用者，改它要先查全部调用点（`shell.go:2557`、
   `extensions/custom.go:257`、`progname` 自己那族 pin）。**不是本轮范围**，记在这里。
