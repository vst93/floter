# P13-hx：确认条件刷新后的出口

开盘 fe52ed35，pull 已最新、工作树干净。中文 locale 基线实测 4171 PASS / 0 FAIL / 2904 函数。独立 Codex 执行；本轮不调用外部执行器。

## Stash 清点

逐条导出 patch 到 /tmp/floter-P13-hx 并 reverse-check；八条 SHA 与 P13-hw 完全一致，均退出 1。已接续的 stale-kill 均不在活动清单，以下按 backlog 的语义审核结论保留，不能用旧主题与 Tauri 链覆盖现产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局/探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

规格八个候选逐批事件复现，只有已复现修复的条目计数。初次事件夹具点击文字 label 未聚焦输入，尚不作为产品缺陷；校正事件命中后再验证。

### 第一批：丢弃问题的 dirty 与 schema 条件

1. 现象：编辑 Token 后提问，外部存储变为同值；带子隐藏但 Escape 被吞，再编辑会复活旧问题。改动：View 路由键盘前同步已有 draft 的外部值，若已无差异就 disarm，保留编辑面。验证：深浅各 return/escape 事件；Configure 焦点恢复，新输入保留，关闭 1 次或旧问题不复活，操作 0 次。
2. 现象：问题展开期间 schema 清空，集成仍在；Escape 被吞，恢复 schema 后旧问题重现。改动：无字段时撤问题与时钟，保留原 draft/configOpen。验证：深浅四场景，恢复后 Token 仍为 edited，旧确认不出现。

M1 回注旧条件八场景实跑红：`discard question survived changed clean condition`、`clean draft lost its configuration opener focus`、`invalid discard question swallowed Escape: close=0`、`discard question revived when schema condition returned`。MD5 恢复 OK，touch 后 10 PASS。V/B/S 旧断言修改 0/0/0；未新增 Skip。

第一批相关回归 210 PASS / 0 FAIL、包 vet 净；6bf0242d 已提交推送。浅色第一次目视：第二次输入 second edit 仍在原字段，旧丢弃带撤走，输入焦点不再被 Cancel 夺走。

### 第二批：owner 与 orphan 分支

3. 现象：host 配置切到 tool owner 后旧丢弃问题藏在外部配置入口后面，Escape 只取消看不见的问题；切回 host 后又问。改动：按 host 配置块的实际可见条件撤旧问题，保留草稿。验证：深浅 × 返回/退出四场景，edited 保留、操作 0 次。
4. 现象：已提问移除的目标刷新成 orphan，维护入口提前返回，但问题仍阻挡 Escape，恢复登记后复活。改动：清除未派发的移除问题、时钟与错误；在飞移除仍交原回调结算。验证：四场景，退出 1 次或恢复后问题为关，不派发移除。

M2 回注旧条件八场景红：`discard question survived changed owner condition`、`remove question revived when orphan condition returned`、两类 `invalid … question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关回归 220 PASS / 0 FAIL，包 vet 净。旧 V/B/S 0/0/0。

第二批 fae5f208 已提交推送。

### 第三批：停用前提不再成立

5. 现象：停用问题展开时，外部状态已经 Disabled，旧确认仍可见可答。改动：未派发的问题随 Enabled=false 退役，清时钟与旧错误。验证：深浅 × 返回/退出四场景，刷新不派发动作，目标再次启用不带旧问题。
6. 现象：目标变 Broken 后开关已不可操作，旧停用答案却仍可用。改动：沿用开关的可用条件撤旧问题；同时覆盖 orphan 的同源提前返回。验证：四场景，Escape 关闭设置 1 次，恢复正常后不复活旧停用答案。

M3 回注旧条件八场景红：`disable question survived changed disabled condition`、`disable question revived when broken condition returned`、`invalid disable question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关 230 PASS / 0 FAIL，包 vet 净；旧 V/B/S 0/0/0。

最初整包运行编译的是尚未校正的输入夹具，最终仅两条新测试的八场景失败（Token 未命中）；原 B 测试均无失败。校正后的相关测试已绿，最终普通/verbose 全量将使用最终代码重跑，不把该次失败计作成功。

第三批 f3970015 已提交推送。

### 第四批：组件卸载条件与未提交选择

7. 现象：组件卸载浮层打开后目标变 orphan；浮层虽隐藏，Escape 被吞，恢复登记后旧浮层连同已取消的 Tool data 勾选复活。改动：未派发时撤浮层、旧错误与未提交的勾选草稿。验证：真实 Click/Tab/Space 打开浮层并取消一项，深浅 × 返回/退出四场景；问题和草稿都退役，卸载调用 0 次。
8. 现象：npm 包刷新成普通本地链接，已经不走组件卸载，但旧浮层还允许选择数据类别。改动：按 componentizedRemoval 当前条件撤不再适用的浮层及草稿。验证：相同四场景；恢复 npm 后旧浮层不复活，Escape 正常退出。

M4 回注旧条件八场景红：`uninstall question survived changed kind condition`、`invalid dialog retained abandoned component choices`、`uninstall question revived when orphan condition returned`、`invalid uninstall question swallowed Escape: close=0`。MD5 恢复 OK，touch 后相关 **247 PASS / 0 FAIL**，包 vet 净。包含额外六场景：停用/移除/组件卸载已派发时刷新不取消 busy，迟到失败仍可读、调用恰好 1 次。旧 V/B/S 0/0/0，无新增 Skip。
