# P13-bu · 规格：列表里的状态行——「注」还是「这一页的结论」，由旁边有没有行决定

> 总目标（2026-10-30）：细节打磨 + 轻便液态玻璃 + 空间感。§6 差距表已无 `未开始` 行，
> 选题回通用队列的「状态反馈」：**P13-bo「仍待」第 1 条亲手写下的那条分界规则**
> （「列表非空时它是注，列表为空时它是结论」），并把它落成一条可断言的判据。
> 合约新增 `docs/liquid-glass.md` §6.39。

## 一、意图

同一个形状（一条状态行：`3 of 4 shown`、`the history is off`）在本仓有**两个判据**：

| 位置 | 现状 | 实测 |
|---|---|---|
| `internal/launcher/launcher.go:2335`（启动器列表的 `item.status`） | P13-bo 抬到陈述档 `Emphasis` + `TextStrong` + 480 | 浅 **226** / 深 **214**（P13-bo 报告） |
| `internal/shell/detach.go:560`（插件窗口列表里的 `row.Status`） | 留在参考值 `Caption` + `TextMuted`（P13-be 判「它是列表的从属注」） | 浅 **131** / 深 **130** |

两者都**无条件**：启动器那一臂永远取陈述档，插件窗口那一臂永远取注档。于是插件窗口里
**一条状态行是列表里唯一的内容**时（插件回了「没有匹配」这一类状态行），这一页的**唯一一句话**
是窗口上最轻的东西——比它自己的 bar 标题还轻或同级，与 P13-bo 判掉的那个缺陷**是同一个缺陷**，
只是换了一个窗口。

**为什么现在做**：P13-bo 把这条分界写成了一句中文（「列表非空时它是注，列表为空时它是结论」），
但没写成代码——它在报告里明说「会同时改两条表面，按『一个表面一轮』的纪律另开一轮」。
这一轮就是那一轮：判据落在**列表**这一层，两个窗口各自读它。

## 二、判据（写进 §6.39）

1. **一条状态行的档位由它的邻居决定，不由它自己决定**：
   - 列表里**只有**状态行（没有任何行盘 / 可选行）→ 它是**这一页的结论**：
     `tokens.Emphasis` + `TextStrong` + 580（§4.7 的陈述档，与 `settingsui.emptyState`、
     `detachedStateLine` 的 `detachedStatement` 同一对函数值）。
   - 列表里**还有别的行** → 它是**列表的从属注**：`tokens.Caption` + `TextMuted`（参考值，
     `launcher.css:1783-1789`），因为那一档的主体是行盘，不是这句话。
2. **判据写在列表上，不写在行上**：一个 `detachedStatusAlone(rows)` 谓词读整张列表，
   逐行绘制时只消费它的答案——同一个列表里不许出现两条状态行一条取陈述、一条取注。
3. **只换墨与字阶，不换盒**：行盘、行间距（`detachedRowGap`）、正文内边距
   （`detachedBodyInset`）、可选中性与 copy-on-select 逐像素/逐行为不变。

## 三、逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/shell/detach.go:677` `detachedList` | `if row.Status { detachedListNote(...) }`——无条件注档 | `alone := detachedStatusAlone(rows)`；`alone` 时走 `detachedStateLine(c, row.Title, detachedStatement, tokens, t)`，否则走 `detachedListNote` |
| 2 | `internal/shell/detach.go:560` `detachedListNote` 的注释 | 写着「它说的是这个列表……所以留在参考值上」 | 补一句：**只在列表还有别的行时成立**；列表只剩状态行时它是这一页的结论（§6.39） |
| 3 | `internal/launcher/launcher.go:2335` 的 `item.status` 臂 | 无条件陈述档（P13-bo） | 代码**零改动**：`search.go:388-397` 的 `clipboardItems` 只在插件关掉时发**一条**状态行、没有任何行盘，所以按 §6.39 它取陈述档——补一行注释把这条判据引出来，让两处读同一条规则 |
| 4 | `internal/launcher/output.go:809`（捕获输出页的 `row.Status`） | 行盘 + 灰墨的一整行 | **故意不做**：它不是「状态行」这个形状（它保留 28u 图标板与行盒，是这一页的**行**），§6.39 判的是「无盘的一句话」；改它属另一族 |

## 四、行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**零改动**。
本轮不改 `RowInfo`/`extensions.Row` 的语义、不改 `windowRows` 的带宽计算（状态行仍吃 30u）、
不改 `rowSelectable`、不改 copy-on-select 与翻页触发；只换一句话的墨与字阶。

## 五、门槛（开工实测基线）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿
--- PASS 计数                → 基线 3213
--- SKIP 计数                → 基线 118
测试函数数（git grep 求和）   → 基线 2371
三平台 build                 → 全过
```

## 六、变异计划（≥2 条，每条说明会让哪个 pin 变红）

| # | 注入 | 应红的 pin |
|---|---|---|
| M1 | `alone` 恒为 `false`（回到无条件注档） | `statusalone_test.go` 的「唯一一条状态行必须落在陈述档（weight ≥ 100）」 |
| M2 | `alone` 恒为 `true`（回到无条件陈述档） | `stateladder_test.go` 的 `TestTheListsOwnNoteStaysUnderTheStatement`（有邻居时注被抬到陈述档，gap < 60） |
| M3 | `detachedStatusAlone` 改成「只看第一行」 | 同 M2 的 pin 在「状态行在前、行盘在后」的臂上红（本轮新增该臂） |

## 七、出图

`FLOTER_P13BU_DUMP=1 go test ./internal/shell/ -run TestDumpP13BU` →
`/tmp/floter-P13-bu-window-{alone,note}-{dark,light}-{before,after}.png`。
