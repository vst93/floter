# P10-b 规格：「命令词面的 stem」补测试 pin

## 本轮意图

补齐「命令词面的 stem」这一族的测试断言。当前代码在三个位置用 `progname.FromPath`
生成连接程序的 Command 字段（`extensions/custom.go:257`、`shell.go:2557`、`shell.go:2687`），
但没有任何测试断言「同一个可执行文件在**别名面**（`apps.executableName`，一个扩展名都不剥）
与**命令词面**（`progname`，剥启动器后缀如 `.exe/.bat/.cmd/lnk`）给出的名字故意不同」。

选择理由：来自任务书「现在该做什么」的交棒条目，P10-a「仍待」第 4 条，属于 B 类（行为/
度量/接线），测试零改动必须全绿。这是视觉轮之后的第一个基础功能自查项。

## 落地项：位置 → 现状 → 应改成

### ① `extensions/custom.go:257`（Command 字段的 fallback）

```go
// 当前（改前）
if cfg.Command == "" {
    cmd, _ := progname.FromPath(cfg.Path)
    cfg.Command = cmd
}
```

**现状**：从路径生成命令词时剥 `.exe/.bat/.cmd/lnk`，例如 `C:\tools\rg.exe` → `"rg"`。
**应改成**：新增测试函数断言「同一程序」在别名面 vs 命令词面的差异，并写入注释引
参考 `windows.rs:353`、`linux.rs:412,435`。

### ② `shell.go:2557`（搜索行的显示文案）

```go
// 当前（改前）
case *extension.Custom:
    displayName := progname.FromPath(ext.Path) // line number may drift
```

**现状**：搜索行显示「命令词面」的名字，已经做过一次剥离。
**应改成**：增加一对测试用例，覆盖同文件在别名面和命令词面的分岔点，pin 住差异。

### ③ `shell.go:2687`（快捷键标签 / keycap 显示）

```go
// 当前（改前）
keycapText := progname.FromPath(path)
```

**现状**：键帽槽里显示的也是剥离后的版本。
**应改成**：同上，增加 pin 证明「别名面 ≠ 命令词面」是故意的，不是 bug。

## 行为契约红线（本轮零改动）

- **高度链**：搜索行高度不变
- **键盘模型**：快捷键绑定表不变
- **会话管理**：会话创建逻辑不变
- **设置写回**：`custom.go` 的 `cfg.Command` 写回语义不变
- **i18n 文案**：所有可见文案不动
- **命令接线**：`FromPath` 的调用点位置和参数不变
- **PIN 台账**：V=0 / B 既有 pin 零改动全绿 / S=0

## 门槛命令 + PASS 基线

实测前的基线（P10-a 收尾值，本轮用「新增 pin 的 PASS 行数」反算核对：
2065 − 14 = 2051，14 = 1 + 9 + 4 三条新 pin 各自产出的 `--- PASS` 行）：
- **PASS**: 2051
- **SKIP**: 15
- **测试函数**: 1671
- **包数**: 36（34 有测试 + 2 无测试）

本轮目标：PASS 计数**不许减少**，若因新增测试函数而涨也许可到 2053+，但不能有回归。

```bash
gofmt -l cmd internal # 空
go vet ./...          # 净
go test -count=1 ./... # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS" # 只涨不跌
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

**M1**: `internal/apps/filestem_test.go` 中 `TestFileStemArithmetic` 的一条期望故意改错，
     例如把 `rg.exe.txt` 的返回值从 `"rg.exe"` 改为 `"rg"`，预期该测试失败。

**M2**: `progname/frompath_test.go` 中对 `rg.exe` 的期望从 `"rg"` 改为 `"rg.exe"`，
     预期整个 suite 中依赖该行为的测试红。

**M3**: 修改 `custom.go:257` 去掉剥离逻辑（假设直接返回 `filepath.Base`），
     预期自定义扩展的命令行会带上 `.exe` 后缀，导致关联测试红。

mutation 完成后：真实运行 → 抓报错串 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑。

## 参考资料定位

- Rust `windows.rs:353` (`executable_name` for shortcuts)
- Rust `linux.rs:412,435` (`identifier_aliases` vs `Exec=` parsing)
- 本仓 `internal/apps/shortcut.go:226` (`fileStem`)
- 本仓 `internal/progname/progname.go:30` (`FromPath`)

## 交付判断

- 新增至少**2 个测试函数**，分别断言 alias vs command 的差异
- V=0 / B 既有 pin 零改动全绿 / S=0
- mutation 实跑验证通过
- 三平台 build 过
- 报告与 done.log 写入实测数字
