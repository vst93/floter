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
