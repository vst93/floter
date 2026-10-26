# P9-y 报告：升级 mygo v0.6.3 → v0.6.4，并钉住「中文词边界」这条收益

规格：`docs/rounds/P9-y-spec.md`。起点 HEAD `b36d668`（P9-z 已推送，工作树干净）。

## 1. 升级实跑

```
$ go get github.com/egoist/mygo@v0.6.4
go: github.com/egoist/mygo@v0.6.4 requires go >= 1.27.3; switching to go1.27.3
go: upgraded go 1.27.1 => 1.27.3
go: upgraded github.com/egoist/mygo v0.6.3 => v0.6.4
$ go mod tidy        # 无输出
$ go version
go version go1.27.3 linux/amd64
```

`go.mod` 只动两行（`go 1.27.1 → 1.27.3`、`mygo v0.6.3 → v0.6.4`），`go.sum` 换成 v0.6.4 的 h1。
`go 1.27.3` 不是手写的，是上游 `mygo@v0.6.4/go.mod:3` 的硬要求（`crypto/x509` 安全修复
GO-2026-6809，发布说明 Internal 段原文），`go get` 自己带出来的。

**升级前**先跑了一遍门槛确认基线没漂：`--- PASS 2046` / `--- SKIP 15` / 测试函数 1666 / 36 包
——与任务书记录的基线**逐项相等**。

## 2. 上游 v0.6.4 逐条定性

发布说明 26 条（Highlights 4 / New 4 / Fixes 18）里与本仓有关的是 16 条，逐条读代码定性；
表在规格 §「上游 v0.6.4 逐条定性」，此处只记**结论分布**：

- **落地并被 pin 钉住：1 条** — #671 中日文按字切词（下面第 3 节）。
- **白捡（零代码，本仓直接受益）：6 条** — #706 Linux 托盘首次点击（`internal/shell/system.go:159`
  `tray.OnClick(func() { a.Toggle() })` 就是那一次点击）、#699/#701 macOS AutoFill 辅助进程、
  #661 四字中文按钮不再被省略号截断（本仓中文文案里四字标签成片：`internal/i18n/copy.go:1075`
  `取消收藏`、`:1101` `取消`）、#679 终端滚动条可拖（本仓条被推出裁剪区
  `internal/launcher/launcher.go:1224`，与 #624 同结论）、#696 Kitty 图片占位符、Go 1.27.3 本身。
- **不适用（本仓没有那个面）：8 条** — #642 `MYGO_LDFLAGS`（`.github/` 里 `ldflags` 零命中）、
  #700（原生 UI，无 `mygo-runtime`）、#686（`grep -rn "\.Watch(\|WatchDir" internal/` 零命中）、
  #607（`grep -rn "OnDragOver\|DragOver" internal/` 零命中）、#685（无 Router 历史）、
  #711/#707/#584/#684（`grep -rn "ui/editor" internal/ cmd/` 零命中）、#710/#712/#695/#691（无 webview）。
- **仍接不上（等拍板）：1 条** — #688/#689 镜像会话，卡在 `Terminal.NewMirror`（P9-g「需用户拍板」表）。

## 3. 本轮唯一的产品级落地：中文词边界

### 为什么是它

发布说明 #671 原文：*"In Chinese and Japanese text, each character now counts as its own word.
Word-by-word deleting, moving and double-click selection no longer swallow a whole clause."*
落地在 `internal/text/word.go`（新增：`IsWordRune` / `StandsAlone` / `WordBoundary`）与
`ui/textbuffer.go:302` 的 `nextWord` / `:329` 的 `prevWord`（注释：「each ideograph and kana is a word」）。

本仓**没有**自研词切分可以复用：`grep -rn "isWordChar\|wordBoundary\|prevWordStart" internal/ cmd/`
零命中；`internal/inputtext/inputtext.go` 只有 `isSeparator`/`NormalizeSpaces`/`StripPasteNewline`。
所以这条**只能靠升级拿到**——正好是「基础功能」优先级，且直接命中本仓的中文用户：
搜索框里用输入法打一句中文，再用 ⌘⌫/Ctrl+⌫ 修剪最后一个字。

### 新增的两条 pin（`internal/launcher/cjkword_test.go`）

