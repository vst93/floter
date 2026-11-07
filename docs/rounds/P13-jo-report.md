# P13-jo · 失败反馈与健康恢复

20:52 UTC 开盘，HEAD 2df719ab，pull 已最新、工作树干净。按最新 P13-jn 仍待选题；Codex 独立执行。done.log 与 stash-backlog 指定路径开盘不存在。

## Stash 清点

八条均为此前登记的独立早期 WIP，SHA 与 P13-jn 相同，无新增 stale-kill。逐条导出 reverse-check 都退出 1，沿用已归档语义审计，保留原稿；不把旧主题/Tauri 工作重复覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri，reverse=1 | 保留 |

## 增量实施

规格已落盘。基线执行中，测试函数起点实测 3328。没有提前填写 PASS 或 mutation 数字。

1. 现象：当前有效诊断报告问题，浮层却显示成功勾。改动：将 DiagnosisFailed 传到既有反馈路由，显示警告字形和警告墨。验证：真实提供者子进程由设置 DiagnoseIntegration 动作启动，深浅 ui.Tester 均复现 `diagnosis-failed failure wears a success mark: type="success" timeout=4s`，改后警告类型/原 warningNotice 与可见句子同时通过。
2. 现象：剪贴板清除失败也显示成功勾。改动：清除结果沿原完成出口携带失败类型。验证：清除失败完成回调深浅图与准确类型/时效断言；原 hidden 系统通知及成功启动器路由继续通过。不声称模拟了磁盘写失败，后续补实事件验证。

第一批相关 **34 PASS / 0 FAIL**（batch1-green.txt），shell vet 退出0，旧 V/B/S 断言修改 **0/0/0**。深浅前后图已导出。正式 mutation 在最终产品上执行。cf949cd2 已推送；其正文及初版报告误写35，本处按原日志纠正为34，不改历史日志。

剪贴板进一步补验真实写失败：创建非空目录占住索引目标，使 Clear 原子写入失败；确认历史仍保留且警告可见，不只调用完成回调。

3. 现象：重连失败文案带成功勾。改动：Reconnect 的失败分支进入 notifyFailure。验证：设置动作真实返回 ErrNoIntegration 后，深浅图与回调均确认失败，改前原串 `reconnect-failed failure wears a success mark: type="success" timeout=4s`。
4. 现象：修复失败文案带成功勾。改动：Repair 的失败分支使用同一警告出口。验证：设置动作真实错误回调、深浅截图，改前 `repair-failed failure wears a success mark: type="success" timeout=4s`，改后失败类型与6s既有警告窗口正确。

第二批相关 **21 PASS / 0 FAIL**，shell vet 退出0。首稿测试导入误落在函数外导致编译错误，修正后才记录有效红绿；旧测试未修改。旧 V/B/S **0/0/0**。

第二批 f46a5dd1 已推送。

5. 现象：安装/接管失败被提示为成功；推荐连接的提交失败同源。改动：注册表准备/提交、孤包接管准备/提交及推荐连接提交五个失败出口统一警告。验证：本地 HTTP 503 与不存在的孤包触发真实动作，深浅均从 type="success" 实红到 error；原安装成功回归保留。
6. 现象：普通移除与组件卸载失败显示成功。改动：两个卸载错误分支使用失败出口。验证：真实 Uninstall/UninstallComponentized 错误回调，两个场景各深浅类型、时效、可见句子均断言；改前 `remove-failed failure wears a success mark` / `uninstall-failed failure wears a success mark`。

第三批相关 **30 PASS / 0 FAIL**，shell vet 退出0，旧 V/B/S **0/0/0**。范围内所有 notifyCompletion 的明确失败分支已排查。

基线首轮错误 locale 的进程在重跑中文基线后主动终止，原日志保留但不作门槛通过依据；中文基线继续。
