# P11-s 规格：一个 `.desktop` 文件只有一个读法

## 意图

全仓有两个 `.desktop` 读入器，问同一句「这个文件的字段是什么」给出三套不同答案
（含参考实现一共三套）。本轮把它们收成一个包 `internal/desktopentry`，两个调用点转发，
并且**按参考 `main:src-tauri/src/commands/apps/linux.rs` 的读法**定案。

为什么现在做：P11-r 已把「行框定」（`\n` / `\r` / BOM）收口到 `internal/textlines`，
但字段层仍是两个答案，P11-r 报告的「仍待」第 3 条就点名了它。

## 证据（动手前实测行号）

- `internal/apps/apps.go:560` `ParseDesktopFile`：locale 阶梯（`desktopLocales`，:506）、
  `unescapeDesktop`（:522）、`Type`/`DBusActivatable`/`TryExec`/`Keywords`/`StartupWMClass`、
  接受规则 `name == "" || len(argv) == 0 || hidden || noDisplay`（:660-661）。
- `internal/inventory/sources.go:146` `parseDesktopEntry`：**没有** locale 阶梯、
  **没有** `unescape`、`Name`/`Comment` 首值胜出、接受规则 `Exec == ""`（:186）。
- 两套 Exec 切法：
  - `internal/apps/apps.go:785` `SplitDesktopExec`：单引号也当引号、未知字段码**丢弃**、
    引号内空 token 保留。
  - `internal/inventory/sources.go:221` `desktopExecArgv`：只有双引号、未知字段码
    **整行作废**（返回 nil）。
  - 参考 `main:src-tauri/src/commands/apps/linux.rs` 的 `parse_exec_argv` +
    `strip_field_codes`：只有双引号、`\` 转义、`%%`→`%`、已知字段码（`f F u U d D n N i c k v m`）
    丢弃、**未知码原样保留**、未闭合引号 → nil、空 token 丢弃；并且 `Exec`/`TryExec` 先过
    `unescape`（`entry.exec = Some(unescape(value))`）。
- 两个 `desktopFileID`：`internal/apps/apps.go:718` 与 `internal/inventory/sources.go:123`，
  实现等价但各写一份（后者不处理 `Rel` 失败/`"."`）。

## 落地项

1. 新增 `internal/desktopentry`：
   - `Entry`（Name/Localized/Comment/Exec/Icon/TryExec/GenericName/Keywords/StartupWMClass/
     Type/DBusActivatable/Hidden/NoDisplay）。
   - `Parse(path) (Entry, bool)`（读文件失败才 false）、`ParseContent(string) Entry`。
   - locale 阶梯 `zh_CN, zh_SG, zh, zh_TW, zh_HK`：`Name[locale]` 最低 rank 胜出，
     `Comment[locale]` 同阶梯且可覆盖无后缀值；`Name`/`Comment` 无后缀时后者首值胜出、
     前者末值胜出（与参考一致）。
   - `Unescape`：`\s \n \t \r \\`，未知转义原样保留（与参考 `unescape` 一致）。
   - `Argv(exec) []string`：参考 `parse_exec_argv` 逐条对齐。
   - `(Entry) Usable() bool`：`Name != "" && !Hidden && !NoDisplay && Type ∈ {"", "Application"}
     && len(Argv()) > 0`。
   - `FileID(root, path) string`：apps 的版本（处理 `Rel` 失败与 `"."`）。
2. `internal/apps`：删 `unescapeDesktop`/`desktopLocales`/`desktopLocaleRank`/解析循环，
   `ParseDesktopFile` 转发；`SplitDesktopExec`、`desktopFileID` 变成转发包装（保留导出名）。
   `desktopAliases` 改收 `desktopentry.Entry`。
3. `internal/inventory`：删 `desktopEntry` 结构、`parseDesktopEntry` 的解析循环、
   `desktopExecArgv`、`stripDesktopFieldCodes`、`desktopExecCommand`、`desktopFileID`；
   `desktopEntryTarget` 改收 `desktopentry.Entry` 并用 `entry.Argv()`。
4. 新增契约扫描 `internal/ci/desktopentry_contract_test.go`：
   `TestDesktopEntriesHaveOneReader` —— `internal/` 里除 `internal/desktopentry` 外不得出现
   `"[Desktop Entry]"` 字面量或 `os.ReadFile(...desktop...)` 的第二个解析器。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线不动。本轮只改「读一个
`.desktop` 文件的字段」这一件事，且刻意**不**采纳参考的两处产品行为：
`Name` 缺失时回落到文件名主干（会让无名条目出现在应用列表）、`Hidden/NoDisplay`
大小写敏感（本仓两个读者一致用 `EqualFold`）。两条都写进报告「仍待」。

## 已知行为变化（V/B/S 台账预判）

- **S**（结构/断言变）：`internal/inventory/desktopentry_test.go:59` 「未知字段码 → nil」
  改为「未知码原样保留」——参考如此；apps 同族行为随之变化（`%z` 不再被吞）。
- **S**：inventory 的 `Comment` 现在走 locale 阶梯与反转义（显示文本变化，pin 若红按 S 改）。
- **B**：其余 pin 零改动必须全绿。

## 门槛命令与 PASS 基线

- 基线（HEAD `79f30bf` 实测）：PASS **2300** / 函数 **1859** / SKIP 15 / FAIL 0 / 38 行。
- `gofmt -l cmd internal` 空；`go vet ./...`、`GOOS=windows go vet ./...`、
  `GOOS=darwin go vet ./...` 净；`go test -count=1 ./...` 全绿；PASS 只许涨；
  三平台 `go build ./...` 过。

## mutation 计划

1. 把 `Argv` 里未知字段码改回「丢弃」→ `internal/apps` 的
   `TestSplitDesktopExec`/`TestDesktopExecFieldCodes` 与 inventory 的 Exec pin 变红。
2. 把 `Unescape` 的 `\s` 分支删掉 → `internal/apps` 的 unescape pin 变红。
3. 把 `FileID` 的 `"."` 保护删掉 → `internal/apps/scanroots_test.go` 的 id pin 变红。
