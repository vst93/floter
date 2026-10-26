# P10-d 规格 · 发现行的连接请求：两处 fallback 搬进纯函数并钉住

## 意图（一句话）

把 `connectDetected` 里那两段「发现行没说全就补一个默认值」的算术搬进纯函数
`detectedToolRequest(settingsui.DetectedTool) settingsui.CustomTool`，并用直接 pin 钉住三件事：
**Name 有值就用（只 trim）、Name 只剩空白就退到路径的命令词、VersionArgs 空才补 `--version`**。
产品语义零改动。

## 为什么现在做（证据）

- 现状算术在 `internal/shell/shell.go:2684-2704`（`connectDetected`）：`strings.TrimSpace(tool.Name)`
  → 空则 `progname.FromPath(tool.Path)`；`tool.VersionArgs` → 空则字面量 `"--version"`。
- **没有任何测试函数直接碰它**：`grep -rn "connectDetected" internal/ cmd/` 只有三处——
  `shell.go:821`（把 `a.connectDetected` 装进 `Settings.Actions.ConnectDetected`）、
  `shell.go:2684`（定义）、`system.go:269`（`registerFromLink` 调用）。
  测试里出现的 `ConnectDetected` 全是 settingsui 侧注入的假回调
  （`settingsui/detectedaction_test.go:88`、`view_test.go:3397` 等），它们看到的是**发现行**，
  不是这层 fallback。
- `registerFromLink`（`system.go:269-273`）自己就把 `Name: filepath.Base(path)` 与
  `VersionArgs: "--version"` 显式填满，所以它**永远走不到**这两条 fallback——「异步全链路间接覆盖」
  其实连间接都没覆盖到 fallback。
- 而 `--version` 这条**是活的**：`computeDetectedTools`（`detectedtools.go:172-177`）从不设
  `VersionArgs`，所以每一个发现行连接都靠这里补默认值，今天无 pin。
- `Name` 只剩空白**可达**：`apps.go:607-609` 把 `Name=   ` 原样存成 `"   "`（第 666 行的
  `name == ""` 判的是**原始**空串，空白串能过），于是发现行带着空白名进来，trim → 路径命令词这一支真的会跑。

## 逐条落地项（动手前已读代码核过行号）

1. `internal/shell/shell.go:2684` `connectDetected`
   - 现状：函数体里两段 fallback + 一个 `settingsui.CustomTool` 字面量（含 `Platforms` 那行注释）。
   - 改成：函数体只剩 `a.connectLocalTool(detectedToolRequest(tool))`；注释搬进纯函数。
2. 新纯函数 `detectedToolRequest`（同文件、`connectDetected` 之前）
   - `Name`：`strings.TrimSpace(tool.Name)`，非空就用（**不**剥扩展名、**不**改大小写）。
   - `Name` 空：`progname.FromPath(tool.Path)`（`progname.go:83` = `Stem(filepath.Base(path))`，
     小写 + 只剥启动器后缀）。
   - `VersionArgs`：原值非空就原样保留（不做 trim，`customRequest` 后面自己 `strings.Fields`）；
     空才 `"--version"`。
   - `Program`/`Description` 原样透传；`Platforms: []string{settingsui.CurrentPlatform()}` 保留，
     连同它「不填就被 `platformRequired` 拒」的那条注释。
3. 新增 `internal/shell/detectedconnect_test.go`，五个测试函数：
   - `TestADetectedNameIsUsedAsItIs`：有值就用（前后空白被 trim、内部空白保留、中文名保留、
     带扩展名的名字**不**被剥——与 fallback 那一支刻意不同）。
   - `TestABlankDetectedNameFallsBackToThePathWord`：`" "` / `"\t"` / `"\n"` / 全角空格
     → `progname.FromPath`；`/usr/bin/RG.EXE` → `rg`（小写 + 剥 `.exe`）、
     `/opt/My Tool/bin/gh` → `gh`、`C:\bin\rg.exe` 在 Linux 宿主上的 `filepath.Base` 结果也钉住
     （宿主分隔符，文档化）。
   - `TestTheBlankNameFallbackHasItsOwnFloor`：Name 空白 + Path 空 → Name 仍是 `""`
     （fallback 是兜底不是校验，`PrepareCustom` 才是拒绝那一层）。
   - `TestVersionArgsDefaultOnlyWhenEmpty`：`""` → `--version`；`"-V"`/`"version"` 原样；
     `" "` 原样保留（文档化：`customRequest` 的 `strings.Fields` 之后才是空表，产品里不可达）。
   - `TestTheDetectedRequestCarriesThePathDescriptionAndPlatform`：`Program`/`Description`
     透传、`Platforms == []string{CurrentPlatform()}`、`Mode`/`Args` 等表单字段保持零值
     （发现行不经过表单，`customRequest` 会补 `Mode: "executable"`）。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线 **一行不动**。
本轮是 B 类（算术搬家 + 补 pin）：产品语义不变，唯一产品改动是 `connectDetected` 的 12 行
搬进纯函数。**不是视觉轮**，所以不出 PNG（无像素改动可看）；视觉 pin 台账 V=0 / S=0。

## 门槛命令与 PASS 基线（实测当前值）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2070（只许涨）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 15（=）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
grep -rn "^func Test" --include=*_test.go internal/ cmd/ | wc -l   # 1680（+5）
go test ./... 2>&1 | grep -c "^ok"                     # 34 + 2 无测试包 = 36 包
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

## mutation 计划（每条要说清会让哪个 pin 变红）

| # | 注入 | 应变红 |
|---|---|---|
| M1 | `name := strings.TrimSpace(tool.Name)` → `name := tool.Name` | `TestADetectedNameIsUsedAsItIs`（尾随空白）+ `TestABlankDetectedNameFallsBackToThePathWord`（`" "` 不再落空） |
| M2 | 删掉 `versionArgs` 的 fallback（`versionArgs := tool.VersionArgs`） | `TestVersionArgsDefaultOnlyWhenEmpty`（`""` 那条） |
| M3 | `Platforms` 从请求里去掉 | `TestTheDetectedRequestCarriesThePathDescriptionAndPlatform` |
| M4 | fallback 用 `filepath.Base(tool.Path)` 代替 `progname.FromPath` | `TestABlankDetectedNameFallsBackToThePathWord`（`RG.EXE` 期望 `rg`） |
