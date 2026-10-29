# P11-cf 报告：App 写字段的每一条路径都带光标规则（一把尺子）

## 1. 选它做什么

选题顺序（按用户定的优先级）：用户实测反馈 F1–F4 **已全部收尾**（F1 → P11-y，F2/F3 →
P11-z，F4 → P11-bb/bj/bk/bo）→ `docs/raycast-design.md` §6 全 `已落地`（`grep -n
"未落地\|需拍板\|本版不做" docs/raycast-design.md` 只命中 §4.2 一句历史注脚与 §6 的
「需拍板」条目）→ P11-ce「仍待」第 1 条是 15 条**拍板项**（不许擅自改）、第 2 条
「这一族已复核，没有第二个漏网」（＝**复核过、没钉住**）、第 3 条「补全的另一个分支没有
独立 pin」、第 4 条是尺子盲点。

于是落在「交互」这一档里唯一不牵拍板的一条：把**上一轮刚修的那个用户可见 bug**（下一个
字符插进词中间）的回归资产补齐。规格 `docs/rounds/P11-cf-spec.md`。

## 2. 本轮实测：这一族的「答案」只活在七行手写的同一个标志里

```
$ grep -n "a\.Query = " internal/launcher/*.go | grep -v _test    # 23 处
$ grep -rn "pendingCaret" --include=*_test.go internal/
internal/launcher/view_test.go:2451:	a.pendingCaret = true      # 夹具
internal/launcher/tabcaret_test.go:16:…                        # 只有补全那一条的注释/pin
```

写**非空串**的七处：`history.go:53`（↑）、`:66`/`:70`（↓ 与草稿）、`launcher.go:1084`
（`SetQuery`，深链）、`:1899`（命令模式的 argv）、`:2132`/`:2134`（补全的两个分支）、
`output.go:418`（捕获输出页的 insert）。它们都置了 `a.pendingCaret = true`（P11-ce 读过
一遍），但**只有 `appendWord` 那一支有断言**——把那七行里的任意一行删掉，P11-ce 之前
全仓一条测试都不会红。本轮把这件事钉成两把尺子。

## 3. 落地（产品代码零改动）

新增 `internal/launcher/querycaret_test.go`：

| pin | 断言 | 臂 |
|---|---|---|
| `TestEveryExternalWriteLeavesTheCaretAtTheEnd` | 外部写入之后键入 `x`，`x` 必须落在**行尾** | `deep_link`（首帧深链，对照臂）、`deep_link_over_an_open_field`（字段已在用、光标停在词中间 → 这才是会被弄坏的那一态）、`history_recall`（↑ 召回）、`escape_clears_the_field`（空写入的对照臂：光标夹到 0，下一字符是**开头**） |
| `TestACompletionThatStartsANewWordLeavesTheCaretAtTheEnd` | `appendWord` 的**第二个**分支（行尾有空格 → 补进一个新词，`trimmed + " " + word + " "`）之后键入 `x` 落在行尾 | 命令模式里键入 `-f ` → Tab → 断言 `x` 追加在整行之后（P11-ce 仍待第 3 条） |
| `TestEveryQueryWriteCarriesItsCaret` | **源码契约**：扫 `internal/launcher` 的生产 `.go`，凡是写**非空串**的 `a.Query = …`，其后 6 行内必须有 `a.pendingCaret = true`；写空串的 16 处豁免（`min(caret,0)` 已经是 0） | 扫描命中的条数也断言（≥7 非空 / ≥10 空），否则改名后静默通过 |

**没动的**：`a.Query` 的 23 个写点本身、`pendingCaret` 的消费点（`launcher.go:1486`）、
高度链、键盘模型、i18n、命令接线、墨与面。**本轮零像素改动，故不出图**（理由同 P11-ce：
改的是「光标停在哪」，证据是字段里的字符串）。

## 4. 变异（实跑，逐条抄原始报错串）

`md5sum` 基线：`launcher.go` / `history.go` → `/tmp/cf.md5`，每条还原后 `md5sum -c` 逐字节核对「成功」。

| # | 注入 | 结果 | 原始报错 |
|---|---|---|---|
| M1 | 删 `launcher.go:1089`（`SetQuery` 的 `a.pendingCaret = true`） | **红**（两条 pin） | `querycaret_test.go:65: typing after a deep link landed at "gitx status --short", want "git status --shortx"`；`querycaret_test.go:174: launcher.go:1084 writes "query" without setting a.pendingCaret = true within 6 lines…` |
| M2 | 删 `history.go:54`（↑ 的那一行） | **红** | `querycaret_test.go:68: typing after a history recall landed at "xgit status", want "git statusx"`；`history.go:53 writes "a.History[a.historyIndex]" without setting …` |
| M3 | **对照**：把 `appendWord` 的 flag 挪到 `if/else` **之前**（两个分支都覆盖，行为不变） | **绿**（预期） | `ok floter/internal/launcher 0.331s` |

**M1 第一次跑只有源码契约红、行为臂绿**——值得记一笔：「首帧深链」那一臂**对这条标志不
敏感**（字段那时还是全新的空缓冲，`min(0, 10)` 本来就是 0/末尾），所以本轮把它降级成
对照臂，另加 `deep_link_over_an_open_field` 作为真正会红的那一臂（M1 第二次跑即由它报红）。
M3 绿说明这把尺子钉的是「有没有」，不是「写在哪一行」。

## 5. 门槛（全部本轮实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2762（基线 2755，+7 = 3 函数 + 4 子测试）
--- SKIP                              → 55（不变）
^func Test 计数                       → 2052（基线 2049，+3）
GOOS=linux/windows/darwin go build ./... → 三平台过
```

台账：**V=0 / B=0 / S=0**（没有一条旧 pin 需要改）。

## 6. 仍待（P11-cf）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **这一族里唯一没有行为臂的是 `output.go:418`（捕获输出页的 `insert`）**：它只由源码契约
   守着。要给它一条行为臂得先搭一个带 `Action.Type == "insert"` 的 `OutputView` 并走
   `runOutputRow`（`output.go` 的 `insert` 分支），本轮的尺子够用，故没为它铺夹具。
3. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
4. `internal/launcher/scratch_p11cd_test.go` 的 `TestScratchPasteMatch` 是 P11-cd 留下的
   **探针**（只 `Logf`、零断言）。删它会减一个测试函数（纪律不许），把它改成断言又是另一
   件事，故留着并在此记账。
