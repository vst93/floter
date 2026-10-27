# P11-h 报告：一个属性列表文件只有一个读法

规格 `docs/rounds/P11-h-spec.md`。方向：用户实测四条全卡实机复测、合约 §6 全「已落地」、
上一轮「仍待」第 1 条同样卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，
落在**基础功能 / 稳定性**：`internal/plist`（520 行 / 3 条 pin）与 `internal/apps` 的 UTF-16 解码。
`internal/plist` 是 macOS 侧「这个应用叫什么名字」的唯一读入口（`apps/darwin.go:58` 的
`readInfoPlist`、`browser/safari.go:22` 的书签），解码器没有 `GOOS` 分支，所以在 Linux CI 上
就能钉住 macOS 的行为。

## 1. 病灶（`git rev-parse HEAD` = `0270e42`，行号本轮实测）

### 1.1 `Parse` 对二进制文档也剥空白 → 长度落在 0x0a 的合法文件被拒

`internal/plist/plist.go:34-44` 先 `trimmed := bytes.TrimSpace(data)` 再判 `bplist` 头。
`bytes.TrimSpace` 按 `unicode.IsSpace` 剥 `\t \n \v \f \r ' '`，而二进制属性列表的
**最后 32 字节是 trailer**，它的最后一个字节是 `offsetTableOffset` 的低字节——偏移表起点
≡ 0x09/0x0a/0x0b/0x0c/0x0d/0x20 (mod 256) 时，尾巴被削掉、`data[len-32:]` 随之错位。

实测（同一份文档，只差一个填充字节；`/tmp/p-0a.bin` 与 `/tmp/p-ok.bin`）：

```
/tmp/p-0a.bin lastbyte=0xa -> <nil> err=plist: not a property list: bad binary trailer
/tmp/p-ok.bin lastbyte=0xe -> 42 err=<nil>
```

### 1.2 UTF-16 在本仓有三条答案，两条把非 BMP 字符打碎

| 位置 | 做法 | 名字含 🎮 |
|---|---|---|
| `internal/plist/plist.go:466`（0x6 UTF-16BE 串） | `rune(unit)` 逐码元 | 两个 `U+FFFD` |
| `internal/apps/darwin.go:158`（UTF-16LE `.strings`） | `rune(unit)` 逐码元 | 两个 `U+FFFD` |
| `internal/apps/darwin.go:168`（UTF-16BE `.strings`） | `rune(unit)` 逐码元 | 两个 `U+FFFD` |
| `internal/apps/shortcut.go:146`（`.lnk` LinkInfo） | 手写配对，未配对 → `U+FFFD` | 正确 |
| `internal/apps/registry.go:248` | `utf16.Decode` | 正确 |

实测（`plistlib` 写出的同一份文档，两个编码）：

```
binary name="�� Games" blob=00016869
xml    name="🎮 Games" blob=4141466f61513d3d
```

macOS 上一个名字带 emoji / 扩展区汉字的 app，启动器行标题就是 `�� …`；Windows 的 `.lnk`
走的是已经正确的那条——同一个问题两条答案。

### 1.3 扩展宽度（0x1f / 0x2f）的整数与实数按 `1<<n` 读

`length`（`:481`）对 `marker&0x0f == 0x0f` 返回的 `size` 是**字节数**（规范：后面跟一个
整数对象给长度），整数/实数分支却用 `1<<size`（那是内联字段的语义：log2 字节数）：

```
/tmp/p-int.bin  (0x1f + 整数 8 + 8 字节值 42)  -> err=read of 256 bytes at 18
/tmp/p-real.bin (0x2f + 整数 8 + float64 1.5) -> err=read of 256 bytes at 11
```

### 1.4 XML 的 `<data>` 没解码：同一份文档两种编码给出两组字节

`readXMLElement`（`:129`）与 `readXMLElementAfter`（`:225`）都是
`[]byte(strings.TrimSpace(text))`——返回 base64 **文本**；二进制一路返回真字节。
实测 `binary blob=00016869` / `xml blob=4141466f61513d3d`（`AAFoaQ==` 的 ASCII）。

### 1.5 自引用 / 深嵌套文档 → 栈溢出，进程直接死

`object`（`:344`）与 `value`（`:360`）互递归、XML 侧 `readXMLElement`/`readXMLDict`/
`readXMLArray` 同样，**都没有深度上限**。一个自引用文档（对象 0 是「含 1 个指向对象 0
的数组」）实测：

```
runtime: goroutine stack exceeds 1000000000-byte limit
fatal error: stack overflow
```

包注释写的是「a malformed document is an error, never a partly-filled map」，而这里是
**进程级崩溃**：`readBundle` 在扫描时对每个 `.app` 的 `Info.plist` 调 `Parse`
（`internal/apps/darwin.go:58`），一个坏文件就能让整个启动器起不来。

## 2. 落地

- `internal/plist/plist.go`
  - `Parse`：`bplist` 头按规范在偏移 0，**不剥空白**就判二进制；只有 XML 一路剥
    （`<?xml` 前的空行是合法的）。注释写清「末尾的 0x0a 是数据不是空白」。
  - 0x6 UTF-16BE 串改 `string(utf16.Decode(units))`。
  - `length` 多返回一个 `extended bool`；整数/实数分支按「扩展则 `size` 字节、否则
    `1<<size` 字节」取宽度，其余分支语义不变。
  - 新增 `decodeXMLData`：剥空白 → `StdEncoding` → 退 `RawStdEncoding`；坏 base64 →
    `ErrInvalid`；空 `<data>` → 零字节。
  - 新增 `maxDepth = 128` 与 `tooDeep()`；`object`/`value`/`readXMLElement`/`readXMLDict`/
    `readXMLArray`/`readXMLValue` 全部带深度，超限 → `ErrInvalid` + `nesting`。
