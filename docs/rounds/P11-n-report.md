# P11-n 报告：PATH 列表只有一个切法，PATH 上找程序只有一个查法

规格 `docs/rounds/P11-n-spec.md`。起点 HEAD `28a3af2`（P11-m 已推送），分支 `mygo-rewrite`。

## 1. 选题理由

优先级 1（用户实测四条 ①透明 ②外圈/三大金刚 ③卡死 ④打包）全部卡在**实机复测**或
**拍板**；优先级 2（合约 `docs/raycast-design.md` §6 差距清单）全「已落地」，
§6 末尾两条明写「本版不做 / 需拍板」；优先级 3（上一轮「仍待」第 1 条）同样是实机复测。
因此落到优先级 4，按用户定的顺序取第一档：**三平台一致性**。

PATH 是 Windows 与 Unix 差异最硬的一处，而参考在整族里只有一个答案——
`std::env::split_paths`：`linux.rs:203`（`XDG_DATA_DIRS`）、`linux.rs:479-481`（`TryExec=`）、
`extensions/catalog.rs:1003-1006`（PATH 命令扫描）、`extensions/inventory.rs:505,1622-1624`。
本仓有三套（实测行号见规格 §「为什么现在做」）。

## 2. 落地

### 2.1 新增 `internal/pathlist`（一个包，两个函数）

- `Split(value string) []string`：`filepath.SplitList` 之后丢空条目。**选它的理由写在包注释里**：
  Windows 上 `filepath.splitList` 会剥掉 PATH 条目两侧的引号
  （`"C:\Program Files\Git\cmd"`，安装器常写），`strings.Split` 不会。
- `Lookup(name string) (string, bool)`：名字带分隔符 → 只问 `progtype.Host`，**不拼 PATH**；
  否则按 `Split(PATH)` 顺序找第一个 `progtype.Host(join)`；最后 `exec.LookPath` 兜底
  （管 `PATHEXT` 与「空条目＝当前目录」）。测试接缝 `lookupIn(path, name)`。

### 2.2 六个调用点全部转发

| 位置 | 改成 |
|---|---|
| `internal/apps/commands.go:17` `CommandDirs` | `pathlist.Split(os.Getenv("PATH"))`（一行函数） |
| `internal/apps/apps.go:493` `executableAvailable` | `_, found := pathlist.Lookup(program)` |
| `internal/apps/apps.go:176` `XDG_DATA_DIRS` | `xdg.DataDirs()`（`XDG_DATA_DIRS` 只剩一个读入口） |
| `internal/xdg/xdg.go:41` `DataDirs` | `pathlist.Split` + 保留自己的 TrimSpace 过滤 |
| `internal/inventory/sources.go:526` `LookPath` | `return pathlist.Lookup(name)` |
| `internal/extensions/runtimepath.go:233` `splitPath` | **删除**，三处调用（`:39`、`:58`、`:227`）改 `pathlist.Split` |

净减：`internal/` 里三套切法 → 一套；`os.Getenv("PATH")` 的切点从 3 处收到
`internal/pathlist`（`extensions.SearchPath` 仍读它做**合并**，那是另一件事，见 §5）。

### 2.3 修掉的真 bug（`inventory.LookPath`）

旧实现不判名字里有没有分隔符，直接 `filepath.Join(directory, name)`：

```
PATH=<dir> 且 <dir>/bin/foo 可执行
inventory.LookPath("bin/foo")  →  ("<dir>/bin/foo", true)      ← 不是 PATH 命中，是拼接巧合
```

参考在 `linux.rs:476-478` 先 `if program.contains(std::path::is_separator)` 短路；
`apps.executableAvailable` 原本也短路——同一个问题两个答案，其中一个是错的。现在两侧都走
`pathlist.Lookup` 的分隔符分支。变异 2 的实跑报错原文就是这条：

```
--- FAIL: TestLookupShortCircuitsOnASeparator (0.00s)
    pathlist_test.go:71: a relative name with a separator resolved to "/tmp/.../001/bin/foo"
```

`inventory.LookPath` 目前**零调用点**（`grep -rn "LookPath" internal/ cmd/` 只有定义与自身），
所以这个 bug 还没有用户可见路径；转发后它不再是第二个答案，将来接上调用点就是对的。

## 3. 新增 pin（6 条）

`internal/pathlist/pathlist_test.go`：

1. `TestSplitDropsTheEmptyEntries` — `""` → 空；`a::b`（按平台分隔符拼）→ `a,b`；
   且**不 TrimSpace**（`" /c"` 原样）。
2. `TestSplitUnquotesOnWindowsOnly` — fixture 按平台选（Windows 用
   `"C:\Program Files\Git\cmd"`，Unix 用 `"/opt/Program Files/Git/cmd"`，因为 Windows 路径
   自带冒号，喂给 Unix 切法测的是冒号不是引号）。**无 Skip、无 build tag**，两个分支都在。
