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
