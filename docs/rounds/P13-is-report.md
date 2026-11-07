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
