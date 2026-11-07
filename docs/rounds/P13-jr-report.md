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

第一批 9ea9f128 已提交推送，退出0。开盘完整基线 **5840 PASS / 0 FAIL / 3353函数**，退出0。

## 第二批：执行中的操作出口

3. 现象：健康刷新执行中重复点击会启动多份进程。改动：shell 在启动前进入健康忙态，所有结束路径在主线程退出忙态。验证：真实点击与宿主重复请求均须只接受一次，结束后可再点，按钮尺寸不变。
4. 现象：健康刷新期间同行检查/重连/停用仍可并行写同一记录。改动：健康忙态接入既有 rowBusy 与 rowBusyExceptRepair。验证：健康刷新期间点击检查、重新检查不得调用；另一工具可执行；结束后原出口恢复。

第二批相关 **20 PASS / 0 FAIL**，两包vet净。M2撤回两处行锁后，重复启动和竞争操作断言实红；原文件通过MD5逐字节恢复并touch复验。before 图为该组缺陷回注，非开盘整仓。

第二批 e88bebd4 已提交推送，M2还原后 **6 PASS / 0 FAIL**。

## 第三批：失败反馈

5. 现象：健康刷新无法绑定运行时，仅写日志，点击没有明确结束反馈。改动：沿用既有失败通知，先刷新绑定故障再显示；保留原健康报告。验证：真实缺失 executable、错误 toast 类型和文案、原报告时间、故障码及修复后重试。
6. 现象：仓库文件损坏时刷新只写日志，界面没有本次操作的失败反馈。改动：读取错误也进入已有错误通知；失效/被替代的请求不报旧错误。验证：实际写入损坏 JSON，断言错误反馈与忙态释放，恢复文件后健康检查成功且无新增错误通知。

第三批修改前深浅事件实红：`health refresh binding failure is silent` 与 `health refresh repository failure is silent`。原错误不是成功消息，而是完全没有消息；此处接活现有 error toast。