两条都**走真实按键路径**（`tt.Type` 送 IME 文本、`tt.Key(historyDeleteMods, ui.KeyBackspace)` 送键），
不直接调 `prevWord`：启动器在字段非空时**不认领**这个键（`internal/launcher/fieldkeys.go:124-125`
注释「claimed only while … nothing is typed in it」），所以这个按键真的落到编辑器上——
也就是升级改的那段代码。

| pin | 断言 |
|---|---|
| `TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter` | `搜索应用` + ⌘⌫ → `搜索应` |
| `TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord` | `git status` + ⌘⌫ → `git `（拉丁仍删整词，升级不许把它打回退） |

台账：**V=0**（没碰任何像素/数值 pin）· **B 既有 pin 零改动全绿** · **S=0** · 新增 **+2** 测试函数。

## 4. 门槛实测（HEAD `b36d668` + 本轮改动，`touch` 后复跑）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净（exit 0，零诊断）
go test -count=1 ./...                    → exit 0，36 包（34 ok + 2 无测试）
--- PASS 2048   （基线 2046，+2）
--- SKIP 15     （= 基线 15）
--- FAIL 0
测试函数 1668    （基线 1666，+2）
GOOS=linux go build ./...                 → exit 0
GOOS=windows go build ./...               → exit 0
GOOS=darwin go build ./...                → exit 0
```

## 5. mutation（3 条实跑变红 + 逐字节还原）

**M1 真正的降级**（`go.mod` 回 `v0.6.3` + `go mod tidy`）：

```
=== RUN   TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter
    cjkword_test.go:35: ⌘⌫ on "搜索应用" gave "", want "搜索应" — one character, not the clause
--- FAIL: TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter (0.07s)
=== RUN   TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord
--- PASS: TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord (0.00s)
```

这是本轮最有说服力的一条：**整句被吞成 `""`** 就是 v0.6.3 的真实行为，而拉丁那条在旧版仍绿
——证明这两条 pin 钉的正是「这次升级带来的差别」，不是恒真断言。还原后
`md5sum -c` 两文件成功（`go.mod: 成功` / `go.sum: 成功`）。

**M2 本仓代码变异**（`internal/launcher/fieldkeys.go:124` `return a.historyDeleteKey()` → `return true`，
即让字段无条件吞掉 ⌘⌫）：

```
    cjkword_test.go:35: ⌘⌫ on "搜索应用" gave "搜索应用", want "搜索应" — one character, not the clause
    cjkword_test.go:56: ⌘⌫ on "git status" gave "git status", want "git " — a spaced-script word, all of it
--- FAIL: TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter (0.06s)
--- FAIL: TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord (0.01s)
```

两条同时红 → 证明它们走的是真实按键接线，不是直接调库函数。`md5sum -c` 还原成功。

**M3 同族另一半不许恒真**（把拉丁 pin 的期望 `git ` 改成 `git s`，即假设上游把拉丁也按字切）：

```
    cjkword_test.go:56: ⌘⌫ on "git status" gave "git ", want "git s" — a spaced-script word, all of it
--- FAIL: TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord (0.04s)
```

`md5sum -c` 还原成功；随后 `touch internal/launcher/cjkword_test.go` 再复跑第 4 节那组数字
（避免测试缓存复用）。

## 6. 仍待（下一轮候选，按优先级）

1. **用户侧实机复测**（Windows 透明 / 三大金刚 / 卡死 / 几何日志）——连续第 30 轮挂着，本仓无可改代码。
2. **需用户拍板的四条**（`.pending-bugs.md` 的 P9-g 表）：`Options.OnCopy` 复制反馈、
   `Terminal.NewMirror`（v0.6.4 的 #688/#689 就卡在这）、`plugins/terminal` 接 `InputZoom`、
   `--help` 文案。
3. **v0.6.4 新增的两条候选，需拍板才动**：① #692 日语 locale —— 加语言就是改 i18n 文案面，
   属拍板；② #642 `MYGO_LDFLAGS` —— 本仓没有 `-X` 注入需求，除非以后要把版本号编进二进制。
4. **自查候选（基础功能）**：`apps.ShortcutTargetName`（`internal/apps/shortcut.go:198`）与
   `apps.desktopFileStem`（`internal/apps/apps.go:710`）剥**任意**扩展名，与 `internal/progname`
   剥**启动器**后缀是两套规则——P9-z「仍待」第 5 条，仍是下一轮最实的一块（属 B 类）。
