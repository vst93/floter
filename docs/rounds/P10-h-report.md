# P10-h 报告：深链的查询参数表就是判据

规格 `docs/rounds/P10-h-spec.md`。方向：任务书「现在该做什么」交棒条目（P10-g 报告「仍待」第 3 条）。

## 1. 一句话

`floter://connect` / `floter://register` 现在只接受参考参数表里写明的键；表外的键、以及同一个
槽位填两次，都在打开页面前被拒，用既有那句 `settings.deepLinkRejected`。

## 2. 参考的答案

`main:src-tauri/src/deep_link.rs:467-492` 的循环：`connect` 只收 `manifest`（`:468`），`register`
只收 `cmd`（`:472`）、`args`（`:476`）、`confirm`（`:487`，无槽位、可重复），**每个值槽只填一次**
（`command.is_none()` 之类的守卫），其余键落到 `:491` `Reject::UnknownParameter`。该拒绝不在
`is_silent` 名单里（`:225-228`）→ 用户会看到一句话。参考自己的测试就是本轮的核心用例：
`deep_link.rs:1515-1528`，`floter://register?cmd=rg&manifest=/a/tool.json` 必须
`Err(Reject::UnknownParameter(_))`。

## 3. 落地

- **新 `internal/shell/linkparams.go`**：`linkParamSlots`（动作 → 键 → 槽位）+ `linkQueryFits`。
  `register` 的三个名字拼写（`cmd`/`tool`/`program`，本仓既有别名）共用一个 `name` 槽；
  `args`/`confirm` 在表内但本仓无读者（README 已承诺 `args` 可选；`confirm` 是终端 `--yes` 的
  折叠，本仓的 register 链接已走一次性接入，没有第二步可跳）——注释与 pin 都写明「故意接受、
  无人读」，避免下一轮当遗漏重领。
- **`HandleURL` 加一道闸门**（`system.go:190-199`）：先取 `action`，`linkQueryFits` 不过就
  `refuseLink()` 返回，再 `switch`。拒绝在 `a.Open(...)` 之前，所以被拒的链接是一句话，不是一次跳页。
- **没有新文案**：`settings.deepLinkRejected` 复用。
- **本仓自有主机不接这张表**（`settings`/`terminal`/`search`/裸启动器）：它们不是参考动作
  （`ACTIONS` 只有 `open`/`connect`/`register`），`q`/`query` 预填是 `docs/mygo-rewrite-plan.md:226`
  写明的自有契约。参考的 `open` 一个参数都不收——这条分歧被**钉住**而不是套用，套用会删功能。

### 修掉的活路径（mutation 1 的原样证据）

删掉闸门后，测试直接跑通了真实机器上的 `rg`：

```
--- FAIL: TestARegisterLinkRefusesAParameterItDoesNotName (0.00s)
    linkparams_test.go:122: floter://register?cmd=rg&manifest=/a/tool.json switched surface to 1
    linkparams_test.go:125: ... landed on page 3, want the page it started on
    linkparams_test.go:128: ... marked "/usr/sbin/rg"
    linkparams_test.go:131: ... reads "", want "That link could not be opened"
```

即：本轮之前，`floter://register?cmd=rg&manifest=/a/tool.json` 会真的把 `/usr/sbin/rg` 接成集成，
`floter://connect?manifest=…&cmd=rg` 会照常进入审查并安装。

## 4. 新增 pin（5 函数）

`internal/shell/linkparams_test.go`：

1. `TestLinkQueryFits`（纯表，24 格）：每个动作的接受/拒绝逐格；重复键、跨拼写同槽、大小写、
   以及「不是参考动作的主机没有意见」。
2. `TestTheCommandLineNeverSpellsAnUnknownParameter`：`LinkFromArguments` 的七条命令行归一化后
   全部落在表内——`floter register rg --hidden src` 的尾巴是被丢掉而不是折成 `args=`。
