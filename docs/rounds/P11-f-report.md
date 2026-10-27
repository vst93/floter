# P11-f 报告：读之前先读盘——`usage.Store` 的每个入口都从文件开始

规格：`docs/rounds/P11-f-spec.md`。改前 HEAD `ec2a1e0`（P11-e）。

## ① 病灶与证据

选题：用户实测四条全卡实机、合约 §6 全「已落地」、上一轮「仍待」第 1 条同样卡实机 →
按用户优先级「三平台一致性 → 基础功能 → 像素细节」落在**基础功能**，且三平台同错（本包无 GOOS 分支）。

`usage.Store` 是懒加载的（`Open` 只记路径，`internal/usage/usage.go:43-45`）。同包三个读入口里
`Record`（`:56`）、`Seed`（`:76`）、`Count`（`:97`）都先 `loadLocked`，**`Top`（`:104-112`）没有**。
`Top` 是冷启动问的第一个问题：`internal/shell/shell.go:1576` 的 `refreshRecents` 第一句就是它
（`:1583`），`Count` 在其后（`:1585`，而且只在非空结果上）。

冷启动时唯一可能先读盘的是 `Seed`（`shell.go:1030`），而它在 `len(counts) == 0` 时提前返回
（`usage.go:70-77`）；`launch_counts` 是**旧构建**写在 settings 里的字段（`internal/settings/plugins.go:230`），
本构建新用户的 settings 里没有 → 这条路径对新用户恒不读盘。于是真实序列是
`Open`(不读) → `Seed(空)`(不读) → 扫描到达 → `Top`(不读) → **空**：空查询页整块「Recently launched」
在本会话第一次启动 app 之前都不会出现。

既有 pin `TestAFreshStartOffersTheMostUsedApps`（`internal/shell/recentsstartup_test.go`）没抓到，
因为它在 `scanApps` 之前用 `a.Usage.Record(...)` 造数据，而 `Record` 自己会 `loadLocked`——
**pin 走的不是真实冷启动路径**。本轮补的端到端 pin 用「另一个 store 写盘 → 新 store 只读」复现。

改前对照（实测）：

```
--- FAIL: TestAColdReadSeesTheFileItDidNotLoad
    usage_test.go:151: a cold read's top = [], want the file's two ids, A first
--- FAIL: TestTopLoadsWithoutACountFirst
    usage_test.go:174: filtered cold read = [], want [B]
    usage_test.go:177: the filter was never asked: the read answered before loading
--- FAIL: TestSaveWithoutAReadKeepsTheFile
    usage_test.go:205: an unread store's save replaced the file: before {"…A.app": {count 2}} / after {}
--- FAIL: TestAColdStartReadsTheRecentsFromDisk
    recentsstartup_test.go:283: recents read from disk = [], want [/Applications/Safari.app /Applications/Editor.app]
```

## ② 落地（1 个产品文件，2 处；测试 2 个文件，4 条 pin）

- `internal/usage/usage.go:104-112`：`Top` 在 `Lock` 之后先 `s.loadLocked()`（与 `Count` 同形）。
  排序（次数降序、同数按 `Last` 降序）、`keep` 过滤、`limit` 语义**一字不动**。
- `internal/usage/usage.go:134-146`：`Save` 在 `path` 判空之后先 `s.loadLocked()`，并删掉
  `if s.records == nil { s.records = map[string]Record{} }`（`loadLocked` 已保证非 nil）。
  这个 store 没有「整份替换」入口，所以「没读盘就 save」只能是错误——而它以前是**静默的**
  数据丢失（`{}` 覆盖历史）。
- `internal/usage/usage_test.go`：新增 3 条 pin（冷读、`keep` 被调用、未读盘 save 不覆盖）。
- `internal/shell/recentsstartup_test.go`：新增 1 条端到端 pin（真实冷启动形状：settings 无
  `launch_counts`，`usage.json` 由另一个 store 写好，`scanApps` 后页上必须有块与行、顺序按次数）。

