# P13-hv：内联确认的退出、超时与并存

开盘 `d0fd58d4`，pull 最新、工作树干净。done.log 不存在；已读取实际存在的 stash-backlog。基线正在实跑，完成后补录。开始时间 14:29 UTC 左右。

## Stash 清点

既有 stale-kill 抢救已在前轮归档接续；以下活动 stash SHA 与 P13-hu 一致，本轮逐条 reverse-check 均为 1，保留独立旧 WIP，不覆盖新主题或恢复 Tauri 链。

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

## 实施记录

仅完成且通过事件验收的项计入；旧 V/B/S 断言修改暂为 0/0/0。

### 第一批：外部收起的键盘出口

1. 现象：停用确认在 Escape 或 3 秒到期后丢失焦点，Space 不能重开。改动：用跨帧 Handle 识别正在退出的答案，仅在它拥有键盘时归还原开关。验证：深浅 × Escape/timeout × 焦点仍在/已移到检查按钮，共八场景；取消派发 0 次，原入口可以 Space 重开。
2. 现象：移除确认同样丢失原入口。改动：同族按归属回到移除按钮，进入组件卸载时保留浮层焦点。验证：同样八场景，并回归已有卸载与显式取消路径。

基线全量实测 **4083 PASS / 0 FAIL / 2888 测试函数**。第一批相关回归 **97 PASS / 0 FAIL**。M1 回注旧产品：`external exit lost its keyboard opener`、`Space cannot reopen the cancelled question`，8 个场景红；还原后 MD5 通过、touch 复跑 **18 PASS**，settingsui vet 通过。V/B/S 旧断言修改 **0/0/0**。
