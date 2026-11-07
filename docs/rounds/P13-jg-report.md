# P13-jg：导入结果完整性与备份失败恢复

开盘 8835887e，pull 已同步、工作树干净。2026-11-07 16:48 UTC 起，Codex 独立执行。done.log 开盘不存在。按 P13-jf 最新仍待接续；本报告增量记录，未完成项不计交付。

## Stash 清点

无新增 stale-kill；下列 SHA 与前轮一致。reverse-check 均为 1；沿用已完成语义审核的早期独立 WIP 处置，保留原件，不重新合入旧主题或 Tauri 链。

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

## 逐项记录

1. 现象：同一真实导入包含成功、失败、跳过时，失败详情藏掉已生效结果。改动：保留原汇总与失败详情并列，零成功且全失败仍只显示失败。验证：原产品深浅事件均报 `partial failure hides the already-applied result`；导入实际计数 1/1/1。
2. 现象：未携带包的未安装集成只显示 unchanged 1，原因只进日志。改动：宿主保留 Skipped 的 ID 与 Reason，报告用既有警告墨显示诊断。验证：原产品深浅事件均报 `skipped import hides the integration and its reason`；修后检查同一报告与忙态释放。

首批相关 **21 PASS / 0 FAIL**，shell/settingsui vet 净。基线 verbose 全量退出 0，**5480 PASS / 0 FAIL**；基线 HEAD 用 `git grep -h '^func Test' HEAD -- '*_test.go'` 实数为 **3247**，前轮报告的 3245 未沿用。开盘计数只包括已跟踪文件，与后来新增的未跟踪测试无关。旧 V/B/S 断言修改 0/0/0；新增 Skip 0。

第三项原候选长来源路径越界未复现，深浅均通过；保留边界验证、不计修复，改查失败尾部详情。
