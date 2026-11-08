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

第一批过程偏差：84825694在定向测试exit1后被错误继续提交推送，正文中的vet净不成立（test失败后vet未执行）。新增两项测试通过，但旧PendingOutletRepairOrphan/ToggleOrphan断言抓到“已安装→孤包”转换后的旧行忙态阻断新出口。补修将孤包门控限定为孤包自身操作，使用既有disabledActionButton熄灭冠线；新夹具改为真正的孤包等待态。旧B断言不改。后续提交命令由同一shell的test && vet && commit && push严格串联。

72ac1237补修已推送，相关57 PASS、settingsui/shell vet净。开盘完整基线5950 PASS/0 FAIL，3385测试函数。

## 第二批：权限回答的声明归属

3. 现象：权限窗口等待时manifest新增clipboard-read，旧回答仍登记并批准新权限。改动：仅PrepareAdopt产物在Commit复核当前manifest摘要。验证：实际接管Click→权限门控→磁盘修改→回答，仓库字节不变、无成功/失败旧通知、原行可重试。M2原始红串 `obsolete permissions adoption rewrote current repository`。
4. 现象：package.json转向新的current.json，旧回答仍接管旧声明。改动：复核当前解析路径、包名与版本。验证：同样真实事件中保留当前包和仓库，不派发过期结果，接管入口恢复。M2红串 `obsolete package adoption rewrote current repository`。

事件夹具最初点到推荐工具的同名接管按钮，失败 `orphan click dispatched to ""` 不算产品证据；移除夹具推荐区后才进行有效before/after。M2正式注入保护缺失后两场景深浅实红，MD5还原并touch后6 PASS；扩展审批回归和vet净。

M2第一次夹具运行时不可用，只复现过期失败通知，脚本因未观察到写盘红串拒收；将fixture runtime名称设为真实测试程序路径后重跑，不改产品语义或断言。

过程偏差补录：仅修改runtime名称仍不足，ResolveRuntime使用StubToolLookup而非StubToolCandidates。第二批提交因M2验收拒收未执行；随后97444804错误合入中间四项，且漏暂存orphans.go的声明，形成短暂不完整提交。立即以单独修正提交补齐并重跑M2/M3；不将该过程说成四批严格每两项完成。最终产品证据以修正夹具后的日志为准。

## 第三批：孤包身份刷新

5. 现象：权限确认期间另一来源登记了同ID，旧接管仍能重写其记录。改动：仅接管Commit再次核对当前仓库中确实无该ID。验证：当前disabled记录逐字节保留，结束后显示已安装行，无旧接管通知。
6. 现象：权限确认期间包目录消失，旧回答报安装失败且孤包卡仍在。改动：提交前复核目录存在，ErrNoOrphan作为失效问题退出并刷新列表。验证：深浅目标行消失、仓库不改、旧失败通知0条。

真实Click→确认门控→磁盘变化→答案返回；第三批红串、MD5还原、touch复验见M3原始日志。普通安装重复ID规则未变。
