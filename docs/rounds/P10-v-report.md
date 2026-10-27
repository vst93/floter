# P10-v 报告 — 剪贴板监听「一次轮询该记什么」按参考的判据

轮号 `P10-v`。分支 `mygo-rewrite`，动手前 HEAD `6140608`（P10-u），工作树干净。
规格 `docs/rounds/P10-v-spec.md`。

## 1. 意图与为什么现在做

一次轮询该记什么、按什么顺序、用什么身份去重——本仓与参考是三套不同答案。
本仓 `captureClipboard`（`internal/shell/system.go`，HEAD `6140608` 的 469-499 行）
用「剪贴板的**格式列表**变了才去看图片」当去重判据，而两张不同的截图格式列表完全一样
（都是 `image/png`）：**第一张之后的每一张截图都被丢掉**。同一处还把 `ReadFiles()`
排在文本之后，而参考的理由写在 `main:src-tauri/src/clipboard_history/monitor.rs:118-125`：
「a Finder/Explorer selection usually ALSO carries a plain text rendition of its paths,
and recording that instead would lose the file semantics」——文件复制在本仓被记成一段路径文本。
第三条同源：图片命中后同一次复制带的文本被丢掉，而 `Entry.Preview`
（`internal/clipboard/clipboard.go:186-199`）**早就**按「图片条目的 caption 就是它的 `Text`」
实现，`[image WxH]` 只是 caption 缺失时的回退——数据模型与渲染面都已就位，**缺的只是生产者**。

方向来源：优先级 4（三平台一致性 → 基础功能 → 像素细节），P10-u 报告「仍待」第 7 条。
量完先排除了「写入面有分歧」：`systemClipboardWrite`（`shell.go:3036-3058`）三条系统全走
`mygo.Clipboard`，框架内部按平台实现，写入面**同一答案**；分歧全在读取面。

## 2. 落地

### 2.1 `internal/clipboard/files.go`（新增）

- 上限常量 `MaxFiles=4096`、`MaxFilePathBytes=512*1024`、`MaxTextBytes=512*1024`、
  `MaxImagePNGBytes=20*1024*1024`（参考 `monitor.rs:24-30`）。
- `FilesCapture{Paths, Hash}` + `PrepareFiles(paths, exists) (FilesCapture, bool)`：
  上限按**原样列表**判；逐条丢空、丢重复原串、丢 `!exists`；哈希 =
  `HashText(join(EvalSymlinks(path) 或原串, "\n"))`——**存的仍是用户复制的原串**，
  canonical 只喂哈希（参考 `monitor.rs:88-93` 的同一取舍）。

### 2.2 `internal/clipboard/clipboard.go`

- `ImageHash(png, caption)`：caption 为空时**逐字节等于** `hashBytes(png)`，
  所以 `AddImage` 的摘要一个字节没动（B）。
- `CaptionFor(text)`：trim；空或超 512KB → `""`。
- `AddImageCaption(png, w, h, caption)`：哈希含 caption，`Entry.Text = CaptionFor(caption)`；
  `AddImage` 改成 `AddImageCaption(..., "")` 的薄封装，签名与断言零改动。
- `AddFilesCapture(FilesCapture)`；`AddFiles` 的行为（trim、`HashText(join(原串))`）抽到
  共用的 `addFilesEntry(paths, hash)`，逐字节不变。

### 2.3 `internal/shell/system.go`

- `clipSample` + `readClipboardSample()`（文件列表、文本、格式列表）+ `readImage()`
  （只在格式列表含 `image/png` 时才取 PNG：成本闸门保留，**去重判据换掉**）。
- `planCapture(sample)` 纯函数：图片优先，`CaptionFor(text)` 作 caption；
  `len(png) > MaxImagePNGBytes` → **整轮不记（连文本也不记）**；文本超 512KB 或空白 → 不记。
- `captureClipboard`：**文件第一**（`PrepareFiles` + `AddFilesCapture`），
  否则 `readImage` → `planCapture` → 按 `plan.Hash` 去重（`lastClipboardHash`）。
  失败日志带上 `formatKey(sample.formats)`，`formatKey`/`containsFormat` 继续在产线使用
  （`TestClipboardFormatHelpers` 零改动）。
- `clipboardSample` / `clipboardImageReader` / `clipboardFileExists` 三个包级 `var`
  作注入点（沿用 `platformExtract` 的写法）。
- `App.lastClipboardText` + `lastClipboardFormats` 合并成 `lastClipboardHash`。
- `selfCopied`（本应用自己写的文本不进历史）判据不变，位置从文本分支内挪到 plan 落地前。

## 3. V / B / S 台账

**V=0 / B=0 / S=3**，既有测试**零改动**全绿。

| 类别 | 项 | 说明 |
|---|---|---|
| S-1 | `captureClipboard` 的判据 | 旧结构：`lastClipboardText` + `lastClipboardFormats` 两个字段、格式列表比较。新结构：单个 `lastClipboardHash`，内容哈希比较。没有测试钉旧结构，故无断言改动。 |
| S-2 | 图片条目的身份 | 旧：`hashBytes(png)`。新：`ImageHash(png, caption)`，caption 为空时退化成旧值（`TestImageHash` 钉住这条退化）。 |
| S-3 | 文件条目的身份 | 旧：`HashText(join(原串))`（`AddFiles` 保留）。新入口 `AddFilesCapture` 用 canonical 哈希，`PrepareFiles` 是唯一产生者。 |

