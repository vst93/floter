# P13-iw：快捷键录制结束与注册状态恢复

开盘 c00585f3；pull 已最新，工作区干净。Codex 独立执行，按 qiaomu-design 已有方向路径。done.log 开盘不存在。基线运行中，以下增量记实测。

## Stash 清点

八条 SHA 与 P13-iv 相同、reverse-check 全部退出 1，属于已审核早期独立 WIP，没有新的 stale-kill 抢救稿。逐条保留，不把历史主题/Tauri 工作混进本轮。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 早期 keycap，reverse 1 | 保留 |
| stash@{1} | 8af94a07 | 早期 rows/slider，reverse 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局与探针，reverse 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键校验，reverse 1 | 保留 |
| stash@{6} | 90d314b2 | 旧启动器动作栏，reverse 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse 1 | 保留，不覆盖现有发布链 |

## 复现与批次

基线实测 **5150 PASS / 0 FAIL / 3143 测试函数**，退出 0。原代码八个探针全红，原始输出在 P13-iw/original-red.txt；初次探针编译出现局部变量名笔误，修正测试后才取得上述事件失败，编译错误不计 mutation。

重复注册唤起键仅证明多一次调用，降为附带修正不计八项；第 3 项改验“录制中恢复默认提前启用系统键”。其余原始失败保留。旧 V/B/S 修改 0/0/0，新增 Skip 0。

### 第一批（1–2）

1. 现象：取消唤起键录制后 OS 拒绝恢复，系统已无此绑定，页面无提示。改动：当前可用性与历史拒绝分开，恢复结果进入原行/既有警告面；成功恢复撤提示，并请求一帧显示延后到 View 结束才取得的结果。验证：深浅真实录制→Escape→拒绝→再次录制→Escape→恢复，设置写入 0 次；原串 `resume refusal has no visible row feedback`。
2. 现象：取消自定义录制后 OS 拒绝恢复，原行仍像可用。改动：ApplyCustomShortcuts 的当前失败发布到该绑定行，与编辑拒绝历史分开；重试成功清当前失败。验证：同一真实事件链，两主题检查 occupied 文案出现/退场及 key 实际存在，原设置未改。

附带：唤起新键已取得时不重复注册，防止把本轮新增可见反馈误套到自己的已注册键；不作为独立修复数。未来批次测试暂未加入产品树，完整原始探针已在 /tmp 保留，避免提交尚未实现的红测试。

第一批相关 **114 PASS / 0 FAIL**，shell/settingsui vet 退出 0。M1 同时撤去两类当前失败发布，实测 **6 FAIL**（含父测试），原串 `resume refusal has no visible row feedback`；MD5 两文件均 OK，touch 后 **6 PASS**。全量基线 5150 PASS，最终全量在四批完成后执行。
