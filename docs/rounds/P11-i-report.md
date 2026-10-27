# P11-i 报告：转义序列只有一个读法，charset 指示符不是文本

规格 `docs/rounds/P11-i-spec.md`。方向：用户实测四条仍卡实机复测、合约 §6 全「已落地」、
上一轮「仍待」第 1 条同样卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，
落在**基础功能**：`internal/ansi`（325 → 352 行 / 12 → 15 条 pin）与
`internal/extensions/helpargs.go`（唯一一个绕过它、自己又写了一遍剥转义的地方）。
无 `GOOS` 分支，Linux CI 上钉住三平台同一行为。

## 1. 病灶（`git rev-parse HEAD` = `edbe4bf`，行号本轮实测）

### 1.1 全仓有两个「读掉转义序列」的实现

| 位置 | 谁在用 | 走法 |
|---|---|---|
| `internal/ansi/ansi.go:87` `Parse` / `Plain` | launcher 文本输出（`launcher/output.go:126-127`） | 按字节，CSI/OSC/两字节序列都认 |
| `internal/extensions/helpargs.go:504` `stripANSI`（25 行） | `--help` 推导（`helpargs.go:78`、`:569`） | 按 `[]rune`，自己一套 CSI/OSC 规则 |

同一批输入两边答案不同（本轮实测）：

```
输入                        ansi.Plain           stripANSI
"\x1b(Bhello"               "Bhello"             "Bhello"     ← 共有缺陷
"\x1b)0hello"               "0hello"             "0hello"     ← 共有缺陷
"\x1b#8hello"               "8hello"             "8hello"     ← 共有缺陷
"\x1b%Ghello"               "Ghello"             "Ghello"     ← 共有缺陷
"a\x1b[\ab"                 "a[\ab"              "a"          ← stripANSI 吃正文
"a\x1b[\nfoo"               "a[\nfoo"            "aoo"        ← stripANSI 吃正文
```

### 1.2 共有缺陷：三字节指示符只吃掉两字节，第三个字节当正文

`skipEscape` 的 `default: return next + 1`（`ansi.go:173-174`）与 `stripANSI` 的
`default: i++`（`helpargs.go:542-544`）都只多跳一个字节。而
`ESC ( B`（ASCII 进 G0）、`ESC ) 0`（线框字符集进 G1）、`ESC # 8`（DECALN）、
`ESC % G`（选 UTF-8）是 **ESC + 中间字节 + 终止字节** 的三字节指示符，终端上不产生任何字符。
于是 `\x1b(Bhello` 在文本模式输出里显示成 `Bhello`，在 `--help` 推导里让 `--verbose`
变成 `B--verbose`（不再被认成选项行，整个参数被丢掉）。

### 1.3 `stripANSI` 的第二处自有答案：CSI 里夹控制字节时吃正文

`stripANSI` 在 `ESC [` 之后无条件扫到第一个 `0x40-0x7e` 就收尾，中间的控制字节（`\n`、`\a`）
不算终止；于是 `a\x1b[\nfoo` → `aoo`（丢了 `f`），`a\x1b[\ab` → `a`（丢了 `b`）。
`ansi` 的契约相反且已被 `ansi_test.go:196` 钉住：「不是控制序列就只丢 ESC，其余照读」。

## 2. 落地

- `internal/ansi/ansi.go`：新增 `isDesignator`（`( ) * + - . / # %`），`skipEscape` 对它们
  返回 `next + 2`（连第三字节一起吃掉）；不足三字节按「未终止序列」吞到末尾。
  `ESC =`/`ESC >`/`ESC 7`/`ESC 8`/`ESC M`/`ESC E` 这些**两字节**序列的读法一字不动。
  `Parse` 里 `default` 分支的注释从「两字节序列」改成「两字节或三字节序列」；
  `Plain` 的注释补一句它同时是 `--help` 推导前的清洗入口。
- `internal/extensions/helpargs.go`：**删掉 `stripANSI`（25 行）**，
  `DeriveArguments` 与 `DeriveSubcommands` 两处改成 `helpOutput = ansi.Plain(helpOutput)`，
  包内新增 `import "floter/internal/ansi"`（`ansi` 只 import 标准库，无环）。
- 行为契约零改动：`Parse` 的 span 切分、SGR 语义、`Plain` 拼接、两个 Derive 的行分类与上限、
  `launcher/output.go` 的所有调用、i18n 文案、设置写回、`GOOS` 分支全部未动。
  非视觉轮，不出图。

## 3. pin（6 条，3 个新测试函数 + 3 条并入既有文件）

