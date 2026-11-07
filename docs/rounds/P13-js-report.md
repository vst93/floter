# P13-js · 普通检查的执行状态与当前目录

开盘2026-11-07 23:03 UTC，HEAD a0b6d11d，pull已同步、工作树空。实际最新轮为P13-jr，按其仍待推进。Codex独立执行，已读用户指定设计skill及本轮参考。

## Stash清点

逐条导出patch并reverse-check，SHA与已归档审计一致，无活动stale-kill；旧独立WIP不覆盖当前Go实现。此轮不重新合入已审计的旧主题或Tauri链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧行距/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载度量/调试，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

done.log开盘不存在；收尾创建指定目录和记录。完整基线运行中，仅记实测结果。

## 第一批：普通检查忙态

1. 现象：真实点击Check后，提供者仍在等待，第二次点击和宿主重复派发令epoch升到3。改动：开始前接上行锁，异步worker的defer在主线程释放；包括失败与过期退出。验证：深浅色真实Click、文件门控的提供者、epoch必须为1，完成后原Check可点。
2. 现象：普通检查等待时，健康刷新与修复各可再派发一次。改动：诊断忙态进入rowBusy及repair专用并集，沿用已有操作面。验证：深浅两种事件中竞争调用从2降到0，完成后健康刷新可再次派发。

修改前两项均实红，原始串为`pending diagnosis starts repeatedly: epoch=3`、`pending diagnosis permits competing actions: 2`、`diagnosis never entered and settled its row busy state`。before为原实现真实帧。相关测试 **39 PASS / 0 FAIL**，shell/settingsui vet退出0。旧V/B/S修改0/0/0。

第一批919d6382。开盘全量实测 **5872 PASS / 0 FAIL / 3361函数**，退出0。

## 第二批：生命周期阶段的命令同步

3. 现象：必选probe退出7，健康已显示失败，但等待提供者时仍能搜索Check Ping。改动：生命周期结果走现有完整目录刷新，立即撤下不可运行命令。验证：真实子进程、提供者release门控，命令数组与搜索结果同时从1变0。
4. 现象：此前probe_failed的工具重新检查成功，等待提供者时仍搜索不到命令。改动：同一刷新恢复当前可用目录。验证：失败→成功子进程，两主题命令与可见结果0→1；disabled反例仍为0。

两项改前均红：`diagnosis lifecycle-failure leaves launcher commands stale: got=1 want=0`、`diagnosis lifecycle-recovery leaves launcher commands stale: got=0 want=1`。图来自修改前真实等待帧。

第二批相关 **51 PASS / 0 FAIL**、shell/settingsui vet退出0。首次目视浅色等待帧：健康刷新与底部操作均使用原禁用面，没有新增面板；已失效命令将从原搜索行撤下。旧V/B/S修改0/0/0。

第二批88816c81。

## 第三批：完成时的当前启停状态

5. 现象：没有生命周期probe的提供者等待中，同一工具在仓库停用，检查完成后设置已停用而搜索仍有Check Ping。改动：完成时刷新完整目录而不只读Store。验证：真实磁盘写入、release事件、深浅命令数组与搜索同时1→0，诊断正常结果仍保留。
6. 现象：同一路径从停用恢复启用，检查结束后搜索仍为空。改动：完整刷新恢复当前已启用命令，沿用原命令开关。验证：深浅当前记录Enabled=true、诊断ok、命令与搜索同时0→1。

改前实红：`diagnosis completion-disable leaves launcher commands stale: got=1 want=0`、`diagnosis completion-enable leaves launcher commands stale: got=0 want=1`；不是重复第二批的生命周期变化，而是提供者等待期间外部启停记录刷新。

第三批相关 **76 PASS / 0 FAIL**、shell/settingsui vet退出0，旧V/B/S修改0/0/0。

第三批677cccd8。

## 第四批：读取失败有结束反馈

7. 现象：生命周期开始读取仓库失败，仍执行旧提供者；最终没有失败通知。改动：读取失败刷新当前事实后沿既有错误通知结束，不继续无法确认归属的提供者。验证：真实损坏JSON、Click事件、提供者started文件不存在、error toast存在；修复文件后原Check可重试。
8. 现象：提供者等待期间仓库损坏，完成时无当前归属，静默丢弃结果。改动：读取失败与“归属被替代”分开处理，当前请求使用既有失败通知；epoch过期保持安静。验证：release门控后error toast、忙态退出、恢复文件后成功；另加旧epoch反例。

改前实红：`unreadable repository still launches the old provider`、`diagnosis repository-start read failure is silent`、`diagnosis repository-completion read failure is silent`。两个深浅场景均已保存原实现before帧。

第四批首次回归141 PASS并失败：新测试在Frame消费toast队列后才读queuedToast，报告为空，但实图已经显示错误通知。修正新夹具为绘制前读取Type/Title、绘制后再断言真实文本可见；未改旧B断言，也未删失败条件。原始失败日志保留，修正后需重新实跑变异以证明断言能挡住缺陷。

第二次144 PASS仍有启动阶段夹具红：无窗口Tester允许快速失败在启动操作的同一Frame被消费。把真实Click的目标派发与后台执行按真实窗口Update队列顺序串行，仍断言准确ID、后台结果Type/Title及画面文本；没有用sleep等待或弱化断言。两次新夹具失败日志均保留，产品代码无需为此改动。

第四批修正后相关 **147 PASS / 0 FAIL**、shell/settingsui vet退出0；旧V/B/S修改0/0/0。前两项新测试一并使用串行Click派发夹具，真实事件仍在、重复epoch/竞争调用断言不变。第四批提交包含八项的32张原始PNG和深浅局部对照；正式mutation与完整门槛随后进行。
