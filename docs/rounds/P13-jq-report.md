# P13-jq · 诊断过程的当前事实与完整失败说明

开盘 2a8b1e1c，工作区干净、ff-only 已同步。沿最新 P13-jp 的实际遗留继续，P13-hw 线索已由后续轮次覆盖。规格先落盘，八个候选尚未算成果。

## stash 清点

当前没有新 stale-kill；以下 SHA 与既有语义审计一致。reverse=1 不能说明独有工作，不把旧主题或 Tauri 链重复接回。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap；reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider；reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 卸载/scratch；reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 数字字段；reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 插件字段；reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 快捷键；reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 动作栏；reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | main/Tauri；reverse=1 | 保留 |

## 增量记录

基线运行中。已读 qiaomu-design SKILL、偏好账本、craft-loop、preflight、chinese-typography 与 Apple DESIGN。仅由当前 Codex 执行，沿既有方向打磨。

1. 现象：磁盘移除 lifecycle probes 后，ErrNoProbes 仍执行旧 provider。改动：这一返回值与已失效目标共同刷新后结束。验证：真实提供者标记文件出现，原串 `removed lifecycle declaration still launches the stale provider` 深浅均红；恢复后要求未启动、无旧通知、当前声明已刷新。
2. 现象：没有 lifecycle probes 时，provider 在飞期间磁盘换声明，旧失败结果和通知仍发布，后续检查也读旧来源。改动：完成出口先刷新库存，再按现有来源/epoch 校验发布。验证：真实提供者门控，原串 `provider completed against stale disk owner and published old facts`、`fresh diagnosis cannot recover after disk owner changes` 深浅均红；恢复后还验证新来源能正常检查。

首次红图夹具在输出前重复 Open 会清通知；已取消重复 Open，正式 mutation 使用修正后的同一夹具重出前后图，不将旧夹具缺失的 toast 当证据。

第一批相关回归 **53 PASS / 0 FAIL**，shell vet 净、gofmt 完成。原 V/B/S 断言修改 0/0/0；两项六个节点实红后绿，正式 MD5 mutation 待轮末统一执行。完整基线仍运行中，未宣称全量通过。

第一批 dd96d76d 已推送。

3. 现象：lifecycle 已失败并写盘，provider 尚在等待时卡片仍显示 healthy、没有失败探针。改动：已完成报告在第二阶段开始前刷新进库存。验证：真实 probe exit7 后 provider 门控，原串 `completed lifecycle failure stays invisible while provider waits: health="healthy" broken=false` 深浅均红；恢复后同一帧显示 unhealthy、broken 和 exit7。
4. 现象：lifecycle 失败后 provider 协议损坏，摘要仅剩 JSON 错误。改动：协议失败追加到已有失败原因，不覆盖。验证：真实子进程输出错误类型 JSON，原串 `provider protocol failure erased lifecycle reason` 深浅均红；恢复后摘要完整显示 `Unhealthy; extensions: invalid diagnose answer...`。

开盘基线完成：**5808 PASS / 0 FAIL / 3345 测试函数**。第二批相关 **59 PASS / 0 FAIL**、shell vet 净；旧 V/B/S 断言 0/0/0。六个节点实红后绿。

第二批 9506c8b5 已推送。

5. 现象：两个阶段同时返回结构化失败，lifecycle 的 unhealthy 丢失，界面只显示 provider 的 Error。改动：合并已有摘要，并投影“有前阶段失败”这一事实使原摘要位置显示两阶段原因；独立 provider 的状态标题不改。验证：`provider findings erased lifecycle reason` 与 `combined failure summary hides one stage behind provider status` 深浅均红；恢复后保留原 status/error、明细和完整摘要。
6. 现象：顶层成功词为带空格的 ok 或大写 OK 时，状态墨能识别成功，失败标志和通知却按失败处理。改动：按既有 UI 分类方式 trim/equal-fold，仅归类、保留原值。验证：真实提供者四场景的 `successful provider status is painted/notified as failure` 实红；通知队列在首次帧移交 toastViewport，因此通知断言在首次 Tester 帧前读取，画面仍在帧后验。

第三批两包相关 **81 PASS / 0 FAIL**、两包 vet 净，旧 V/B/S 断言 0/0/0。独立 provider 原 status 标题的旧 pin 全部保留。
