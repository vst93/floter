# P10-t 报告：macOS 的 `.icns` 没有 PNG 条目时去问平台（参考问 sips）

规格 `docs/rounds/P10-t-spec.md`。方向来自 P10-s 报告 §5.6 遗留第 6 条。

## 1. 量到的现状（HEAD `67ab986`）

`readIcon`（`internal/apps/icons.go:126-139`）在 `IconPath` 非空时只读那个文件：`.icns`
交给 `LargestPNGFromICNS`，返回 nil 就 `return nil`——**不再问平台**。而 `icns.go:17-29` 的
`icnsPNGTypes` 只有 11 个 PNG 类型，`icns.go:9-13` 的注释自己承认「the older raw-bitmap
types are skipped, and a bundle whose icon is one of those simply shows no icon」。

参考在同样情形下不做取舍：`macos.rs:509-530` 把文件交给
`/usr/bin/sips -s format png <source> --out <target>`，sips 认得容器里所有条目类型。
**结论：参考不会返回空，是本仓的缺口。**

三条附带事实（决定了改动面）：

1. `extractIcon` 的契约本来就是「a file this build can read 之外的那条路」
   （`icons_other.go:5-10`），但 `.icns` 分支从不走到它——契约在调用点就断了。
2. `IconPath` 只有 darwin（`apps.go:243` ← `darwin.go:70`）与 Linux（`apps.go:456`）会写；
   Windows 恒为空（`icons_windows.go:71` 的 `deferredIcon` 就靠这个），所以这条回退在
   Windows 上**不可达**，不会把 PowerShell 路径拖进来。
3. 参考把转换结果落盘缓存（`mod.rs:549-590`：`app-icons/<fnv16>.png` + `.source` 边车，
   内容 `len:mtimeNanos`），因为它的渲染面要的是一个**路径**。本仓渲染面吃**字节**
   （`ui.DecodeBitmap`），进程内已有 `iconCache`（`icons.go:26-28`），所以**不做磁盘缓存**
   ——同一次会话里每个 bundle 只付一次 sips。

## 2. 落地

| 文件 | 改动 |
|------|------|
| `internal/apps/icons.go` | `readIcon` 抽出 `iconFromFile(path, data)`；文件读不出 PNG（含读文件失败）时落到 `platformExtract(app)` 而不是 `return nil`。`platformExtract` = `extractIcon` 的变量别名，只为让这条回退能在 Linux 门槛机上被钉住 |
| `internal/apps/icns.go` | 新增 `icnsConvertArgs`（参考 argv 逐项）与 `convertICNS(source, run)`：临时 `.png` 目标 → 跑 run → 读回 → `isPNG` 复核 → `defer` 删临时文件；任一步失败返回 nil |
| `internal/apps/icons_darwin.go`（新） | `extractIcon` = `IconPath` 是 `.icns` 时 `convertICNS(..., exec.Command("/usr/bin/sips", args...).Run)`；`deferredIcon` 恒 false（同步，理由见文件注释：快路径零进程，老式图标每 bundle 每会话一次） |
| `internal/apps/icons_other.go` | build tag `!windows` → `!windows && !darwin`，注释同步（macOS 也有平台自己的那条路） |
| `internal/apps/icnsfallback_test.go`（新，7 函数） | argv 真值表 / 转换返回 PNG 且临时文件已删 / 失败与非 PNG 都拒 / 老式 icns 去问平台 / 能读的 icns 不问平台 / 读不到的文件也问平台 / `iconFromFile` 真值表（含大写 `.ICNS`） |

行为面：Windows 不可达；Linux 上 `extractIcon` 恒 nil（`icons_other.go`），**三平台除
「macOS 老式 icns」外零变化**。`Icon`/`IconPending`/`extractIconAsync`/`iconCache`/
`deferredIcon` 的语义一字不改，Windows 的 `iconExtractScript` 一字不改。

## 3. V / B / S 台账