| # | 位置 | 断言 | 类 |
|---|---|---|---|
| 1 | `internal/ansi/ansi_test.go` `TestDesignationsAreThreeBytes` | 9 个指示符（`( ) * + - . / # %`）各配两种位置（行首 / 两段正文之间）：正文原样、指示符一个字节不留 | B |
| 2 | 同文件 `TestTwoByteEscapesStayTwoBytes` | `\x1b=` `\x1b>` `\x1b7` `\x1b8` `\x1bM` `\x1bE` 仍只吃两字节 | B |
| 3 | 同文件 `TestTruncatedDesignation` | 结尾半截 `\x1b(` 吞到末尾、`\x1b` 单独不吐控制字节、不 panic | B |
| 4 | `internal/extensions/helpargs_test.go` `TestCharsetDesignationsNeverBecomePartOfAName` | 带 `\x1b(B` 的 help：`--verbose` 不带 `B`；子命令名 `jv` 不带 `B` | B |
| 5 | 同文件 `TestHelpDerivationHasOneEscapeReader` | **一个读法**：`DeriveArguments(原始)` ≡ `DeriveArguments(ansi.Plain(原始))`，名字/ValueHint/Description 逐字段相等（含 OSC 8 超链接、SGR、指示符、破损 CSI 混在一份输入里） | B |
| 6 | 同文件 `TestBrokenControlSequenceKeepsTheLine` | 破损 CSI 后面的 `-sort` 行照旧被推导出来 | B |

台账：**V = 0**（无视觉数值）· **B = 6** · **S = 0**（无结构断言变化）。
既有 pin 一条都没改；`TestANSIColorCodesAreStrippedBeforeParsing`（`helpargs_test.go:316`）
作为回归护栏一并变红/变绿（见 §5 mutation 3）。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                                空
go vet ./...                                          净
go test -count=1 ./...                                全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"     2256   （基线 2250，+6）
--- SKIP                                              15
--- FAIL                                              0
测试函数                                              1827   （基线 1821，+6）
34 ok + 2 no test files                               36 行
GOOS=linux / windows / darwin go build ./...          三平台过
```

## 5. mutation（3 条实跑，全部变红、逐字节还原、`touch` 复跑）

### M1 `skipEscape` 的 `return next + 2` → `return next + 1`

```
--- FAIL: TestDesignationsAreThreeBytes (0.00s)
    ansi_test.go:225: Plain("\x1b(Bhello") = "Bhello", want "hello"
    ansi_test.go:229: Plain("a\x1b(Bb") = "aBb", want "ab"
    ...（9 个指示符 × 2 个位置共 18 行，全部 Bhello/Ahello/0hello/8hello/Ghello 形态）
FAIL
--- FAIL: TestCharsetDesignationsNeverBecomePartOfAName (0.00s)
    helpargs_test.go:622: names = []
```

### M2 把 `'('` 从 `isDesignator` 的引导集里删掉（只留 `) * + - . / # %`）

```
--- FAIL: TestDesignationsAreThreeBytes (0.00s)
    ansi_test.go:225: Plain("\x1b(Bhello") = "Bhello", want "hello"
    ansi_test.go:229: Plain("a\x1b(Bb") = "aBb", want "ab"
FAIL
--- FAIL: TestCharsetDesignationsNeverBecomePartOfAName (0.00s)
    helpargs_test.go:622: names = [-sort]
```

`TestTwoByteEscapesStayTwoBytes` 同时跑、仍绿——证明 pin 2 钉的是另一件事（两字节序列
不许被读成三字节），不是 pin 1 的重复。

### M3 `DeriveArguments` 去掉 `ansi.Plain`（回到「不清洗」）

```
--- FAIL: TestANSIColorCodesAreStrippedBeforeParsing (0.00s)
    helpargs_test.go:329: names = []
--- FAIL: TestCharsetDesignationsNeverBecomePartOfAName (0.00s)
    helpargs_test.go:622: names = []
--- FAIL: TestHelpDerivationHasOneEscapeReader (0.00s)
    helpargs_test.go:644: raw = [], read = [{Names:[-sort] ...} {Names:[-out] ...} {Names:[--verbose] ...}]
```

`TestBrokenControlSequenceKeepsTheLine` 在 M3 下仍绿（那一行本就不依赖清洗）——如实记下：
规格 §5 里写的「pin 6 也红」**没有发生**，实际变红的是 pin 4、pin 5 与既有 pin。
`md5sum -c` 三次还原均逐字节通过（`ansi.go` `fccce6d1…`、`helpargs.go` `c353dc1a…`），
还原后 `touch` 两文件再跑一次，绿。

## 6. 仍待

1. 用户实机复测四条（①透明 ②外圈底色/三大金刚 ③卡死 ④打包），连续第 64 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」 + 非 macOS 快捷键词序 +
   `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. `internal/ansi` 的读法本轮收口（`stripANSI` 已删），**别在里面再找小修**。
   未做的两条都属另一个契约、需要拍板或另开一轮：
   - `\r`（进度条回车）不是转义序列，`ansi` 不管；文本模式输出里 `curl` 式进度条仍会带 `\r`。
   - `internal/terminalui` 从 pty 流里抽 OSC 7 / OSC 9 / OSC 777（`terminalui.go:62`、`:406`）
     是**另一件事**：它必须把字节原样转发给终端、只顺手抄一份报告，不能「读掉」任何东西，
     因此不该并到 `ansi` 里。
4. 下一处候选：`internal/inputtext`（205 行 / 7 条）的编辑行为三平台是否同一套；
   `internal/calc`（875 行 / 7 条）与 `internal/calculator`（1204 行 / 12 条）的边界是否
   只有一个答案（P11-g 已收口规范形，剩「要不要每次写都从磁盘重读」需拍板）。
