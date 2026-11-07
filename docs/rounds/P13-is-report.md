# P13-is：菜单与表单刷新的身份和恢复

07:00 UTC 前后开盘，起点 `753d3979`，pull 最新、工作树净。基线 **5025 PASS / 0 FAIL / 3101 函数**。只用 Codex 独立执行，读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。

## Stash 清点

八条仍为此前逐轮登记的早期独立 WIP；本轮逐条 reverse-check 全部退出 1。stale-kill 已由历史轮次接续归档；下表保留既有处置，不把旧主题/Tauri 链覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

done.log 与 stash-backlog 的已知路径本机未提供；以 git stash list 和仓库前轮处置为准。未全盘搜索。

## 进展

P13-hx 已覆盖任务书中目标仍在但 owner/schema/状态变化的八条确认线索；本轮接实际最新 P13-ir 的菜单刷新遗留，不重复计数。

## 第一批：语言只换标签

1. 现象：参数菜单高亮 Number 后切到中文，Enter 留在 text。改动：参数类型按已有 paramKinds ID 选择，译文只显示。验证：深浅真实 Tab/Down/Enter，最终 number，类型入口保持焦点。
2. 现象：驻留高亮 10 秒后切中文，Enter 留在 0。改动：按已有 choices ID 选择。验证：深浅真实事件最终 10 秒，焦点仍在驻留入口。

相关 **25 PASS / 0 FAIL**，包 vet 净。回注两处旧实现 **6 FAIL**，原串 `translation lost parameter highlight: text`、`translation lost residency highlight: 0, want 10`。MD5 两文件 OK，touch 后 **6 PASS**。V/B/S 旧断言修改 **0/0/0**。原帧已入仓。

整行动作菜单移除和重排探针为绿，不计修复。下一批已复现 command-refresh 和 palette-unknown 覆盖新值；其测试暂放 /tmp，待修后单独提交。

第一批 `8662be1f` 已推送。目视深色：切中文后“数字”仍蓝色高亮，焦点环留在类型入口；原图菜单已缩回，准备选择的位置丢失。

## 第二批：显示回落不能掩盖原值刷新

3. 现象：动作菜单展开后，宿主把命令从 say original 改为 say updated，两者都显示命令行；Enter 却提交旧高亮 plugin:browser。改动：选择器额外观察原始命令源值，源变而显示槽不变时退役旧菜单、保留键盘入口。验证：深浅真实 Down/Home/Enter，宿主新命令保留、写入 0 次，入口仍有焦点。
4. 现象：调色板被宿主更新为未知 ID 后，控件把它伪装为首项，Enter 写回旧 contrast。改动：保留未知 ID，复用现有未知值菜单退役路径。验证：深浅真实事件保留 retired-palette、写入 0 次，焦点可继续选择。

包 vet 净；相关与 mutation 实测计数见同目录 batch2-test/M2 日志。回注旧实现原串 `command-refresh overwrote refreshed value: "plugin:browser" writes=1`、`palette-unknown overwrote refreshed value: "contrast" writes=1`；MD5 还原、touch 后复验。V/B/S 旧断言修改 **0/0/0**。

第二批 `c626a424` 已推送；相关 **92 PASS**，M2 **6 FAIL → MD5 OK → touch → 6 PASS**。

## 第三批：声明整块退场的两个入口

5. 现象：参数名称输入到 Edited target 时宿主移除声明，Enter 无法继续添加参数。改动：记录正在使用的参数板，仅当该板退场且仍拥有焦点时归还 Add。验证：深浅真实 Tab/Type/Enter，移除后 Add 聚焦，Enter 新增一条；不改变原保存时机。
6. 现象：参数类型菜单展开时宿主移除声明，浮层撤走但键盘没有出口。改动：参数板跟踪覆盖菜单触发器的焦点归属。验证：深浅 Down 展开后移除，Add 保有焦点，Enter 可新增。

相关测试与包 vet 通过。M3 回注旧实现，原串 `param-editor-removed lost Add outlet`、`param-menu-removed lost Add outlet`，以及各自 `could not add after refresh`；MD5 还原 touch 后两主题复验。旧 V/B/S **0/0/0**。驻留两项仅见红，尚未计为完成。