3. `TestLookupShortCircuitsOnASeparator` — 上面那个真 bug 的反证 + 绝对路径 + `./foo`。
4. `TestLookupWalksPathInOrder` — 两个目录都有 `tool`，取第一个；空 PATH 找不到。
5. `TestLookupIsTheSameAnswerEverywhere` — `Lookup` 命中/目录不算程序/名字不存在。

`internal/ci/pathlist_contract_test.go`：

6. `TestPathListsHaveOneSplitter` — 扫 `internal/` + `cmd/` 的 `.go`（跳过 `_test.go`、
   跳过 `pathlist` 目录、剥 `//` 注释），除 `internal/pathlist` 外出现 `filepath.SplitList`
   或「在列表分隔符上 `strings.Split`」判红。`strings.Join(…, os.PathListSeparator)`
   （`extensions.SearchPath` 写回）与单数 `os.PathSeparator`（「名字里有分隔符吗」）放行，
   测试注释里写明为什么。这是 P11-k `TestStateFilesHaveOneWriter` 的同族守卫。

## 4. 台账：V = 0 / B = 0 改动 / S = 0

- **V（视觉数值）**：0。本轮不碰墨与面，非视觉轮不出图。
- **B（行为/度量/接线）**：**零测试改动**。`internal/apps/commands_test.go`、
  `apps_test.go`（`TestExecutableAvailable`）、`internal/xdg`、
  `internal/inventory`、`internal/extensions` 的既有断言一条没改，全绿——
  这就是「产品行为没动」的证据。
  - 途中一个**我自己写错的怀疑**已核实并撤回：起初怀疑 `commands_test.go:47` 硬编码
    `":"` 会在 Windows 上必红，实测该文件首行是 `//go:build !windows`，只在 Unix 编译，
    硬编码 `:` 正确。规格已改正，那处 fixture 一个字符没动。
- **S（结构真变了）**：0（新增包不算结构断言变更；没有测试因为结构改动需要改断言）。

## 5. 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
go test -count=1 ./...                    → 36 行：34 ok + 2 no test files，0 FAIL
go test -count=1 -v ./... | grep -c "--- PASS"   → 2271（基线 2265，+6）
go test -count=1 -v ./... | grep -c "--- SKIP"   → 15（基线 15，未增）
测试函数                                           → 1842（HEAD 同法实测 1836，+6）
GOOS=linux / windows / darwin go build ./...      → 三平台全过
```

## 6. 变异（5 条，全部实跑变红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑）

| # | 注入的缺陷 | 变红的 pin（原始报错摘要） |
|---|---|---|
| 1 | `Split` 改回 `strings.Split(value, sep)`（不丢空项） | `TestSplitDropsTheEmptyEntries`：`Split = [/one  /two  ]`；`TestCommandDirs`：`dirs = [/one /two  /three]`；连带 `TestLookupShortCircuitsOnASeparator`（空 PATH 变出 `foo`） |
| 2 | `Lookup` 删掉分隔符短路 | `TestLookupShortCircuitsOnASeparator`：`a relative name with a separator resolved to "/tmp/.../001/bin/foo"` |
| 3 | `Lookup` 按 PATH **倒序**走 | `TestLookupWalksPathInOrder`：`= (".../002/tool", true), want (".../001/tool", true)` |
| 4 | `inventory/sources.go` 塞回一行 `filepath.SplitList` | `TestPathListsHaveOneSplitter`：`internal/inventory/sources.go:529 calls filepath.SplitList` |
| 5 | `Split` 无条件剥引号（`strings.Trim(dir, "\"")`） | `TestSplitUnquotesOnWindowsOnly`：`Split("\"/opt/Program Files/Git/cmd\"")[0] = "/opt/Program Files/Git/cmd" on linux` |

每条还原后 `md5sum -c` 均输出「成功」（逐字节一致），随后 `touch` 该文件避免测试缓存复用。

## 7. 仍待（下一轮候选）

1. **用户实机复测四条**（①透明 ②外圈/三大金刚 ③卡死 ④打包），连续第 69 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **PATH 这一族只剩「合并」没碰**：`extensions.SearchPath()`（`runtimepath.go:33-53`）
   把「进程 PATH + macOS 登录 shell 的答案 + 固定基线目录 + `~/.local/bin`」合并去重。
   参考对应 `runtime_path.rs` 的 `MACOS_BASELINE_PATH_DIRS`（`runtime_path.rs:34`）。
   本轮只切不拼，**合并顺序与去重没有逐条对照过**——这是同族下一个可做的点（不是拍板项）。
4. `internal/inputtext`、`internal/ansi`、`internal/calc`、`statefile`、剪贴板族、
   深链族、`inventory`/`apps` 图标链均已收口（见任务书「侦察结论」），**别重领**。
5. 稳定性（用户 ③）：本轮复核 `allowSpawn` 单飞门、`sizeRequest` 合并、缩略图离绘制线程
   均在位，无新增可改点。
