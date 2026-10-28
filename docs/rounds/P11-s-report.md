# P11-s 报告：一个 `.desktop` 文件只有一个读法

规格 `docs/rounds/P11-s-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**三平台一致性**：`.desktop` 是 Linux 专有格式，但它被两个包各读一遍，同一份文件在
「应用列表」和「工具列表」里可以有两个意思——而这两个列表在同一个界面上并排显示。

## 1. 做了什么

新增 `internal/desktopentry`（`desktopentry.go`，280 行）：

- `Entry` + `Parse`/`ParseContent`：字段读取（`Name`/`Comment` 的 `zh_CN, zh_SG, zh, zh_TW, zh_HK`
  阶梯、`\s \n \t \r \\` 反转义、`Type`/`Icon`/`TryExec`/`GenericName`/`Keywords`/
  `StartupWMClass`/`DBusActivatable`/`Hidden`/`NoDisplay`）。
- `Unescape`、`Argv`、`(Entry) Usable()`、`FileID`。
- 行框定仍走 P11-r 的 `internal/textlines`。

两个调用点转发：

- `internal/apps/apps.go`：`ParseDesktopFile` 从 128 行解析循环变成 26 行转发；
  删 `desktopLocales`/`desktopLocaleRank`/`unescapeDesktop`（共 52 行）；
  `SplitDesktopExec`（60 行实现）与 `desktopFileID` 变成一行转发（导出名保留）。
  `desktopAliases` 改收 `desktopentry.Entry`。
- `internal/inventory/sources.go`：删 `desktopEntry` 结构、`parseDesktopEntry` 的解析循环、
  `desktopExecArgv`、`stripDesktopFieldCodes`、`desktopExecCommand`、`desktopFileID`
  （共 177 行），`desktopEntryTarget` 改收 `desktopentry.Entry` 并用 `entry.ArgvFor()`。

净减 **416 行 / 增 57 行**（`git diff --stat`，不含新包与新测试）。

## 2. 按参考定案的四处（本轮真正的「一个答案」）

参考：`main:src-tauri/src/commands/apps/linux.rs` 的 `DesktopEntry::parse` +
`parse_exec_argv` + `strip_field_codes` + `unescape`。

| 问题 | `internal/apps` 旧答案 | `internal/inventory` 旧答案 | 参考 | 本轮 |
|---|---|---|---|---|
| 未知字段码 `%z` | 丢弃 → `["rg"]` | 整行作废 → `nil` | 原样保留 | `["rg","%z"]` |
| 单引号 `'` | 当引号（分组） | 普通字符 | 普通字符 | 普通字符 |
| `Exec=`/`TryExec=` 的 `\s` 等 | 不反转义 | 不反转义 | 先 `unescape` | 先 `unescape` |
| 已定义字段码 | 任意 `%x` 都丢 | 只有 `f F u U i c k` | `f F u U d D n N i c k v m` | 同参考 |

另外两处顺带统一（都是「同一份文件两个读法」的直接后果）：

- `Comment` 现在走 locale 阶梯并反转义（inventory 之前是第一个裸值）；
- 重复键：`Name` 末值胜出、无后缀 `Comment` 首值胜出（inventory 之前两者都首值胜出）。

`FileID` 收成一份（apps 的版本，处理 `Rel` 失败与 `"."`；inventory 的旧版本会把
root 自身变成 `"."`）。

## 3. 刻意**不**采纳参考的两条（写进「仍待」）

1. 参考在 `Name` 缺失时回落到文件名主干（`path.file_stem()`），本仓两个读者都拒绝无名条目。
   采纳会让无名 `.desktop` 出现在应用列表里——可见的产品变化，且要重量 pin，本轮不动。
2. 参考的 `Hidden`/`NoDisplay` 是大小写敏感的 `== "true"`；本仓两个读者本来就一致用
   `EqualFold`（手写条目常见 `Hidden=True`），保持现状。

## 4. V/B/S 台账

- **V = 0**：本轮不碰墨与面。
- **B = 0**：`internal/apps` 的既有断言**零改动**全绿（`TestSplitDesktopExec` 九条全部落在
  新读法下仍然成立）；`internal/inventory` 的过滤表、target 表零改动全绿。
- **S = 2**：
  1. `internal/inventory/desktopentry_test.go`「未知字段码」pin：`nil` → `["rg","%z"]`，
     注释 `P11-s 重定基线：旧值 nil（整行作废）→ 原样保留`；测试改为调用
     `desktopentry.Argv`（读法已搬家），并新增一句「这条 pin 钉的是工具清单走同一个读法」。
  2. 结构：inventory 的 `desktopEntry` 结构与其私有切法被删除，改用 `desktopentry.Entry`
     （旧结构 → 新结构）。

新增测试函数 9 条（`internal/desktopentry/desktopentry_test.go` 8 条 +
`internal/ci/desktopentry_contract_test.go` 1 条）。

## 5. 门槛实测（HEAD `79f30bf` 起点）

- `gofmt -l cmd internal`：**空**
- `go vet ./...`：净 · `GOOS=windows go vet ./...`：净 · `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**0 FAIL**
- PASS 计数：**2308**（基线 2300）
- 测试函数：**1868**（基线 1859）
- SKIP：**15**（基线 15，未新增）
- 三平台 `go build ./...`：过
- 行数：38 行（36 ok + 2 no test files，不变）

非视觉轮，不出图。

## 6. mutation（3 条实跑，全部先红后逐字节还原）

注入文件 `internal/desktopentry/desktopentry.go`，`md5sum -c` 三次均「成功」。

1. `stripFieldCodes` 的 `default` 分支从「原样保留」改成「丢弃」：
   - `desktopentry_test.go:144: an unknown field code: Argv("rg %z") = []string{"rg"}, want []string{"rg", "%z"}`
   - `internal/inventory/desktopentry_test.go:71: an unknown field code: desktopentry.Argv("rg %z") = ["rg"], want ["rg" "%z"]`
2. 删掉 `Unescape` 的 `case 's'`：
   - `desktopentry_test.go:70: GenericName = "File\\sManager", want the \s escape undone`
   - `desktopentry_test.go:116: Unescape("a\\sb") = "a\\sb", want "a b"`
   - `desktopentry_test.go:157: argv = []string{"/opt/SomesApp/editor", "--new-window"}, want ...`
3. `FileID` 删掉 `relative == "."` 保护：
   - `desktopentry_test.go:212: FileID = ".", want "applications"`

## 7. 仍待（P11-s）

1. 用户实机复测四条，连续第 74 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. 本轮新增两条拍板候选：**`Name` 缺失是否回落到文件名主干**（参考如此，会让无名条目
   出现在应用列表）、**`Hidden`/`NoDisplay` 是否改成大小写敏感**（参考如此，会丢掉手写
   `Hidden=True` 的条目）。
4. 下一处候选：`internal/inputtext`（205 行 / 7 条）的编辑行为三平台是否同一套
   （任务书建议的下一处；先量 pin 再动）。
5. `.desktop` 的 `Exec` 读法已收口，但 `internal/xdg.WalkDesktopEntries` 的**深度**与
   inventory 的搜索根表是另外两件事（P9-v/P10-n 已收口），不要在 `desktopentry` 里找。
