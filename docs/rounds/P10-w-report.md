# P10-w 报告 — 文件列表的身份是路径集合；搜索判据与参考 18 条向量逐条钉住

轮号 `P10-w`。分支 `mygo-rewrite`，动手前 HEAD `d750a66`（P10-v），工作树干净。
规格 `docs/rounds/P10-w-spec.md`。

## 1. 意图与为什么现在做

一次复制的一批文件，身份该由**这批路径**决定。本仓按「到达顺序 + `\n` 拼接」取摘要，参考按
「排序去重 + 长度前缀」取摘要——两个答案，两种错法，都能被用户看见：

1. **同一批文件换个顺序复制 = 两条历史。** 动手前 `internal/clipboard/files.go:75`：
   `HashText(strings.Join(canonical, "\n"))`。`["a","b"]` 与 `["b","a"]` 摘要不同，于是
   `addFilesEntry`（`clipboard.go:531-536`）的 `entries[0].Hash == hash` 判据不成立，第二条
   历史行凭空出现。参考 `main:src-tauri/src/clipboard_history/store.rs:220-230` 的 `files_hash`
   先 `sort_unstable()` + `dedup()`，两条摘要相同，pin 在 `store.rs:676-688`
   （`files_hash_is_order_insensitive_but_content_sensitive`）。文件选择器的到达顺序就是用户
   的点击顺序，macOS/Windows 都不保证稳定——不是理论问题。
2. **`\n` 拼接把路径边界抹平。** Linux/macOS 的文件名里可以有换行：复制一个叫 `/a\n/b` 的
   文件，与复制 `/a`、`/b` 两个文件，在本仓摘要**相同**，第二次复制被当成同一次复制而丢。
   参考用 `b"files\0"` 前缀 + 每条 `u64` 大端长度 + 字节（`store.rs:224-228`），pin 在
   `store.rs:434-444`（`files_hash_distinguishes_newlines_in_names_and_path_boundaries`）。
3. **一处两个身份产生者。** `Store.AddFiles`（`clipboard.go:507-518`）自己再算一遍
   `HashText(strings.Join(cleaned, "\n"))`，与 `PrepareFiles` 的 canonical 身份是两套答案。
4. **换身份必须搬老账。** 参考在 `load_index`（`main:src-tauri/src/clipboard_history/mod.rs:90-101`）
   里对每条 `files` 条目重算身份（原注释：「Old indexes used newline-joined file hashes.
   Refresh their identities without rewriting references」）。本仓 `loadLocked`
   （`clipboard.go:630-655`）读完就用。本轮**必须**改身份函数，不刷新就等于：升级后用户库里
   每条文件条目的旧摘要都对不上新算法，下一次复制同一批文件必然多出一条重复行。

方向来源：优先级 4（三平台一致性 → 基础功能 → 像素细节）。P10-v 报告「仍待」第 7 条点名
`internal/clipboard`；先按建议核对了搜索字段（结论见 §2.4：**已经同构，零改动**，顺手把参考的
18 条共享向量钉进 Go 侧），顺着同族的「条目身份」量到上面四条。

## 2. 落地

### 2.1 `internal/clipboard/files.go`

- 新增 `FilesHash(paths []string) string`：`slices.Sort` + `slices.Compact`，编码
  `"files\x00"` + 每条 `binary.BigEndian.PutUint64(len)` + 字节，SHA-256 十六进制——
  参考 `store.rs:220-230` 的逐字节同构。
- 新增 `filesIdentity(paths []string) string`：`FilesHash(canonicalPaths(paths))`，
  **唯一**的身份产生者（`PrepareFiles` / `Store.AddFiles` / `loadLocked` 三处都走它）。
- `PrepareFiles`（`files.go:75`）：`HashText(strings.Join(canonical, "\n"))` → `FilesHash(canonical)`。
  「存的仍是用户复制的原串、canonical 只喂哈希」这条 P10-v 的取舍**没动**。

### 2.2 `internal/clipboard/clipboard.go`

- `AddFiles`（`clipboard.go:517`）：`HashText(strings.Join(cleaned, "\n"))` → `filesIdentity(cleaned)`。
- `loadLocked`（`clipboard.go:645-655`）：反序列化后，对 `Kind == KindFiles && len(Paths) > 0`
  的条目 `Hash = filesIdentity(Paths)`——**只改内存里的身份，不碰 `Paths`，不落盘**（落盘发生在
  下一次写，参考 `mod.rs:90-101` 同取舍）。

