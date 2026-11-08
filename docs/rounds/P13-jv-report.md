# P13-jv · 孤包接管确认的当前条件与退出恢复

开盘7ab8b039，pull同步，工作树干净。Codex独立执行，已读qiaomu-design、偏好账本、craft-loop、preflight、chinese-typography与Apple DESIGN。指定done.log开盘不存在。产品和验收按批增量追加。

## Stash 清点

八条SHA与P13-ju一致，无新stale-kill。reverse-check均退出1；沿用已归档语义审核保留旧WIP，不恢复旧主题/Tauri发布链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留原件 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留原件 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留原件 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留原件 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留原件 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留原件 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留原件 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留原件 |

## 当前验证

全量基线运行中，八项选题见spec。为什么更轻/更有空间感：等待状态留在原行，过期回答不占浮层，当前目标重新成为唯一注意中心；不堆叠材质。

## 第一批：孤包行等待状态

1. 现象：接管正在等待时再次点击仍派发，深浅原始红串 `pending orphan permits repeat: adopts=2 deletes=0`。改动：孤包操作接入行锁，接管使用既有忙态胶囊。验证：真实Click后第二次派发0次，结束后可重试；既有行互斥回归不改断言。
2. 现象：同一行等待期间删除仍派发，红串 `pending orphan permits delete: adopts=1 deletes=1`。改动：删除入口遵循同一个行锁，宿主也拒绝重复/竞争调用。验证：真实Click期间删除0次，结束后删除1次；深浅PNG已生成。

旧V/B/S断言0/0/0，新Skip0。定向test/vet通过后立即提交推送，计数见提交正文。
