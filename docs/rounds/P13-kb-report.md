# P13-kb · 短窗口内的错误定位与权限折叠出口

开盘 247b0ef3，pull 已同步、工作树干净。最新 ka 的遗留优先于任务书旧轮次。Codex 独立执行；指定 done.log 开盘不存在。首个时钟 06:17:28 UTC（开盘阅读此前已开始）。

## Stash 清点

活动八条 SHA 与已有语义审计相同，逐条 reverse-check 均 exit1，无新增 stale-kill。原抢救账注明已接续的 jw/jt 对象不在活动清单；不把 reverse 失败误判为可合入的新工作。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse1 | 保留，不覆盖当前 Go 链 |

## 实施记录

基线与逐批现象→改动→验证将在实测后增量追加。旧 V/B/S 断言修改目标 0/0/0。

1. 名称：720×520 页底提交后焦点在名称、输入裁成 0×0（scroll=748.1001，编辑态793.1001）→ 拒绝时沿原容器最小滚动 → 深浅/新建/编辑四事件与直接修正重试验证。
2. 命令：相同短窗口中非法命令拒绝后控件仍为 0×0 → 命令目标一次 reveal → 四事件检查完整目标和之后自由滚轮。两项旧源码均实红，原串 `refusal remains outside viewport`，见 M1-before.txt。

第一批定向 **20 PASS / 0 FAIL / exit0**，settingsui vet exit0。M1 撤回两行 reveal 后八场景实红；两份源码 MD5 成功、touch 后20 PASS。浅色图目视：名称蓝环与红色错误行都回到页首内容区域，侧栏位置保持，底部无新增浮层。开盘全量仍运行，不提前写最终计数。
