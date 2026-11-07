# P13-jm · 当前工具声明与状态归属

起点 022e27d7；ff-only pull 已最新，工作树干净，19:52 UTC 开盘。Codex 独立执行，本文增量记录实测。

## Stash 清点

没有活动 stale-kill。八条均与 P13-jl 及交接登记 SHA 相同，reverse-check 均退出1；沿用已登记语义结论，不将早期主题和 Tauri 链覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 增量实施

已读取最新交接、用户反馈、视觉合约与 qiaomu-design 打磨资料。当前八项为代码线索，正在建立写盘刷新与实际界面的证据。

1. 现象：同路径 runtime.type 从不可解析的脚本运行时切换到可用系统程序，旧徽标仍说不可用。改动：事实身份纳入当前运行时类型。验证：真实 manifest 写盘、Store.Refresh、列表投影及深浅 ui.Tester；修前 `runtime-type: refreshed declaration kept obsolete runtime badge: got=false want=true`，修后可用。
2. 现象：脚本解释器改为本机可用语言，旧不可用状态仍留在卡片。改动：身份同时纳入语言。验证：真实声明刷新、ResolveRuntime 与卡片投影；修前 `runtime-language: refreshed declaration kept obsolete runtime badge: got=false want=true`，修后可用。

第一批相关 **8 PASS / 0 FAIL**，shell vet 退出0。旧V/B/S 0/0/0，无新增Skip。M1正式回注安排在八项完成后；修前两项深浅已实红。基线仍运行，暂不写数字。夹具最初一次编译因未用 import 和时钟字段名错误失败，已纠正；不将编译失败计入mutation。

第一批 **7e176690** 已推送。

3. 现象：系统程序候选名已经改成存在的程序，卡片仍显示不可用。改动：把有序候选名纳入当前事实身份。验证：真实 PATH 解析、manifest 写盘与 Store.Refresh，深浅修前 `runtime-names: refreshed declaration kept obsolete runtime badge: got=false want=true`；修后徽标撤下。
4. 现象：平台声明已不支持当前主机，旧可用状态仍保留。改动：事实身份同时记录 manifest 是否有效。验证：真实平台校验返回错误，深浅修前 `runtime-platform: refreshed declaration kept obsolete runtime badge: got=true want=false`；修后不可用徽标出现。原计划脚本越界会同时清空解析结果，证据独立性不够，改为保留运行时声明但平台校验失败的真实事件。

第二批原验收实际 **11 PASS / 3 FAIL**，候选程序夹具在组合运行中未改变可用性；被错误记录成14 PASS并由 **45aa492a** 提前提交推送。这是执行偏差，不能算通过，原日志保留。原因是测试临时改PATH不适合进程已初始化的路径探测。第一次改为绝对路径仍失败（候选名解析不支持绝对路径），最终改用本机原PATH中的平台原生解释器候选，补验后再提交。四项额外断言实际绘制标签，旧V/B/S 0/0/0，不放宽断言。

基线完整 verbose 实测 **5678 PASS / 0 FAIL**，退出0，起点函数数 **3311**。

5. 现象：健康记录已经失败，新鲜度详情仍显示成功，与下方健康状态矛盾。改动：当前事实身份纳入报告状态。验证：真实报告落盘/Store.Refresh，深浅修前 `health-result: refreshed report still paints old result: got=success want=failed`，修后必须同时看到失败文本。
6. 现象：健康检查时间已刷新到现在，上次探测仍显示前天。改动：当前事实身份纳入 CheckedAt。验证：真实时间字段更新、当前记录与可见“刚刚”联合断言；修前 `health-time: refreshed check still paints old timestamp`。

第三批及第二批补验：**20 PASS / 0 FAIL**、shell vet退出0。候选程序使用本机已有解释器名称，四种声明与两种健康刷新全部通过。正式M1将在最终代码上串行注入；夹具失败日志保留。
