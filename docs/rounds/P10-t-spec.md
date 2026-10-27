# P10-t 规格：macOS 的 `.icns` 没有 PNG 条目时，去问平台（参考问的是 sips）

## 意图

`readIcon`（`internal/apps/icons.go:126-139`）是「一个应用到底有没有图标」的唯一答案。
它今天在 `IconPath` 非空时**只读那个文件**：`.icns` 走 `LargestPNGFromICNS`，拿不到 PNG
就 `return nil` —— 不再问平台，也不再看别的来源。参考在同一情形下把文件交给
`/usr/bin/sips` 转一次（`main:src-tauri/src/commands/apps/macos.rs:509-530`），sips 认得
容器里**所有**条目类型，包括本仓主动跳过的 pre-10.7 位图（`is32`/`il32`/`ih32`/`it32`）。
于是「老式 `.icns` 的 bundle」在参考里显示图标、在本仓显示首字母——同一个问题两个答案。

为什么现在做：P10-s 报告 §5.6 遗留第 6 条（本仓建议「先量参考 `macos.rs:509-560` 在那种
情况下做什么，再决定改产品还是钉住」）。量完的结论是**参考不会返回空**，它转换；所以这
是本仓的缺口，不是参考的怪癖。

## 量出来的证据（HEAD `67ab986`，读码 + 参考逐行对照）

1. 本仓唯一的 macOS 取图路径：
   `icons.go:132-134` → `LargestPNGFromICNS`（`icns.go:36-59`）；`icns.go:17-29` 的
   `icnsPNGTypes` 只列 11 个 PNG 类型，`icns.go:9-13` 的注释自己写着「the older raw-bitmap
   types are skipped, and a bundle whose icon is one of those simply shows no icon」。
2. `extractIcon` 的契约是「a file this build can read 之外的那条路」
   （`icons_other.go:5-10`），但 `readIcon` 在 `.icns` 分支上从不走到它 —— 契约在调用点就断了。
3. 参考 `macos.rs:509-530`：`source = source_hint.filter(is_file).or_else(icon_source(path))`，
   然后 `Command::new("/usr/bin/sips").args(["-s","format","png"]).arg(&source).arg("--out").arg(&target)`。
   没有任何「先自己解析 PNG」的分支 —— 转换就是它的答案。
4. `IconPath` 只有 darwin（`apps.go:243` ← `darwin.go:70` 的 `iconFileIn`）与 Linux
   （`apps.go:456`）会写；Windows 恒为空（`icons_windows.go:71` 的 `deferredIcon` 就靠这个），
   所以这条回退**在 Windows 上不可达**，不会把 Windows 的 PowerShell 路径拖进来。
5. 参考把转换结果落盘缓存（`mod.rs:549-590`，`app-icons/<fnv16>.png` + `.source` 边车），
   因为它的渲染面要的是一个**路径**。本仓渲染面吃**字节**（`ui.DecodeBitmap`），已有进程内
   缓存 `iconCache`（`icons.go:26-28`），所以不需要磁盘缓存 —— 记进报告，不实现。

## 落地项

### 1. `internal/apps/icons.go` —— 文件读不出东西，不等于没有图标

现状 `126-139`：

```go
if app.IconPath != "" {
    data, err := os.ReadFile(app.IconPath)
    if err != nil { return nil }
    if strings.HasSuffix(... ".icns") { return LargestPNGFromICNS(data) }
    if !isPNG(data) { return nil }
    return data
}
return extractIcon(app)
```

应改成：把「一个文件 → PNG」抽成纯函数 `iconFromFile(path, data) []byte`（`.icns` 走
`LargestPNGFromICNS`，其余要求 `isPNG`），`readIcon` 在它返回 nil（含读文件失败）时
**落到 `platformExtract(app)`**。`platformExtract` 是 `extractIcon` 的变量别名，只为让
这条回退能在 Linux 门槛机上被钉住（本仓 CI 只跑 Linux，darwin 文件根本不编译）。
行为面：Windows 不可达（证据 4），Linux 上 `extractIcon` 恒 nil（`icons_other.go`），
所以三平台**除 macOS 老式 icns 外零变化**。

