# P11-i 规格：转义序列只有一个读法，charset 指示符不是文本

## 0. 意图（一句话）

终端输出里的转义序列，本仓只留**一个**读法（`internal/ansi`）；顺带修掉这个读法自己的
一个洞——三字节的 charset / 屏幕对齐指示符只被吃掉两个字节，第三个字节当文本留在屏幕上。

为什么现在做（证据）：

- 全仓有两个「把转义序列读掉」的实现：`internal/ansi/ansi.go:87`（`Parse` / `Plain`，
  launcher 文本输出用，`internal/launcher/output.go:126-127`）与
  `internal/extensions/helpargs.go:504`（`stripANSI`，`--help` 推导用，
  调用点 `helpargs.go:78` 与 `helpargs.go:569`）。两者对**同一份字节**给出不同答案
  （下表实测，`git rev-parse HEAD` = `edbe4bf`）。
- 两个实现**共有一个**缺陷：`ESC ( B` / `ESC ) 0` / `ESC # 8` / `ESC % G` 这类
  「ESC + 中间字节 + 终止字节」的三字节指示符，两边都只吃两字节，第三字节 `B`/`0`/`8`/`G`
  被当成正文。终端上这类序列**不产生字符**，于是用户会在文本模式输出与推导出的参数名里
  看到一个凭空多出来的字母。
- `stripANSI` 还比 `ansi` 多两处自己的答案：CSI 里夹控制字节时它会把后面的**正文吃一个字符**
  （`a\x1b[\nfoo` → `aoo`，`a\x1b[\ab` → `a`），而 `ansi` 的契约是「不是控制序列就只丢 ESC」
  （`ansi_test.go:196` 已钉）。

实测（本轮，同一批输入）：

```
输入                        ansi.Plain           stripANSI
"\x1b(Bhello"               "Bhello"             "Bhello"     ← 共有缺陷
"\x1b)0hello"               "0hello"             "0hello"     ← 共有缺陷
"\x1b#8hello"               "8hello"             "8hello"     ← 共有缺陷
"\x1b%Ghello"               "Ghello"             "Ghello"     ← 共有缺陷
"a\x1b[\ab"                 "a[\ab"              "a"          ← 吃正文
"a\x1b[\nfoo"               "a[\nfoo"            "aoo"        ← 吃正文
```

选它的理由：`internal/ansi` 是 P8 之后文本输出的唯一读入口（12 条 pin），
`internal/extensions/helpargs.go` 是唯一一个绕过它、自己又写了一遍的地方；
上一轮报告「仍待」第 4 条点的正是这一处。属于用户定的「三平台一致性 → 基础功能」里的
**基础功能**，且三平台同错（无 `GOOS` 分支，Linux CI 就能钉住）。

## 1. 逐条落地项（行号本轮实测）

### 1.1 `internal/ansi/ansi.go:154` `skipEscape`：三字节指示符吃掉第三字节

- 现状：`default: return next + 1`——`ESC` 之后无论是什么，都只多跳一个字节。
  `ESC ( B` 因此只吃掉 `ESC` 与 `(`，`B` 作为正文留下（实测 `Plain("\x1b(Bhello")` = `"Bhello"`）。
- 应改成：ESC 后的字节若属于指示符引导集 `( ) * + - . / # %`，且后面还有字节，
  连第三个字节一起吃掉（`return next + 2`）；不足三字节时按「未终止序列」处理（吞到末尾）。
  `ESC =`、`ESC >`、`ESC 7`、`ESC 8`、`ESC M` 这些**两字节**序列的读法不动。
- 依据：ECMA-48 里 `ESC (`/`ESC )`/`ESC *`/`ESC +`/`ESC -`/`ESC .`/`ESC /` 是
  「中间字节 + 终止字节」的 3 字节指示符，`ESC #`（DECALN 等）、`ESC %`（UTF-8 选择）同理；
  它们都不产生可显示字符。

### 1.2 `internal/extensions/helpargs.go:500-546`：删掉 `stripANSI`，改调 `ansi.Plain`