- `internal/apps/darwin.go`：两个 UTF-16 分支改 `string(utf16.Decode(units))`。
- `internal/apps/shortcut.go`：删掉手写的 `utf16Decode`（18 行），改调标准库——
  本仓 UTF-16 从此只有 `unicode/utf16` 一个答案（`registry.go` 早就用它）。

台账 **V=0 / B=6 / S=1**（S = `plist_test.go:106` 那条钉了旧错答案的断言，已注明重定基线）。

## 3. pin（7 条，全是新函数）

`internal/plist/encoding_test.go`
- `TestTheSameDocumentReadsTheSameInBothEncodings`：`plistlib` 写出的同一份文档两个编码 →
  `🎮 Games` 与 `\x00\x01hi` 各自相等（M2、M3 会让它红）。
- `TestABinaryDocumentIsNotTrimmed`：偏移表落在 0x0a 与 0x0e 的两份文档都读出 42（M1）。
- `TestExtendedWidthsReadTheByteCount`：0x1f 的 8 字节整数 = 42、0x2f 的 float64 = 1.5。
- `TestADeepOrSelfReferencingDocumentIsAnError`：自引用 → `ErrInvalid` 且含 `nesting`；
  200 层 XML → `ErrInvalid`；10 层仍能读到底。
- `TestAnUnpairedSurrogateIsOneReplacement`：孤立高位代理 → **一个** `U+FFFD`。
- `TestXMLDataMustBeBase64`：坏 base64 → `ErrInvalid`；换行包裹的 base64 与空 `<data>` 都读对。
`internal/apps/utf16_test.go`
- `TestALocalizedNameOutsideTheBMPReadsAsOneCharacter`：UTF-16LE 与 UTF-16BE 的
  `InfoPlist.strings` 都读出 `🎮 Games`。

`internal/plist/plist_test.go:106-110` 重定基线：`<data>aGk=</data>` 的断言从 base64 文本
`aGk=` 改为解码后的 `hi`，注释 `P11-h 重定基线`。

## 4. 门槛（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿（34 ok + 2 no test files）
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2250（基线 2243，+7）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 15（=）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
grep -rh "^func Test" --include=*_test.go internal cmd | wc -l  # 1821（基线 1814，+7）
GOOS=linux / windows / darwin go build ./...           # 三平台过
```

非视觉轮，不出图（`internal/plist` 没有像素）。

## 5. mutation（3 条实跑，全部变红、逐字节还原、`touch` 复跑）

| # | 注入 | 变红的 pin（原始报错串） |
|---|---|---|
| M1 | `parseBinary(data)` → `parseBinary(bytes.TrimSpace(data))` | `encoding_test.go:82: pad 0 (last byte 0xa): Parse: plist: not a property list: bad binary trailer`；`encoding_test.go:160: Parse: plist: not a property list: bad binary trailer` |
| M2 | 0x6 串改回逐码元 `rune(unit)` | `encoding_test.go:38: binary: display name = "�� Games", want "🎮 Games"` |
| M3 | XML `<data>` 改回 `[]byte(strings.TrimSpace(text))` | `encoding_test.go:48: xml: blob = "AAFoaQ==", want "\x00\x01hi"`；`encoding_test.go:172: Parse = map[...]{"blob":[]uint8{0x6e, ...}}, <nil>, want ErrInvalid`；`encoding_test.go:183: wrapped base64 = "AAFoaQ=="`；`plist_test.go:108: data = []byte{0x61, 0x47, 0x6b, 0x3d}` |

每条注入后 `md5sum -c` 逐字节核对（`internal/plist/plist.go` = `5d04aabc85be94cb4e77e6ccc30597f9`）
并 `touch` 复跑，`go test ./internal/plist ./internal/apps` 回到 `ok`。
深度上限那一条的「改前」行为在侦察阶段实测到（`fatal error: stack overflow`，见 §1.5），
它的 pin 在 M1/M2/M3 的复跑里始终绿。

## 6. 仍待

1. 用户实机复测四条，连续第 63 轮。
2. 需拍板不变（`.pending-bugs.md` P9-g 表 + 剪贴板轮询间隔 600 vs 900 + 复制回来是否置顶 +
   计算器 `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」）。
3. `internal/plist` 的解码契约本轮收口（嗅探 / UTF-16 / 扩展宽度 / `<data>` / 深度），
   **别在里面再找小修**；`Parse` 仍未读的对象类型只有 `0x7`（UTF-8 串，规范里没有）与
   `0xb`（有序集合）——两者都不是 Apple 会写的，真遇到再按需加。
4. 下一处候选：`internal/ansi`（533 行 / 12 条 pin，终端输出解析）与
   `internal/terminalui`（6010 行 / 79 条）的转义序列处理是否只有一套；`internal/theme`
   的 22 条 pin 是像素面，等视觉轮。要动 `internal/launcher` 的 `filters.go`/`keycap.go`
   三个 `runtime.GOOS` 分支，先拍板「非 macOS 词序」。
