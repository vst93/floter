# P11-r 规格：一个文本文件的行只有一个读法——`\r` 与 BOM 不该改变解析结果

## 意图

同一个「把文本切成行」的问题，全仓有 7 个答案（6 处 `strings.Split(text, "\n")` + 1 处
`bufio.Scanner`），其中 2 处自己补了 `TrimSuffix(line, "\r")`、1 处自己补了
`TrimPrefix(line, "\ufeff")`、4 处什么也没补。结果是**同一份文本在 Windows 上被读成另一种
东西**——本轮把它收成一个读法，并修掉由此产生的两个真 bug。

为什么现在做：用户定的方向是「三平台一致性 → 基础功能 → 像素细节」。这一族正是三平台
一致性：`\r` 只出现在 Windows 的进程输出里，而解析代码在 Linux 门禁上永远看不到它。

## 证据（实测，非旧报告）

`internal/extensions/helpargs.go:86` 用 `strings.Split(helpOutput, "\n")` 后把**未 trim 的
`line`** 交给 `parseOptionLine(line)`（`:221`）。`parseOptionLine` 用
`strings.IndexAny(rest, " \t")` 切 token，行尾的 `\r` 因此粘在最后一个 token 上，
`flagToken`/`placeholderValue` 都不认它。本机实测（临时测试，已删除）同一份 help 文本：

```
LF:   {Names:[-A --after-context] Kind:string Description:"Show NUM lines after each match." TakesValue:true ValueHint:NUM}
      {Names:[-n] Kind:string Description:"count" TakesValue:true ValueHint:int}
      {Names:[--quiet] Kind:flag Description:"be quiet"}
CRLF: {Names:[-A --after-context] Kind:flag Description:"<NUM>" TakesValue:false}
      {Names:[-n] Kind:flag Description:"int" TakesValue:false}
      （--quiet 整条消失）
```

即 Windows 上：带值的选项被当成开关（扩展表单不再要值、拼出的 argv 少一个参数）、
真正的描述被占位符文本顶掉、没有描述的开关**整条从提示里消失**。

第二个 bug：`internal/apps/apps.go:583` 的 `.desktop` 读法用
`strings.TrimSpace(scanner.Text())`——`U+FEFF` 不是空白，所以带 BOM 的 `.desktop` 首行是
`\ufeff[Desktop Entry]`，`inEntry` 永远为 false，**整个应用消失**；而同一批文件的另一个读法
`internal/inventory/sources.go:153` 明确 `TrimPrefix(line, "\ufeff")`，会认出它。同一个文件，
两个答案。

## 落地项（动手前已核对行号）

| # | 位置 | 现状 | 应改成 |
|---|------|------|--------|
| 1 | 新增 `internal/textlines/textlines.go` | 无 | `Split(text string) []string`：按 `\n` 切；每行去掉一个行尾 `\r`；**第一行**去掉一个行首 `\ufeff`。一个函数，一条规则 |
| 2 | `internal/apps/apps.go:583` | `bufio.Scanner` + `TrimSpace(scanner.Text())` | `os.ReadFile` + `textlines.Split`（与 inventory 同读法；顺带解掉 Scanner 64KB 行上限） |
| 3 | `internal/apps/registry.go:95-96` | `Split` + 手写 `TrimSuffix(line, "\r")` | `textlines.Split`，删手写 trim |
| 4 | `internal/browser/tabs.go:179-180` | 同上 | 同上 |
| 5 | `internal/extensions/helpargs.go:86` | `Split`，原始 `line` 进 `parseOptionLine` | `textlines.Split`（bug 1 的修法） |
| 6 | `internal/extensions/helpargs.go:538` | `Split` | `textlines.Split`（同一读法，本轮不改行为） |
| 7 | `internal/settingsui/settingsui.go:6760` | `Split` 用户输入的选项 | `textlines.Split`（Windows 粘贴的 `\r\n` 不再留 `\r`） |
| 8 | `internal/inventory/sources.go:152-153` | `Split` + 手写 BOM trim | `textlines.Split`，删手写 BOM trim |
| 9 | 新增 `internal/ci/textlines_contract_test.go` | 无 | `TestTextLinesHaveOneReader`：`internal/`、`cmd/` 的非测试源码里，除 `internal/textlines` 外不许出现 `strings.Split(…, "\n")` / `TrimSuffix(…, "\r")` / `TrimPrefix(…, "\ufeff")` |

**不动的**（写在这里免得下一轮当缺陷重领）：
- `internal/inputtext` 的 `HasSuffix(text, "\r\n")`：它问的是「粘贴的尾巴是不是一个换行」，
  不是切行。
- `internal/apps/darwin.go` 的 UTF-16 BOM：那是**编码**嗅探（`0xFE 0xFF` / `0xFF 0xFE`），
  不是行框定。
- `internal/launcher/wordgap.go` 的 `\ufeff`：搜索词归一化的零宽字符集合。
- 两个 `.desktop` 读法的**字段覆盖**差异（locale 阶梯、`\s` 反转义、`TryExec`）本轮不动：
  合并它们要改 inventory 的显示名，牵连既有 pin，单开一轮。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动。本轮只改「文本 → 行」
这一个边界：非 Windows 上的解析结果必须**逐字节不变**（由既有 pin 守着，B 类零改动）。

## 门槛命令与 PASS 基线（HEAD `ecaf18c` 实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2282
grep -c -- "--- SKIP"（同上输出）                        → 15
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 1853
三平台 go build ./...                                   → 过
```

## mutation 计划（每条都要实跑、看到红、`md5sum -c` 还原、`touch` 复跑）

1. `textlines.Split` 去掉 `TrimSuffix(line, "\r")` → 期望
   `TestHelpFromWindowsParsesLikeHelpFromUnix` 与 `internal/apps/registry` 的 CRLF pin 变红。
2. `textlines.Split` 去掉第一行的 BOM trim → 期望
   `TestDesktopFileWithBOMIsStillAnApp`（apps）与 `internal/inventory` 的 BOM pin 变红。
3. `helpargs.DeriveArguments` 改回 `strings.Split(helpOutput, "\n")`（绕过新读法）→ 期望
   `TestHelpFromWindowsParsesLikeHelpFromUnix` 变红且 `TestTextLinesHaveOneReader` 变红。