### 2.3 测试

| 文件 | 动作 |
|---|---|
| `files_test.go:89-99` | **S 类重定基线**：`the order is part of the identity` → `the order is not part of the identity`，注释写明「P10-w 重定基线：顺序敏感 → 集合身份（参考 store.rs:220-230）」；同一子测试补一条「两份不同的列表身份不同」 |
| `files_test.go` 新增 `TestFilesHash` | 参考四条不变量逐条：顺序/重复无关、内容敏感、`["/a/b","/c"]` ≠ `["/a","/b/c"]`、`["/a\n/b"]` ≠ `["/a","/b"]`，外加「与 `HashText` 不同命名空间」「空列表身份唯一」 |
| `clipboard_test.go` 新增 `TestAddFilesIsOrderInsensitive` | `AddFiles(["a","b"])` 后 `AddFiles(["b","a"])` 不新增条目；另一份列表仍是新条目 |
| `clipboard_test.go` 新增 `TestLoadRefreshesFilesIdentity` | 写一份带旧摘要（`HashText(join(paths,"\n"))`）的 index → 读入后 `Hash` 已是 `filesIdentity(paths)`、`Paths` 逐字符未变、且 `AddFilesCapture(PrepareFiles(同一批))` 判为重复 |
| `clipboard_test.go` 新增 `TestSearchMatchesTheSharedContractVectors` | 参考 `search.rs:107-124` 的 **18 条共享向量**逐条搬进 Go（本仓 `searchFields`/`matches` 零改动即全过） |

## 3. V / B / S 台账

**V=0 / B=0 / S=1**（V 类无：本轮不碰像素；B 类无：没有产品行为被测试挡住）。

| 类别 | 项 | 说明 |
|---|---|---|
| S-1 | `files_test.go:89` 的顺序 pin | 旧断言：重排的列表**必须**摘要不同（P10-v 写的）。新断言：重排的列表**必须**摘要相同、另一份列表不同。断言方向整体反转，不是放宽——注释按 `P10-w 重定基线：旧值 → 新值` 写明。 |

无 pin 的两处结构变更（旧结构没有任何测试钉住，故无断言改动）：

| 类别 | 项 | 说明 |
|---|---|---|
| S-2 | `AddFiles` 的身份 | 旧：`HashText(join(原串))`。新：`filesIdentity`（canonical 集合）。既有调用点全是测试夹具，无人钉具体摘要值。 |
| S-3 | `loadLocked` | 旧：读完即用。新：读入时刷新 `files` 条目的 `Hash`。旧行为没有测试钉住（`legacyIndex` 夹具只断言 kind/label/未知键，不断言该条目的 hash）。 |

**零改动面**：`searchFields`/`matches`/`Entry.Preview`/`Label`/`FilesSummary`、`internal/shell`
的 `captureClipboard`/`planCapture`、渲染面、i18n、键盘模型、会话管理、设置写回、命令接线、
`index.json` 键集合、保留策略（30 天 / maxItems / 收藏豁免）、`AddText`/`AddImage`/`AddImageCaption`
的摘要——**一行未动**。

### 3.1 搜索字段的核对结论（P10-v 建议的第 7 条，本轮量完不改）

`searchFields`/`matches`（`clipboard.go:390-425`）与参考 `search.rs` 的
`entry_search_fields`/`matches_tokens` **逐条同构**：files 只认 paths 不认 text、非 files 认
text、image 另认 `image`/`img`/`图片`、词表 AND、每个词由**某一个**字段回答（不跨字段拼接）、
空字段先丢弃、空查询全中。参考的 18 条共享向量在 Go 侧零改动即全过，故**不改产品**，只把向量
钉进 `TestSearchMatchesTheSharedContractVectors`——以后改搜索面必须同时面对参考那张表。

## 4. 门槛实测（全部本轮实跑）

| 门槛 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，36 行（34 `ok` + 2 `no test files`） |
| `--- PASS` | **2195**（基线 2186，+9） |
| `--- SKIP` | 15（=） |
| `--- FAIL` | 0 |
| 测试函数 | **1771**（基线 1767，+4） |
| `GOOS=linux/windows/darwin go build ./...` | 三条全过 |

