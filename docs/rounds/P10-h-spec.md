# P10-h 规格：深链的查询参数表就是判据——表外的键一律拒绝

## 意图

`floter://connect` 与 `floter://register` 只接受参考参数表里写明的键；表外的键
（`register?cmd=rg&manifest=/a/tool.json`）在参考里是一句话拒绝，在本仓是静默忽略后照常
连接/接入。本轮把这张表搬进路由器，让「这条链接的形状对不对」只有一处答案。

## 为什么现在做（证据）

- 参考 `main:src-tauri/src/deep_link.rs:467-492`：`parse_url` 的循环逐个查询参数比对
  **动作名 + 键名 + 槽位是否已填**（`:468` `connect`/`manifest`，`:472` `register`/`cmd`，
  `:476` `register`/`args`，`:487` `register`/`confirm` 无槽位可重复），任何别的键落到
  `:491` `return Err(Reject::UnknownParameter(key))`。
- 该拒绝**不是静默的**：`is_silent`（`deep_link.rs:225-228`）只把 `UnknownAction` 与
  `UnsupportedScheme` 算静默，`UnknownParameter` 会给用户一句话。参考的测试就是本轮要的
  那个形状：`deep_link.rs:1515-1528`，`floter://register?cmd=rg&manifest=/a/tool.json`
  必须 `Err(Reject::UnknownParameter(_))`。
- 本仓现状 `internal/shell/system.go:190-238`：`connect` 只 `Query().Get("manifest")`
  （`:200`），`register` 只取 `cmd`/`tool`/`program` + 位置路径（`:217-219`），其余键
  **静默忽略**。所以 `floter://connect?manifest=/a/tool.json&cmd=rg` 会照常进入审查并安装，
  `floter://register?cmd=rg&manifest=/x.json` 会照常接入 `rg`——同一个链接在参考里是拒绝。
- 重复键同样静默：`?cmd=rg&cmd=fd` 在本仓取第一个（`url.Values.Get`），参考在第二次出现时
  就是 `UnknownParameter`（`:472` 的 `command.is_none()`）。
- 本仓文档 `docs/extensions/README.zh-CN.md:97-104` 已写明 `cmd` 可带可选 `args`，所以
  `args` 属于「表内的键」（即使本仓没有读者）——见下面的落地项 3。

## 落地项

### 1. 新文件 `internal/shell/linkparams.go`：参数表 + 一个判据

- 位置：新文件（`internal/shell/`），与 `linkname.go` 并列。
- 现状：不存在。键的合法性没有任何地方表达。
- 应改成：`linkParamSlots`（动作 → 键 → 槽位名）与 `linkQueryFits(action, url.Values) bool`：

  | 动作 | 键 | 槽位 | 依据 |
  |---|---|---|---|
  | `connect` | `manifest` | `manifest` | `deep_link.rs:468` |
  | `register` | `cmd` / `tool` / `program` | `name`（三者同一槽） | `:472`（`tool`/`program` 是本仓既有的两个别名拼写，`system.go:209-210`） |
  | `register` | `args` | `args` | `:476`；本仓无读者，接受但不用（README 已承诺） |
  | `register` | `confirm` | 无（可重复） | `:487`；本仓无读者（`linkargs.go:36-42` 已说明） |

  判据：动作不在表里 → `true`（本仓自有的 `settings`/`terminal`/`search`/裸启动器不是参考
  动作，它们的 `q`/`query` 是本仓自己的契约，见落地项 4）；表里的动作，每个键必须在行内，
  同一槽位第二次出现（同键两次或 `cmd`+`tool` 同时出现）→ `false`。
- 命名与注释要求：注释写清「表就是判据」，并写清 `args`/`confirm` 是「表内但本仓无读者」，
  避免下一轮把它当遗漏。

### 2. `HandleURL` 在分派前加一道闸门

- 位置：`internal/shell/system.go:190`（`switch strings.ToLower(parsed.Host)` 之前）。
- 现状：每个分支自己 `Get` 自己关心的键，别的键没有判据。
- 应改成：把 host 先取成变量，`if !linkQueryFits(action, parsed.Query()) { a.refuseLink(); return }`，
  再 `switch action`。拒绝发生在 `a.Open(...)` 之前——被拒的链接是一句话，不是一次跳页
  （与 `linkname.go` 那条路径判据同一处置）。复用 `refuseLink`（`notices.go:486`）与既有
  `settings.deepLinkRejected`，**不新增任何文案**。