- 现状：`stripANSI` 25 行自写实现（按 `[]rune` 走），与 `ansi` 对同一输入给出不同答案（见 §0）。
- 应改成：删掉该函数，`helpargs.go:78` 与 `helpargs.go:569` 两处改成
  `helpOutput = ansi.Plain(helpOutput)`，包内 `import "floter/internal/ansi"`。
  文档注释里「CSI runs ending in a final byte…」那段随函数一起删，理由写到调用点上方一行。
- 依赖方向：`internal/ansi` 只 import 标准库（`math`/`strconv`/`strings`），
  `internal/extensions` → `internal/ansi` 无环。

### 1.3 顺带收口：`ansi.Plain` 的注释要写清「谁在用」

- 现状：`Plain` 的注释只说「复制到剪贴板或纯文本字段」。
- 应改成：补一句它同时是 `--help` 推导前的清洗入口（`internal/extensions`），
  这样下一个想写「剥转义」的人会先看到它。

## 2. 行为契约零改动红线

- **不动**：`Parse` 的 span 切分、SGR 语义、`Plain` 的拼接、`DeriveArguments` /
  `DeriveSubcommands` 的行分类与上限、`internal/launcher/output.go` 的任何调用、
  任何 i18n 文案、任何设置写回、任何 `GOOS` 分支。
- 视觉：本轮不出图（无渲染改动；`theme.ANSI` 与 `launcher/output.go` 的取色一字不动）。

## 3. 门槛命令 + PASS 基线（本轮实测，`edbe4bf`）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
go test -count=1 ./...                   # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2250
测试函数 1821 · SKIP 15 · FAIL 0 · 34 ok + 2 no test files
GOOS=linux/windows/darwin go build ./...  # 过
```

PASS 计数只许涨；测试函数数只许涨。

## 4. pin 计划

| # | 位置 | 断言 | 类 |
|---|---|---|---|
| 1 | `internal/ansi/ansi_test.go` | `\x1b(B` / `\x1b)0` / `\x1b#8` / `\x1b%G` / `\x1b-A` 后面紧跟的正文原样保留、指示符一个字节都不留 | B（新函数，改前 `Bhello`） |
| 2 | 同上 | 两字节序列 `\x1b=` `\x1b>` `\x1b7` `\x1bM` 仍然只吃两字节，其后的正文保留 | B（防把两字节也当三字节） |
| 3 | 同上 | 结尾处的半截指示符（`"x\x1b("`）不 panic、不吐控制字节 | B |
| 4 | `internal/extensions/helpargs_test.go` | 带 `\x1b(B` 的彩色 help：参数名 `--verbose` 不带前缀 `B`；子命令名不带 `B` | B（改前 `B--verbose`） |
| 5 | 同上 | **一个读法**：`DeriveArguments(x)` ≡ `DeriveArguments(ansi.Plain(x))`、`DeriveSubcommands` 同（同一份脏输入，两边逐字段相等） | B |
| 6 | 同上 | 破损 CSI（`a\x1b[\nfoo`）后面的正文不再被吃：`-sort` 行照旧被推导出来 | B（改前丢一个字符） |

V = 0（无视觉数值）；S = 0（无结构断言变化）；B = 6。既有 pin 一条都不改。

## 5. mutation 计划（≥2 条，说明各自会红哪个 pin）

1. 把 `skipEscape` 的 `return next + 2` 改回 `return next + 1`
   → pin 1 红（`Bhello`），pin 4 红（`B--verbose`）。
2. 把指示符引导集里的 `'('` 删掉（只留 `#%`）
   → pin 1 红（`\x1b(B` 那条），pin 2 仍绿（证明 pin 2 钉的是另一件事）。
3. 在 `helpargs.go` 里把 `ansi.Plain` 换回「原样返回」（模拟漏了清洗）
   → pin 4、pin 5、pin 6 红（既有 `TestANSIColorCodesAreStrippedBeforeParsing` 也会红）。
