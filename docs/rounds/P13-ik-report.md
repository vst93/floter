# P13-ik · 条件刷新后的键盘归处

2026-11-07约01:33 UTC开盘。起点6ed8c2d8，pull最新、树净。Codex独立执行。基线4689 PASS / 0 FAIL / 3019函数；中文locale。最新交接是P13-ij，按其仍待选择。

## Stash清点

八条均实际导出patch并reverse-check=1，SHA与交接一致。已接续stale-kill不在活动表内；以下早期独立WIP沿用已审核的语义处置，避免把旧主题/Tauri链重新合入。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 复现

脚本删空→重新输入→复制/导出两条原已绿，不计修复。工具卸载后的撤销历史不纳入本轮；程序字段定位最初命中同名分段选项，未计缺陷。八个最终候选均在原产品的深浅事件中实红，日志P13-ik/probe.txt。新增测试首次与已有recoveryDump重名造成编译失败，已更名，不计产品bug。

## 第一批（1–2）

1. 现象：复制执行计划后命令清空，原计划按钮失焦，Enter不再复制。改动：复制按钮使用动作名建立稳定节点身份。验证：深浅Click→Tab往返→条件刷新→Enter，第二次复制仍为完整计划JSON。
2. 现象：命令从空恢复后，原计划焦点误落命令，Enter复制weather。改动：同族身份隔离覆盖按钮插入。验证：深浅第二次只复制原计划，不复制命令；成功状态保留在计划。

相关95 PASS / 0 FAIL，包vet净。M1实跑6条FAIL记录（4个深浅叶子），原串`command presence changed plan focus`、`plan keyboard copied [... "weather"], want two plans`；MD5一致、touch后6 PASS / 0 FAIL。旧V/B/S修改0/0/0。完整日志已落盘。

## 第二批（3–4）

3. 现象：模式刷新为脚本后，仍可见的输出分段失焦，Left不能回终端。改动：输出行使用不随前面条件行变动的身份。验证：深浅真实点击后台→Tab往返→模式刷新→Left，输出回终端。
4. 现象：同一刷新后平台勾选失焦，Space不再切换。改动：平台行建立独立身份。验证：深浅刷新前后保持同一平台焦点，Space改变原平台值。

相关101 PASS / 0 FAIL，包vet净。M2实跑6条FAIL记录（4叶子）；原串`mode refresh lost output focus`、`mode refresh blocked output keyboard: "background"`、`mode refresh blocked platform keyboard`。MD5恢复一致，touch后6 PASS。第一批aa7478cc已推送。第一轮目视浅色计划图：蓝色焦点仍圈在“执行计划已复制”，左侧新出现“复制命令”没有取得焦点。
