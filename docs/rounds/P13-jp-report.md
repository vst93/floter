# P13-jp · 检查结果的恢复、归属与可读边界

起点97e16695；规格已落盘，尚未把线索算作修复。当前基线在跑，未预填数字。每两项验收后提交推送；末尾补完整门槛。

## stash清点

当前无新stale-kill；八条SHA与已归档语义审计相同。reverse=1不证明独有成果，不重复合入已过时的主题、字段或Tauri发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri，reverse=1 | 保留 |

## 增量记录

开盘已核对最新报告与.pending-bugs.md顶部，沿P13-jo实际待办推进。统一中文locale。测试函数与PASS数字待基线结束后填写。

1. 现象：重新探测已成功恢复3条命令，旧“没有可用帮助输出”仍在诊断行。改动：成功沿同一个epoch/来源检查清除旧诊断，并在工作线程完成刷新。验证：真实脚本先静默再恢复帮助输出，原串 `successful command retry retained old diagnosis` 深浅均红，修复后通过。
2. 现象：磁盘已有更新的健康报告，过期探测仍下结论并通知“报告了问题”。改动：ErrProbeSuperseded/ErrNoIntegration刷新当前事实后结束。验证：真实健康子进程门控制磁盘更新、不提前刷新Inventory；原串 `discarded lifecycle check published a diagnosis or notice: diagnosis="ok" failed=true` 深浅均红，修复后无旧诊断/通知且新时间可见。

第一批相关 **37 PASS / 0 FAIL**、shell vet净；gofmt已执行。旧V/B/S断言修改0/0/0。首次新增测试有未使用import，编译拒绝后已修正，该次不计有效红。截图首查发现命令卡长于1600px，正式mutation前将统一增高夹具以完整覆盖诊断区。正式mutation尚未执行，不预填结果。