### 3. `register` 的 `args`/`confirm` 保持「接受但不用」

- 位置：`system.go:208-226` 不变；新表把它们列成合法键。
- 现状：静默忽略（因为表外的一切都静默忽略）。
- 应改成：仍不读，但**不再是因为「谁都不检查」**——`linkparams.go` 的注释与 pin 明确写出
  「这两个键合法、本仓无读者」，与 `linkargs.go:36-42` 对 `--yes` 的说明同一口径。

### 4. 本仓自有主机不接这张表（钉住的分歧）

- 位置：`system.go:191-194`、`:227-237`。
- 现状：`floter://settings`、`floter://terminal`、`floter://search?q=…`、`floter://?q=…`。
- 应改成：不动。参考的 `ACTIONS` 只有 `open`/`connect`/`register`（`deep_link.rs:104`），
  参考的 `open` 更是一个参数都不收（`:465-492` 的循环对 `open` 的每个键都落到 `:491`）；
  本仓的四个主机是 `docs/mygo-rewrite-plan.md:226` 写明的自有扩展，其 `q`/`query` 预填是
  用户可见功能。本轮把这条分歧**钉住**（`settings?foo=1` 仍开设置页、`search?q=x` 仍预填），
  不把参考的表套到参考没有的动作上。

## 行为契约零改动红线

- **文案**：零新增、零修改。拒绝仍说 `settings.deepLinkRejected`（`i18n` 不动）。
- **高度链 / 键盘模型 / 会话管理 / 设置写回 / 命令接线**：全部不动（本轮只碰路由判据）。
- **命令接线**：`LinkFromArguments`（`linkargs.go`）一行不改——它生成的 URL 只含 `manifest=` /
  `cmd=`，天生落在表内；本轮补一条 pin 证明这一点。
- **接受路径**：`connect?manifest=…`、`register?cmd=…`、`register?tool=…`、`register?program=…`
  的既有行为逐字节不变（既有 2091 PASS 全绿是证据）。

## 台账预判

- **V = 0**（无视觉数值）。
- **B = 0**（既有行为零改动；本轮只把「静默忽略」变成「拒绝」，而参考在这一格本来就是拒绝）。
- **S = 0**（无结构变化）。新增 4 个测试函数、1 个新文件。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 实测：空
go vet ./...                                           # 实测：净
go test -count=1 ./...                                 # 实测：全绿，36 包
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 实测：2091
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 实测：15
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 实测：0
grep -rc "^func Test" --include=*_test.go internal cmd | awk -F: '{s+=$2} END {print s}'  # 实测：1701
GOOS=linux|windows|darwin go build ./...               # 实测：过
```

本轮目标：PASS 2091 → **2095**（+4），函数 1701 → **1705**（+4），SKIP 15 不变，FAIL 0。

## mutation 计划（≥2，每条说明会让哪个 pin 变红）

1. **删掉闸门**：把 `linkQueryFits` 改成 `return true`。预期
   `TestARegisterLinkRefusesAParameterItDoesNotName`、
   `TestAConnectLinkRefusesAParameterItDoesNotName`、
   `TestALinkTakesOneValuePerSlot`、`TestLinkQueryFits`（纯表）四处变红。
2. **表里删掉 `connect` 的 `manifest` 行**（或把 `manifest` 从 connect 行移走）：预期
   `TestLinkQueryFits` 与 `TestAConnectLinkRefusesAParameterItDoesNotName` 的接受侧变红
   （合法 connect 被拒），`TestDeepLinkConnectAndRegister` 等既有 connect 测试同时变红。
3. **取消槽位判据**（保留键判据，删 `len(values) > 1 || filled[slot]`）：预期
   `TestLinkQueryFits` 与 `TestALinkTakesOneValuePerSlot` 的重复键子用例变红，
   其余（未知键）仍绿——这条证明「重复键」与「未知键」是两个独立判据。
4. **把 `args`/`confirm` 从 `register` 行删掉**：预期 `TestLinkQueryFits` 的
   `cmd=rg&args=--hidden`、`confirm=1` 接受侧变红，`TestALinkTakesOneValuePerSlot` 的
   接受侧变红——证明这两个键是**故意**留在表内的。

每条：注入 → 跑对应测试看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
