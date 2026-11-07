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

第一批 **10225ea5**，相关实测 **42 PASS / 0 FAIL**，shell/settingsui vet 净。提交正文误写为 44，正确数字为日志中的 42，此处明确更正，不沿用错误数字。第二批新夹具首次编译发现 bool 索引和不存在的 Commands 方法，修正后才作行为复现；编译错误不计 mutation。

## 第二批：当前声明与命令文件

3. 现象：真实 Click 后帮助子进程等待，manifest 已更新，旧输出仍覆盖命令并写入“新鲜”时间。改动：帮助返回后复核读取的声明，过期结果安静退出。验证：深浅事件，descriptor 字节不变、freshness 文件不生成、当前行无旧失败。
4. 现象：等待期间保存了 Current command 描述文件，旧帮助仍将其改回 Original command 并追加子命令。改动：写前逐字节核对 descriptor，当前编辑优先；shell 先刷新再发布诊断，过期结果不污染当前行。验证：两主题同样断言文件、freshness 与可见行，before 真实覆盖。

改前四场景全红：`late command reprobe overwrote current manifest descriptor`、`late command reprobe overwrote current descriptor descriptor`、`obsolete … probe wrote a freshness record`。旧 V/B/S 仍 0/0/0。

开盘全量实测 **5902 PASS / 0 FAIL / 3371 函数**，exit0。基线完整日志留存。

第二批 **7e6be57d**，相关 **22 PASS / 0 FAIL**，shell/extensions vet 净。首次目视浅色等待帧：底部重新探测、检查、编辑以及命令开关统一进入原禁用面，无额外浮层或说明占高。

## 第三批：目标来源刷新

5. 现象：帮助等待期间，同 ID 已改为发布者维护的包，旧探测仍写入其描述文件。改动：写回前复核当前 publisher/provider 与包来源，失效请求退出。验证：深浅真实事件，当前文件字节保留、无伪新鲜度、无过期失败提示。
6. 现象：等待中运行时已经换绑，旧程序的帮助仍追加到新目标的命令目录。改动：复核 ExecutablePath、RuntimeRoot 与包版本/路径后才接受输出。验证：深浅文件和 freshness 断言，当前行保持新运行时事实。

改前两类均实红：`late command reprobe overwrote current publisher descriptor`、`late command reprobe overwrote current runtime descriptor`；均错误写了 freshness。旧 V/B/S 仍 0/0/0。

第三批 **940ad2a6**，相关 **28 PASS / 0 FAIL**，shell/extensions vet 净。

## 第四批：读取失败的反馈与重试

7. 现象：开始时仓库 JSON 损坏，重新探测结束却没有任何失败通知。改动：保留实际读取错误，刷新后用既有失败通知收尾，释放行锁。验证：真实损坏文件、Click 与深浅可见错误文案，文件恢复后原入口重试成功。
8. 现象：帮助已开始、仓库随后损坏，操作也静默结束。改动：完成阶段区分读取错误与目标退役；不提交旧描述文件，当前 epoch 使用原通知反馈失败。验证：两主题通知类型/标题/实际文本，以及 descriptor 字节保留、freshness 不写、修复文件后的命令数恢复到 2。

改前四场景红：`command reprobe repository-start read failure is silent`、`command reprobe repository-completion read failure is silent`。旧 V/B/S 仍 0/0/0。

第四批新夹具首次回归暴露测试观察对象错误：生成命令默认未开启搜索，不能用启动器数组数目验证描述文件刷新。改为当前 Store 描述的准确两个命令（第二个 ID=fresh）以及设置页真实可见的 Original command fresh；未改任何旧 B 断言。保留首次失败日志并在修正后重跑变异。

函数数更正：按开盘树 `git grep '^func Test' 2afab0d9 -- cmd internal` 为 **3369**。前述 3371 是基线结束时工作树已新增两函数后的统计，不是开盘树；完整 PASS 基线 5902 不变。
