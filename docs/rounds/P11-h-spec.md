# P11-h 规格：一个属性列表文件只有一个读法

方向：用户实测四条全卡实机、合约 §6 全「已落地」、上一轮「仍待」第 1 条同样卡实机 →
按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，落在**基础功能 / 稳定性**：
`internal/plist`（520 行、3 条 pin）与 `internal/apps` 的 UTF-16 解码。
`internal/plist` 是 macOS 侧「应用叫什么名字」的唯一读入口（`apps/darwin.go:58`、
`browser/safari.go:22`），解码器没有 GOOS 分支，所以在 Linux CI 上就能钉住 macOS 的行为。

## 意图

**同一份属性列表，无论它用哪种编码、无论它的长度落在哪个字节上，本仓必须读出同一个答案；
读不出来时必须是 `error`，不能是错误的值，更不能让进程死。**

## 逐条落地项（行号本轮实测，`git rev-parse HEAD` = `0270e42`）

### 1. `Parse` 对二进制文档也剥空白 → 长度落在 0x0a 的合法文件被拒

位置 `internal/plist/plist.go:34-44`。现状：

```go
trimmed := bytes.TrimSpace(data)
switch {
case bytes.HasPrefix(trimmed, []byte("bplist")):
    return parseBinary(trimmed)
```

`bytes.TrimSpace` 按 `unicode.IsSpace` 剥 ASCII `\t \n \v \f \r ' '`。二进制属性列表的
**最后 32 字节是 trailer**，它的最后一个字节是 `offsetTableOffset` 的低字节——只要偏移表
起点 ≡ 0x09/0x0a/0x0b/0x0c/0x0d/0x20 (mod 256)，这个合法文件就会被削掉尾巴，
`data[len(data)-32:]` 随之错位。

实测（同一份文档，只差一个填充字节）：

| 文件 | 末字节 | `Parse` |
|---|---|---|
| `/tmp/p-0a.bin` | `0x0a` | `plist: not a property list: bad binary trailer` |
| `/tmp/p-ok.bin` | `0x0e` | `42`，`err=nil` |

应改成：`bplist` 头按规范就在偏移 0，**不剥空白**就判二进制；只有 XML 一路才剥
（XML 前面的空白/`<?xml` 之前的空行是合法的）。

### 2. UTF-16 只有一个答案：非 BMP 字符不再碎成两个 `U+FFFD`

位置与现状（三处手写解码器，两种行为）：

| 位置 | 做法 | 非 BMP（🎮 / 𠀀 / 部分符号） |
|---|---|---|
| `internal/plist/plist.go:466-471`（0x6 UTF-16BE 串） | `rune(unit)` 逐码元 | **两个 `U+FFFD`** |
| `internal/apps/darwin.go:158-165`（UTF-16LE `.strings`） | `rune(unit)` 逐码元 | **两个 `U+FFFD`** |
| `internal/apps/darwin.go:168-175`（UTF-16BE `.strings`） | `rune(unit)` 逐码元 | **两个 `U+FFFD`** |
| `internal/apps/shortcut.go:146-171`（`.lnk` 的 LinkInfo） | 手写配对，未配对 → `U+FFFD` | 正确 |
| `internal/apps/registry.go:248` | `utf16.Decode` | 正确 |

实测（`plistlib` 写出的同一份文档，两个编码）：

```
binary name="�� Games" blob=00016869
xml    name="🎮 Games" blob=4141466f61513d3d
```

用户可见：macOS 上一个名字里带 emoji / 扩展区汉字的 app，启动器行标题是 `�� …`；
Windows 的 `.lnk` 走的是已经正确的那条，所以这是**同一个问题在本仓的两条答案**。
应改成：四处都走标准库 `unicode/utf16.Decode`（未配对码元 → 一个 `U+FFFD`，与
`shortcut.go` 现有语义一致），删掉 `shortcut.go` 的手写版本。

### 3. 扩展宽度（0x1f / 0x2f）的整数与实数读的是 `1<<n` 字节