- **V = 0**：本轮非视觉，没有任何像素/数值 pin 变化，不出图。
- **B = 0**：既有行为测试零改动全绿（`apps_test.go` 的 `TestIcons`、`TestICNSRejectsJunk`
  原样通过）。
- **S = 2**：①`readIcon` 的结构从「读不到就 nil」变成「读不到就问平台」；②
  `icons_other.go` 的适用范围从 `!windows` 收窄到 `!windows && !darwin`（macOS 有了自己的
  `extractIcon`）。两者都由新测试钉住，没有改动任何既有断言。

## 4. 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
go test -count=1 ./...                    → 全绿
--- PASS 2152   （基线 2145，HEAD 67ab986 实测）        +7
--- SKIP 15     （=）    --- FAIL 0      测试函数 1754（基线 1747，+7）  包 36
GOOS=linux/windows/darwin go build ./...  → 三条都过
GOOS=darwin go vet ./internal/apps/       → 净（darwin 文件在本机不参与默认 vet）
```

## 5. mutation（5 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

基线校验和：

```
7368ebb61708a3675a9e75c695e939f1  internal/apps/icons.go
9bcb7114095425f7ccb0fca291bb9753  internal/apps/icns.go
```

| # | 注入 | 原始报错 |
|---|------|---------|
| M1 | `readIcon` 落点改回 `return nil` | `--- FAIL: TestReadIconAsksThePlatformForALegacyICNS`<br>`icnsfallback_test.go:90: readIcon = 0 bytes, want the platform's 122`<br>`--- FAIL: TestReadIconAsksThePlatformForAnUnreadableFile`<br>`icnsfallback_test.go:126: readIcon = 0 bytes, want the platform's 101` |
| M2 | `icnsConvertArgs` 的 `"--out"` → `"-o"` | `icnsfallback_test.go:20: args = ["-s" "format" "png" ".../AppIcon.icns" "-o" "/tmp/out.png"], want [... "--out" "/tmp/out.png"]` |
| M3 | `convertICNS` 去掉读回后的 `isPNG` 复核 | `icnsfallback_test.go:57: a non-PNG result returned 16 bytes` |
| M4 | `iconFromFile` 删掉 `.icns` 分支 | `icnsfallback_test.go:110: readIcon = 0 bytes, want the largest PNG entry (155)`<br>`icnsfallback_test.go:145: an uppercase .ICNS returned 0 bytes` |
| M5 | `readIcon` 把平台回退提到文件读取之前 | `icnsfallback_test.go:113: the platform was asked about a bundle whose icon this build can read` |

每条还原后 `md5sum -c` 两文件均「成功」，`touch` 后复跑 7 个新测试全绿（`--- PASS` 7）。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 49 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）、register 只留 `cmd`、
   `open` 是否拒一切 query、未知宿主是否改成静默忽略。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. **新拍板候选（本轮新增，不是缺陷）**：参考对**每一个** icns 都跑 sips 并落盘缓存
   （`mod.rs:549-590`）；本仓保留自己的纯 Go 读取作为快路径、只对老式图标起一次 sips、
   不落磁盘。差异的代价是「老式图标每会话一次 ~50-100ms 进程」；如果用户觉得该付这个代价
   换「跨启动不重复转换」，那就得引入一个按 `len:mtime` 失效的磁盘缓存——属新增缓存面，
   不擅自实现。
5. **SVG/XPM 图标**仍是拍板项（P10-s 记录）：本仓渲染面只吃 PNG。
6. 下一轮建议：`internal/apps` 的图标链到这里三平台各有一条完整答案（Windows=shell 提取、
   macOS=自己的读取 + sips 兜底、Linux=主题搜索），**别再在这条链上找小修**。下一处不需要
   拍板的表面建议去 `internal/browser`（`tabs.go:247` 的 `osascript` 是 macOS 专属的
   「问平台」路径，尚未核对参考的三平台实现是否一致）。
