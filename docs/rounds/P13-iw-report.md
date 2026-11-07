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

第一批 b132484a 已提交推送。

### 第二批（5–6，先补暂停期间的刷新边界）

5. 现象：录制器仍占键盘时，设置刷新直接注册了新唤起键。改动：刷新在暂停期只记待恢复，退出录制按最新设置注册；不是恢复过期旧值。验证：真实打开录制，Store 更新并触发宿主刷新，live 集合必须为空；Escape 后最新键可用，旧键不复活。原串 `settings refresh re-enabled global keys while recording: [Ctrl+Shift+Z]`。
6. 现象：同一期间自定义键刷新也抢先注册。改动：暂停期不运行全量自定义注册，恢复时再取最新键与动作。验证：同样事件链，录制中系统零绑定、取消后新自定义键存在，原串同上。

第二批 **116 PASS / 0 FAIL**，包 vet 净。M2 撤掉两类暂停守卫，**2 FAIL**；两文件 MD5 还原 OK，touch 后 **2 PASS**。旧 V/B/S 0/0/0。

第二批 87531661 已提交推送。

### 第三批（3–4）

3. 现象：唤起/自定义任一录制器仍在等待按键时，点击恢复默认立即注册 Ctrl+Space，后续按键可能执行系统动作。改动：恢复默认仍按原时机提交设置，但在录制结束前释放已取得的默认键，录制者继续持有键盘。验证：两个真实录制器分别点击恢复默认，live 必须为空；Escape 后默认键与自定义键都恢复，旧唤起键不复活。原串 `reset activated global shortcuts under active recorder: [Ctrl+Space]`。
4. 现象：新自定义键刚保存，resume 又释放并重取，竞争者在第二次取得前占用会让刚成功的键失效。改动：记录暂停后是否仍有待恢复的自定义注册，显式录制已完成的注册不再重复。验证：钩子第一次允许、第二次拒绝；真实录制后新键取得次数 1 且 live 存在，原命令与新绑定已保存。原串 `accepted chord was claimed again or lost on resume: claims=2 live=false`。

附带断言验证唤起新键同样只取一次，不另计修复数。第三批原始复现见 batch3-before.txt。

第三批最终 **121 PASS / 0 FAIL**，包 vet 净。M3 回注重复恢复与恢复默认的提前激活，**4 FAIL**（包含父测试），源码 MD5 OK、touch 后同批 121 PASS。自定义当前可用性发布下沉到既有统一注册入口，确保显式重录成功也清当前失败；没有清除另一端冲突的历史拒绝。

### 第四批（7–8）

7. 现象：首次注册唤起键失败，错误只在宿主，打开设置看不到问题。改动：启动使用已有 ApplySummonShortcut 入口，并记实际未取得状态，原行显示当前失败。验证：首次注册钩子拒绝，深浅页实际可见既有警告文案；原串 `initial unavailable summon key looks usable`。
8. 现象：配置已经是 Ctrl+Space、实际注册失败，恢复默认误判为无须注册。改动：原键是否相同与是否实际持有分开判断，成功取得同名键时不再把它注销。验证：真实录制取消造成默认键不可用，解除占用后点击恢复默认，live 中必须有默认键；原串 `reset of unavailable default key did not retry registration`。

截图复核发现夹具未经过 shell 的主题解析；本轮测试夹具补上相同 launcher.Resolve 入口，最终前后截图会按同一主题入口重取，旧图不当作材质验收。另补检查失败提示出现后的真实 Enter 重试出口。

第四批最终 **128 PASS / 0 FAIL**、包 vet 净。补验原键重录可恢复失败注册：自定义入口已可恢复；唤起入口恢复后曾残留宿主拒绝标识（initial-recovery-before.txt），已连同当前错误清掉，未修改页面的历史拒绝规则。

M4 把六个注册/恢复函数完整回注为开盘 c00585f3 的版本，保留新测试与同一主题入口：**20 FAIL / 0 PASS**，原串除前述八项外含 `initial unavailable key is not visible`。两文件 MD5 全部 OK、touch 后 **20 PASS / 0 FAIL**。该次也重取最终 before 图；它们是原函数真实事件绘制，非手工改图。八项实测与附带断言分开记数。
