# P10-j 报告：`connect` 的 manifest 值判据——六条参考规则逐条定性，两条分歧钉住

## 1. 本轮做了什么

`floter://connect?manifest=…` 的值从「非空就交给 `os.Stat`」变成一条有名字的判据：
新 `internal/shell/linkmanifest.go` 的 `linkManifestValue` 把值解析成要 stat 的路径（或拒绝），
`manifestFileFits` 管文件形态的 `.json` 后缀，`manifestTraverses` 管 `..` 组件。
`HandleURL`（`internal/shell/system.go:205-218`）在开页之前过这道闸门；`connectFromLink`
（`internal/shell/shell.go:2364-2400`）不再自己展开 `~`/相对路径，只负责 stat、目录推断与安装。

参考来源：`main:src-tauri/src/deep_link.rs:270-338`（`validate_manifest_source` 的判据表）
与 `:1575-1626`（矩阵测试）。

## 2. 六条判据的定性台账

| # | 参考规则 | 参考拒绝码 | 本仓处置 | 理由 |
|---|---------|-----------|---------|------|
| 1 | 空值 | `MissingManifest` | **接**（`linkManifestValue` 第 1 条） | 原来在 `system.go` 里判，现在只有一个答案 |
| 2 | 控制字符 | `ManifestInsecure` | **接** | NUL 会在到达文件系统时截断值 |
| 3 | 显式 scheme（只收 `https` 远程） | `ManifestInsecure` / `ManifestNotJson` | **接一半**：一切 `://` 都拒 | 本仓没有远程 manifest 落地路径（`PrepareLocal` 是唯一入口，`install.go:68`；npm 走 `PrepareRegistry`，按名字+版本装） |
| 4 | 相对路径必须拒 | `ManifestNotAbsolute` | **钉住分歧**：接受，按 home 解析 | `docs/mygo-rewrite-plan.md:690-695` 文档化；终端拼写 `floter connect ./tool.json` 也折成同一条链接（`linkargs.go:78-82`） |
| 5 | `..` 组件 | `ManifestTraversal` | **接**，且比参考更统一 | 参考用 `Component::ParentDir`，POSIX/Windows 两个答案；本仓按 `/` 与 `\` 都切，三平台一个答案 |
| 6 | `.json` 后缀 | `ManifestNotJson` | **接，但只管文件形态**（`manifestFileFits`） | 本仓文档化的值可以是**包目录**（三个测试直接传目录），目录没有后缀可查；文件形态下文件名本来从不参与判断，不加这条 `/tmp/notes.txt` 会装掉 `/tmp` |

## 3. 修掉的活路径（改前/改后）

| 值 | 改前 | 改后 |
|----|------|------|
| `/pkg/notes.txt`（`/pkg` 里有 `floter.extension.json`） | 装掉 `/pkg` | 拒绝（`ManifestNotJson` 那条） |
| `/home/u/../../etc/tool.json` | 照常 stat 上溯后的路径 | 拒绝（`ManifestTraversal`） |
| `https://example.com/tool.json` | 拼成 home 下的怪文件名再 stat | 拒绝（`ManifestInsecure`） |
| `/home/u/to%0Aol.json` | 走到 stat | 拒绝（`ManifestInsecure`） |
| `/home/u/tool.json`、`~/x.json`、`./x.json`、`/pkg`（目录） | 装 | **装**（文档化形态，零改动） |

## 4. 台账

**V=0 / B=0 / S=0**：无视觉数值；文案、接线、审查链、既有 pin 全部零改动。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                    # 空
go vet ./...                            # 净
go test -count=1 ./...                   # 全绿
--- PASS                                 # 2104（基线 2099，+5 = 新增 5 个测试函数）
--- SKIP                                 # 15（=）
--- FAIL                                 # 0
包                                       # 36（34 ok + 2 no test files）
测试函数                                  # 1714（基线 1709，+5）
GOOS=linux/windows/darwin go build ./... # 三个都过
```

## 6. 变异（6 条实跑变红 + 1 条幸存者 → 落地时删掉那条死分支）

每条：注入 → 跑 → 抄原始报错 → `cp` 还原 → `md5sum -c` 逐字节核对（三个源文件全绿）→ `touch` 后复跑。

1. **删掉 `manifestTraverses` 的调用**（`linkmanifest.go`）→ 红：
   `"/home/u/../../etc/tool.json" was accepted as "/home/u/../../etc/tool.json", want a refusal: traversal: ManifestTraversal (:328-333)`，
   外加 `TestTheConnectLinkRefusesAManifestThatIsNotAPath` 的 `"/home/u/../../etc/tool.json" switched surface to 1`。
2. **删掉控制字符判据**（连带删 `unicode` import）→ 红：
   `"/home/u/to\nol.json" was accepted as "/home/u/to\nol.json", want a refusal: a control character is ManifestInsecure (:296-299)`。
   （第一次注入忘了删 import，报的是 `"unicode" imported and not used` 的 build failed，不算红，重做了一次。）
3. **删掉 `://` 判据** → 红：
   `"https://example.com/tool.json" was accepted as "/home/tar/https:/example.com/tool.json", want a refusal: divergence: no remote manifest install path in this build`
   （`filepath.Join` 把 `//` 折成一个，正好说明旧行为是「拼成 home 下的怪文件名」）。
4. **删掉 `connectFromLink` 里的 `manifestFileFits`** → 红：
   `the refusal never arrived`（`linkmanifest_test.go:187`），且日志里出现 `floter: connected JSON only local`
   —— 即 `manifest=<pkg>/notes.txt` 真的把 `/pkg` 装掉了。这是本轮修掉的那条活路径的直接证据。
5. **删掉 `~` 展开** → 红：
   `"~/tools/tool.json" resolved to "/home/tar/~/tools/tool.json", want "/home/tar/tools/tool.json": divergence: ~ is this build's documented shorthand`。
6. **把 `system.go` 的 `linkManifestValue` 换回 `strings.TrimSpace` + 非空** → 红：
   `"/home/u/../../etc/tool.json" switched surface to 1` / `landed on page 3, want the page it started on` /
   `reads "", want "That link could not be opened"` —— 证明判据真的接在活路径上，不是只测了纯函数。
7. **幸存者**：把 `filepath.Join(home, strings.TrimPrefix(rest, "/"))` 改回 `filepath.Join(home, rest)` → **全绿**：
   `filepath.Join` 自己会 Clean 掉 `~` 留下的分隔符。所以落地时 `TrimPrefix` 没有进仓，注释写明原因
   （与 P10-g 对 `value.split('.')`、P10-i 对前导 `-` 分支的处置一致）。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 40 轮，本仓无可改代码。
2. 需拍板四条不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`。
3. 参考的 `connect` 有 **review surface**（链接只带出审查页，用户再按一次才装）；本仓用权限对话框
   完成同一次审查（`docs/mygo-rewrite-plan.md:695` 已写明这条差异）。要在本仓补一个独立审查页，
   是新增视图 → 需拍板。
4. `open` 的参数分歧（参考零参数 / 本仓 `q`、`query`）已钉住，要收需用户拍板。

## 8. 本轮非视觉

只改路由层的取值判据与一份文档，没有像素改动，按任务书不要求出图（`ui.Tester` 未导出 PNG）。
