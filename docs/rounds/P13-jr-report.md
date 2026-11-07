# P13-jr · 健康刷新入口、执行反馈与命令恢复

开盘 2026-11-07 22:31 UTC，HEAD ecc53976，pull 已同步，工作区空。执行者 Codex；已读 qiaomu-design、偏好账本、craft-loop、preflight、中文排版与 Apple 供体。没有使用外部模型。

## stash 清点

八条均与前轮已审计的早期 WIP SHA 相同，reverse-check 均为1；无新 stale-kill。独有内容的既有语义审计继续有效，不把旧主题/旧 Tauri 链重新覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap WIP，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider WIP，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载度量/调试 WIP，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧 number-field WIP，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽 WIP，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验 WIP，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器 action bar WIP，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri WIP，reverse=1 | 保留 |

当前基线测试运行中；报告只在实际结果落盘后记数。

## 第一批：当前声明

1. 现象：probes 全部移除，历史报告旁仍可点击刷新。改动：由当前声明投影不可刷新状态，撤下入口但保留历史报告。验证：真实 manifest 写入、Store 刷新、Tester 点击；失效时调用不增加，恢复后再次可点。
2. 现象：manifest 损坏或身份换成另一工具，旧报告仍提供刷新。改动：读取错误与 ID 不匹配同样撤下入口。验证：两类真实磁盘变更各覆盖深浅色，修复声明后恢复调用；不改原报告和既有 B pin。

第一批开盘实红：`current declaration cannot run probes but still offers health refresh`、`stale health refresh still dispatches a request`，三个条件深浅均复现。before 为修改前真实事件帧；after 使用相同夹具。

第一批相关 **44 PASS / 0 FAIL**，shell/settingsui vet净，diff --check净。原V/B/S断言0/0/0。开盘全量仍在运行，未拿局部数字冒充完整门槛。
