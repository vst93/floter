# P11-q 报告 · 「同一个文件」只有一个判据

规格 `docs/rounds/P11-q-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」：
本轮落在**三平台一致性**，因为同一个问题（两个路径是不是同一个文件）在三个平台上
有六种答案，其中两处已经在用户机器上出错。

## 1. 本轮找到的真问题（不是风格问题）

1. **同名函数两个答案**。`extensions.sameExecutable`（旧 `candidates.go:133`）先判
   `a == b` → `("", "")` 是「同一个程序」；`shell.sameExecutable`（旧 `system.go:367`）
   先判空 → `("", "")` 不是。同一句问话，两个结论。
2. **Detected 列表的集合不对称（用户可见）**。旧 `detectedtools.go:145-166` 把集成的
   `path` / `Clean(path)` / `EvalSymlinks(path)` 三种拼写都装进集合，查找却只查前两种。
   用户通过符号链接（`~/.local/bin/rg`）连上的工具，在 Detected 列表里以解析后的路径
   （`~/.cargo/bin/rg`）**再出现一次**，可以再连一遍。这是本轮修的 bug，pin 是
   `TestAConnectedToolIsNotDetectedAgainThroughItsSymlink`（变异 3 让它红）。
3. **Linux 上折大小写（用户可见）**。旧 `settings/migrate.go:193` 的 `samePath` 用
   `strings.EqualFold`，在大小写敏感的文件系统上把 `Settings.json` 与 `settings.json`
   判成同一个文件 → 迁移认为「源就是目标」，报 `no-legacy`，用户被留在默认设置上。
   pin 是 `TestTheMigrationDoesNotConfuseTwoCaseSpellings`（两分支都断言：大小写不敏感卷
   必须报 `settings-exist`，敏感卷必须真的迁移）。
4. **硬链接、Windows 8.3 短名、大小写不敏感卷**：字符串比较永远看不见。现在由
   `os.SameFile` 回答（变异 1 让硬链接用例红）。

## 2. 落地

新增 `internal/pathid`（`pathid.go`，约 130 行，含包注释说明为什么问文件系统）：

| 导出 | 语义 |
|---|---|
| `Key` / `KeyFor(platform, path)` | 集合键：空→空；`Abs`（失败则 `Clean`）+`Clean`+`ToSlash`；**只在 Windows 折大小写**。`KeyFor` 让 Linux 上的测试能断言 Windows 的折法（与本仓 `progname.Ladder` / `progtype.For` 同一写法） |
| `Keys` | 同一个文件的所有拼写：`Key(path)` + 能解析时的 `Key(EvalSymlinks(path))` |
| `Same` | 空串永不匹配（含 `("","")`）；`Keys` 相交→是；否则两边 `Stat` 成功时用 `os.SameFile` |
| `SameSpelling` | 折分隔符 + **无条件**折大小写，给「本机不存在的路径表」用（Windows 源表在 Linux 上评估） |

调用点（7 处，全部改成转发）：

- `internal/extensions/candidates.go`：删 `sameExecutable`，`SystemRuntimeCandidates` 改
  `pathid.Same`；`lookToolsIn` 的 `seen` 键改 `pathid.Keys`（同一个程序不再因两种拼写占两行）。
- `internal/shell/system.go`：删 `sameExecutable`，`hasExecutable` 改 `pathid.Same`。
- `internal/shell/shell.go:1702`：register 高亮改 `pathid.Same`。
- `internal/shell/detectedtools.go`：新增 `connectedKeys(paths)` / `connected(set, path)`
  两个纯函数，两侧都走 `pathid.Keys` —— 修掉上面第 2 条。
- `internal/inventory/inventory.go`：`normalized` → `pathid.Key`（语义不变）。
- `internal/inventory/sources.go`：`sameRoot` → `pathid.SameSpelling`（语义不变；它的表是
  Windows 路径，无条件折大小写的理由仍然成立，`TestTwoSpellingsOfOneRootAreOneRoot` **零改动**全绿）。
- `internal/settings/migrate.go`：删 `samePath`，改 `pathid.Same` —— 修掉上面第 3 条。

新增契约扫描 `internal/ci/pathid_contract_test.go` 的 `TestPathIdentityHasOneAnswer`：
禁止在 `internal/pathid` 之外定义 `func sameExecutable(` / `func samePath(`；禁止在
`internal/pathid` 之外调 `filepath.EvalSymlinks`（白名单两条：`inventory.fingerprint` 是
**内容**键——路径+大小+mtime，不是两两比较；`clipboard.canonicalPath` 是文件列表哈希的输入）；
并正向断言六个调用点仍然 import `pathid`（seam 不许变成摆设）。

## 3. V / B / S 台账

- **V = 0**：本轮不碰任何视觉数值、不碰任何绘制代码。
- **B = 1**（零改动且必须全绿的那类）：`internal/inventory/sourcetable_test.go` 的
  `TestTwoSpellingsOfOneRootAreOneRoot` **一行未改**，`sameRoot` 换实现后仍全绿 ——
  证明第 6 条改造是纯搬家。
- **S = 1**：`internal/shell/register_test.go` 的 `TestSameExecutableResolvesSymlinks`
  调用目标从 `sameExecutable` 换成 `pathid.Same`（helper 搬进 `pathid`），**断言表一字未改**。
- 新增测试函数 8 条（`internal/pathid` 5 条 + `internal/ci` 1 条 + `internal/shell` 1 条 +
  `internal/settings` 1 条），删除 0 条。

## 4. 门槛（实测，HEAD `dc46975` 基线 → 本轮）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → exit 0，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2282（基线 2274，+8）
grep -c -- "--- SKIP"                                  → 15（基线 15，未新增）
测试函数 grep -rhn '^func Test' internal/ cmd/ | wc -l  → 1853（基线 1845，+8）
三平台 go build ./...                                   → BUILDS-OK
```

## 5. 变异（3 条实跑，全部变红并逐字节还原）

1. `pathid.Same` 的 `os.SameFile` 换成名字比较（`leftInfo.Name() == rightInfo.Name()`）：

```
--- FAIL: TestSameAsksTheFilesystem (0.00s)
    pathid_test.go:87: Same("…/hard-rg", "…/rg") = false: a hardlink is the same file
    and no string key can say so
```

2. `KeyFor` 去掉 Windows 折大小写（`key = strings.ToLower(key)` → `key = key`）：

```
--- FAIL: TestKeyFoldsCaseOnlyWhereTheFilesystemDoes (0.00s)
```

3. `detectedtools.connected` 改回只查一种拼写（`pathid.Keys(path)` →
`[]string{pathid.Key(path)}`）：

```
--- FAIL: TestAConnectedToolIsNotDetectedAgainThroughItsSymlink (0.00s)
    detectedsymlink_test.go:35: a candidate at "…/rg-link" read as unconnected while
    "…/rg" is connected
```

每条之后 `cp` 还原 + `md5sum -c` 输出「成功」+ `touch` 再跑（避免测试缓存）。
最后按第 4 节全量复跑一次，2282 PASS。

## 6. 仍待

1. 用户实机复测四条（透明 / 三大金刚 / 卡死 / 打包），连续第 72 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. **故意没进本族的两处**，理由写在这里免得下一轮当缺陷重领：
   - `inventory.fingerprint`（`inventory.go:288`）：问的是「这个二进制还是不是原来那个」
     （路径 + 大小 + mtime 的内容键），不是两两比较，解析拼写是它的正确输入。
   - `clipboard.canonicalPath`（`files.go:124`）：文件列表哈希的输入要**一个**字符串，
     换成 `Keys` 会改哈希值、动既有条目身份 pin。若哪天要动，单开一轮。
4. `sameRoot` 保留无条件折大小写（`pathid.SameSpelling`）：它的输入是 Windows 源表，
   在 Linux 上评估，本机文件系统根本无权回答；`TestTwoSpellingsOfOneRootAreOneRoot`
   是这条的 pin。**不要**把它改成 `pathid.Same`。
5. 下一处候选：`internal/calc` / `internal/calculator` 边界（只剩「每次写要不要从磁盘重读」
   需拍板）；`internal/usage` 的 `Record` 写失败回滚（同需拍板）。
