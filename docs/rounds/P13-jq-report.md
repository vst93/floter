# P13-jq · 诊断过程的当前事实与完整失败说明

开盘 2a8b1e1c，工作区干净、ff-only 已同步。沿最新 P13-jp 的实际遗留继续，P13-hw 线索已由后续轮次覆盖。规格先落盘，八个候选尚未算成果。

## stash 清点

当前没有新 stale-kill；以下 SHA 与既有语义审计一致。reverse=1 不能说明独有工作，不把旧主题或 Tauri 链重复接回。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri；reverse=1 | 保留 |

## 增量记录

基线运行中。已读 qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。仅由当前 Codex 执行，沿既有方向打磨。

1. 现象：磁盘移除 lifecycle probes 后，ErrNoProbes 仍执行旧 provider。改动：这一返回值与已失效目标共同刷新后结束。验证：真实提供者标记文件出现，原串 `removed lifecycle declaration still launches the stale provider` 深浅均红；恢复后要求未启动、无旧通知、当前声明已刷新。
2. 现象：没有 lifecycle probes 时，provider 在飞期间磁盘换声明，旧失败结果和通知仍发布，后续检查也读旧来源。改动：完成出口先刷新库存，再按现有来源/epoch 校验发布。验证：真实提供者门控，原串 `provider completed against stale disk owner and published old facts`、`fresh diagnosis cannot recover after disk owner changes` 深浅均红；恢复后还验证新来源能正常检查。

首次红图夹具在输出前重复 Open 会清通知；已取消重复 Open，正式 mutation 使用修正后的同一夹具重出前后图，不将旧夹具缺失的 toast 当证据。

第一批相关回归 **53 PASS / 0 FAIL**，shell vet 净、gofmt 完成。原 V/B/S 断言修改 0/0/0；两项六个节点实红后绿，正式 MD5 mutation 待轮末统一执行。完整基线仍运行中，未宣称全量通过。
