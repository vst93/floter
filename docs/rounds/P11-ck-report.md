# P11-ck 报告：捕获输出页的 insert 也有行为臂了

## 0. 选题

用户实测反馈 F1–F4 已收尾 → 合约 §6 全 `已落地` → P11-cf「仍待」第 1 条是拍板项、
第 2 条正是本轮：**`output.go:418`（捕获输出页的 `insert`）没有行为臂，只由源码契约守着
（`TestEveryQueryWriteCarriesItsCaret`），要行为臂得先搭一个带 `Action.Type == "insert"` 的
`OutputView` 走 `runOutputRow`**。第 3、4 条是记账与尺子盲点。规格 `docs/rounds/P11-ck-spec.md`。

## 1. 改了什么

产品代码**零改动**（`output.go` 的 md5 本轮前后一致：`95f152efdd4f577598992f9cd68fac1b`）。
`internal/launcher/querycaret_test.go` 的行为表
`TestEveryExternalWriteLeavesTheCaretAtTheEnd` 新增一臂 `t.Run("output insert", …)`：

1. 搭一个一行的列表 `OutputView`，行的动作是
   `{Type: "insert", Text: "git status --short"}`；
2. `render` 后按 `Enter`（走键盘接线，不是直调 `runOutputRow`）；
3. 断言离开输出页、字段里就是那段文本；
4. 再 `tt.Type("x")`，断言文本落在**行尾**（`"git status --shortx"`）。

## 2. 本轮查到的结构事实（为什么这条臂「天生」对 M1 不敏感）

动手前以为删掉 `a.pendingCaret = true` 会让臂变红（与 P11-ce 的 `appendWord` 同族）。
实测（探针，已删）：**不会**。根因在 `View()` 的带子结构：`a.output != nil` 时
`launcher.go:1293` 早退，**搜索字段整行根本不进这一帧的树**；离开输出页后
`textInputBase`（mygo `ui/textinput.go:36-40`）发现 `st.editor == nil`，新建编辑器，
而新建的规则就是 `caret = buf.n`——光标**自然落在末尾**。所以 insert 这一支的
`pendingCaret` 在当前结构下是**防御性冗余**，删它的后果由**源码契约**报红（见 §4 M1），
行为臂报的是另一类后果（顺序，见 §4 M2）。两把尺子互补，都要有——这正是 P11-cf
把臂与契约并列的理由，本轮把最后一支臂补齐。

参考侧同样不需要这个标志：DOM `<input>` 由 app 写 value 时光标本来就在末尾。

## 3. 门槛（实测）

| 命令 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿 0 FAIL |
| `--- PASS` 计数 | **2763**（基线 2762，+1 = 新增的 `output_insert` 子臂） |
| `--- SKIP` | 55（不变） |
| 测试函数数 | 2052（不变：加的是子测试，不新开函数） |
| `GOOS={linux,windows,darwin} go build ./...` | 全过 |

## 4. 变异（实跑）

- **M1**：删 `output.go:419` 的 `a.pendingCaret = true`。
  行为臂**绿**（§2 的结构事实），源码契约**红**，原样报错串：
  `querycaret_test.go:233: output.go:418 writes "row.Action.Text" without setting a.pendingCaret = true within 6 lines: the caret would stay where it was and the next keystroke would land inside the line`
- **M2**：把 `a.Query = row.Action.Text` 挪到 `a.leaveOutput()` **之前**（`leaveOutput`
  会把字段清成空串，插入文本被吞）。行为臂**红**，原样报错串：
  `querycaret_test.go:119: after insert the field holds "", want "git status --short"`
- 两条变异改完都 `md5sum -c /tmp/p11ck-md5.txt` 逐字节核对还原
  （`internal/launcher/output.go: 成功`），再 `touch` 复跑两条相关测试全绿。

## 5. 台账与图

V=0 / B=0 / S=0（一条断言没动，只加臂；测试文件本身无格式之外的改动）。
本轮**零像素改动**，故不出图——证据是字段里的字符串与两条红的原样报错串。

## 6. 仍待

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. §2 的结构事实意味着：输出页期间字段不在树里。若哪天产品把字段留在输出页上方
   （交互决定，需拍板），`pendingCaret` 在 insert 这一支会从冗余变成必需——届时
   M1 会让行为臂也变红，不需要重设计这条 pin。
3. `internal/launcher/scratch_p11cd_test.go` 的 `TestScratchPasteMatch` 探针记账不变（P11-cf 仍待第 3 条）。
4. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