3. `TestARegisterLinkRefusesAParameterItDoesNotName`：路由器层，5 条链接零跳页、零接入、一句话。
4. `TestAConnectLinkRefusesAParameterItDoesNotName`：同上 3 条；外加接受侧（只有 `manifest` 的
   connect 仍同步打开集成页）。
5. `TestALinkTakesOneValuePerSlot`：重复槽 5 条被拒；`args`/`confirm` 接受侧落到名字探针的
   「未找到」句子——证明它们是「接受但不用」，不是被拒。

## 5. 台账

- **V = 0**（无视觉数值）。
- **B = 0**（既有 2096 PASS 全绿；文案、接线、`LinkFromArguments` 一行未改）。本轮唯一的行为
  变化是「表外键从静默忽略变成拒绝」，而参考在这一格本来就是拒绝。
- **S = 0**（无结构变化）。

## 6. 门槛（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿，36 包（34 ok + 2 no test files）
--- PASS  2096   （基线 2091，+5）
--- SKIP  15     （=）
--- FAIL  0
测试函数 1706    （基线 1701，+5）
GOOS=linux / windows / darwin go build ./...           # 三平台过
```

## 7. mutation（4 条实跑变红，逐字节还原）

| # | 注入 | 变红 | 原样报错（节选） |
|---|---|---|---|
| 1 | `linkQueryFits` 首行 `return true`（等于删掉闸门） | `TestLinkQueryFits`、`TestARegisterLinkRefusesAParameterItDoesNotName`（及另两条路由器层 pin） | `linkQueryFits("register", "cmd=rg&manifest=/a/tool.json") = true, want false (the reference's own refusal case (:1515))` |
| 2 | 删掉 `connect` 行的 `manifest` | `TestLinkQueryFits`、`TestAConnectLinkRefusesAParameterItDoesNotName`、既有 `TestDeepLinkConnectAndRegister` | `linkQueryFits("connect", "manifest=/opt/tool.json") = false, want true` / `an accepted connect landed on surface 0 page 0` / `shell_test.go:3295: the review never ran` |
| 3 | 删掉槽位判据（`len(values) > 1 \|\| filled[slot]`） | `TestLinkQueryFits` 的 5 个重复格、`TestALinkTakesOneValuePerSlot` 的重复侧；**未知键那几格仍绿** | `linkQueryFits("register", "cmd=rg&cmd=fd") = true, want false (the name slot filled twice)` |
| 4 | 从 `register` 行删掉 `args`/`confirm` | `TestLinkQueryFits` 的 3 个接受格、`TestALinkTakesOneValuePerSlot` 的接受侧 | `linkQueryFits("register", "cmd=rg&args=--hidden") = false, want true (args is in the table (README:97-104))` |

每条都：注入 → 跑对应 pin 看到红 → 还原 → `md5sum -c /tmp/p10h-md5.txt`（两文件「成功」）→
`touch` 后复跑全绿。第 3 条同时证明「未知键」与「重复键」是两个独立判据，不是一条断言的两种说法。

## 8. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 38 轮，本仓无可改代码。
2. 需拍板四条不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. **下一轮第一项**（基础功能）：参考的 `validate_arguments`（`deep_link.rs:392-424`）对 `args`
   逐 token 判字符集与上限（16 token / 单 token 64 / 总 256）。本仓**接受但不读** `args`，
   所以这条今天不可观测；要么接同一个判据（在表内就有判据的位置拒绝，与「无读者」不冲突），
   要么在 `.pending-bugs.md` 写清「本版不读 args，故不判」并钉住。先读 `linkargs.go` 与 README
   的承诺再决定——不要静默改 README。
4. `open` 的参数分歧（参考零参数 / 本仓 `q`、`query`）已钉住（`TestLinkQueryFits` 的三格 +
   既有 `shell_test.go:651-658`）。若将来要收，需用户拍板。