同族（三处 `Store`）核对后确认**只有这一处漏**：`internal/clipboard` 的 `Entries`/`Search`/
`mutateLocked` 都先 `loadLocked`（P10-w 已收口）；`internal/calculator` 是 eager（`NewStore` 构造时
整份读入，P11-d 已记录）。所以这一族本轮收口，没有第五处。

**故意不动**：`Record` 写失败**不回滚内存**——与 P11-c/P11-d 的 `mutateLocked` 不是同一族。
本包文档明写「Nothing here is authoritative … a write failure never stops a launch」：一次失败的计数
留在内存里正是本会话该看到的答案，下一次成功 `Save` 会把它提交；回滚反而会让「刚启动过的 app
在本次会话里不算数」。写进「判不准项」供拍板，不在本轮擅自改。

## ③ 台账

**V=0 / B=4 / S=0**：4 条新 pin 全是行为类；既有测试**零改动**（`TestRecordAndTop`、
`TestMissingOrBrokenFile`、`TestSeedFromAnEarlierBuild`、`TestAFreshStartOffersTheMostUsedApps`
一行没动，全部仍绿）。产品代码只改 `internal/usage/usage.go` 两处，零视觉 / 零交互 / 零文案。

## ④ 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `go test -count=1 ./...` | 34 ok + 2 no test files（36 行），0 FAIL |
| `--- PASS` | **2235**（基线 2231，+4） |
| `--- SKIP` | 15（=） |
| `--- FAIL` | 0 |
| 测试函数 | **1806**（基线 1802，+4） |
| `GOOS=linux/windows/darwin go build ./...` | 三平台过 |

非视觉轮，不出图（无墨与面改动；页上多出来的那块正是 P9-p 已出图钉住的同一块）。

## ⑤ mutation（2 条实跑变红 + `md5sum -c` 逐字节还原 + `touch` 复跑）

1. 删 `Top` 的 `s.loadLocked()`（`internal/usage/usage.go:112` 之前一行）：
   ```
   --- FAIL: TestAColdReadSeesTheFileItDidNotLoad (0.00s)
       usage_test.go:151: a cold read's top = [], want the file's two ids, A first
   --- FAIL: TestTopLoadsWithoutACountFirst (0.00s)
       usage_test.go:174: filtered cold read = [], want [B]
   --- FAIL: TestAColdStartReadsTheRecentsFromDisk (0.00s)
       recentsstartup_test.go:283: recents read from disk = [], want [/Applications/Safari.app /Applications/Editor.app]
   ```
   → `md5sum -c`：`internal/usage/usage.go: 成功`。
2. 删 `Save` 的 `s.loadLocked()`：
   ```
   --- FAIL: TestSaveWithoutAReadKeepsTheFile (0.00s)
       usage_test.go:205: an unread store's save replaced the file:
           before { "/Applications/A.app": { "count": 2, "last": … } }
           after  null
   ```
   → 还原后 `md5sum -c` 成功，`touch` 复跑 `internal/usage` + `internal/shell` 两包全绿。

第 3 条（规格里写的「对照」）实跑结论：shell 侧产品代码**不需要改**——`refreshRecents` 保持
`Top` 在前、`Count` 在后即正确，所以修的是 store 自己的读契约，不是调用点。

## ⑥ 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 61 轮。
2. 需拍板不变（P9-g 表 + 剪贴板轮询间隔 600 vs 900 + 复制回来是否置顶 + 计算器 `loadLocked`）。
3. **本轮新增候选（要拍板）**：`usage.Record` 的写失败要不要像 P11-c/P11-d 那样回滚内存
   （现为「不回滚，下一次成功 Save 提交」；理由见 §2「故意不动」）。
4. `usage.Store` 的读入口这一族（`Top`/`Count`/`Record`/`Seed`/`Save`）本轮全部走 `loadLocked`，
   下一轮不要再在这个包里找小修。
