# P13-kt · 健康报告与运行结果重排后的可见出口

开盘HEAD6d91dd02，pull已最新，工作区干净。任务书当前进度滞后于仓库，以P13-ks仍待为准；P13-jx安装恢复不重复。统一验证入口复用并增加分批定向模式。指定done.log不存在，收尾创建。

## stash清点

本轮八条reverse-check均1，完整SHA/原始错误见P13-kt/logs/stash-audit.txt。与交接SHA一致，均为早期独立WIP，非新增stale-kill。沿用已有语义审计，保留旧工作，不恢复旧主题或Tauri链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/滑块 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段 | 保留 |
| stash@{4} | 363f5a42 | 旧字段/确认栏 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏 | 保留 |
| stash@{7} | 7a9f6e13 | 旧Tauri发布链 | 保留 |

## 增量记录

已读取qiaomu-design、偏好账本及craft-loop/preflight/chinese-typography；使用既定方向打磨路径。视觉与行为候选先复现，批次结论随后追加。

### 第一批：健康详情变化后的输入与选择

1. 编辑槽：健康失败详情插入后，活动参数5被推至Y763.99994、可见W/H=0 → 按报告内容快照显现原编辑槽 → text/number/path深浅均验，增/删/再插入后可继续输入24，手动滚轮不被抢回。
2. 选择控件：同一报告变化挤走开关与选项 → 原活动包络在内容变化时显现 → Space仍只改p4，Right仍从北方到南方，深浅均验。

定向55 PASS/exit0。M1撤回产品差异后实红：`feedback reflow hides active outlet "参数 5": target={X:204.8 Y:763.99994 W:0 H:0}`；MD5还原OK、touch后17 PASS/exit0。旧V/B/S修改0/0/0。首次夹具编译错误（回调参数与开关label）已纠正，原始日志保留，不计产品失败或变异证据。初始toggle案例未正确聚焦，后续单独补实红。
