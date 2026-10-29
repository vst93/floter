# P11-ce 报告：补全之后，光标落在行尾

## 1. 选它做什么

选题顺序：用户实测反馈 F1–F4 全部收完 → `docs/raycast-design.md` §6 全 `已落地`
（只剩拍板项）→ P11-cd「仍待」第 1 条是 13 条拍板项、第 2 条是新拍板项、第 3 条已作废、
第 4 条是尺子盲点 → 按用户定的优先级自己找（界面质感与交互 → 基础功能 → 三平台一致性
→ 像素细节）。

落在**输入框的交互**上：插件命令的参数模式里，Tab 补全一个参数之后，键盘的下一个字符
落在补全词的**中间**。

## 2. 复现（修前，用临时探针实测，探针已删）

```go
a := commandApp()          // internal/launcher/view_test.go:280 的既有夹具
tt := render(t, a)
tt.Key(0, ui.KeyTab); tt.Frame()   // 进入参数模式
tt.Type("-m"); tt.Frame()
tt.Key(0, ui.KeyTab); tt.Frame()   // 补全 → "-mode "
tt.Type("x"); tt.Frame()           // 用户接着打字
```

```
after completion: "-mode "
after typing x:   "-mxode "        ← 修前
```

两条路径都中：Tab 键（`launcher.go:1755`）与补全行自己的 `Run`（`search.go:573`
的 `Run: func() { a.appendWord(value) }`，点击 / Enter）——它们共用同一个写入口。

## 3. 根因（不是「加个过滤」，是把缺的那一步补上）

`a.Query` 是 App 写的字符串，字段里的编辑器另有一份缓冲与光标。框架对**外部写入**的文本
只做夹取，不移动插入点：`mygo@v0.7.3/ui/editor_history.go:59`
`ed.caret = min(ed.caret, ed.buf.n)`。补全把行从 `-m`（光标 2）写成 `-mode `（长 6），
光标仍是 2 —— 下一个字符就插在那儿。

App 自己早就有这一步：`pendingCaret`（`internal/launcher/launcher.go:379` 注释、
`:1486` 消费，`field.SetTextSelection(len(a.Query), len(a.Query))`）。同族的每一条
「App 自己写字段」的路径都置了它：`ResetQuery`（`:1033`）、`SetQuery`（`:1089`）、
历史 ↑↓（`history.go:54/67/71`）、进入命令模式（`:1901`）、剪贴板/浏览器/计算器/文件页
（`:2060`、`:2762`、`calculator.go:66`、`files.go:64`）、捕获输出页（`output.go:419`）。
**`appendWord` 是唯一漏网**：它写的是**变长的非空行**，其余要么写空串（光标自然回到 0）、
要么已经置过。

参考那边的答案同样是「行尾」：`main` 的输入是受控 DOM `<input>`，App 从外部写 `value`
时浏览器把选区放回末尾（React 受控输入的既有行为），所以补全后接着打字是行的常态。

## 4. 落地

- `internal/launcher/launcher.go:2137`：`appendWord` 尾部加 `a.pendingCaret = true`，
  与同族五条路径同一个做法；`:2126` 起补一段注释，写清「外部写入只夹光标」这条机制与
  实测后果，免得下次又当「多余的标志」删掉。
- `internal/launcher/tabcaret_test.go`（新增 1 条 pin）：`TestACompletionLeavesTheCaretAtTheEnd`，
  **两条臂**——Tab 键与补全行的 `Run`——每条都断言「补全后键入 `x` → `-mode x`」。

**没动的**：补全字符串本身的拼法、`Selected/chosenRow` 的重置、`FocusSearch`、
字段的墨/几何/字号、高度链、键盘模型、i18n、命令接线。**只改了「光标在哪」。**

## 5. 台账（V / B / S）

