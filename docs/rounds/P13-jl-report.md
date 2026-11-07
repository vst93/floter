# P13-jl · 同步读取与配置刷新交错

起点087bee9f，ff-only pull 已最新、工作树干净，19:28 UTC 开盘。唯一执行者 Codex。本文增量记录实测结果。

## Stash 清点

活动清单无 stale-kill；八条 SHA 与 P13-jk 及交接清单相同。逐条导出原patch并 reverse-check 均退出1；沿用语义核对结论，保留独立旧 WIP，不覆盖新主题或旧 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载/scratch，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 增量实施

已读取最新“仍待”、视觉合约与 qiaomu-design 打磨资料。同步首读/补读使用无条件 publish，正在建立事件证据；不把代码线索当已修bug。

1. 现象：同步首读取到旧地址后，真实导入成功，旧结果仍使表单显示 before.example。改动：首读原子记录所属代，发布被拒后重新读取当前结果。验证：深浅真实导入、实际编辑框 Cmd+A/C 和磁盘；修前 `cold-import: visible editor reverted to "https://before.example"`，修后 imported.example。
2. 现象：旧首读失败晚于成功保存返回，endpoint 变空。改动：同一首读保护覆盖失败结果，保存使旧代失效。验证：深浅真实 SaveConfiguration、磁盘和编辑框；修前 `cold-save-read-error: visible editor reverted to ""`，修后 saved.example。

第一批相关 **14 PASS / 0 FAIL**，shell vet净、gofmt空。旧 V/B/S 0/0/0；修前6个测试节点实红，M1集中注入待后续。浅色图目检：endpoint正确显示 imported.example，原字段、圆角、焦点操作都保留。基线全量仍运行，未冒写最终数字。

第一批 **7a353649** 已推送。基线普通源码的 verbose 实测 **5654 PASS / 0 FAIL**、退出0。基线脚本在结束时数到了新加两条函数，函数数改由起点087bee9f树独立核对，避免把当前工作树计入基线。

3. 现象：补读第二个已安装工具时，第一工具保存成功，旧合并基底仍把 before.example 交给调用帧。改动：单行补读发布同时核对原表代际，失效就重新获取。验证：两个真实provider、真实保存、补读返回值；修前 `added-neighbour-save: synchronous read returned retired value https://before.example`。
4. 现象：补读自己的配置期间导入新值，旧读取让打开的字段回到旧地址。改动：补读结果也受同一代际保护。验证：真实导入、实际编辑框复制与磁盘；修前 `added-own-import: visible editor reverted to "https://before.example"`，修后保持 imported.example。

第二批相关 **20 PASS / 0 FAIL**，shell vet净、gofmt空，V/B/S旧断言0/0/0。第一帧证据额外使用实际applyIntegrationConfig映射返回快照，防止后续帧自动修复掩盖错误；M1会再次验证这一加强后的事件。基线函数数从087bee9f树实测为 **3303**。

第二批 **25eccc8f** 已推送。

5. 现象：同步首次探测的旧不可用结果晚于刷新完成，已修好的工具重新显示 Tool unavailable。改动：事实首读检查所属代，旧结果被拒后取当前已确认状态。验证：真实provider、RefreshIntegrations、实际卡片；深浅修前 `cold-runtime: synchronous probe disabled a repaired tool`，修后工具可用。
6. 现象：首次摘要读取的旧值在刷新后返回，批准卡重新显示 manifest changed 警告。改动：事实首读保护覆盖批准摘要，保留已确认的新摘要。验证：真实批准记录/磁盘摘要、卡片与返回快照；深浅修前 `cold-approval: synchronous probe revived approval warning`，修后摘要等于磁盘。

第三批相关 **18 PASS / 0 FAIL**，shell vet净、gofmt空、旧V/B/S 0/0/0。M2将在四批完成后统一回注旧代码。

第三批 **8df4c0a0** 已推送。

7. 现象：新行补探测与原行修复刷新交错，旧合并基底重新禁用原工具。改动：补探测同时核对基底代际，保留刷新后原行的可用状态。验证：两真实provider、实际刷新与卡片，深浅修前 `added-neighbour-runtime: synchronous probe disabled a repaired tool`，修后可用。
8. 现象：补探测自己时返回旧摘要，已批准工具重新出现变更警告。改动：补探测结果发布检查代际，拒绝失效摘要。验证：真实批准记录、实际磁盘摘要、卡片和返回值，深浅修前 `added-own-approval: synchronous probe revived approval warning`，修后匹配。

第四批相关 **24 PASS / 0 FAIL**、shell vet净、gofmt空、旧V/B/S 0/0/0；八项全部已红绿复现，完整门槛与两组mutation随后执行。
