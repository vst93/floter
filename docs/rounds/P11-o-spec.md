# P11-o 规格：扩展的 PATH 合并只有一个答案——补上 `.cargo/bin`，Windows 不再追加 Unix home 目录

## 意图（一句话）

`extensions.SearchPath()` 的「进程 PATH + 登录 shell + 固定基线 + home 目录」这条合并链，
与参考 `runtime_path.rs` 逐条对照后剩两处不一致：**漏了 `~/.cargo/bin`**（rustup 装
`cargo`/`rustc` 的位置），以及**在 Windows 上也追加 `~/.local/bin`**（参考只在 Unix 上追加
home 目录）。本轮把这两处对齐参考，合并链从此只有一个答案。

## 为什么现在做（证据）

优先级：用户实测四条仍卡实机复测、合约 `docs/raycast-design.md` §6 全「已落地」、
上一轮「仍待」第 3 条明写「**PATH 族只剩「合并」没碰**：`extensions.SearchPath()`
（`runtimepath.go:33-53`）的顺序与去重没有逐条对照过参考 `runtime_path.rs`」→ 按用户定的
「三平台一致性 → 基础功能 → 像素细节」，本轮落在**三平台一致性**（`SearchPath` 是
`internal/shell`、`internal/inventory`、`internal/extensions` 三处工具解析的唯一入口，
见 `shell/detectedtools.go:159`、`inventory/inventory.go:118,169`、`shell.go:631`）。

逐条对照（实测 HEAD `6701c81`，`internal/extensions/runtimepath.go:35-55` vs
`main:src-tauri/src/extensions/runtime_path.rs:131-155`）：

| # | 参考 | 本仓 | 判定 |
|---|---|---|---|
| 1 | 进程 PATH 在前，去重 | `pathlist.Split(os.Getenv("PATH"))` + `appendUnique` | 一致 |
| 2 | macOS：登录 shell 的 `$PATH` 追加在进程之后、固定基线之前 | `loginShellDirs()` 在 `baselineDirs[GOOS]` 之前 | 一致 |
| 3 | `#[cfg(unix)]` 才追加 home 目录 | **无条件**（含 Windows）追加 `~/.local/bin` | **不一致（偏差 #2）** |
| 4 | `HOME_BASELINE_PATH_DIRS = [".cargo/bin", ".local/bin"]` | 只有 `.local/bin` | **不一致（缺陷 #1）** |
| 5 | 去重保首次出现、丢空项 | `appendUnique` 保首次、丢空 | 一致 |
| 6 | 分隔符 `:` | `pathlist.Split`（平台分隔符，Windows `;`） | 本仓更正确（参考 `:` 在 Windows 上本就错） |

**缺陷 #1 的真后果（macOS Finder/Dock 启动）**：Finder 启动继承的是 launchd 基线
（`/usr/bin:/bin:/usr/sbin:/sbin`），rustup 的 `cargo`/`rustc` 装在 `~/.cargo/bin`，不在
基线上。参考 `runtime_path.rs` 的模块注释原话：「Everything the user installed with
Homebrew (`/opt/homebrew/bin`), **rustup (`~/.cargo/bin`)** or a per-user installer
(`~/.local/bin`) lives outside it」。本仓只补了 Homebrew 与 `~/.local/bin`，`rustup` 那条
漏了——`floter://register?cmd=cargo` 或任何走 `LookTool`/`FindInterpreter` 的 rust 工具链
在 Finder 启动下说「没找到」，而同一台机器的终端里一切正常。这就是「三平台/启动方式不一致」的本体。

**偏差 #2 的真后果（Windows）**：参考把 home 目录整段 `#[cfg(unix)]` 关掉——Windows 上
rustup / scoop 等安装器**自己写进程 PATH**，`~/.local/bin` 不是 Windows 的约定目录。
本仓在 Windows 上也拼 `%USERPROFILE%\.local\bin`，多一个不存在的目录（无害但非参考答案），
且与「home 目录是 Unix 约定」这一判断相悖。

## 落地项（动手前已核对行号，HEAD `6701c81`）

### 1. `internal/extensions/runtimepath.go:48-52` — home 基线补 `.cargo/bin`、限定 Unix

现状（`SearchPath` 的 `searchOnce.Do` 内）：

```go
if home, err := os.UserHomeDir(); err == nil && home != "" {
    merged = appendUnique(merged, filepath.Join(home, ".local", "bin"))
}
```

应改成：把 home 基线抽成一个 `homeBaselineDirs()` 函数（顺序 `.cargo/bin` → `.local/bin`，
与参考 `HOME_BASELINE_PATH_DIRS` 同序），且**只在非 Windows 上产出**：

