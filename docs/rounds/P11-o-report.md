# P11-o 报告：扩展的 PATH 合并只有一个答案

## §0 方向

用户实测四条仍卡实机复测、合约 `docs/raycast-design.md` §6 全「已落地」、上一轮「仍待」
第 3 条明写「PATH 族只剩「合并」没碰」→ 按用户定的「三平台一致性 → 基础功能 → 像素细节」，
本轮落在**三平台一致性**：`extensions.SearchPath()` 的合并链与参考 `runtime_path.rs`
逐条对照，补上 `~/.cargo/bin`、并把 home 目录限定在 Unix。规格 `docs/rounds/P11-o-spec.md`。

## §1 意图

`SearchPath()` 是 `internal/shell`、`internal/inventory`、`internal/extensions` 三处工具
解析的唯一入口（`shell/detectedtools.go:159`、`inventory/inventory.go:118,169`、`shell.go:631`）。
它与参考 `runtime_path.rs` 逐条对照后剩两处不一致：

1. **缺陷 #1**：参考 `HOME_BASELINE_PATH_DIRS = [".cargo/bin", ".local/bin"]`
   （`runtime_path.rs:131-132`），本仓只追加了 `~/.local/bin`，漏了 `~/.cargo/bin`——
   rustup 装 `cargo`/`rustc` 的位置。Finder/Dock 启动继承 launchd 基线，`~/.cargo/bin`
   不在上面，于是 `floter://register?cmd=cargo` 与任何走 `LookTool`/`FindInterpreter` 的
   rust 工具链在 Finder 启动下说「没找到」，而终端里一切正常。
2. **偏差 #2**：参考把 home 目录整段 `#[cfg(unix)]` 关掉（Windows 安装器自己写进程 PATH），
   本仓在 Windows 上也拼 `%USERPROFILE%\.local\bin`。

## §2 落地（实测行号，HEAD `6701c81`）

`internal/extensions/runtimepath.go`：

- 新增 `homeBaselineDirs()`：非 Windows 上返回 `[~/.cargo/bin, ~/.local/bin]`
  （与参考同序），Windows 返回 nil（对齐 `#[cfg(unix)]` 门）。
- 把 `SearchPath` 的合并体抽成纯函数 `mergeSearchPath(process, shell, baseline, home []string) []string`
  ——进程在前、登录 shell 其次、固定基线再次、home 最后；去重保首次、丢空项。
  `SearchPath` 的 `searchOnce.Do` 仍调它，生产行为不变。
- `internal/extensions/runtimepath_test.go` 新增两条 pin：
  - `TestTheSearchPathAppendsThePerUserToolDirectories`（正面钉缺陷 #1 + Windows 守卫）
  - `TestTheSearchPathMergeKeepsTheProcessEntriesFirst`（合并链顺序 + 去重 + 丢空）

## §3 V/B/S 台账

- **V = 0**：没有旧视觉数值。
- **B = 0 既有断言改动**：两条都是**新增** pin，既有测试零改动（行为对齐参考，不是回归）。
- **S = 1**：`SearchPath` 的合并体抽成 `mergeSearchPath`（结构），但无既有断言被改。

## §4 mutation（3 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

md5 基线：
```
b4622bc2bb0b967f47a551460d513113  internal/extensions/runtimepath.go
606b64d006777b684a1c9edb676674b1  internal/extensions/runtimepath_test.go
```

1. **删掉 `.cargo/bin`**（`homeBaselineDirs` 只留 `.local/bin`）→
   ```
   --- FAIL: TestTheSearchPathAppendsThePerUserToolDirectories
       runtimepath_test.go:34: homeBaselineDirs = [/home/tar/.local/bin], want [/home/tar/.cargo/bin /home/tar/.local/bin]
   ```
2. **对调 home 基线顺序**（`.local/bin` 在前）→
   ```
   --- FAIL: TestTheSearchPathAppendsThePerUserToolDirectories
       runtimepath_test.go:34: homeBaselineDirs = [/home/tar/.local/bin /home/tar/.cargo/bin], want [/home/tar/.cargo/bin /home/tar/.local/bin]
   ```
3. **`mergeSearchPath` 把基线放到进程之前** →
   ```
   --- FAIL: TestTheSearchPathMergeKeepsTheProcessEntriesFirst
       runtimepath_test.go:64: merged[0] = "/opt/homebrew/bin", want "/usr/bin" (whole: [/opt/homebrew/bin /usr/bin /bin /shell/bin /home/u/.cargo/bin /home/u/.local/bin])
   ```

三条还原后 `md5sum -c` 均「成功」，`touch` 后复跑 `TestTheSearchPath*` 全绿。

## §5 门槛（实测，HEAD 本轮改动后）

```
gofmt -l cmd internal        → 空
 go vet ./...                → 净
go test -count=1 ./...       → 全绿（36 行：34 ok + 2 no test files）
GOOS=linux/windows/darwin go build ./... → 三平台全过
```

| 指标 | 实测 | P11-n 基线 |
|---|---|---|
| PASS | **2273** | 2271 |
| SKIP | **15** | 15 |
| FAIL | **0** | 0 |
| 测试函数 | **1844** | 1842 |
| 包行数 | 36（34 ok + 2 no test files） | 36 |

PASS 与函数数各 +2，正是本轮新增的两条 pin。

> 附注：`GOOS=windows go vet ./internal/extensions/` 会报 `undefined: testPlatforms`
> ——这是**预先存在**的测试缺口（`commandword_test.go` 用 `testPlatforms`，定义它的
> `custom_test.go` 带 `//go:build !windows`），与本轮无关；门槛要求的 `GOOS=windows go build`
> 已通过，本轮测试文件本身在 windows/darwin 下 `go build` 也通过。

## §6 仍待

1. 用户实机复测四条，连续第 70 轮。
2. 需拍板不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器 `loadLocked`
   + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序 + `Esc`/`Escape`
   + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **PATH 族已收官**：切法（P11-n）+ 查法（P11-n）+ 合并链（本轮）。`SearchPath` 与参考
   `runtime_path.rs` 逐条对齐（进程→登录 shell→固定基线→home，去重保首次丢空），home 基线
   补上 `.cargo/bin`、限定 Unix。**别在里面再找小修。**
4. `inputtext`/`ansi`/`calc`/`calculator`/`statefile`/剪贴板族/深链族/`inventory`+`apps`
   图标链均已收口，别重领。

