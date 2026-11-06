# P13-hy：异步结果与条件刷新后的键盘恢复

开盘 405191bc；pull 已最新、工作树干净。独立 Codex 执行。基线实测 4218 PASS / 0 FAIL / 2913 函数。八项真实事件初次复现全部红。

## Stash 清点

八条 SHA 与 P13-hx 相同，逐条导出并 reverse-check 均退出 1。backlog 明确它们是早期非 stale-kill WIP，继续保留原件，不重新合入旧主题/Tauri；历史抢救已接续归档。

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

## 逐项实施（增量）

1. 现象：保存失败插入错误带后 Save 失焦，Space 只派发 1 次。改动：配置动作行独立稳定 key。验证：深浅真实编辑、Tab、Space、延迟失败后保持 Save 焦点，Space 可派发第 2 次。
2. 现象：点击重试后 Saving 与上次 Save failed 同时出现。改动：开始新保存时清旧错误。验证：深浅新回调仍在飞、调用 2 次、错误带消失。

旧 V/B/S 断言改动 0/0/0；无新增 Skip。后续批次、mutation 与正式门槛按实际补录。
