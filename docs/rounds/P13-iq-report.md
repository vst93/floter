# P13-iq · 快捷键控件变化后的焦点归还

2026-11-07 05:23 UTC 开盘，5013b357，pull 最新，工作树净。Codex 独立执行。基线 4961 PASS / 0 FAIL，实数 3083 测试函数；上轮报告的 3085 与本轮 git grep 计数不同，未据此编数字。done.log 开盘不存在。

## Stash 清点

八条 SHA 与上轮、backlog 相同，无新增 stale-kill。逐条 reverse-check 均退出 1；沿已有语义审查结论保留早期 WIP，不合回旧主题或已撤销 Tauri 链。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧 keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧 rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载探针，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧插件字段，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧 main/Tauri，reverse=1 | 保留 |

## 实施记录

候选尚未验证不计修复。取消录制的两类事件已绿，不算新修复。冲突另一端消失后清旧提示与旧 B 断言冲突，写入待拍板；未修改旧断言。旧 V/B/S 修改 0/0/0。

### 第一批

1. 现象：自定义快捷键录成新键后焦点消失，Enter 无法重新录制。改动：将焦点一次性交给接受后的 key 对应录制器，失效请求随目标消失清除。验证：深浅真实 Tab/Enter/Ctrl+Shift+R，写入一次；随后 Enter 重新录制仍为原第二行。取消录制的原路径继续通过。
2. 现象：录制 New command 时，唤出快捷键也显示录制态。改动：空 action 只代表唤出动作，统一后精确匹配录制 ID。验证：深浅真实 Tab/Enter，原先四条 prompt（两块控件各有 label/text）缩为两条（唯一录制器），Escape 可取消。初版测试误将 label/text 视为一条，夹具已纠正，原始误计记录不算产品缺陷。

相关 58 PASS / 0 FAIL，包 vet 净。M1 回注旧逻辑，6 FAIL：`custom-accepted lost recorder focus`、`Enter cannot restart the original recorder`、`one recording painted 4 prompt entries, want 2 for one pane`。MD5 两文件还原、touch 后 6 PASS。旧 V/B/S 0/0/0。

第一批 f06e320e 已推送。

### 第二批

3. 现象：保存行切换到“命令行”会变成草稿，但键盘停在退场选择器，直接打字无效。改动：转换后一次性聚焦新命令框。验证：真实选择菜单、直接输入 say converted，草稿获得完整文本，绑定仍未保存。最初夹具使用不在选项中的 action:settings，已经显示命令模式；改为真实 plugin:clipboard 后才取得产品红证据。
4. 现象：删除未完成草稿后没有焦点，Enter 无法继续添加。改动：删除后回到原 Add 入口。验证：两主题真实添加、录键、Tab 到删除、Enter，草稿退出且绑定数 0，Enter 再开录制。

相关 64 PASS / 0 FAIL，包 vet 净。M2 撤掉两条焦点请求，6 FAIL：`command conversion lost its input focus`、`converted command cannot type: ""`、`draft cancel lost the Add outlet`、`Enter cannot add again after cancel`。MD5 还原 touch 后 6 PASS。旧 V/B/S 0/0/0。

第二批 7c2fd45b 已推送。

### 第三批

5. 现象：草稿按 Bind 成功后控件退场，Enter 不能继续编辑新绑定。改动：绑定成功后一次性聚焦接受后的录制器，未完整的草稿不跳走。验证：两主题真实 Add/录键/Tab/Enter，只有 Ctrl+Shift+R → plugin:clipboard 一条，Enter 再录制目标为第 0 行。
6. 现象：删除已保存绑定后键盘没有出口。改动：用户删除回到 Add。验证：两主题真实 Tab/Enter 删除，列表为空，Enter 立即再次打开草稿录制。

相关 70 PASS / 0 FAIL，包 vet 净。M3 回注两处遗漏，6 FAIL：`bound draft lost the saved recorder focus`、`saved removal lost the Add outlet`、`Enter cannot continue after binding exit`。MD5 还原 touch 后 6 PASS。旧 V/B/S 0/0/0。