`Entry.Preview` / `Label` / `searchFields` / 渲染面 / i18n / 键盘 / 会话 / 设置写回：**一行未动**。
`index.json` 键集合不变（`text` 本来就在 `knownEntryKeys` 里），老条目照旧读得进。

## 4. 门槛实测（全部本轮实跑）

| 门槛 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，36 行（34 `ok` + 2 `no test files`） |
| `--- PASS` | **2186**（基线 2161，+25） |
| `--- SKIP` | 15（=） |
| `--- FAIL` | 0 |
| 测试函数 | **1767**（基线 1757，+10） |
| `GOOS=linux/windows/darwin go build ./...` | 三条全过 |

新增测试函数 10 个：`internal/clipboard/files_test.go` 5 个（`TestPrepareFiles` 6 子测试、
`TestCaptionFor` 5 子、`TestImageHash`、`TestAddImageCaption`、`TestAddFilesCapture`）、
`internal/shell/capture_test.go` 5 个
（`TestCaptureClipboardKeepsEveryImage`、`...KeepsTheCaptionBesideTheImage`、
`...RecordsFilesBeforeTheirTextRendition`、`...SkipsItsOwnWrite`、`TestPlanCapture` 4 子）。

本轮**非视觉**，无渲染面改动，**不出图**（视觉 pin 规则不适用）。

## 5. mutation 实跑（6 条，全部看到具体测试变红并逐字节还原）

`md5sum -c` 逐字节核对 + `touch` 复跑（避开测试缓存）：

```
4feaa420b4181ee785908ce8d86ea627  internal/shell/system.go
00f7c70c089472469e54308ccc24f7c6  internal/clipboard/files.go
5526338533fe90af42934a37b05abf10  internal/clipboard/clipboard.go
```

1. **`planCapture` 里图片分支被短路**（`if false && len(sample.PNG) > 0 ...`）：
   ```
   --- FAIL: TestCaptureClipboardKeepsEveryImage (0.00s)
       capture_test.go:90: the first image was not recorded
   --- FAIL: TestCaptureClipboardKeepsTheCaptionBesideTheImage (0.00s)
       capture_test.go:136: kind = "text", want the image, not the text alone
   --- FAIL: TestPlanCapture/the_image_wins_and_the_text_is_its_caption
       capture_test.go:220: plan = "text" / "", want an image captioned
   ```
2. **去重退回格式列表**（`formatKey(sample.formats) == a.lastClipboardHash`）——**原始 bug 复现**：
   ```
   --- FAIL: TestCaptureClipboardKeepsEveryImage (0.00s)
       capture_test.go:95: the second image was dropped: the poll compared format lists, not content
   ```
3. **`PrepareFiles` 去掉 `exists` 过滤**：
   ```
   --- FAIL: TestPrepareFiles/keeps_the_originals_and_drops_the_rest
       files_test.go:23: paths = ["/tmp/a.txt" "/tmp/gone.txt" "/tmp/b.txt"], want ["/tmp/a.txt" "/tmp/b.txt"]
   --- FAIL: TestPrepareFiles/nothing_left_is_no_capture
       files_test.go:37: a list of vanished paths was accepted
   ```
4. **去掉 `MaxImagePNGBytes` 上限**：
   ```
   --- FAIL: TestPlanCapture/an_oversized_image_records_nothing,_not_even_its_text (1.21s)
       capture_test.go:253: an oversized image produced {Kind:image Text: Caption:caption PNG:[0 0 0 0 ...
   ```
5. **`ImageHash` 忽略 caption**：
   ```
   --- FAIL: TestImageHash (0.00s)
       files_test.go:128: two captions share one identity
   --- FAIL: TestAddImageCaption (0.00s)
       files_test.go:162: a second caption was dropped: false, <nil>
   ```
6. **文件列表不再是第一优先**（文本在场就清掉 `Paths`）：
   ```
   --- FAIL: TestCaptureClipboardRecordsFilesBeforeTheirTextRendition (0.00s)
       capture_test.go:172: kind = "image", want the file list, not its text rendition
   ```

每条都还原后 `md5sum -c` 三个文件全部「成功」，`touch` 后复跑 `--- PASS` 仍是 2186。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 51 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. icns 转换落盘缓存（P10-t）、SVG/XPM 图标（P10-s）仍是拍板项。
6. **本轮新发现但未做的两处（同族但属另一条契约，留给下一轮量）**：
   - 轮询间隔：参考 `POLL_INTERVAL_MS = 900`（`monitor.rs:22`），本仓 `clipboardPoll = 600ms`
     （`system.go:417`）。这是**行为节奏**而不是记录内容，改它要连带看「复制后多久能搜到」的
     既有体感，没在本轮擅自动。
   - 图片的 RGBA→PNG：参考在监听侧编码并检查 `MAX_IMAGE_PNG_BYTES`（编码**后**），本仓直接
     读平台给的 PNG（`mygo.Clipboard.Read(transfer.PNG)`），上限判的是读回来的字节数——
     判据同向、测量点不同，已如实记录。
7. 下一轮建议：`internal/shell` 的**剪贴板搜索字段**与参考 `clipboard_history/search.rs` 逐条核对
   （`searchFields`/`matches` 的 AND 语义与「一个词只能由一个字段回答」这条本仓自定的规则）。
