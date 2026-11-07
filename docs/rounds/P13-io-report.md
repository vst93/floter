# P13-io · 编辑状态与键盘边界

开盘 bb8c6567，04:18 UTC，pull 最新、工作树净。Codex 独立执行，已读本轮设计 skill 与三个参考。中文 locale 完整基线实测 **4888 PASS / 0 FAIL / 3068 测试函数**。

## Stash 清点

八条 SHA 与 P13-in/backlog 一致；当前无新增 stale-kill，早期抢救已归档。本轮逐条导出 patch、实跑 reverse-check 均退出 1。沿已审查结论保留，不能用旧主题/Tauri 链覆盖当前产品。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留 |

## 实施记录

旧 V/B/S 断言修改 0/0/0。尚未验证的候选不计入完成。

### 第一批：命令忙态键盘出口

1. 现象：别名输入持焦时，repair/disable/remove/uninstall/toggle 任一整行忙态使输入变灰，键盘停在禁用框，Enter 不能退出。改动：命令板仍存在但整行已锁定时归还既有页头 Close。验证：五种忙态 × 深浅，真实 Tab → 刷新 → 解锁 → Enter，Close 恰好 1 次、写回 0 次，解锁不抢焦。
2. 现象：命令开关持焦时遇到上述五类锁定，同样没有键盘出口。改动：同一命令板出口覆盖开关。验证：五种忙态 × 深浅，退出 1 次、开关与别名回调 0 次；另测未持焦命令不抢页头位置。

相关回归、包 vet 与 M1 已完成，原始数字见本批日志。M1 回注开盘产品实际变红：`busy command stranded keyboard instead of returning to Close`、`busy roundtrip cannot exit: closed=0 writes=0`。恢复源码后 md5sum -c 一致、touch 复跑通过。旧 V/B/S 0/0/0，新 Skip 0。

目视浅色忙态：两块命令板保持原几何与暗线，焦点环明确在右上关闭键，未留在灰掉的输入上。

第一批相关 **139 PASS / 0 FAIL**、包 vet 净，M1 注入 **22 FAIL**，逐字节恢复并 touch 后 **23 PASS / 0 FAIL**；7951a323 已推送。

### 第二批：未写回别名的显示与恢复

3. 现象：输入 json 后一帧普通重绘就显示旧别名，再输入 view，最终只写回 view。改动：待写队列在 180ms 窗口内同时作为显示草稿，重绘不重置文本编辑器。验证：真实 Type → Frame → Frame → 全选复制精确读值 → 继续输入，179ms 写回 0 次，181ms 恰好写回 jsonview。
4. 现象：已输入的别名因命令临时 undeclared/orphan/整卡消失而撤回，恢复后显示 stored；空草稿也复活旧值。改动：新建别名输入从已有 pendingAliases 取值，空字符串按存在性处理。验证：四类路径 × 深浅，恢复后全选复制精确读取 draft 或空串，再续写 x，Flush 只向 ext/jv 写回正确内容一次。

相关 **59 PASS / 0 FAIL**，包 vet 净。M2 回注旧显示来源实跑 **12 FAIL**，原串 `redraw replaced the pending alias: "", want json`、`continued typing lost pending prefix: [[ext jv view]]`、`redeclared editor lost its pending alias: "stored", want "draft"`；MD5 恢复、touch 后 **12 PASS / 0 FAIL**。初版新测试误用 HasText 读取输入内容，改为真实全选复制并严格比较；旧 B 断言零改，没有放宽容差或删除校验。
