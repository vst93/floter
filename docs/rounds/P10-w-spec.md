# P10-w 规格 — 文件列表的身份是它的路径**集合**，不是到达顺序

轮号 `P10-w`。分支 `mygo-rewrite`，动手前 HEAD `d750a66`（P10-v），工作树干净。

## 1. 意图与为什么现在做

一次复制的一批文件，身份应该由**这批路径**决定。本仓现在是「按到达顺序 + `\n` 拼接后取摘要」，
参考是「排序去重 + 长度前缀编码后取摘要」——两个答案，两种错法：

- **同一批文件换个顺序复制 = 两条历史**。本仓 `internal/clipboard/files.go:75`：
  `HashText(strings.Join(canonical, "\n"))`，`["a","b"]` 与 `["b","a"]` 摘要不同；
  参考 `main:src-tauri/src/clipboard_history/store.rs:220-230` 的 `files_hash` 先
  `sort_unstable()` + `dedup()` 再编码，两条摘要相同（其 pin 在 `store.rs:676-688`：
  `files_hash_is_order_insensitive_but_content_sensitive`）。文件选择器的到达顺序就是
  用户的点击顺序，macOS/Windows 的资源管理器不保证稳定，所以这不是理论问题。
- **`\n` 拼接把路径边界抹平**：Linux/macOS 的文件名里可以有换行。复制一个叫 `/a\n/b` 的
  文件，与复制 `/a`、`/b` 两个文件，在本仓摘要相同 → 第二次复制被当成「同一次复制」而丢
  （`addFilesEntry` 的 `entries[0].Hash == hash` 判据，`clipboard.go:531-536`）。参考的编码
  是 `b"files\0"` 前缀 + 每条 `u64` 大端长度 + 字节（`store.rs:224-228`），并有 pin
  `files_hash_distinguishes_newlines_in_names_and_path_boundaries`（`store.rs:434-444`）。
- **一处两个身份产生者**：`Store.AddFiles`（`clipboard.go:507-518`）自己再算一遍
  `HashText(strings.Join(cleaned, "\n"))`，与 `PrepareFiles` 的 canonical 身份是两套答案。
- **换了身份就必须搬老账**：参考在 `load_index`（`main:src-tauri/src/clipboard_history/mod.rs:90-101`）
  里对每条 `files` 条目重算身份——「Old indexes used newline-joined file hashes. Refresh their
  identities without rewriting references」。本仓 `loadLocked`（`clipboard.go:630-655`）读完就
  用，不刷新。**本轮要改身份函数**，不刷新就等于：升级后用户库里每条文件条目的旧摘要都
  对不上新算法，下一次复制同一批文件必然多出一条重复行。

方向来源：优先级 4（三平台一致性 → 基础功能 → 像素细节）；P10-v 报告「仍待」第 7 条点名
`internal/clipboard`，先按建议核对了搜索字段（结论见 §5「本轮排除项」），顺着同族的
「身份判据」量到这一处。参考侧同一函数还有两条 pin（顺序、换行边界），本仓现有 pin
（`files_test.go:89-95`）钉的恰好是**相反的**规则，属于 S 类重定基线。

## 2. 落地项（动手前逐条读过代码确认行号）

### 2.1 `internal/clipboard/files.go` — 一个身份函数

| 位置 | 现状 | 应改成 |
|---|---|---|
| `files.go:75`（`PrepareFiles` 尾部） | `Hash: HashText(strings.Join(canonical, "\n"))` | `Hash: FilesHash(canonical)` |
| `files.go:81`（`canonicalPath` 之后） | 无 | 新增 `FilesHash(paths []string) string`：**排序 + 去重**，编码 `"files\0"` + 每条 `len` 的 8 字节大端 + 字节，SHA-256 十六进制（参考 `store.rs:220-230` 逐字节同构） |
| 同上 | 无 | 新增 `filesIdentity(paths []string) string`：`FilesHash(canonicalPaths(paths))`——**唯一**的身份产生者，`PrepareFiles` / `AddFiles` / `loadLocked` 三处都走它 |

`FilesHash` 导出：`loadLocked` 要刷新身份，测试要直接钉参考的四条不变量。

### 2.2 `internal/clipboard/clipboard.go` — 另两个身份产生者收口 + 老账刷新

