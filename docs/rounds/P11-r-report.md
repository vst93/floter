# P11-r 报告：一个文本文件的行只有一个读法

规格：`docs/rounds/P11-r-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**三平台一致性**：`\r` 只出现在 Windows 的进程输出里，Linux 门禁永远看不到它。

## 1. 意图

「把一段文本切成行」这个问题全仓有 7 个答案（6 处 `strings.Split(text, "\n")` + 1 处
`bufio.Scanner`），其中 2 处自己补了 `TrimSuffix(line, "\r")`、1 处自己补了
`TrimPrefix(line, "\ufeff")`、4 处什么也没补。本轮把它收成 `internal/textlines.Split`
一个读法，并修掉由此产生的两个真 bug。

## 2. 实测证据（本轮动手前）

**bug 1：Windows 上 `--help` 的解析结果与 Linux 不同。**
`internal/extensions/helpargs.go:86` 把**未 trim 的 `line`** 交给 `parseOptionLine`
（`:221`），后者用 `strings.IndexAny(rest, " \t")` 切 token，行尾 `\r` 粘在最后一个 token 上，
`flagToken`/`placeholderValue` 都不认。临时测试（已删除）实测同一份文本：

```
LF:   [-A --after-context] kind=string desc="Show NUM lines after each match." TakesValue=true hint=NUM
      [-n] kind=string desc="count" TakesValue=true hint=int
      [--quiet] kind=flag desc="be quiet"
CRLF: [-A --after-context] kind=flag desc="<NUM>" TakesValue=false
      [-n] kind=flag desc="int" TakesValue=false
      （--quiet 整条消失）
```

**bug 2：带 BOM 的 `.desktop` 在应用列表里消失，在工具列表里还在。**
`internal/apps/apps.go:583` 用 `TrimSpace(scanner.Text())`；`U+FEFF` 不是空白（Cf 而非 Zs），
首行读成 `\ufeff[Desktop Entry]`，`inEntry` 永远 false。同一批文件的另一个读法
`internal/inventory/sources.go:153` 明确 `TrimPrefix(line, "\ufeff")`，会认出它。

## 3. 落地

- 新增 `internal/textlines/textlines.go`：`Split(text string) []string` —— 按 `\n` 切，
  每行去掉**一个**行尾 `\r`，**第一行**去掉**一个**行首 BOM。只有 `\n` 断行（孤立的 `\r`
  是内容：终端进度条的重绘）；第二个 `\r`、文件中间的 BOM 都是内容。
- 七个调用点转发：`apps.ParseDesktopFile`（`os.Open`+`bufio.Scanner` → `os.ReadFile`+`Split`，
  顺带解掉 Scanner 的 64KB 单行上限）、`apps.ParseRegistryPrograms`、`browser.parseAppleScriptTabs`、
  `extensions.DeriveArguments`、`extensions.DeriveSubcommands`、`settingsui.editorParams`、
  `inventory.parseDesktopEntry`。
- 新增契约扫描 `internal/ci/textlines_contract_test.go` 的 `TestTextLinesHaveOneReader`：
  `internal/`、`cmd/` 的非测试源码里，除 `internal/textlines` 外不许出现
  `strings.Split(…, "\n")` / `TrimSuffix(…, "\r")` / `TrimPrefix(…, "\ufeff")`。

**故意不动**（免得下一轮当缺陷重领）：`internal/inputtext` 的 `HasSuffix(text, "\r\n")`（问的是
「粘贴的尾巴是不是换行」）；`internal/apps/darwin.go` 的 UTF-16 BOM（**编码**嗅探，不是行框定）；
`internal/launcher/wordgap.go` 的 `\ufeff`（搜索词归一化的零宽字符集合）。两个 `.desktop` 读法的
**字段覆盖**差异（locale 阶梯、`\s` 反转义、`TryExec`）本轮不动——合并要改 inventory 的显示名，
牵连既有 pin，单开一轮。

## 4. 台账

- **V = 0**（没有视觉数值）。
- **B = 0 既有断言改动**：`internal/apps/registry_test.go` 的 CRLF 夹具、
  `internal/browser/tabs_test.go`、`internal/inventory` 既有全部用例、`helpargs_test.go` 30 条
  一律零改动全绿。
- **S = 1**：`internal/inventory/desktopentry_test.go` 的过滤表**新增**一行夹具
  （BOM+CRLF 的 `.desktop`），既有行一字未动。
- 新增测试函数 6 条（`textlines` 2、`helpargs_crlf` 2、`apps` BOM 1、`ci` 契约 1）。

## 5. 门槛（实测，HEAD `ecaf18c` 基线 → 本轮）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → exit 0，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2300（基线 2282，+18）
grep -c -- "--- SKIP"                                  → 15（基线 15，未新增）
包行数（^(ok|\?)）                                      → 40（38 ok + 2 no test files；基线 38 = 36+2）
测试函数 grep -rhn '^func Test' internal/ cmd/ | wc -l  → 1859（基线 1853，+6）
三平台 go build ./...                                   → BUILDS-OK
```

