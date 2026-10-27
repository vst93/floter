# P10-v 规格 — 剪贴板监听「一次轮询该记什么」按参考的判据

轮号：`P10-v`。分支 `mygo-rewrite`，动手前 HEAD `6140608`（P10-u），工作树干净，
基线 **2161 PASS / 1757 测试函数 / 36 包**（本轮实测，`go test -count=1 -v ./...`）。

方向来源：优先级 4（三平台一致性 → 基础功能 → 像素细节）。P10-u 报告「仍待」第 7 条点名的
下一处表面：`internal/clipboard` 的跨平台写入路径。量完发现**写入面三平台同一条路**
（`shell.go:3036-3058` 全走 `mygo.Clipboard`，框架内部按平台实现），
**真正的分歧在读取面**——`captureClipboard`（`internal/shell/system.go:469-499`）
与参考 `main:src-tauri/src/clipboard_history/monitor.rs:107-186` 的 `poll_once` 逐条对不上。

## 意图

一次轮询该记什么、按什么顺序、用什么身份去重，本仓与参考是三套不同答案：
本仓「格式列表变了才看图片」会**丢掉第二张之后的每一张截图**，且**文件复制被记成文本**，
参考则是「文件优先 → 图片（带文本作 caption）→ 文本」，去重一律用**内容哈希**。
本轮把这三条判据搬过来，一族的同源 bug 一次修完。

## 证据（动手前实测，行号以 HEAD `6140608` 为准）

| # | 位置 | 现状 | 参考 |
|---|---|---|---|
| 1 | `internal/shell/system.go:469-499` `captureClipboard` | 文本先判，图片靠 `formatKey(formats)` 与上一轮**格式列表**比较决定要不要读；同一格式列表下的第二张图（`image/png` 两次）直接 `return false` | `monitor.rs:107-186`：每轮都读，用**内容哈希**（`store::content_hash` / `image_hash`）和 `last_hash` 比较 |
| 2 | 同上 | `ReadFiles()` 排在**最后**，文本先命中就返回 | `monitor.rs:118-146` 文件**第一**：「a Finder/Explorer selection usually ALSO carries a plain text rendition of its paths, and recording that instead would lose the file semantics」 |
| 3 | 同上 | 图片命中后只 `AddImage(png,w,h)`，同一次复制里带的文本**被丢掉** | `monitor.rs:148-176` 文本与图片**一起读**：`caption_for` 取 trim 后 ≤512KB 的文本，`image_hash(w,h,rgba,caption)`，存成**一条**带 caption 的图片条目，「never the text alone shadowing the image」 |
| 4 | `internal/clipboard/clipboard.go:433-468` `AddImage` | 哈希只覆盖 PNG 字节，`Entry.Text` 恒空 | `monitor.rs:196-206` `image_hash` 含 width/height/rgba/caption：同像素不同 caption 是两条历史 |
| 5 | `internal/clipboard/clipboard.go:470-501` `AddFiles` | 只 trim，不去重、不查存在、无上限；哈希覆盖**原样路径** | `monitor.rs:56-101` `prepare_files_capture`：`MAX_FILES=4096`、`MAX_FILE_PATH_BYTES=512KB`、丢空/重复/已消失的路径、哈希覆盖**canonicalize 后**的路径、存的仍是原串 |
| 6 | `internal/shell/system.go:469-499` | 无任何尺寸上限 | `monitor.rs:24-30`：`MAX_TEXT_BYTES=512KB`、`MAX_IMAGE_PNG_BYTES=20MB`（超限**整轮不记**，连文本也不记） |
| 7 | `internal/clipboard/clipboard.go:186-199` `Entry.Preview` | 早已按「图片条目的 caption 是它的 `Text`」实现（`[image WxH]` 只是 caption 缺失时的回退） | 数据模型已经对齐，**只是监听器从不提供 caption** → 本轮补上生产者，渲染面零改动 |

结论：`Preview`/`Label`/`searchFields` 三个消费端都已经是「图片可带 caption」的形状，
唯一的缺口在生产者。本轮只改生产者，不动任何渲染与搜索。

## 逐条落地项

### 1. `internal/clipboard/files.go`（新增）

- `FilesCapture{Paths []string; Hash string}`：原串 + canonical 哈希。
- `PrepareFiles(paths []string, exists func(string) bool) (FilesCapture, bool)`：
  `MAX_FILES=4096` / `MAX_FILE_PATH_BYTES=512*1024` 先按**原样**判（参考 `monitor.rs:60-62`）；
  逐条丢空、丢重复原串、丢 `!exists`；哈希 = `HashText(join(canonical, "\n"))`，
  canonical 用 `filepath.EvalSymlinks`，失败回落原串（参考 `monitor.rs:88-93`）。
