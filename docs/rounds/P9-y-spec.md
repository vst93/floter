# P9-y 规格：升级 mygo v0.6.3 → v0.6.4，并把「中文词边界」这条升级收益钉住

## 意图（一句话）

把上游 v0.6.4 升进来（`go 1.27.1 → 1.27.3` + `mygo v0.6.3 → v0.6.4`），
并**用一条 floter 自己的 pin 证明这次升级真的修好了本仓用户会碰到的一件事**：
搜索框里 Ctrl+⌫ 在中文里只删一个字，而不是吞掉整句。

## 为什么现在做

- 任务书「现在该做什么」的交棒条目就是这一条：`go.mod:5` 实测仍是
  `github.com/egoist/mygo v0.6.3`，`/home/tar/go/pkg/mod/github.com/egoist/` 里
  `mygo@v0.6.4` 已可解析（`go list -m -versions` 末位是 `v0.6.4`）。
- 上游 v0.6.4 发布说明里 **#671** 原文：*"In Chinese and Japanese text, each character now
  counts as its own word. Word-by-word deleting, moving and double-click selection no longer
  swallow a whole clause."* 这条落在 `internal/text/word.go`（新增）与
  `ui/textbuffer.go:302-345` 的 `nextWord`/`prevWord`——**本仓搜索框就是 `ui` 的文本输入**，
  中文用户按 Ctrl+⌫ 会整句消失。这是「基础功能」优先级，不是像素细节。
- 本仓无自研词切分可复用：`grep -rn "isWordChar\|wordBoundary\|prevWordStart" internal/ cmd/`
  **零命中**，`internal/inputtext/inputtext.go` 只有 `isSeparator`/`NormalizeSpaces`/
  `StripPasteNewline` 三件，与词边界无关。所以这条**只能靠升级拿到**，不能自己修。

## 逐条落地项（动手前实测的行号，HEAD `b36d668`）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `go.mod:3` | `go 1.27.1` | `go 1.27.3`（上游 `mygo@v0.6.4/go.mod:3` 就是 `1.27.3`；`go get` 自动带出） |
| 2 | `go.mod:5` | `require github.com/egoist/mygo v0.6.3` | `v0.6.4` |
| 3 | `go.sum` | v0.6.3 的 h1 | v0.6.4 的 h1（`go mod tidy` 收敛，不留旧版行） |
| 4 | 新 `internal/launcher/cjkword_test.go` | 无 | 两条 pin：中文 Ctrl+⌫ 删**一个**汉字；拉丁词 Ctrl+⌫ 仍删**整词**（防升级把拉丁打回退） |

**不改**：任何产品代码。本轮是「升级 + 钉收益」，`internal/` 下的 .go 一个字节不动
（除新增测试文件）。若某条上游变化需要改交互，进 `.pending-bugs.md`「需用户拍板」。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线 **全部不动**。
本轮唯一会碰键盘模型的地方是**新增 pin 去读它**（Ctrl+⌫ 在非空字段里本来就该落到编辑器，
`internal/launcher/fieldkeys.go:118-121` 的注释明写「claimed only while … nothing is typed in it」）。

## 上游 v0.6.4 逐条定性（16 条 Highlights/New/Fixes 里与本仓有关的）

| 上游条目 | 对 floter | 证据（本轮实读的 file:line） |
|---|---|---|
| #671 中日文按字切词 | **落地并被 pin 钉住** | `ui/textbuffer.go:302` `nextWord` 注释「each ideograph and kana is a word」；本仓入口 `internal/launcher/launcher.go` 的搜索框是 `ui` 文本输入 |
| #706 Linux 托盘首次点击被丢 | **白捡，直接命中** | `internal/shell/system.go:159` `tray.OnClick(func() { a.Toggle() })`——本仓托盘就是靠这一次点击 |
| #699/#701 macOS AutoFill 辅助进程 | 白捡（CPU/滚动卡顿） | 本仓无代码可改，纯框架侧 |
| #661 四字中文按钮被省略号截断 | 白捡 | 本仓中文文案有大量四字标签（`internal/i18n/copy.go:1075` `取消收藏`、`:1101` `取消`） |
| #679 终端滚动条可拖 | 白捡 | 本仓滚动条被推出裁剪区（`internal/launcher/launcher.go:1224` `ScrollbarInsets(0, 4096, 0, 0)`），不吃点击，与 #624 同结论 |
| #696 Kitty 图片占位符 | 白捡 | 终端是上游插件，本仓零接线 |
| #688/#689 镜像会话的鼠标与按键模式 | **仍接不上（等拍板）** | `Terminal.NewMirror` 在「需用户拍板」表里（P9-g 表），本轮不动 |
| #642 `MYGO_LDFLAGS` | **不适用** | 本仓 `grep -rn "ldflags" .github/` 只有版本号回显（`release-mygo.yml:112`），无 `-X` 注入需求 |
| #700 `mygo build` 不再乱生成 TS 客户端 | 不适用 | 本仓是原生 UI，无 `mygo-runtime` 依赖 |
| #686 原子保存算一次写 | 不适用 | `grep -rn "\.Watch(\|WatchDir" internal/` **零命中**，本仓不监听文件 |
| #607 拖到边缘滚动列表 | 不适用 | `grep -rn "OnDragOver\|DragOver" internal/` **零命中**，本仓 `internal/drops` 只做路径归一化 |
| #685 macOS 滑动返回 | 不适用 | 本仓是单面板 launcher，无 Router 历史 |
| #711/#707/#584/#684 编辑器 | 不适用 | `grep -rn "ui/editor" internal/ cmd/` 零命中 |
| #710/#712/#695/#691 WebView | 不适用 | 原生 UI，无 webview |
| Go 1.27.3（crypto/x509 GO-2026-6809） | **随升级拿到** | 本机 `go version` 升级前 `go1.27.1`，升级后 `go1.27.3` |

## 门槛命令 + PASS 基线（本轮实测，HEAD `b36d668`，升级前）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
go test -count=1 ./...                    → 36 包全绿（34 ok + 2 无测试）
--- PASS 2046 · --- SKIP 15 · --- FAIL 0
测试函数 1666
GOOS=linux/windows/darwin go build ./...  → 三个 exit 0
```

**注意**：升级前先跑一遍上面这组，PASS 必须**正好 2046**（= 任务书记录的基线），
否则说明工作树不干净，停下来先查。

## mutation 计划（≥2 条，说明每条让哪个 pin 变红）

- **M1（真正的降级）**：`go.mod` 退回 `v0.6.3` + `go 1.27.1` → 中文 pin
  `TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter` 必须红
  （期望 `搜索应`，实得 `""`——整句被 `prevWord` 吞掉）；拉丁 pin 必须仍绿。
  这是本轮最有说服力的一条：它证明这条 pin 钉的是**升级带来的变化**，不是摆设。
- **M2（本仓代码变异）**：`internal/launcher/fieldkeys.go:121` 把
  `return a.historyDeleteKey()` 改成 `return true` → 字段吞掉 Ctrl+⌫，两条 pin 都红
  （字段被清空的路径变了）。证明 pin 走的是真实按键路径，不是直接调函数。
- **M3（同族另一半）**：把新增拉丁 pin 的期望从 `term` 改成 `ter`（即假设上游把拉丁也按字切）
  → 该 pin 必须红，证明它真的在钉「拉丁仍按词」而不是恒真。