## 6. 变异（3 条实跑，全部变红并逐字节还原）

1. `textlines.Split` 去掉 `TrimSuffix(line, "\r")`（`lines[i] = line`）：

```
--- FAIL: TestSplitRemovesTheLineFraming/windows_lines
    textlines_test.go:34: Split("a\r\nb\r\n") = ["a\r" "b\r" ""], want ["a" "b" ""]
--- FAIL: TestHelpFromWindowsParsesLikeHelpFromUnix
    helpargs_crlf_test.go:42: CRLF help parsed differently:
```

（`internal/apps/registry` 的 CRLF pin 仍绿——它的 `TrimSpace(parts[2])` 本来就能吃掉 `\r`，
那条手写 trim 是双保险；如实记在这里。）

2. `textlines.Split` 去掉第一行的 BOM trim：

```
--- FAIL: TestSplitRemovesTheLineFraming/bom_on_the_first_line
--- FAIL: TestDesktopFileWithBOMIsStillAnApp/bom_only
    desktopbom_test.go:33: ParseDesktopFile rejected a BOM'd entry
--- FAIL: TestTheDesktopEntryFiltersWhatIsNotATool
    desktopentry_test.go:138: an entry with a windows BOM and CRLF: parsed = false, want true
```

3. `DeriveArguments` 改回 `strings.Split(helpOutput, "\n")`：

```
--- FAIL: TestHelpFromWindowsParsesLikeHelpFromUnix
--- FAIL: TestTextLinesHaveOneReader
    textlines_contract_test.go:89: text lines are read outside internal/textlines:
    internal/extensions/helpargs.go: for _, line := range strings.Split(helpOutput, "\n") {
```

每条之后 `cp` 还原 + `md5sum -c` 输出「成功」+ `touch` 再跑（避免测试缓存）。最后按第 5 节
全量复跑一次：2300 PASS / 15 SKIP / 0 FAIL。

## 7. 非视觉轮

不出图。

## 8. 仍待

1. 用户实机复测四条（透明 / 三大金刚 / 卡死 / 打包），连续第 73 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. 下一处候选：**两个 `.desktop` 读法的字段覆盖差异**（`apps.ParseDesktopFile` 有 locale 阶梯 /
   `\s` 反转义 / `TryExec`，`inventory.parseDesktopEntry` 没有）。行框定已由本轮收口，字段层
   仍是两个答案；合并会改 inventory 的显示名，需要先量清楚哪些 pin 依赖它。
4. `internal/textlines` 只处理 `\n` 与 BOM。**不要**在里面加 UTF-16 解码（那是
   `internal/apps/darwin.go` 与 `internal/plist` 的编码嗅探，另一个问题）。