- 常量 `MaxFiles` / `MaxFilePathBytes` / `MaxTextBytes` / `MaxImagePNGBytes`。

### 2. `internal/clipboard/clipboard.go`

- `ImageHash(png []byte, caption string) string`：`hashBytes(png || caption)`。
  caption 为空时**逐字节等于** `hashBytes(png)`，`AddImage` 的既有哈希不变（B）。
- `CaptionFor(text string) string`：trim；空或 `> MaxTextBytes` → `""`（参考 `caption_for`）。
- `AddImageCaption(png []byte, width, height int, caption string)`：哈希走 `ImageHash`，
  `Entry.Text = CaptionFor(caption)`。`AddImage` 改成 `AddImageCaption(..., "")` 的薄封装，
  签名与行为逐字节不变。
- `AddFilesCapture(FilesCapture)`：用 capture 的 `Paths`/`Hash` 落库；
  `AddFiles` 的行为（trim、`HashText(join(原串))`）抽到 `addFilesEntry(paths, hash)` 共用，零改动。

### 3. `internal/shell/system.go`

- 新增 `clipSample{Paths, Text, PNG, Width, Height, formats}` 与
  `readClipboardSample()`（读文件列表 + 文本 + 格式列表；图片字节留给下一步）。
- `sample.readImage()`：只在格式列表含 `image/png` 时读 PNG（成本闸门保留，
  **去重判据换掉**），`formats` 仍由 `formatKey` 汇总——`formatKey`/`containsFormat`
  继续在产线使用，`TestClipboardFormatHelpers` 零改动。
- `planCapture(sample clipSample) (capturePlan, bool)`：纯函数，图片优先于文本；
  图片带 `CaptionFor(text)`；`len(png) > MaxImagePNGBytes` → 整轮不记；
  文本 `len > MaxTextBytes` 或空白 → 不记。
- `captureClipboard`：文件第一（`PrepareFiles` + `AddFilesCapture`），然后
  `readImage` → `planCapture` → 按 `plan.Hash` 去重（`lastClipboardHash`），
  写入失败时日志带上 `formatKey` 汇总。
- `clipboardSample` / `clipboardFileExists` 两个包级 `var`（测试注入点，沿用
  `platformExtract` 的写法）。
- `App.lastClipboardText` / `lastClipboardFormats` → 合并成 `lastClipboardHash`。

## 行为契约零改动红线

- **不碰**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线。
- 本轮非视觉：不改任何 `tokens`、颜色、几何、文案。`Entry.Preview`/`Label`/`searchFields`
  一行不动。
- 磁盘格式不变：`index.json` 的键集合不变（`Entry.Text` 本来就在 `knownEntryKeys` 里），
  老条目照旧读得进。
- `AddImage` / `AddFiles` 的签名与既有断言逐字节不变（B）。
- `selfCopied`（本应用自己写的文本不进历史）行为保留，位置从「文本分支内」挪到
  「plan 落地前」，判据不变。

## 门槛命令与 PASS 基线

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 必须净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 基线 2161，只许涨
grep -rn "^func Test" --include=*_test.go internal/ cmd/ | wc -l   # 基线 1757，不许减
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

`--- SKIP` 基线 15（不许增）。本轮非视觉，**不出图**（无渲染面改动）。

## mutation 计划（每条都要看到具体测试变红）

| # | 注入的缺陷 | 预期变红的 pin |
|---|---|---|
| 1 | `planCapture` 里把文本分支挪到图片分支之前 | `TestPlanCaptureImageCarriesTheCaption` / `TestCaptureClipboardKeepsEveryImage` |
| 2 | `captureClipboard` 用 `sample.formats` 而不是 `plan.Hash` 做去重 | `TestCaptureClipboardKeepsEveryImage` |
| 3 | `PrepareFiles` 去掉 `exists` 过滤 | `TestPrepareFilesDropsVanishedPaths` |
| 4 | `planCapture` 去掉 `MaxImagePNGBytes` 上限 | `TestPlanCaptureRejectsAnOversizedImage` |
| 5 | `AddImageCaption` 的哈希忽略 caption | `TestAddImageCaptionKeepsPixelsApartFromCaption` |