- **V = 0**：没有一个颜色、半径、字号、padding、gap 或几何数被改；没有出图（见 §7）。
- **B = 0**：既有 pin 零改动、全绿（`TestCommandModeFiltersCompletesAndRuns`、
  `TestTabExpandsAnExtensionCommandIntoItsArguments`、`TestEscapeLeavesTheCommandMode`、
  `TestDeletingTheCommandLeavesTheMode`、`TestCommandModeRowsCarryTheSource` 等全在）。
- **S = 0**：没有结构变更。
- 新增 1 条 pin（2 条臂在同一函数里）。

## 6. 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2754（HEAD）→ 2755（本轮）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      → 55（未变）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2048 → 2049
三平台 build                                            → 过
```

**记账方法**：HEAD 与本轮各跑一次完整 `-v`，`grep -- "--- PASS" | sed 's/ (.*//' | sort`
后 `diff`，**只多一行** `--- PASS: TestACompletionLeavesTheCaretAtTheEnd`；
`2754 /tmp/p11ce-head.txt`、`2755 /tmp/p11ce-with.txt`。

（实测插曲，写下来免得下次当成回归：中途有一次本轮的 `-v` 计数报 2754，与上面的 diff
矛盾；换成「两次都存名字再 diff」之后差额正好是新增的那一条——单看总数会读到包间
输出交错造成的少一行。）

## 7. 为什么没有图

本轮**没有一个像素被改**：改的是光标停在哪，画面上的墨、面、几何逐字节不变。证据是
字段里的字符串（上表 `-mxode ` → `-mode x`）与 pin，而不是截图；`ui.Tester` 的字树
在这条断言上比 PNG 更直接。按 `docs/AGENT-NOTES.md` 的轮次档案约定，图只对视觉轮强制。

## 8. 变异（实跑，全部 `md5sum` 逐字节还原）

文件基线 `internal/launcher/launcher.go` = `353f770f8d551a488c5fb0e09ff202d8`。

1. **M1**：删掉 `appendWord` 的 `a.pendingCaret = true`（`touch` 测试文件后复跑，
   避免缓存）：

   ```
   --- FAIL: TestACompletionLeavesTheCaretAtTheEnd (0.20s)
       tabcaret_test.go:40: typing after a completion landed at "-mxode ", want "-mode x":
         the caret stayed where the word being completed began
       tabcaret_test.go:71: typing after a completion taken from the list landed at "-mxode ", want "-mode x"
   ```

   还原后 `md5sum -c`：`internal/launcher/launcher.go: 成功`。

2. **M2**：把消费 `pendingCaret` 的那句 `field.SetTextSelection(len(a.Query), len(a.Query))`
   改成 `SetTextSelection(0, 0)` —— 证明这条 pin 量的是**光标**，不只是那个布尔字段：

   ```
   --- FAIL: TestACompletionLeavesTheCaretAtTheEnd (0.18s)
       tabcaret_test.go:40: typing after a completion landed at "x-mode ", want "-mode x"
       tabcaret_test.go:71: typing after a completion taken from the list landed at "x-mode "
   ```

   还原后 `md5sum` = `353f770f8d551a488c5fb0e09ff202d8`（与基线逐字节相同）。

## 9. 仍待（P11-ce）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **同族已复核，没有第二个漏网**：全仓「App 写 `a.Query`」的七处都是空串或已置
   `pendingCaret`（清单见 §3）；`output.go` 的 `a.Query = row.Action.Text` 也置了。
   下次要重领这条族，先 `grep -n "a.Query = "` 再逐条对 §3 的表。
3. **没有第二条 pin 守「Tab 之后还能接着打」的其它形态**：比如补全进**新的**一个词
   （`appendWord` 的 `trimmed != a.Query` 分支）、以及模式外的 Tab（进入模式）。
   本轮的 pin 只走 `-m` → `-mode ` 这一条（行尾补空格）；另一分支由同一句
   `pendingCaret` 覆盖，但**没有独立断言**。
4. 尺子老盲点不变：只量字盒；`Tester.Find` 按字符串去重；滚动容器裁掉的行不进
   `Texts()`；PTY 自己画的行不在高度尺子上。
