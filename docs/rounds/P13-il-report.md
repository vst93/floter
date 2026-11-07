# P13-il · 参数条件刷新与验证恢复

起点 1bf385a8；pull 最新、工作树净。Codex 独立执行，使用 qiaomu-design 既有方向打磨路径；已读主规则、偏好账本、craft-loop、preflight、chinese-typography。done.log 开盘不存在。

## Stash 清点

当前八条均为历史已审核的独立 WIP；本轮实际反向应用检查退出码全部为 1（开盘命令输出 reverse=0 指“未通过”，不是退出码）。与 P13-ik 和 stash-backlog SHA 一致。早期 stale-kill 已归档接续，不重新合入旧主题和 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse exit 1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse exit 1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse exit 1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse exit 1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse exit 1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse exit 1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse exit 1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse exit 1 | 保留 |

## 复现与基线

初次全量测试继承 C.UTF-8，出现历史字体 pin 失败；按 docs/AGENT-NOTES 的环境契约改用 zh_CN.UTF-8 重跑正式基线，未改旧 pin。数字、事件与图像证据增量追加于下。

正式基线 **4719 PASS / 0 FAIL / 3029 测试函数**。八条候选在旧产品的深浅事件均红（24 个 FAIL 记录含父测试）；选择默认值首次保存按钮超出 1100px 夹具，后扩展到 1300px，不记产品缺陷。

## 第一批（1–2）

1. 现象：数字默认值输入 invalid，没有行内原因且保存直接关闭编辑器。改动：本地声明校验复用后端的有限十进制规则，原错误行用现有本地化句子说明默认值不合法。验证：Type→Save 不调用宿主且保留编辑器，改为 12 后保存恰好一次。
2. 现象：布尔默认值输入 invalid，同样无提示且进入保存。改动：声明校验补齐后端 true/false 规则，保持空默认值可用。验证：Type→Save 被拒绝，修正为 true 后旧提示消失且可提交。

相关 **33 PASS / 0 FAIL**，extensions/settingsui vet 净。M1 去掉两条默认值规则后 6 FAIL 记录（4 个深浅叶子），原串 `invalid default has no inline reason`、`invalid number default dispatched 1 saves, open=false`；MD5 恢复一致，touch 复跑 6 PASS。旧 V/B/S 断言修改 0/0/0，新 Skip 0。

## 第二批（3–4）

3. 现象：选择类型的默认值不属于当前选项仍进入保存，且没有本地错误。改动：校验默认值必须属于 Options，沿用后端严格匹配规则。验证：真实输入 invalid 后保存不调用宿主，修正 fast 后提示消失并保存一次；完整 1300px 夹具复现确认。
4. 现象：占位符输入已获键盘焦点，类型刷新为布尔后输入退场且失焦。改动：给占位符保存独立焦点句柄，退场时回同盘类型选择。验证：Tab 往返→类型刷新→方向键/Home/Enter 可回文字类型，提示草稿仍为“记住此提示”。

相关 **39 PASS / 0 FAIL**，包 vet 净。M2 去掉选择默认值校验与占位符回焦，6 FAIL 记录，原串 `invalid select default dispatched 1 saves, open=false`、`hidden placeholder lost kind focus`；两文件 MD5 一致、touch 后 6 PASS。
