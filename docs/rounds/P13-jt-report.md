# P13-jt · 重新探测命令的执行状态与写回归属

2026-11-07 23:33 UTC 开盘，HEAD 2afab0d9，pull 已同步、工作树干净。实际最新 P13-js，按其首条仍待推进。Codex 独立执行；已读取 qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。本轮只修既有入口和异步结果的归属。

## Stash 清点

下列 SHA 与已归档语义审计一致；逐条 reverse-check 均为 1。活动 stale-kill 已在历史轮接续归档，当前八条为早期独立 WIP，不重新覆盖现有 Go 主题或恢复 Tauri 链。

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

done.log 开盘不存在，收尾按指定路径创建。基线运行中；以下逐批只记真实完成结果。

## 第一批：探测期间的行操作

1. 现象：真实点击重新探测，再点击一次并重复宿主派发，同一工具 epoch 达到 3。改动：复用既有 BeginDiagnosis/FinishDiagnosis 的行锁和主线程收尾；不再另建忙态系统。验证：深浅真实 Click、Go 子进程 --help 门控，epoch 必须为 1，完成后原入口可再次点击。
2. 现象：帮助程序尚未返回，“检查”和“健康刷新”各派发一次，竞争调用共 2。改动：重新探测加入现有整行互斥，沿用禁用面；完成自动释放。验证：深浅两套，等待中竞争派发为 0，完成后重试 1 次。

改前运行已红：`pending command reprobe starts repeatedly: epoch=3`、`pending command reprobe permits competing actions: 2`、`command reprobe never entered and settled its row busy state`。before 为原代码实际等待帧；旧 V/B/S 断言修改 0/0/0。