| 位置 | 现状 | 应改成 |
|---|---|---|
| `clipboard.go:517`（`AddFiles` 尾部） | `addFilesEntry(cleaned, HashText(strings.Join(cleaned, "\n")))` | `addFilesEntry(cleaned, filesIdentity(cleaned))` |
| `clipboard.go:630-655`（`loadLocked`） | 反序列化后只补 `extra`、按 `CreatedAt` 排序 | 追加：`Kind == KindFiles && len(Paths) > 0` 的条目，`Hash = filesIdentity(Paths)`（**只改内存里的身份，不改 `Paths`，不落盘**——落盘发生在下一次写；参考 `mod.rs:90-101` 同取舍） |

不新增/不删除 `index.json` 键；`AddText` / `AddImage` / `AddImageCaption` 的摘要**一个字节不动**。

### 2.3 测试

| 文件 | 动作 |
|---|---|
| `internal/clipboard/files_test.go:89-95` | **S 类重定基线**：`the order is part of the identity` → `the order is not part of the identity`，断言反过来，注释写 `P10-w 重定基线：顺序敏感 → 集合身份（参考 store.rs:220-230）` |
| `internal/clipboard/files_test.go` | 新增 `TestFilesHash`：参考四条不变量逐条——顺序无关、内容敏感、`["/a/b","/c"]` ≠ `["/a","/b/c"]`、`["/a\n/b"]` ≠ `["/a","/b"]`，外加「与 `HashText` 不同命名空间」 |
| `internal/clipboard/clipboard_test.go` | 新增 `TestAddFilesIsOrderInsensitive`：`AddFiles(["a","b"])` 后 `AddFiles(["b","a"])` 不新增条目 |
| `internal/clipboard/clipboard_test.go` | 新增 `TestLoadRefreshesFilesIdentity`：写一份带旧摘要（`HashText(join(paths,"\n"))`）的 index，`NewStore` 读入后该条目的 `Hash` 已是 `filesIdentity(paths)`，`Paths` 逐字符未变，且随后 `AddFilesCapture(PrepareFiles(同一批))` 判为重复 |

## 3. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不动**。
本轮非视觉，渲染面（`Preview`/`Label`/`FilesSummary`/`searchFields`/`matches`）**一行不动**；
`internal/shell` 的 `captureClipboard` / `planCapture` 一行不动（它只消费 `PrepareFiles` 的 `Hash`）。
`Entry` 的 JSON 键集合、保留策略（30 天 / maxItems / 收藏豁免）、`AddText`·`AddImage` 摘要：不变。

## 4. 门槛与基线（本轮实测，HEAD `d750a66`）

| 门槛 | 基线 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，36 行（34 `ok` + 2 `no test files`） |
| `--- PASS` | **2186**（本轮实测，`git stash` 前后各测一次；与 P10-v 报告一致） |
| `--- SKIP` | 15 |
| `--- FAIL` | 0 |
| 测试函数 | **1767** |
| `GOOS=linux/windows/darwin go build ./...` | 三条全过 |

## 5. 本轮排除项（量过，不改）

- **搜索字段**（P10-v 建议的第 7 条）：`searchFields`/`matches`（`clipboard.go:390-425`）与参考
  `main:src-tauri/src/clipboard_history/search.rs` 的 `entry_search_fields`/`matches_tokens`
  逐条同构——files 只认 paths 不认 text、非 files 认 text、image 另认 `image`/`img`/`图片`、
  词表 AND、每个词由**某一个**字段回答、空字段先丢弃、空查询全中。参考 18 条共享向量
  （`search.rs:107-124`）里没有一条会与本仓分歧，故本轮不改搜索面（pin 进报告 §6 供下轮用）。
- 轮询间隔 `clipboardPoll=600ms` vs 参考 `POLL_INTERVAL_MS=900`：行为节奏，属另一条契约，未动。
- 图片上限的测量点：本仓读平台 PNG、参考判编码后字节，同向，未动。

## 6. mutation 计划（≥2，每条说明哪个 pin 变红）

1. `FilesHash` 去掉 `sort`+去重 → `TestFilesHash`（顺序无关）与 `TestAddFilesIsOrderInsensitive` 红。
2. `FilesHash` 退回 `strings.Join(paths, "\n")` 再摘要 → `TestFilesHash` 的换行/边界两条子测试红。
3. `loadLocked` 去掉刷新 → `TestLoadRefreshesFilesIdentity` 红。
4. `AddFiles` 退回 `HashText(join)` → `TestAddFilesIsOrderInsensitive` 红（同批不同序新增一条）。

每条都要：注入 → 看到具体红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑。