基线复测方法：`git stash push -- internal/clipboard` 前后各跑一次全量 `-v`，2186 → 2195。
本轮**非视觉**，无渲染面改动，**不出图**（视觉 pin 规则不适用）。

## 5. mutation 实跑（5 条，全部看到具体测试变红并逐字节还原）

`md5sum -c` 逐字节核对 + `touch` 复跑（避开测试缓存）：

```
d1bfb5ab7c0b1914c69c9d5c66aaa188  internal/clipboard/files.go
7c6a012f426d8dd76408650275ceea7a  internal/clipboard/clipboard.go
```

1. **`FilesHash` 去掉排序与去重**（`slices.Sort`/`Compact` → `_ = slices.Contains(sorted, "")`）：
   ```
   --- FAIL: TestAddFilesIsOrderInsensitive (0.00s)
       clipboard_test.go:582: a reordered list was added twice: true, <nil>
       clipboard_test.go:585: history holds 2 entries, want 1
   --- FAIL: TestFilesHash (0.00s)
       --- FAIL: TestFilesHash/order_and_repeats_do_not_matter (0.00s)
           files_test.go:213: a reordered list hashed two ways
           files_test.go:216: a repeated path changed the identity
   ```
2. **编码退回 `\n` 拼接**（去掉 8 字节长度前缀、改用 `path + '\n'`）：
   ```
   --- FAIL: TestFilesHash (0.00s)
       --- FAIL: TestFilesHash/a_newline_inside_a_name_is_not_two_paths (0.00s)
           files_test.go:241: a path containing a newline impersonated two paths
   ```
3. **`loadLocked` 去掉身份刷新**：
   ```
   --- FAIL: TestLoadRefreshesFilesIdentity (0.00s)
       clipboard_test.go:621: hash = "1c03e327135d04c7b26a21bc601525ac405e50e01bb72639c99295aeb8a70fe4", want the refreshed "e6d72f77a2bcc99e88a96dcd46a21891e0ba34936b107da31181ae198177d3ae"
       clipboard_test.go:634: capture hash = "e6d72f77a2bcc99e88a96dcd46a21891e0ba34936b107da31181ae198177d3ae", loaded hash = "1c03e327135d04c7b26a21bc601525ac405e50e01bb72639c99295aeb8a70fe4"
       clipboard_test.go:637: the same files were added again after the refresh: true, <nil>
   ```
   （第 621 行的 `1c03a3…` 正是旧算法 `HashText(join(paths,"\n"))` 的值——刷新没做时它原样留在内存里。）
4. **`AddFiles` 退回 `HashText(join)`**：
   ```
   --- FAIL: TestAddFilesIsOrderInsensitive (0.00s)
       clipboard_test.go:582: a reordered list was added twice: true, <nil>
       clipboard_test.go:585: history holds 2 entries, want 1
   ```
5. **image 的名字词表去掉 `图片`**（`searchFields`）：
   ```
   --- FAIL: TestSearchMatchesTheSharedContractVectors (0.00s)
       clipboard_test.go:679: shared vector drifted: bare image answers to its name: matches("图片") = false, want true
       clipboard_test.go:679: shared vector drifted: captioned image: tokens may hit different fields: matches("diagram 图片") = false, want true
   ```

每条都还原后 `md5sum -c` 两个文件全部「成功」，`touch` 后复跑 `--- PASS` 仍是 2195。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 52 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. icns 转换落盘缓存（P10-t）、SVG/XPM 图标（P10-s）仍是拍板项。
6. 剪贴板族**未动**的两处（同族但属另一条契约）：
   - 轮询间隔：参考 `POLL_INTERVAL_MS = 900`（`monitor.rs:22`），本仓 `clipboardPoll = 600ms`
     （`system.go:419`）。行为节奏，牵连「复制后多久搜得到」的体感。
   - 图片上限的测量点：参考编码后判 `MAX_IMAGE_PNG_BYTES`，本仓判读回来的 PNG 字节——
     同向、测量点不同。
7. 下一轮建议：`internal/clipboard` 已从「监听/身份/搜索」三条边量到收口，**建议离开这一族**。
   按优先级 3/4，下一处候选：`internal/shell` 的 `copyClipboardEntry`/`copyClipboardImage`
   （`shell.go:2987-3035`）把条目写回系统剪贴板的三条臂，与参考
   `main:src-tauri/src/commands/clipboard.rs` 的 copy 命令逐条核对——写回面本轮只核了文本一条。