位置 `internal/plist/plist.go:390-419`。`length`（`:481-503`）对 `marker&0x0f == 0x0f`
返回的 `size` 是**字节数**（规范：后面跟一个整数对象给长度），而整数/实数分支用的是
`1<<size`（那是非扩展路径里 `size` 的语义：log2 字节数）。于是：

```
/tmp/p-int.bin  (0x1f + 整数 8 + 8 字节值 42)  -> err=read of 256 bytes at 18
/tmp/p-real.bin (0x2f + 整数 8 + float64 1.5) -> err=read of 256 bytes at 11
```

应改成：整数/实数分支按「扩展则 `size` 字节、否则 `1<<size` 字节」取宽度；
其余分支（data / string / array / dict / set / UID）语义不变。

### 4. XML 的 `<data>` 没解码：同一份文档两种编码给出两组字节

位置 `internal/plist/plist.go:129-134`（`readXMLElement`）与 `:225-230`
（`readXMLElementAfter`）：`[]byte(strings.TrimSpace(text))`——返回的是 base64 **文本**。
二进制一路（`:441-447`）返回的是真字节。实测同一份文档：`binary blob=00016869`、
`xml blob=4141466f61513d3d`（`AAFoaQ==` 的 ASCII）。

应改成：剥掉空白后按 base64 解码（`StdEncoding`，无填充时退 `RawStdEncoding`），
坏 base64 → `ErrInvalid`。既有 pin `plist_test.go:106-108` 钉的是**旧的错答案**
（`string(blob) != "aGk="`），按 **S** 类改断言并注明重定基线。

### 5. 深嵌套 / 自引用文档 → 栈溢出，进程直接死

位置：`internal/plist/plist.go:344`（`object`）、`:360`（`value`）与 XML 侧
`:81/:151/:207` 的互相递归，**都没有深度上限**。一个自引用的二进制文档（对象 0 是
「含 1 个指向对象 0 的数组」）：

```
runtime: goroutine stack exceeds 1000000000-byte limit
fatal error: stack overflow
```

`Parse` 的包注释写的是「a malformed document is an error, never a partly-filled map」，
而这里是**进程级崩溃**：`apps.readBundle` 在扫描时对每个 `.app` 的 `Info.plist` 调它
（`internal/apps/darwin.go:58`），一个坏文件就能让整个启动器起不来。
应改成：二进制与 XML 两侧都带深度上限（128），超限 → `ErrInvalid` + `nesting`。

## 行为契约零改动红线

- 不改 `String`/`firstString` 的语义；不改 `apps` 的名字选择规则（`resolveBundleNames`、
  `englishLocales`/`chineseLocales` 顺序）；不改 `browser/safari.go` 的字段名。
- 不改任何 i18n 文案、设置写回、命令接线、高度链、键盘模型。
- `Parse` 的对外签名与「读不出来就 `error`」的契约不变；`ErrInvalid` 仍是唯一哨兵。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2243，只许涨
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 基线 15
grep -rhc "^func Test" --include=*_test.go internal cmd # 基线 1814
GOOS=linux / windows / darwin go build ./...           # 三平台过
```

非视觉轮：不出图（`internal/plist` 没有像素）。

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `Parse` 改回 `bytes.TrimSpace(data)` 判二进制 | `TestABinaryDocumentIsNotTrimmed`（末字节 0x0a 的那份 `bad binary trailer`） |
| M2 | 0x6 串改回逐码元 `rune(unit)` | `TestNonBMPNamesSurviveBothEncodings`（`�� Games` ≠ `🎮 Games`） |
| M3 | 整数/实数宽度改回 `1<<size` | `TestExtendedWidthsReadTheByteCount`（`read of 256 bytes`） |
| M4 | 去掉深度上限 | `TestADeepOrSelfReferencingDocumentIsAnError`（栈溢出/无 error） |
| M5 | XML `<data>` 改回 `[]byte(text)` | `TestXMLDataIsDecoded`（`aGk=` ≠ `hi`） |
