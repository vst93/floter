# P13-io · 编辑状态与键盘边界

开盘 bb8c6567，04:18 UTC，pull 最新、工作树净。Codex 独立执行，已读本轮设计 skill 与三个参考。基线和逐项结果随实测增量记录。

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
