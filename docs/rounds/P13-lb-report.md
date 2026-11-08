# P13-lb · 前置工具刷新后的命令与行底出口

开盘e23ab358，工作树空，pull已最新；最新队列来自P13-la而非任务书过期的P13-jw。安装恢复P13-jx不重复。完整门槛基线为上轮原始日志7170 PASS / 3658函数。

统一入口先增加每门槛最多两次和逐次日志归档。方向：19px净空保证焦点环与淡边分离，新增材质0、布局增量0，保持轻量空间感。

## Stash交接

|ref|SHA|判断/处置|
|---|---|---|
|stash@{0}|422483b4|reverse=1；早期keycap独立WIP，保留，未接入|
|stash@{1}|8af94a07|reverse=1；旧设置尺寸/hover独立WIP，保留，未接入|
|stash@{2}|7e4aa710|reverse=1；旧卸载尺寸与scratch，保留，未接入|
|stash@{3}|1e3aec94|reverse=1；旧number-field未完成稿，保留，未接入|
|stash@{4}|363f5a42|reverse=1；旧配置字段稿，保留，未接入|
|stash@{5}|9b8ce23c|reverse=1；旧快捷键/Tauri稿，保留，未接入|
|stash@{6}|90d314b2|reverse=1；旧启动器行为稿，保留，未接入|
|stash@{7}|7a9f6e13|reverse=1；旧main/Tauri稿，保留，未接入|

逐条原始reverse日志在本轮logs；八条SHA与交接账一致。reverse失败不能证明独有逻辑仍适用；不据此覆盖当前行为。历史移植未完成，保留为遗留限制。

## 逐项验收（增量）

1. 命令别名：前置区块刷新后仍持焦却离屏 → 命令列表纳入integrationLead → 三来源×深浅插入/增长/撤回/恢复均可见，stored-next仅在181ms后写原forecast命令一次。
2. 命令开关：刷新后开关离屏 → 同一反馈接线 → 三来源×深浅均保持19px净空，Space仅关闭原forecast命令一次。

第一批修前20 FAIL，修后20 PASS / exit0；旧V/B/S断言修改0/0/0，新增Skip0。其余六项修前分别20 FAIL/批，均为活动出口离屏；完整门槛与独立mutation待后续完成，尚不声明全量通过。