```go
// homeBaselineDirs is the per-user tool directories appended after the
// conventional ones: rustup installs rustc into ~/.cargo/bin and the XDG
// convention puts user binaries in ~/.local/bin. Empty off Windows, where the
// installers write the process PATH themselves (the reference gates the same
// list with #[cfg(unix)], runtime_path.rs:131-132).
func homeBaselineDirs() []string {
    if runtime.GOOS == "windows" {
        return nil
    }
    home, err := os.UserHomeDir()
    if err != nil || home == "" {
        return nil
    }
    return []string{
        filepath.Join(home, ".cargo", "bin"),
        filepath.Join(home, ".local", "bin"),
    }
}
```

`SearchPath` 内改为：

```go
for _, dir := range homeBaselineDirs() {
    merged = appendUnique(merged, dir)
}
```

### 2. 新增 `internal/extensions/runtimepath_test.go` — 两条 pin

- `TestTheSearchPathAppendsThePerUserToolDirectories`（`!windows`）：`homeBaselineDirs()`
  必须含 `home/.cargo/bin` 与 `home/.local/bin`，且**顺序是 `.cargo/bin` 在前**
  （参考 `HOME_BASELINE_PATH_DIRS` 的顺序）。这条正面钉住缺陷 #1。
- `TestTheSearchPathMergeKeepsTheProcessEntriesFirst`（平台无关）：用**纯函数**校验合并链的
  「进程在前、登录 shell 其次、基线最后、去重保首次、丢空项」——把 `SearchPath` 的合并体抽成
  `mergeSearchPath(process []string, shell, baseline, home []string) []string` 以便不碰进程
  环境即可断言。这条钉住偏差 #2 的边界（Windows 下 `home` 传 nil 时结果里没有 home 项）与
  整体顺序。

> 抽取 `mergeSearchPath` 只是把现有循环体参数化（`SearchPath` 的 `searchOnce.Do` 仍调它），
> 不改变任何生产行为——它是本轮唯一的结构改动，属于 S 类但测试侧零既有断言改动。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一律不动**。
- `SearchPath` / `SearchDirectories` / `LookTool` / `FindInterpreter` 的**签名与调用面不动**；
  `searchOnce` 仍是每进程一次。
- `loginShellDirs`（macOS 登录 shell 探测）的 5s 超时、哨兵解析、失败即空**不动**。
- `baselineDirs`（`/usr/local/bin` 等固定基线）的**内容与平台表不动**。
- `appendUnique` 的「保首次、丢空」**不动**。

## 门槛命令 + PASS 基线（实测于 HEAD `6701c81`）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P11-n 报告）：PASS **2271** · SKIP **15** · FAIL **0** · 测试函数 **1842**（同法 HEAD 起点 1836）·
**36 行（34 ok + 2 no test files）**。本轮预计新增 2 个测试函数 → 函数 **1844**、PASS ≥ **2273**。

## mutation 计划（≥2 条）

1. 把 `homeBaselineDirs` 的 `.cargo/bin` 删掉（只留 `.local/bin`）→
   `TestTheSearchPathAppendsThePerUserToolDirectories` 必须红（正面 pin 抓到缺陷 #1 回退）。
2. 把 `homeBaselineDirs` 的 `runtime.GOOS == "windows"` 守卫删掉 → 在 Windows 构建下
   `mergeSearchPath` 的 home 断言红；Linux 上则以 `TestTheSearchPathMergeKeepsTheProcessEntriesFirst`
   直接传 `home` 非空来证明「合并体本身会追加 home」——为让守卫的删除在 Linux CI 上也变红，
   第 2 条 mutation 改为：把 `SearchPath` 里 `for _, dir := range homeBaselineDirs()` 换成
   无条件 `merged = appendUnique(merged, filepath.Join(home, ".local", "bin"))` 的旧写法 →
   `TestTheSearchPathMergeKeepsTheProcessEntriesFirst` 在「Windows 语义（home=nil）」下红。
3. 把 `mergeSearchPath` 里「进程在前」的两段顺序对调（baseline 先于 process）→
   `TestTheSearchPathMergeKeepsTheProcessEntriesFirst` 的顺序断言红。

每条：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑一次（防测试缓存复用）。

## 台账预期

V = 0（没有旧视觉数值）· B = 0 既有断言改动（两条都是新增 pin）· S = 1
（`SearchPath` 的合并体抽成 `mergeSearchPath`，属结构，但无既有断言被改）。

## 非视觉轮

不出图（本轮不改任何墨与面）。报告写 `docs/rounds/P11-o-report.md`。
