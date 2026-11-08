# P13-ju · 重连选择的当前归属与修复后的搜索恢复

开盘 HEAD 1a83f975，pull 已同步、工作树干净。Codex 单独执行，读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。无外部模型与代理。指定 done.log 开盘不存在，收尾创建。

## Stash 清点

八条早期 WIP 的 SHA 与 P13-jt 语义审计一致，无新 stale-kill；逐条 reverse-check 退出1，保留原件，不重复接入已归档旧主题/Tauri工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

基线正在运行。生成工具编辑覆盖保留待拍板；本轮转入已有修复和重连的结果恢复，逐项实测追加如下。

## 第一批：搜索恢复

1. 现象：真实点击重新连接后当前行可用，搜索仍为空。改动：Reconnect 完成调用已有 RefreshIntegrations，投影当前命令并清事实缓存。验证：深浅 Click→完成回调→搜索 root，命令准确1条且可见 Recovered command；改前均红 `reconnect-search completed but search stayed unavailable: commands=0`。
2. 现象：真实点击重新检查清除旧 broken 后搜索仍空。改动：Repair 同步采用完整刷新。验证：深浅当前行不再 broken，搜索命令1条且显示；改前均红 `repair-search completed but search stayed unavailable: commands=0`。

旧V/B/S断言0/0/0。before 为改前真实事件帧，after 在同一夹具下重新生成；定向结果见本批提交正文。