### 2. `internal/apps/icns.go` —— 参考的那条命令行，纯函数 + 可注入

新增：

- `icnsConvertArgs(source, target) []string` → `{"-s","format","png",source,"--out",target}`，
  注释指向 `macos.rs:520-528`。
- `convertICNS(source string, run func([]string) error) []byte`：建临时 `.png` 目标、跑
  `run`、读回、`isPNG` 复核、`defer` 删临时文件；任一步失败返回 nil（参考的
  `status.success() && target.exists()` 之后再校验一次字节，宁可空也不交半张图）。

`run` 注入是为了让整条链路在 Linux 上被真跑一遍（假 runner 往 target 写一张真 PNG），
而不是只钉一个 argv 字面量。

### 3. `internal/apps/icons_darwin.go`（新）+ `icons_other.go` 收窄

- 新文件 `//go:build darwin`：`extractIcon(app)` = `IconPath` 是 `.icns` 时
  `convertICNS(app.IconPath, exec.Command("/usr/bin/sips", args...).Run)`；否则 nil
  （与参考 `icon_source` 返回 None 等价：没有 icns 的 bundle 参考也没有图）。
  `deferredIcon(App) bool { return false }` —— 保持同步，理由写进注释：快路径（容器里有
  PNG）一次进程都不起，只有老式图标会起一次，且进程内缓存让它每个 bundle 每会话只付一次。
- `icons_other.go` 的 build tag 由 `!windows` 收窄成 `!windows && !darwin`，注释同步改
  （「macOS 也有一条平台自己的路」）。

### 4. 新 `internal/apps/icnsfallback_test.go`

1. `TestICNSConvertArgsMatchTheReference`：argv 逐项真值表（参考 5 段）。
2. `TestConvertICNSReturnsTheConvertedPNG`：假 runner 往 `args[4]` 写真 PNG → 字节相等；
   且临时文件跑完已删（`os.Stat` 报 not exist）。
3. `TestConvertICNSRefusesAHalfAnswer`：runner 报错 → nil；runner 写非 PNG → nil。
4. `TestReadIconAsksThePlatformForALegacyICNS`：`is32` 容器的 `.icns`（`LargestPNGFromICNS`
   返回 nil 已由 `apps_test.go:591` 一族钉住）→ `readIcon` 返回平台给的字节、且被调用一次。
5. `TestReadIconKeepsThePNGEntryWithoutAsking`：`ic08` 容器 → 返回最大 PNG，平台**没被调用**。
6. `TestIconFromFileTruthTable`：PNG 直通 / 非 PNG 文件 nil / 老式 icns nil / 大小写 `.ICNS`。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：本轮**不碰**。
- `Icon`/`IconPending`/`extractIconAsync`/`iconCache`/`deferredIcon` 的语义不变；
  Windows 的 `deferredIcon`、`iconExtractScript` 一字不改。
- 本轮非视觉：不新增任何像素、不改任何 pin 的视觉数值。

## 门槛命令 + PASS 基线（HEAD `67ab986` 实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2145（本轮只许涨）
--- SKIP 15   --- FAIL 0   测试函数 1747   包 36
GOOS=linux/windows/darwin go build ./...  # 三条都过
```

预期增量：+6 函数 / +6~8 PASS。

## mutation 计划（每条都要实跑变红）

| # | 注入 | 应红的 pin |
|---|------|-----------|
| M1 | `readIcon` 的落点改回 `return nil` | `TestReadIconAsksThePlatformForALegacyICNS` |
| M2 | `icnsConvertArgs` 把 `"--out"` 写成 `"-o"` | `TestICNSConvertArgsMatchTheReference` |
| M3 | `convertICNS` 去掉读回后的 `isPNG` 复核 | `TestConvertICNSRefusesAHalfAnswer` |
| M4 | `iconFromFile` 删掉 `.icns` 分支 | `TestReadIconKeepsThePNGEntryWithoutAsking` |
| M5 | `readIcon` 把平台回退提到文件读取**之前** | `TestReadIconKeepsThePNGEntryWithoutAsking` |

每条：注入 → `go test -run ...` 看红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
