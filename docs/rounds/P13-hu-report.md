# P13-hu：确认带的键盘出口与可见边界

开盘 `f3514951`，pull 最新，工作树干净。done.log 与 stash-backlog 文件在本机不存在。基线实跑中。逐批落盘，完成项才记为修复。

## Stash 清点

八条活动 stash 均为早期非 stale-kill WIP，与 P13-ht 清单 SHA 完全一致。本轮逐条 `git apply --check --reverse` 均退出 1；已抢救的八条早已归档，不重复合入，不以 reverse 失败推断可丢弃。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留独立 WIP |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留独立 WIP |
| stash@{2} | 7e4aa710 | 卸载布局/探针，reverse=1 | 保留独立 WIP |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留独立 WIP |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留独立 WIP |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留独立 WIP |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse=1 | 保留独立 WIP |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留独立 WIP |

## 实施记录

规格中的八项均须真实事件或像素复现，未复现候选不冒充已修。V/B/S 暂为 0/0/0，旧测试未改。

### 第一批：停用与移除的键盘确认

1. 现象：停用确认每帧抢回取消，Tab 无法到达停用。改动：初次出现 AutoFocus，后续保留当前焦点。验证：深浅 × 中英文 × 同步/延迟失败，Tab→Space 执行，失败后 Space 可重试；忙态重复 Space 不派发。旧实现红串 `Tab cannot leave cancel for the confirmation action`。
2. 现象：移除确认同样困住 Tab。改动：同族入口只在挂载时聚焦取消。验证：上述八场景覆盖移除，明确两次确认恰好两次回调；新开仍默认取消，错误原文保留。

相关回归 **34 PASS / 0 FAIL**。M1 将两处 AutoFocus 注回 Focus 实跑，16 场景红；恢复源码 MD5 与 touch 后复跑。全量基线仍运行中，不填估计数。旧 V/B/S 断言修改 0/0/0。
